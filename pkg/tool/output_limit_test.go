package tool

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type outputLimitTestTool struct {
	response     *ToolResponse
	description  string
	overwrite    bool
	renderShort  string
	renderFull   string
	renderJSONL  string
	renderExtras map[string]string

	executeCalls int
	renderCalls  int
	descCalls    int
}

func (t *outputLimitTestTool) Execute(args *ToolCall) *ToolResponse {
	t.executeCalls++
	return t.response
}

func (t *outputLimitTestTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	t.renderCalls++
	return t.renderShort, t.renderFull, t.renderJSONL, t.renderExtras
}

func (t *outputLimitTestTool) GetDescription() (string, bool) {
	t.descCalls++
	return t.description, t.overwrite
}

func TestOutputLimitTool_Execute_PassThroughForSmallOutput(t *testing.T) {
	t.Parallel()

	call := &ToolCall{ID: "test-small", Function: "mock", Arguments: NewToolValue(map[string]any{})}
	wrapped := &outputLimitTestTool{
		response: &ToolResponse{
			Call:   call,
			Result: NewToolValue("small output"),
			Done:   true,
		},
	}

	tool := NewOutputLimitTool(wrapped, 64, t.TempDir())
	response := tool.Execute(call)

	require.NotNil(t, response)
	require.NoError(t, response.Error)
	assert.Equal(t, "small output", response.Result.AsString())
	assert.Equal(t, 1, wrapped.executeCalls)
}

func TestOutputLimitTool_Execute_SavesLargeOutputToFile(t *testing.T) {
	t.Parallel()

	output := strings.Repeat("line\n", 3000)
	call := &ToolCall{ID: "test-large", Function: "mock", Arguments: NewToolValue(map[string]any{})}
	wrapped := &outputLimitTestTool{
		response: &ToolResponse{
			Call:   call,
			Result: NewToolValue(output),
			Done:   true,
		},
	}

	tempDirRoot := t.TempDir()
	nestedTempDir := filepath.Join(tempDirRoot, "nested", "spill")
	tool := NewOutputLimitTool(wrapped, 2048, nestedTempDir)

	response := tool.Execute(call)

	require.NotNil(t, response)
	require.NoError(t, response.Error)
	require.True(t, response.Done)

	message := response.Result.AsString()
	require.NotEmpty(t, message)
	assert.Contains(t, message, "This tool returned output that is too big")
	assert.Contains(t, message, "Please use grep or other scripts/tools")
	assert.Contains(t, message, "(3000 lines)")

	spilledPath := extractSpilledPath(t, message)
	assert.True(t, strings.HasPrefix(spilledPath, nestedTempDir))

	content, err := os.ReadFile(spilledPath)
	require.NoError(t, err)
	assert.Equal(t, output, string(content))
}

func TestOutputLimitTool_Execute_SavesLargeObjectOutputField(t *testing.T) {
	t.Parallel()

	output := strings.Repeat("abc", 1000)
	call := &ToolCall{ID: "test-large-object", Function: "mock", Arguments: NewToolValue(map[string]any{})}
	wrapped := &outputLimitTestTool{
		response: &ToolResponse{
			Call: call,
			Result: NewToolValue(map[string]any{
				"output": output,
			}),
			Done: true,
		},
	}

	tool := NewOutputLimitTool(wrapped, 100, t.TempDir())
	response := tool.Execute(call)

	require.NotNil(t, response)
	require.NoError(t, response.Error)
	message := response.Result.AsString()
	require.Contains(t, message, "saved to ")

	spilledPath := extractSpilledPath(t, message)
	content, err := os.ReadFile(spilledPath)
	require.NoError(t, err)
	assert.Equal(t, output, string(content))
}

func TestOutputLimitTool_GetDescriptionAndRender_DelegateToWrappedTool(t *testing.T) {
	t.Parallel()

	wrapped := &outputLimitTestTool{
		description:  "dynamic description",
		overwrite:    true,
		renderShort:  "short",
		renderFull:   "full",
		renderJSONL:  "jsonl",
		renderExtras: map[string]string{"a": "b"},
	}

	tool := NewOutputLimitTool(wrapped, 1024, t.TempDir())
	desc, overwrite := tool.GetDescription()
	short, full, jsonl, extras := tool.Render(&ToolCall{Function: "mock", Arguments: NewToolValue(map[string]any{})})

	assert.Equal(t, "dynamic description", desc)
	assert.True(t, overwrite)
	assert.Equal(t, "short", short)
	assert.Equal(t, "full", full)
	assert.Equal(t, "jsonl", jsonl)
	assert.Equal(t, map[string]string{"a": "b"}, extras)
	assert.Equal(t, 1, wrapped.descCalls)
	assert.Equal(t, 1, wrapped.renderCalls)
}

func TestLoadToolOutputMessageTemplate_FromConfig(t *testing.T) {
	t.Parallel()

	messageTemplate, err := loadToolOutputMessageTemplate(&conf.CswConfig{
		AgentConfigFiles: map[string]map[string]string{
			"tool_output": {
				"message.md": "saved to {{.Path}}",
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "saved to {{.Path}}", messageTemplate)
}

func extractSpilledPath(t *testing.T, message string) string {
	t.Helper()

	re := regexp.MustCompile(`saved to (.+?) that is`)
	matches := re.FindStringSubmatch(message)
	require.Len(t, matches, 2)
	return matches[1]
}
