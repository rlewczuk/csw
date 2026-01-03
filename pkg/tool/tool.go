package tool

import (
	"encoding/json"
	"fmt"
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

// String returns the value as a string. Returns empty string if not a string.
func (v ToolValue) String() string {
	if s, ok := v.value.(string); ok {
		return s
	}
	return ""
}

// StringOK returns the value as a string and a boolean indicating success.
func (v ToolValue) StringOK() (string, bool) {
	s, ok := v.value.(string)
	return s, ok
}

// Bool returns the value as a bool. Returns false if not a bool.
func (v ToolValue) Bool() bool {
	if b, ok := v.value.(bool); ok {
		return b
	}
	return false
}

// BoolOK returns the value as a bool and a boolean indicating success.
func (v ToolValue) BoolOK() (bool, bool) {
	b, ok := v.value.(bool)
	return b, ok
}

// Int returns the value as an int64. Returns 0 if not a number.
// Handles both int and float64 (JSON numbers are typically float64).
func (v ToolValue) Int() int64 {
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

// IntOK returns the value as an int64 and a boolean indicating success.
func (v ToolValue) IntOK() (int64, bool) {
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

// Float returns the value as a float64. Returns 0 if not a number.
func (v ToolValue) Float() float64 {
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

// FloatOK returns the value as a float64 and a boolean indicating success.
func (v ToolValue) FloatOK() (float64, bool) {
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

// ToolArgs represents the arguments passed to a tool.
// It provides convenient access to named arguments with type conversion.
type ToolArgs struct {
	values map[string]any
}

// NewToolArgs creates a new ToolArgs from a map.
func NewToolArgs(m map[string]any) ToolArgs {
	if m == nil {
		m = make(map[string]any)
	}
	return ToolArgs{values: m}
}

// NewToolArgsFromJSON creates a new ToolArgs from a JSON string.
func NewToolArgsFromJSON(jsonStr string) (ToolArgs, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return ToolArgs{}, fmt.Errorf("failed to parse JSON arguments: %w", err)
	}
	return NewToolArgs(m), nil
}

// Get retrieves an argument by name as a ToolValue.
func (a ToolArgs) Get(name string) ToolValue {
	val, exists := a.values[name]
	if !exists {
		return ToolValue{}
	}
	return NewToolValue(val)
}

// GetOK retrieves an argument by name and returns a boolean indicating if it exists.
func (a ToolArgs) GetOK(name string) (ToolValue, bool) {
	val, exists := a.values[name]
	if !exists {
		return ToolValue{}, false
	}
	return NewToolValue(val), true
}

// Has returns true if the argument exists.
func (a ToolArgs) Has(name string) bool {
	_, exists := a.values[name]
	return exists
}

// String retrieves an argument as a string. Returns empty string if not found or not a string.
func (a ToolArgs) String(name string) string {
	return a.Get(name).String()
}

// StringOK retrieves an argument as a string and returns a boolean indicating success.
func (a ToolArgs) StringOK(name string) (string, bool) {
	v, exists := a.GetOK(name)
	if !exists {
		return "", false
	}
	return v.StringOK()
}

// Bool retrieves an argument as a bool. Returns false if not found or not a bool.
func (a ToolArgs) Bool(name string) bool {
	return a.Get(name).Bool()
}

// BoolOK retrieves an argument as a bool and returns a boolean indicating success.
func (a ToolArgs) BoolOK(name string) (bool, bool) {
	v, exists := a.GetOK(name)
	if !exists {
		return false, false
	}
	return v.BoolOK()
}

// Int retrieves an argument as an int64. Returns 0 if not found or not a number.
func (a ToolArgs) Int(name string) int64 {
	return a.Get(name).Int()
}

// IntOK retrieves an argument as an int64 and returns a boolean indicating success.
func (a ToolArgs) IntOK(name string) (int64, bool) {
	v, exists := a.GetOK(name)
	if !exists {
		return 0, false
	}
	return v.IntOK()
}

// Float retrieves an argument as a float64. Returns 0 if not found or not a number.
func (a ToolArgs) Float(name string) float64 {
	return a.Get(name).Float()
}

// FloatOK retrieves an argument as a float64 and returns a boolean indicating success.
func (a ToolArgs) FloatOK(name string) (float64, bool) {
	v, exists := a.GetOK(name)
	if !exists {
		return 0, false
	}
	return v.FloatOK()
}

// Array retrieves an argument as a slice of ToolValue. Returns nil if not found or not an array.
func (a ToolArgs) Array(name string) []ToolValue {
	return a.Get(name).Array()
}

// Object retrieves an argument as a map of string to ToolValue. Returns nil if not found or not an object.
func (a ToolArgs) Object(name string) map[string]ToolValue {
	return a.Get(name).Object()
}

// Keys returns all argument names.
func (a ToolArgs) Keys() []string {
	keys := make([]string, 0, len(a.values))
	for k := range a.values {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of arguments.
func (a ToolArgs) Len() int {
	return len(a.values)
}

// Raw returns the underlying map.
func (a ToolArgs) Raw() map[string]any {
	return a.values
}

// MarshalJSON implements json.Marshaler.
func (a ToolArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.values)
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *ToolArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &a.values)
}

// ToolResult represents the result of a tool execution.
// It can hold any JSON-compatible value.
type ToolResult struct {
	values map[string]any
}

// NewToolResult creates a new ToolResult from a map.
func NewToolResult(m map[string]any) ToolResult {
	if m == nil {
		m = make(map[string]any)
	}
	return ToolResult{values: m}
}

// Set sets a key-value pair in the result.
func (r *ToolResult) Set(key string, value any) {
	if r.values == nil {
		r.values = make(map[string]any)
	}
	// Convert ToolValue to raw value
	if tv, ok := value.(ToolValue); ok {
		r.values[key] = tv.Raw()
	} else {
		r.values[key] = value
	}
}

// Get retrieves a value by key as a ToolValue.
func (r ToolResult) Get(key string) ToolValue {
	val, exists := r.values[key]
	if !exists {
		return ToolValue{}
	}
	return NewToolValue(val)
}

// GetOK retrieves a value by key and returns a boolean indicating if it exists.
func (r ToolResult) GetOK(key string) (ToolValue, bool) {
	val, exists := r.values[key]
	if !exists {
		return ToolValue{}, false
	}
	return NewToolValue(val), true
}

// Has returns true if the key exists.
func (r ToolResult) Has(key string) bool {
	_, exists := r.values[key]
	return exists
}

// Keys returns all result keys.
func (r ToolResult) Keys() []string {
	keys := make([]string, 0, len(r.values))
	for k := range r.values {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of key-value pairs.
func (r ToolResult) Len() int {
	return len(r.values)
}

// Raw returns the underlying map.
func (r ToolResult) Raw() map[string]any {
	return r.values
}

// MarshalJSON implements json.Marshaler.
func (r ToolResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.values)
}

// UnmarshalJSON implements json.Unmarshaler.
func (r *ToolResult) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &r.values)
}

// ToolCall represents a call to a tool with specific arguments.
type ToolCall struct {
	// ID is a unique identifier for the tool call (typically UUIDv7 represented as string).
	ID string

	// Function is the name of the tool to be called.
	Function string

	// Arguments contains the arguments to be passed to the tool.
	// Supports primitive types (string, int, float, bool), arrays, and nested objects.
	Arguments ToolArgs
}

// ToolResponse represents the response from a tool execution.
type ToolResponse struct {
	// ID is the unique identifier for the tool call (typically UUIDv7 represented as string).
	ID string

	// Error is any error that occurred during the tool execution.
	Error error

	// Result is the result of the tool execution.
	// Supports primitive types (string, int, float, bool), arrays, and nested objects.
	Result ToolResult

	// Done indicates whether the tool execution is complete.
	Done bool
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
}

// ToolSchema defines the JSON Schema for tool arguments.
// It follows the JSON Schema object type specification used by LLM APIs.
type ToolSchema struct {
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

// Tool represents a tool that can be executed by the agent.
// It is responsible for executing the tool and returning the response.
// It can also represent a group of tools, delegating execution to other tools.
type Tool interface {
	// Info returns information about the tool including its name, description, and argument schema.
	Info() ToolInfo
	// Execute executes the tool with the given arguments and returns the response.
	Execute(args ToolCall) ToolResponse
}
