package tool

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
)

// ToolValue represents a value that can be passed as an argument to a tool or returned as a result.
// It supports primitive types (string, int, float, bool, nil), arrays, and nested objects.
// It is designed to be compatible with JSON Schema used by LLM APIs (Anthropic, OpenAI, Ollama).
type ToolValue struct {
	value any
}

// NewToolValue creates a new ToolValue from any Go value.
// Supported types: string, bool, int/int64/float64, nil, []any, map[string]any, []ToolValue, map[string]ToolValue.
func NewToolValue(v any) ToolValue {
	switch val := v.(type) {
	case ToolValue:
		return val
	case map[string]ToolValue:
		m := make(map[string]any, len(val))
		for k, tv := range val {
			m[k] = tv.Raw()
		}
		return ToolValue{value: m}
	case []ToolValue:
		arr := make([]any, len(val))
		for i, tv := range val {
			arr[i] = tv.Raw()
		}
		return ToolValue{value: arr}
	default:
		return ToolValue{value: v}
	}
}

// Raw returns the underlying value as any.
func (v ToolValue) Raw() any {
	return v.value
}

// IsNil returns true if the value is nil.
func (v ToolValue) IsNil() bool {
	return v.value == nil
}

// AsString returns the value as a string. Returns empty string if not a string.
func (v ToolValue) AsString() string {
	if s, ok := v.value.(string); ok {
		return s
	}
	return ""
}

// AsStringOK returns the value as a string and a boolean indicating success.
func (v ToolValue) AsStringOK() (string, bool) {
	s, ok := v.value.(string)
	return s, ok
}

// AsBool returns the value as a bool. Returns false if not a bool.
func (v ToolValue) AsBool() bool {
	if b, ok := v.value.(bool); ok {
		return b
	}
	return false
}

// AsBoolOK returns the value as a bool and a boolean indicating success.
func (v ToolValue) AsBoolOK() (bool, bool) {
	b, ok := v.value.(bool)
	return b, ok
}

// AsInt returns the value as an int64. Returns 0 if not a number.
// Handles both int and float64 (JSON numbers are typically float64).
func (v ToolValue) AsInt() int64 {
	switch n := v.value.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// AsIntOK returns the value as an int64 and a boolean indicating success.
func (v ToolValue) AsIntOK() (int64, bool) {
	switch n := v.value.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

// AsFloat returns the value as a float64. Returns 0 if not a number.
func (v ToolValue) AsFloat() float64 {
	switch n := v.value.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

// AsFloatOK returns the value as a float64 and a boolean indicating success.
func (v ToolValue) AsFloatOK() (float64, bool) {
	switch n := v.value.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// Array returns the value as a slice of ToolValue. Returns nil if not an array.
func (v ToolValue) Array() []ToolValue {
	arr, ok := v.value.([]any)
	if !ok {
		return nil
	}
	result := make([]ToolValue, len(arr))
	for i, item := range arr {
		result[i] = NewToolValue(item)
	}
	return result
}

// ArrayOK returns the value as a slice of ToolValue and a boolean indicating success.
func (v ToolValue) ArrayOK() ([]ToolValue, bool) {
	arr, ok := v.value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]ToolValue, len(arr))
	for i, item := range arr {
		result[i] = NewToolValue(item)
	}
	return result, true
}

// Object returns the value as a map of string to ToolValue. Returns nil if not an object.
func (v ToolValue) Object() map[string]ToolValue {
	obj, ok := v.value.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]ToolValue, len(obj))
	for k, val := range obj {
		result[k] = NewToolValue(val)
	}
	return result
}

// ObjectOK returns the value as a map of string to ToolValue and a boolean indicating success.
func (v ToolValue) ObjectOK() (map[string]ToolValue, bool) {
	obj, ok := v.value.(map[string]any)
	if !ok {
		return nil, false
	}
	result := make(map[string]ToolValue, len(obj))
	for k, val := range obj {
		result[k] = NewToolValue(val)
	}
	return result, true
}

// Has returns true if the key exists in an object. Returns false for non-objects.
func (v ToolValue) Has(key string) bool {
	obj, ok := v.value.(map[string]any)
	if !ok {
		return false
	}
	_, exists := obj[key]
	return exists
}

// Get retrieves a nested value by key. Returns a nil ToolValue if not an object or key doesn't exist.
func (v ToolValue) Get(key string) ToolValue {
	obj, ok := v.value.(map[string]any)
	if !ok {
		return ToolValue{}
	}
	val, exists := obj[key]
	if !exists {
		return ToolValue{}
	}
	return NewToolValue(val)
}

// GetOK retrieves a nested value by key and returns a boolean indicating if the key exists.
func (v ToolValue) GetOK(key string) (ToolValue, bool) {
	obj, ok := v.value.(map[string]any)
	if !ok {
		return ToolValue{}, false
	}
	val, exists := obj[key]
	if !exists {
		return ToolValue{}, false
	}
	return NewToolValue(val), true
}

// Index retrieves a value from an array by index. Returns a nil ToolValue if not an array or index out of bounds.
func (v ToolValue) Index(i int) ToolValue {
	arr, ok := v.value.([]any)
	if !ok || i < 0 || i >= len(arr) {
		return ToolValue{}
	}
	return NewToolValue(arr[i])
}

// Len returns the length of an array or object. Returns 0 for other types.
func (v ToolValue) Len() int {
	switch val := v.value.(type) {
	case []any:
		return len(val)
	case map[string]any:
		return len(val)
	default:
		return 0
	}
}

// Keys returns all keys if the value is an object. Returns nil for non-objects.
func (v ToolValue) Keys() []string {
	obj, ok := v.value.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}

// Set sets a key-value pair if the value is an object. Initializes the object if nil.
// Returns the ToolValue for method chaining.
func (v *ToolValue) Set(key string, val any) *ToolValue {
	obj, ok := v.value.(map[string]any)
	if !ok {
		// Initialize as object if nil or not an object
		obj = make(map[string]any)
		v.value = obj
	}
	// Convert ToolValue to raw value
	if tv, ok := val.(ToolValue); ok {
		obj[key] = tv.Raw()
	} else {
		obj[key] = val
	}
	return v
}

// Type returns a string describing the type of the value.
func (v ToolValue) Type() string {
	switch v.value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "bool"
	case int, int64, float64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

// MarshalJSON implements json.Marshaler.
func (v ToolValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (v *ToolValue) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.value)
}

// NewToolValueFromJSON creates a new ToolValue from a JSON string.
// The JSON must represent an object (map).
func NewToolValueFromJSON(jsonStr string) (ToolValue, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return ToolValue{}, fmt.Errorf("NewToolValueFromJSON() [tool.go]: failed to parse JSON: %w", err)
	}
	return NewToolValue(m), nil
}

// String retrieves a nested string value by key. Returns empty string if not found or not a string.
// This is a convenience method for v.Get(key).AsString().
func (v ToolValue) String(key string) string {
	return v.Get(key).AsString()
}

// StringOK retrieves a nested string value by key and returns a boolean indicating success.
func (v ToolValue) StringOK(key string) (string, bool) {
	val, exists := v.GetOK(key)
	if !exists {
		return "", false
	}
	return val.AsStringOK()
}

// Bool retrieves a nested bool value by key. Returns false if not found or not a bool.
// This is a convenience method for v.Get(key).AsBool().
func (v ToolValue) Bool(key string) bool {
	return v.Get(key).AsBool()
}

// BoolOK retrieves a nested bool value by key and returns a boolean indicating success.
func (v ToolValue) BoolOK(key string) (bool, bool) {
	val, exists := v.GetOK(key)
	if !exists {
		return false, false
	}
	return val.AsBoolOK()
}

// Int retrieves a nested int value by key. Returns 0 if not found or not a number.
// This is a convenience method for v.Get(key).AsInt().
func (v ToolValue) Int(key string) int64 {
	return v.Get(key).AsInt()
}

// IntOK retrieves a nested int value by key and returns a boolean indicating success.
func (v ToolValue) IntOK(key string) (int64, bool) {
	val, exists := v.GetOK(key)
	if !exists {
		return 0, false
	}
	return val.AsIntOK()
}

// Float retrieves a nested float value by key. Returns 0 if not found or not a number.
// This is a convenience method for v.Get(key).AsFloat().
func (v ToolValue) Float(key string) float64 {
	return v.Get(key).AsFloat()
}

// FloatOK retrieves a nested float value by key and returns a boolean indicating success.
func (v ToolValue) FloatOK(key string) (float64, bool) {
	val, exists := v.GetOK(key)
	if !exists {
		return 0, false
	}
	return val.AsFloatOK()
}

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
