package tool

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
)

// ToolCall represents a call to a tool with specific arguments.
type ToolCall struct {
	// ID is a unique identifier for the tool call (typically UUIDv7 represented as string).
	ID string

	// Function is the name of the tool to be called.
	Function string

	// Arguments contains the arguments to be passed to the tool.
	// Must be an object (map[string]any). Supports nested objects and arrays.
	Arguments ToolValue

	// Access controls the permission for this specific tool call.
	// When set to AccessAuto (default), the system uses config to determine permission.
	// When set to AccessAllow or AccessDeny, it overrides the config for this call.
	// This allows re-executing a tool call after permission is granted/denied.
	Access conf.AccessFlag
}

// ToolResponse represents the response from a tool execution.
type ToolResponse struct {
	// Call is a reference to the original tool call that this response is for.
	// This provides access to the tool call ID, function name, and arguments.
	Call *ToolCall

	// Error is any error that occurred during the tool execution.
	Error error

	// Result is the result of the tool execution.
	// Supports primitive types (string, int, float, bool), arrays, and nested objects.
	Result ToolValue

	// Done indicates whether the tool execution is complete.
	Done bool

	// Notifications contains non-error informational notifications produced while handling tool call.
	Notifications []ToolNotification
}

// ToolNotification represents an informational event related to tool execution.
type ToolNotification struct {
	// Type identifies notification category.
	Type string

	// Message is user-facing notification text.
	Message string

	// Path is optional path related to notification.
	Path string
}

// HookFeedbackRequest describes one feedback command invocation.
type HookFeedbackRequest struct {
	Fn   string         `json:"fn"`
	Args map[string]any `json:"args,omitempty"`
	ID   string         `json:"id,omitempty"`
}

// HookFeedbackResponse stores one processed hook feedback result.
type HookFeedbackResponse struct {
	ID     string `json:"id,omitempty"`
	Fn     string `json:"fn"`
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// HookFeedbackExecutor executes hook feedback requests emitted by delegated sessions.
type HookFeedbackExecutor interface {
	ExecuteHookFeedback(request HookFeedbackRequest) HookFeedbackResponse
}

// SubAgentTaskRequest represents delegated child-session task request.
type SubAgentTaskRequest struct {
	Slug                 string
	Title                string
	Prompt               string
	Role                 string
	Model                string
	Thinking             string
	HookFeedbackExecutor HookFeedbackExecutor
}

// SubAgentTaskResult represents delegated task result.
type SubAgentTaskResult struct {
	Status  string
	Summary string
	Error   string
}

// SchemaType represents the type of a property in JSON Schema.
// Supported types are compatible with JSON Schema used by LLM APIs (Anthropic, OpenAI, Ollama).
type SchemaType string

const (
	SchemaTypeString  SchemaType = "string"
	SchemaTypeNumber  SchemaType = "number"
	SchemaTypeInteger SchemaType = "integer"
	SchemaTypeBoolean SchemaType = "boolean"
	SchemaTypeArray   SchemaType = "array"
	SchemaTypeObject  SchemaType = "object"
)

// PropertySchema defines the schema for a single property in tool arguments.
// It follows JSON Schema specification and is compatible with LLM APIs.
type PropertySchema struct {
	// Type is the JSON Schema type of the property.
	Type SchemaType `json:"type"`

	// Description provides context for the LLM about what this property represents.
	Description string `json:"description"`

	// Enum is an optional list of allowed values for string properties.
	Enum []string `json:"enum,omitempty"`

	// Items defines the schema for array elements (required when Type is "array").
	Items *PropertySchema `json:"items,omitempty"`

	// Properties defines nested properties (required when Type is "object").
	Properties map[string]PropertySchema `json:"properties,omitempty"`

	// Required lists the required properties (used when Type is "object").
	Required []string `json:"required,omitempty"`

	// AdditionalProperties when false, disallows extra properties not in the schema.
	// This is used for nested object schemas.
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`
}

// ToolSchema defines the JSON Schema for tool arguments.
// It follows the JSON Schema object type specification used by LLM APIs.
type ToolSchema struct {
	// Schema identifies the JSON Schema version.
	Schema string `json:"$schema,omitempty"`

	// Type is always "object" for tool argument schemas.
	Type SchemaType `json:"type"`

	// Description provides context for the LLM about the tool arguments structure.
	Description string `json:"description,omitempty"`

	// Properties defines the schema for each argument.
	Properties map[string]PropertySchema `json:"properties"`

	// Required lists the names of required arguments.
	Required []string `json:"required,omitempty"`

	// AdditionalProperties when false, disallows extra properties not in the schema.
	// This is recommended for strict mode in OpenAI.
	AdditionalProperties bool `json:"additionalProperties"`
}

// NewToolSchema creates a new ToolSchema with the object type preset.
func NewToolSchema() ToolSchema {
	return ToolSchema{
		Schema:               "https://json-schema.org/draft/2020-12/schema",
		Type:                 SchemaTypeObject,
		Properties:           make(map[string]PropertySchema),
		AdditionalProperties: false,
	}
}

// AddProperty adds a property to the schema.
func (s *ToolSchema) AddProperty(name string, prop PropertySchema, required bool) {
	if s.Properties == nil {
		s.Properties = make(map[string]PropertySchema)
	}
	s.Properties[name] = prop
	if required {
		s.Required = append(s.Required, name)
	}
}

// ToolInfo represents information about a tool.
// It is used to provide LLM with information about tool purpose and arguments.
type ToolInfo struct {
	// Name is the unique identifier for the tool.
	Name string `json:"name"`

	// Description explains what the tool does, helping the LLM decide when to use it.
	Description string `json:"description"`

	// Schema defines the JSON Schema for the tool's arguments.
	Schema ToolSchema `json:"parameters"`
}

// TaskRecord stores task metadata used by task tools.
type TaskRecord struct {
	UUID          string
	Name          string
	Description   string
	Status        string
	StatusSet     bool
	FeatureBranch string
	ParentBranch  string
	Role          string
	Deps          []string
	SessionIDs    []string
	SubtaskIDs    []string
	ParentTaskID  string
	CreatedAt     string
	UpdatedAt     string
}

// TaskSessionSummary stores task session summary metadata.
type TaskSessionSummary struct {
	SessionID   string
	Status      string
	StartedAt   string
	CompletedAt string
	TaskID      string
}

// TaskRunOutcome stores task run operation outcome.
type TaskRunOutcome struct {
	Task           TaskRecord
	SessionID      string
	SummaryMeta    *TaskSessionSummary
	SummaryText    string
	Merged         bool
	TaskBranchName string
}

// TaskSessionRef exposes task ID from current session for task tools.
type TaskSessionRef interface {
	TaskID() string
	SetTaskID(taskID string)
	TaskStatusUpdatedInSession() bool
	SetTaskStatusUpdatedInSession(updated bool)
}

// ShortDescription returns the first line of the markdown description
// trimmed of leading and trailing spaces.
func (t *ToolInfo) ShortDescription() string {
	lines := strings.Split(t.Description, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// Tool represents a tool that can be executed by the agent.
// It is responsible for executing the tool and returning the response.
// It can also represent a group of tools, delegating execution to other tools.
type Tool interface {
	// Execute executes the tool with the given arguments and returns the response.
	Execute(args *ToolCall) *ToolResponse

	// Render returns a string representation of the tool call result.
	// First string is one-line summary of the tool call (equivalent of DisplayModeShort).
	// Second string is full information (equivalent of DisplayModeFull).
	// Third string is JSONL representation of the tool call.
	// Map contains additional properties that can be used to display in the UI.
	Render(call *ToolCall) (string, string, string, map[string]string)

	// GetDescription returns the description of the tool.
	// This can be used to add dynamic information to the description.
	// Result of this method (if not empty) will replace or be appended to the static description from tool info.
	// Second return value indicates whether static description should be fully overwritten.
	// If false, first result value will be appended to the static description.
	GetDescription() (string, bool)
}

// formatRenderError formats an error for display in Render output.
// It returns a one-liner version (with newlines converted to spaces) and a full version.
// The full version includes the ERROR: prefix for clear identification.
func formatRenderError(errMsg string) (oneLiner, full string) {
	if errMsg == "" {
		return "", ""
	}
	// Convert to one-liner by replacing newlines with spaces
	oneLiner = strings.ReplaceAll(errMsg, "\n", " ")
	oneLiner = strings.ReplaceAll(oneLiner, "\r", "")
	// Collapse multiple spaces into one
	for strings.Contains(oneLiner, "  ") {
		oneLiner = strings.ReplaceAll(oneLiner, "  ", " ")
	}
	oneLiner = strings.TrimSpace(oneLiner)
	// Full version with ERROR: prefix
	full = "ERROR: " + errMsg
	return oneLiner, full
}

// truncateString truncates a string to the specified maximum length,
// adding an ellipsis in the middle if truncated.
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	// Remove excess text from the middle, keep beginning and end
	ellipsis := "..."
	keepLen := maxLen - len(ellipsis)
	leftLen := keepLen / 2
	rightLen := keepLen - leftLen
	return s[:leftLen] + ellipsis + s[len(s)-rightLen:]
}

// truncateOutput truncates the output to at most maxLines lines.
// If the output is truncated, a message is appended indicating truncation.
// A maxLines value of 0 means no limit.
func truncateOutput(output string, maxLines int) string {
	if maxLines <= 0 {
		return output
	}

	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}

	truncated := strings.Join(lines[:maxLines], "\n")
	truncated += "\nOutput is truncated."
	return truncated
}

// inferRenderStatusAndTime derives status and time from a rendered tool call.
func inferRenderStatusAndTime(call *ToolCall) (string, string) {
	status := "success"
	timeStr := time.Now().UTC().Format(time.RFC3339Nano)
	if call == nil {
		return status, timeStr
	}

	if call.Arguments.String("error") != "" {
		status = "error"
	}
	if exitCode, ok := call.Arguments.IntOK("exit_code"); ok && exitCode != 0 {
		status = "error"
	}
	if explicitStatus := strings.TrimSpace(call.Arguments.String("status")); explicitStatus != "" {
		status = explicitStatus
	}

	for _, key := range []string{"time", "timestamp"} {
		if explicitTime := strings.TrimSpace(call.Arguments.String(key)); explicitTime != "" {
			timeStr = explicitTime
			break
		}
	}

	return status, timeStr
}

// buildToolRenderJSONL builds one JSON object suitable for JSONL rendering.
func buildToolRenderJSONL(toolName string, call *ToolCall, extra map[string]any) string {
	status, timeStr := inferRenderStatusAndTime(call)
	obj := map[string]any{
		"tool":   strings.TrimSpace(toolName),
		"time":   timeStr,
		"status": status,
	}
	for k, v := range extra {
		if strings.TrimSpace(k) == "" {
			continue
		}
		if k == "tool" || k == "time" || k == "status" {
			continue
		}
		obj[k] = v
	}

	content, err := json.Marshal(obj)
	if err != nil {
		fallback, _ := json.Marshal(map[string]any{
			"tool":   strings.TrimSpace(toolName),
			"time":   timeStr,
			"status": status,
		})
		return string(fallback)
	}

	return string(content)
}
