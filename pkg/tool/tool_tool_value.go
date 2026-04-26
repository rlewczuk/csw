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
		return ToolValue{}, fmt.Errorf("NewToolValueFromJSON() [tool_tool_value.go]: failed to parse JSON: %w", err)
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
