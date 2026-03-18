package tool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
	"gopkg.in/yaml.v3"
)

const customToolToolDirKey = ".tooldir"

// RoleRestrictedTool is implemented by tools that are available only for selected roles.
type RoleRestrictedTool interface {
	// IsRoleAllowed reports whether the tool is available for the role.
	IsRoleAllowed(roleName string) bool
}

type customToolDefinition struct {
	Command  any               `json:"command" yaml:"command"`
	Cwd      string            `json:"cwd,omitempty" yaml:"cwd,omitempty"`
	Env      map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Result   any               `json:"result,omitempty" yaml:"result,omitempty"`
	Error    any               `json:"error,omitempty" yaml:"error,omitempty"`
	Timeout  any               `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	LogLevel string            `json:"loglevel,omitempty" yaml:"loglevel,omitempty"`
	Roles    []string          `json:"roles,omitempty" yaml:"roles,omitempty"`
}

// CustomCommandTool executes user-defined tool command from configuration.
type CustomCommandTool struct {
	name        string
	workdir     string
	tooldir     string
	runner      runner.CommandRunner
	command     any
	cwdTemplate string
	env         map[string]string
	result      any
	error       any
	timeout     time.Duration
	loglevel    string
	roles       []string
	logger      *slog.Logger
}

func (t *CustomCommandTool) GetDescription() (string, bool) {
	return "", false
}

// RegisterCustomTools discovers and registers custom tools from loaded role tool fragments.
func RegisterCustomTools(registry *ToolRegistry, configStore conf.ConfigStore, workdir string, commandRunner runner.CommandRunner) error {
	if registry == nil {
		return fmt.Errorf("RegisterCustomTools() [custom.go]: registry cannot be nil")
	}
	if configStore == nil {
		return fmt.Errorf("RegisterCustomTools() [custom.go]: configStore cannot be nil")
	}
	if commandRunner == nil {
		return fmt.Errorf("RegisterCustomTools() [custom.go]: commandRunner cannot be nil")
	}

	roleConfigs, err := configStore.GetAgentRoleConfigs()
	if err != nil {
		return fmt.Errorf("RegisterCustomTools() [custom.go]: failed to load role configs: %w", err)
	}

	allRole, ok := roleConfigs["all"]
	if !ok || allRole == nil || len(allRole.ToolFragments) == 0 {
		return nil
	}

	definitions, err := loadCustomToolDefinitions(allRole.ToolFragments)
	if err != nil {
		return fmt.Errorf("RegisterCustomTools() [custom.go]: failed to parse custom tool definitions: %w", err)
	}

	for toolName, definition := range definitions {
		customTool, buildErr := newCustomCommandTool(toolName, workdir, definition, commandRunner)
		if buildErr != nil {
			return fmt.Errorf("RegisterCustomTools() [custom.go]: failed to build custom tool %s: %w", toolName, buildErr)
		}
		registry.Register(toolName, customTool)
	}

	return nil
}

func loadCustomToolDefinitions(toolFragments map[string]string) (map[string]customToolDefinition, error) {
	definitions := make(map[string]customToolDefinition)
	fragmentKeys := make(map[string]map[string]string)

	for key, content := range toolFragments {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		toolName := parts[0]
		if fragmentKeys[toolName] == nil {
			fragmentKeys[toolName] = make(map[string]string)
		}
		fragmentKeys[toolName][parts[1]] = content
	}

	for toolName, files := range fragmentKeys {
		jsonName := toolName + ".json"
		yamlName := toolName + ".yaml"
		ymlName := toolName + ".yml"

		tooldir := strings.TrimSpace(files[customToolToolDirKey])
		rawConfig := ""
		configFile := ""
		useYAML := false

		switch {
		case files[yamlName] != "":
			rawConfig = files[yamlName]
			configFile = yamlName
			useYAML = true
		case files[ymlName] != "":
			rawConfig = files[ymlName]
			configFile = ymlName
			useYAML = true
		case files[jsonName] != "":
			rawConfig = files[jsonName]
			configFile = jsonName
		}

		if rawConfig == "" {
			continue
		}

		var definition customToolDefinition
		if useYAML {
			if err := yaml.Unmarshal([]byte(rawConfig), &definition); err != nil {
				return nil, fmt.Errorf("loadCustomToolDefinitions() [custom.go]: failed to parse %s/%s: %w", toolName, configFile, err)
			}
		} else {
			if err := json.Unmarshal([]byte(rawConfig), &definition); err != nil {
				return nil, fmt.Errorf("loadCustomToolDefinitions() [custom.go]: failed to parse %s/%s: %w", toolName, configFile, err)
			}
		}

		if definition.Env == nil {
			definition.Env = make(map[string]string)
		}
		definition.Env[customToolToolDirKey] = tooldir
		definitions[toolName] = definition
	}

	return definitions, nil
}

func newCustomCommandTool(name string, workdir string, definition customToolDefinition, commandRunner runner.CommandRunner) (*CustomCommandTool, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("newCustomCommandTool() [custom.go]: tool name cannot be empty")
	}
	if definition.Command == nil {
		return nil, fmt.Errorf("newCustomCommandTool() [custom.go]: command is required")
	}

	parsedTimeout, err := parseCustomToolTimeout(definition.Timeout)
	if err != nil {
		return nil, fmt.Errorf("newCustomCommandTool() [custom.go]: invalid timeout for %s: %w", name, err)
	}

	loglevel := strings.TrimSpace(strings.ToLower(definition.LogLevel))
	if loglevel == "" {
		loglevel = "info"
	}
	if !slices.Contains([]string{"quiet", "error", "info", "debug"}, loglevel) {
		return nil, fmt.Errorf("newCustomCommandTool() [custom.go]: invalid loglevel %q", definition.LogLevel)
	}

	tooldir := strings.TrimSpace(definition.Env[customToolToolDirKey])
	delete(definition.Env, customToolToolDirKey)
	if tooldir == "" {
		tooldir = workdir
	}

	return &CustomCommandTool{
		name:        name,
		workdir:     workdir,
		tooldir:     tooldir,
		runner:      commandRunner,
		command:     definition.Command,
		cwdTemplate: definition.Cwd,
		env:         definition.Env,
		result:      definition.Result,
		error:       definition.Error,
		timeout:     parsedTimeout,
		loglevel:    loglevel,
		roles:       append([]string(nil), definition.Roles...),
	}, nil
}

func parseCustomToolTimeout(raw any) (time.Duration, error) {
	if raw == nil {
		return 120 * time.Second, nil
	}

	switch v := raw.(type) {
	case float64:
		if v <= 0 {
			return 0, fmt.Errorf("timeout must be positive")
		}
		return time.Duration(v * float64(time.Second)), nil
	case int:
		if v <= 0 {
			return 0, fmt.Errorf("timeout must be positive")
		}
		return time.Duration(v) * time.Second, nil
	case int64:
		if v <= 0 {
			return 0, fmt.Errorf("timeout must be positive")
		}
		return time.Duration(v) * time.Second, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 120 * time.Second, nil
		}
		if intValue, err := strconv.Atoi(trimmed); err == nil {
			if intValue <= 0 {
				return 0, fmt.Errorf("timeout must be positive")
			}
			return time.Duration(intValue) * time.Second, nil
		}
		duration, err := time.ParseDuration(trimmed)
		if err != nil {
			return 0, fmt.Errorf("invalid duration format: %w", err)
		}
		if duration <= 0 {
			return 0, fmt.Errorf("timeout must be positive")
		}
		return duration, nil
	default:
		return 0, fmt.Errorf("unsupported timeout type %T", raw)
	}
}

// SetLogger assigns the logger used by the tool.
func (t *CustomCommandTool) SetLogger(logger *slog.Logger) {
	t.logger = logger
}

// IsRoleAllowed reports whether the tool is visible for the role.
func (t *CustomCommandTool) IsRoleAllowed(roleName string) bool {
	if len(t.roles) == 0 {
		return true
	}
	for _, role := range t.roles {
		if role == roleName {
			return true
		}
	}
	return false
}

// Execute executes the configured command and returns rendered result.
func (t *CustomCommandTool) Execute(call *ToolCall) *ToolResponse {
	templateData := t.buildCommandTemplateData(call)

	command, err := renderCommandTemplate(t.command, templateData)
	if err != nil {
		return &ToolResponse{Call: call, Error: fmt.Errorf("CustomCommandTool.Execute() [custom.go]: failed to render command: %w", err), Done: true}
	}

	cwd := t.workdir
	if strings.TrimSpace(t.cwdTemplate) != "" {
		renderedCwd, renderErr := renderTemplateString(t.cwdTemplate, templateData)
		if renderErr != nil {
			return &ToolResponse{Call: call, Error: fmt.Errorf("CustomCommandTool.Execute() [custom.go]: failed to render cwd: %w", renderErr), Done: true}
		}
		if strings.TrimSpace(renderedCwd) != "" {
			cwd = renderedCwd
		}
	}
	if cwd != "" && !filepath.IsAbs(cwd) {
		cwd = filepath.Join(t.workdir, cwd)
	}

	renderedEnv := make(map[string]string, len(t.env))
	for key, value := range t.env {
		renderedValue, renderErr := renderTemplateString(value, templateData)
		if renderErr != nil {
			return &ToolResponse{Call: call, Error: fmt.Errorf("CustomCommandTool.Execute() [custom.go]: failed to render env %s: %w", key, renderErr), Done: true}
		}
		renderedEnv[key] = renderedValue
	}

	commandWithEnv := command
	if len(renderedEnv) > 0 {
		commandWithEnv = buildExportPrefix(renderedEnv) + command
	}

	t.logExecution("custom_tool_execute", "tool", t.name, "command", command, "workdir", cwd)

	stdout, stderr, exitCode, execErr := t.runCommand(commandWithEnv, runner.CommandOptions{Workdir: cwd, Timeout: t.timeout})

	outputData := map[string]any{
		"arg":      rawArguments(call),
		"stdout":   stdout,
		"stderr":   stderr,
		"exitCode": exitCode,
	}

	if t.loglevel == "debug" {
		t.logExecution("custom_tool_output", "tool", t.name, "stdout", stdout, "stderr", stderr, "exitCode", exitCode)
	}

	if execErr != nil || exitCode != 0 {
		errValue := t.error
		if errValue == nil {
			errValue = defaultCustomErrorTemplate(execErr)
		}
		renderedError, renderErr := renderTemplateValue(errValue, outputData)
		if renderErr != nil {
			return &ToolResponse{Call: call, Error: fmt.Errorf("CustomCommandTool.Execute() [custom.go]: failed to render error template: %w", renderErr), Done: true}
		}
		errMessage := customValueToString(renderedError)
		if strings.TrimSpace(errMessage) == "" {
			errMessage = "custom tool execution failed"
		}
		t.logExecutionError("custom_tool_error", "tool", t.name, "error", errMessage)
		return &ToolResponse{Call: call, Error: fmt.Errorf("CustomCommandTool.Execute() [custom.go]: %s", errMessage), Done: true}
	}

	resultValue := any(stdout)
	if t.result != nil {
		renderedResult, renderErr := renderTemplateValue(t.result, outputData)
		if renderErr != nil {
			return &ToolResponse{Call: call, Error: fmt.Errorf("CustomCommandTool.Execute() [custom.go]: failed to render result template: %w", renderErr), Done: true}
		}
		resultValue = renderedResult
	}

	return &ToolResponse{Call: call, Result: NewToolValue(resultValue), Done: true}
}

func (t *CustomCommandTool) runCommand(command string, options runner.CommandOptions) (string, string, int, error) {
	type detailedRunner interface {
		RunCommandWithOptionsDetailed(command string, options runner.CommandOptions) (string, string, int, error)
	}

	if detailed, ok := t.runner.(detailedRunner); ok {
		return detailed.RunCommandWithOptionsDetailed(command, options)
	}

	output, exitCode, err := t.runner.RunCommandWithOptions(command, options)
	return output, "", exitCode, err
}

func (t *CustomCommandTool) buildCommandTemplateData(call *ToolCall) map[string]any {
	return map[string]any{
		"arg":     rawArguments(call),
		"env":     readEnvMap(),
		"tooldir": t.tooldir,
		"workdir": t.workdir,
	}
}

func rawArguments(call *ToolCall) map[string]any {
	if call == nil {
		return map[string]any{}
	}
	if raw, ok := call.Arguments.Raw().(map[string]any); ok {
		return raw
	}
	return map[string]any{}
}

func readEnvMap() map[string]string {
	envMap := make(map[string]string)
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envMap[parts[0]] = parts[1]
	}
	return envMap
}

func renderCommandTemplate(command any, data map[string]any) (string, error) {
	switch commandValue := command.(type) {
	case string:
		return renderTemplateString(commandValue, data)
	case []any:
		if len(commandValue) == 0 {
			return "", fmt.Errorf("command array cannot be empty")
		}
		parts := make([]string, 0, len(commandValue))
		for _, item := range commandValue {
			itemString, ok := item.(string)
			if !ok {
				return "", fmt.Errorf("command array must contain only strings")
			}
			rendered, err := renderTemplateString(itemString, data)
			if err != nil {
				return "", err
			}
			parts = append(parts, shellSingleQuote(rendered))
		}
		return strings.Join(parts, " "), nil
	case []string:
		if len(commandValue) == 0 {
			return "", fmt.Errorf("command array cannot be empty")
		}
		parts := make([]string, 0, len(commandValue))
		for _, item := range commandValue {
			rendered, err := renderTemplateString(item, data)
			if err != nil {
				return "", err
			}
			parts = append(parts, shellSingleQuote(rendered))
		}
		return strings.Join(parts, " "), nil
	default:
		return "", fmt.Errorf("unsupported command type %T", command)
	}
}

func renderTemplateString(source string, data map[string]any) (string, error) {
	tmpl, err := template.New("custom-tool").Option("missingkey=zero").Parse(source)
	if err != nil {
		return "", fmt.Errorf("renderTemplateString() [custom.go]: failed to parse template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("renderTemplateString() [custom.go]: failed to execute template: %w", err)
	}

	return buffer.String(), nil
}

func renderTemplateValue(value any, data map[string]any) (any, error) {
	switch typed := value.(type) {
	case string:
		return renderTemplateString(typed, data)
	case map[string]any:
		rendered := make(map[string]any, len(typed))
		for key, item := range typed {
			next, err := renderTemplateValue(item, data)
			if err != nil {
				return nil, err
			}
			rendered[key] = next
		}
		return rendered, nil
	case []any:
		rendered := make([]any, len(typed))
		for index, item := range typed {
			next, err := renderTemplateValue(item, data)
			if err != nil {
				return nil, err
			}
			rendered[index] = next
		}
		return rendered, nil
	default:
		return value, nil
	}
}

func customValueToString(value any) string {
	if value == nil {
		return ""
	}
	if stringValue, ok := value.(string); ok {
		return stringValue
	}
	serialized, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(serialized)
}

func defaultCustomErrorTemplate(execErr error) string {
	if execErr != nil {
		return "{{.stderr}}{{if .stderr}}\n{{end}}" + execErr.Error()
	}
	return "command failed with exit code {{.exitCode}}\n{{.stderr}}"
}

func buildExportPrefix(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}

	parts := make([]string, 0, len(env))
	for key, value := range env {
		parts = append(parts, fmt.Sprintf("export %s=%s;", key, shellSingleQuote(value)))
	}
	return strings.Join(parts, " ") + " "
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (t *CustomCommandTool) logExecution(message string, args ...any) {
	if t.logger == nil {
		return
	}
	if t.loglevel == "quiet" || t.loglevel == "error" {
		return
	}
	t.logger.Info(message, args...)
}

func (t *CustomCommandTool) logExecutionError(message string, args ...any) {
	if t.logger == nil {
		return
	}
	t.logger.Error(message, args...)
}

// Render returns a short and full textual representation of the custom tool call.
func (t *CustomCommandTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	oneLiner := "custom: " + t.name
	full := "custom tool: " + t.name
	jsonl := buildToolRenderJSONL(t.name, call, map[string]any{"name": t.name})
	return oneLiner, full, jsonl, map[string]string{}
}
