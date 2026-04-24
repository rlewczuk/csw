package tool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/rlewczuk/csw/pkg/conf"
)

const (
	toolOutputTemplateSubdir  = "tool_output"
	toolOutputTemplateName    = "message.md"
	toolOutputTempFilePattern = "tool-output-*.txt"
)

const defaultToolOutputMessageTemplate = `This tool returned output that is too big, so it was saved to {{.Path}} that is {{.SizeKB}}kB long ({{.LineCount}} lines).
Please use grep or other scripts/tools to search in the file as it is so big that it can fill up all your free context.`

// OutputLimitTool wraps another tool and spills oversized textual output into a temporary file.
type OutputLimitTool struct {
	tool            Tool
	minBytes        int
	tempDir         string
	messageTemplate string
}

// OutputLimitTemplateData stores template variables for oversized output message rendering.
type OutputLimitTemplateData struct {
	Path      string
	SizeKB    int
	LineCount int
}

// NewOutputLimitTool creates an OutputLimitTool wrapper for the given tool.
func NewOutputLimitTool(tool Tool, minBytes int, tempDir string) *OutputLimitTool {
	if minBytes < 0 {
		minBytes = 0
	}

	messageTemplate := defaultToolOutputMessageTemplate

	loadedConfig, err := conf.CswConfigLoad("@DEFAULTS")
	if err == nil {
		loadedTemplate, loadErr := loadToolOutputMessageTemplate(loadedConfig)
		if loadErr == nil {
			messageTemplate = loadedTemplate
		}
	}

	return &OutputLimitTool{
		tool:            tool,
		minBytes:        minBytes,
		tempDir:         tempDir,
		messageTemplate: messageTemplate,
	}
}

// GetDescription returns wrapped tool description.
func (t *OutputLimitTool) GetDescription() (string, bool) {
	if t.tool == nil {
		return "", false
	}

	return t.tool.GetDescription()
}

// Render returns wrapped tool rendering.
func (t *OutputLimitTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	if t.tool == nil {
		return "", "", buildToolRenderJSONL("outputLimit", call, nil), make(map[string]string)
	}

	return t.tool.Render(call)
}

// Execute executes wrapped tool and stores oversized output in a temporary file.
func (t *OutputLimitTool) Execute(call *ToolCall) *ToolResponse {
	if t.tool == nil {
		return &ToolResponse{
			Call:  call,
			Error: fmt.Errorf("OutputLimitTool.Execute() [output_limit.go]: wrapped tool cannot be nil"),
			Done:  true,
		}
	}

	response := t.tool.Execute(call)
	if response == nil || response.Error != nil || response.Result.IsNil() {
		return response
	}

	textOutput, ok := extractToolOutputText(response.Result)
	if !ok {
		return response
	}

	if len([]byte(textOutput)) <= t.minBytes {
		return response
	}

	tempFilePath, err := t.saveOutputToTempFile(textOutput)
	if err != nil {
		response.Error = fmt.Errorf("OutputLimitTool.Execute() [output_limit.go]: failed to save large output to temp file: %w", err)
		response.Done = true
		return response
	}

	message, err := t.renderLargeOutputMessage(OutputLimitTemplateData{
		Path:      tempFilePath,
		SizeKB:    outputSizeKB(len([]byte(textOutput))),
		LineCount: countOutputLines(textOutput),
	})
	if err != nil {
		response.Error = fmt.Errorf("OutputLimitTool.Execute() [output_limit.go]: failed to render large output message: %w", err)
		response.Done = true
		return response
	}

	response.Result = NewToolValue(message)
	return response
}

func loadToolOutputMessageTemplate(config *conf.CswConfig) (string, error) {
	if config == nil {
		return "", fmt.Errorf("loadToolOutputMessageTemplate() [output_limit.go]: config cannot be nil")
	}

	subdirFiles, ok := config.AgentConfigFiles[toolOutputTemplateSubdir]
	if !ok {
		return "", fmt.Errorf("loadToolOutputMessageTemplate() [output_limit.go]: missing %s config files", toolOutputTemplateSubdir)
	}

	messageTemplate, ok := subdirFiles[toolOutputTemplateName]
	if !ok {
		return "", fmt.Errorf("loadToolOutputMessageTemplate() [output_limit.go]: missing %s/%s template file", toolOutputTemplateSubdir, toolOutputTemplateName)
	}

	trimmed := strings.TrimSpace(messageTemplate)
	if trimmed == "" {
		return "", fmt.Errorf("loadToolOutputMessageTemplate() [output_limit.go]: %s/%s template cannot be empty", toolOutputTemplateSubdir, toolOutputTemplateName)
	}

	return trimmed, nil
}

func (t *OutputLimitTool) saveOutputToTempFile(output string) (string, error) {
	tempDir := strings.TrimSpace(t.tempDir)
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("OutputLimitTool.saveOutputToTempFile() [output_limit.go]: failed to create temp directory: %w", err)
	}

	tempFile, err := os.CreateTemp(tempDir, toolOutputTempFilePattern)
	if err != nil {
		return "", fmt.Errorf("OutputLimitTool.saveOutputToTempFile() [output_limit.go]: failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	if _, err := tempFile.WriteString(output); err != nil {
		return "", fmt.Errorf("OutputLimitTool.saveOutputToTempFile() [output_limit.go]: failed to write output to temp file: %w", err)
	}

	return tempFile.Name(), nil
}

func (t *OutputLimitTool) renderLargeOutputMessage(data OutputLimitTemplateData) (string, error) {
	tmpl, err := template.New("tool-output-limit-message").Parse(strings.TrimSpace(t.messageTemplate))
	if err != nil {
		return "", fmt.Errorf("OutputLimitTool.renderLargeOutputMessage() [output_limit.go]: failed to parse template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("OutputLimitTool.renderLargeOutputMessage() [output_limit.go]: failed to render template: %w", err)
	}

	return strings.TrimSpace(buffer.String()), nil
}

func outputSizeKB(bytesCount int) int {
	if bytesCount <= 0 {
		return 0
	}

	sizeKB := bytesCount / 1024
	if sizeKB == 0 {
		return 1
	}

	return sizeKB
}

func countOutputLines(output string) int {
	if output == "" {
		return 0
	}

	normalized := strings.TrimSuffix(output, "\n")
	if normalized == "" {
		return 1
	}

	return strings.Count(normalized, "\n") + 1
}

func extractToolOutputText(result ToolValue) (string, bool) {
	if textOutput, ok := result.AsStringOK(); ok {
		return textOutput, true
	}

	objectResult, ok := result.ObjectOK()
	if ok {
		if outputField, outputOK := objectResult["output"]; outputOK {
			if textOutput, textOK := outputField.AsStringOK(); textOK {
				return textOutput, true
			}
		}

		if contentField, contentOK := objectResult["content"]; contentOK {
			if textContent, textOK := contentField.AsStringOK(); textOK {
				return textContent, true
			}
		}
	}

	encoded, err := json.Marshal(result.Raw())
	if err != nil {
		return "", false
	}

	return string(encoded), true
}
