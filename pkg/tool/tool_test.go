package tool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolValue_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		wantType string
	}{
		{"nil", nil, "null"},
		{"string", "hello", "string"},
		{"bool true", true, "bool"},
		{"bool false", false, "bool"},
		{"int", 42, "number"},
		{"int64", int64(42), "number"},
		{"float64", 3.14, "number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewToolValue(tt.value)
			assert.Equal(t, tt.wantType, v.Type())
			assert.Equal(t, tt.value, v.Raw())
		})
	}
}

func TestToolValue_String(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		v := NewToolValue("hello")
		assert.Equal(t, "hello", v.String())
		s, ok := v.StringOK()
		assert.True(t, ok)
		assert.Equal(t, "hello", s)
	})

	t.Run("not a string", func(t *testing.T) {
		v := NewToolValue(42)
		assert.Equal(t, "", v.String())
		s, ok := v.StringOK()
		assert.False(t, ok)
		assert.Equal(t, "", s)
	})
}

func TestToolValue_Bool(t *testing.T) {
	t.Run("valid bool", func(t *testing.T) {
		v := NewToolValue(true)
		assert.Equal(t, true, v.Bool())
		b, ok := v.BoolOK()
		assert.True(t, ok)
		assert.Equal(t, true, b)
	})

	t.Run("not a bool", func(t *testing.T) {
		v := NewToolValue("true")
		assert.Equal(t, false, v.Bool())
		b, ok := v.BoolOK()
		assert.False(t, ok)
		assert.Equal(t, false, b)
	})
}

func TestToolValue_Int(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		want   int64
		wantOK bool
	}{
		{"int", 42, 42, true},
		{"int64", int64(100), 100, true},
		{"float64", 3.9, 3, true}, // truncates
		{"string", "42", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewToolValue(tt.value)
			assert.Equal(t, tt.want, v.Int())
			i, ok := v.IntOK()
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, i)
		})
	}
}

func TestToolValue_Float(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		want   float64
		wantOK bool
	}{
		{"float64", 3.14, 3.14, true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"string", "3.14", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewToolValue(tt.value)
			assert.Equal(t, tt.want, v.Float())
			f, ok := v.FloatOK()
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, f)
		})
	}
}

func TestToolValue_Array(t *testing.T) {
	t.Run("valid array", func(t *testing.T) {
		arr := []any{"a", "b", "c"}
		v := NewToolValue(arr)
		assert.Equal(t, "array", v.Type())
		assert.Equal(t, 3, v.Len())

		result := v.Array()
		require.Len(t, result, 3)
		assert.Equal(t, "a", result[0].String())
		assert.Equal(t, "b", result[1].String())
		assert.Equal(t, "c", result[2].String())

		arrOK, ok := v.ArrayOK()
		assert.True(t, ok)
		require.Len(t, arrOK, 3)
	})

	t.Run("not an array", func(t *testing.T) {
		v := NewToolValue("not an array")
		assert.Nil(t, v.Array())
		arr, ok := v.ArrayOK()
		assert.False(t, ok)
		assert.Nil(t, arr)
	})

	t.Run("index access", func(t *testing.T) {
		arr := []any{10, 20, 30}
		v := NewToolValue(arr)
		assert.Equal(t, int64(20), v.Index(1).Int())
		assert.True(t, v.Index(-1).IsNil())
		assert.True(t, v.Index(100).IsNil())
	})
}

func TestToolValue_Object(t *testing.T) {
	t.Run("valid object", func(t *testing.T) {
		obj := map[string]any{
			"name": "test",
			"age":  30,
		}
		v := NewToolValue(obj)
		assert.Equal(t, "object", v.Type())
		assert.Equal(t, 2, v.Len())

		result := v.Object()
		require.Len(t, result, 2)
		assert.Equal(t, "test", result["name"].String())
		assert.Equal(t, int64(30), result["age"].Int())

		objOK, ok := v.ObjectOK()
		assert.True(t, ok)
		require.Len(t, objOK, 2)
	})

	t.Run("not an object", func(t *testing.T) {
		v := NewToolValue("not an object")
		assert.Nil(t, v.Object())
		obj, ok := v.ObjectOK()
		assert.False(t, ok)
		assert.Nil(t, obj)
	})

	t.Run("get access", func(t *testing.T) {
		obj := map[string]any{
			"key": "value",
		}
		v := NewToolValue(obj)
		assert.Equal(t, "value", v.Get("key").String())
		assert.True(t, v.Get("nonexistent").IsNil())

		val, ok := v.GetOK("key")
		assert.True(t, ok)
		assert.Equal(t, "value", val.String())

		val, ok = v.GetOK("nonexistent")
		assert.False(t, ok)
		assert.True(t, val.IsNil())
	})
}

func TestToolValue_Nested(t *testing.T) {
	nested := map[string]any{
		"user": map[string]any{
			"name": "John",
			"tags": []any{"admin", "user"},
		},
		"count": 5,
	}
	v := NewToolValue(nested)

	// Access nested object
	user := v.Get("user")
	assert.Equal(t, "object", user.Type())
	assert.Equal(t, "John", user.Get("name").String())

	// Access nested array
	tags := user.Get("tags")
	assert.Equal(t, "array", tags.Type())
	assert.Equal(t, 2, tags.Len())
	assert.Equal(t, "admin", tags.Index(0).String())

	// Non-existent path
	assert.True(t, v.Get("user").Get("nonexistent").IsNil())
}

func TestToolValue_JSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		v := NewToolValue(map[string]any{
			"name": "test",
			"list": []any{1, 2, 3},
		})
		data, err := json.Marshal(v)
		require.NoError(t, err)

		var result map[string]any
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
	})

	t.Run("unmarshal", func(t *testing.T) {
		jsonStr := `{"name": "test", "count": 42, "active": true}`
		var v ToolValue
		err := json.Unmarshal([]byte(jsonStr), &v)
		require.NoError(t, err)

		assert.Equal(t, "test", v.Get("name").String())
		assert.Equal(t, int64(42), v.Get("count").Int())
		assert.Equal(t, true, v.Get("active").Bool())
	})
}

func TestToolValue_FromToolValueSlice(t *testing.T) {
	slice := []ToolValue{
		NewToolValue("a"),
		NewToolValue("b"),
	}
	v := NewToolValue(slice)
	arr := v.Array()
	require.Len(t, arr, 2)
	assert.Equal(t, "a", arr[0].String())
	assert.Equal(t, "b", arr[1].String())
}

func TestToolValue_FromToolValueMap(t *testing.T) {
	m := map[string]ToolValue{
		"key": NewToolValue("value"),
	}
	v := NewToolValue(m)
	assert.Equal(t, "value", v.Get("key").String())
}

func TestToolArgs_Basic(t *testing.T) {
	args := NewToolArgs(map[string]any{
		"path":    "/tmp/test.txt",
		"content": "hello world",
		"lines":   100,
		"verbose": true,
	})

	assert.Equal(t, 4, args.Len())
	assert.True(t, args.Has("path"))
	assert.False(t, args.Has("nonexistent"))

	keys := args.Keys()
	assert.Len(t, keys, 4)
}

func TestToolArgs_TypedAccess(t *testing.T) {
	args := NewToolArgs(map[string]any{
		"name":   "test",
		"count":  42,
		"rate":   3.14,
		"active": true,
	})

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "test", args.String("name"))
		s, ok := args.StringOK("name")
		assert.True(t, ok)
		assert.Equal(t, "test", s)

		assert.Equal(t, "", args.String("nonexistent"))
		s, ok = args.StringOK("nonexistent")
		assert.False(t, ok)
		assert.Equal(t, "", s)
	})

	t.Run("Int", func(t *testing.T) {
		assert.Equal(t, int64(42), args.Int("count"))
		i, ok := args.IntOK("count")
		assert.True(t, ok)
		assert.Equal(t, int64(42), i)

		assert.Equal(t, int64(0), args.Int("nonexistent"))
		i, ok = args.IntOK("nonexistent")
		assert.False(t, ok)
		assert.Equal(t, int64(0), i)
	})

	t.Run("Float", func(t *testing.T) {
		assert.Equal(t, 3.14, args.Float("rate"))
		f, ok := args.FloatOK("rate")
		assert.True(t, ok)
		assert.Equal(t, 3.14, f)
	})

	t.Run("Bool", func(t *testing.T) {
		assert.Equal(t, true, args.Bool("active"))
		b, ok := args.BoolOK("active")
		assert.True(t, ok)
		assert.Equal(t, true, b)
	})
}

func TestToolArgs_ComplexTypes(t *testing.T) {
	args := NewToolArgs(map[string]any{
		"files": []any{"/a.txt", "/b.txt"},
		"config": map[string]any{
			"timeout": 30,
			"retry":   true,
		},
	})

	t.Run("Array", func(t *testing.T) {
		files := args.Array("files")
		require.Len(t, files, 2)
		assert.Equal(t, "/a.txt", files[0].String())
		assert.Equal(t, "/b.txt", files[1].String())
	})

	t.Run("Object", func(t *testing.T) {
		config := args.Object("config")
		require.NotNil(t, config)
		assert.Equal(t, int64(30), config["timeout"].Int())
		assert.Equal(t, true, config["retry"].Bool())
	})

	t.Run("Get for nested access", func(t *testing.T) {
		configVal := args.Get("config")
		assert.Equal(t, "object", configVal.Type())
		assert.Equal(t, int64(30), configVal.Get("timeout").Int())
	})
}

func TestToolArgs_FromJSON(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		jsonStr := `{"path": "/tmp/test.txt", "mode": 493}`
		args, err := NewToolArgsFromJSON(jsonStr)
		require.NoError(t, err)

		assert.Equal(t, "/tmp/test.txt", args.String("path"))
		assert.Equal(t, int64(493), args.Int("mode"))
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := NewToolArgsFromJSON("not json")
		assert.Error(t, err)
	})

	t.Run("not an object", func(t *testing.T) {
		_, err := NewToolArgsFromJSON(`["array", "not", "object"]`)
		assert.Error(t, err)
	})
}

func TestToolArgs_JSON(t *testing.T) {
	args := NewToolArgs(map[string]any{
		"path":  "/tmp/test.txt",
		"lines": 100,
	})

	data, err := json.Marshal(args)
	require.NoError(t, err)

	var args2 ToolArgs
	err = json.Unmarshal(data, &args2)
	require.NoError(t, err)

	assert.Equal(t, "/tmp/test.txt", args2.String("path"))
	assert.Equal(t, int64(100), args2.Int("lines"))
}

func TestToolArgs_Raw(t *testing.T) {
	original := map[string]any{"key": "value"}
	args := NewToolArgs(original)
	raw := args.Raw()
	assert.Equal(t, original, raw)
}

func TestToolArgs_NilMap(t *testing.T) {
	args := NewToolArgs(nil)
	assert.Equal(t, 0, args.Len())
	assert.False(t, args.Has("any"))
	assert.Equal(t, "", args.String("any"))
}

func TestToolResult_Basic(t *testing.T) {
	result := NewToolResult(nil)
	result.Set("content", "file contents")
	result.Set("size", 1024)
	result.Set("exists", true)

	assert.Equal(t, 3, result.Len())
	assert.True(t, result.Has("content"))
	assert.False(t, result.Has("nonexistent"))

	assert.Equal(t, "file contents", result.Get("content").String())
	assert.Equal(t, int64(1024), result.Get("size").Int())
	assert.Equal(t, true, result.Get("exists").Bool())

	keys := result.Keys()
	assert.Len(t, keys, 3)
}

func TestToolResult_ComplexValues(t *testing.T) {
	result := NewToolResult(nil)
	result.Set("files", []any{"/a.txt", "/b.txt"})
	result.Set("metadata", map[string]any{
		"created": "2024-01-01",
		"size":    100,
	})

	files := result.Get("files").Array()
	require.Len(t, files, 2)
	assert.Equal(t, "/a.txt", files[0].String())

	metadata := result.Get("metadata")
	assert.Equal(t, "2024-01-01", metadata.Get("created").String())
	assert.Equal(t, int64(100), metadata.Get("size").Int())
}

func TestToolResult_SetToolValue(t *testing.T) {
	result := NewToolResult(nil)
	tv := NewToolValue(map[string]any{"nested": "value"})
	result.Set("data", tv)

	assert.Equal(t, "value", result.Get("data").Get("nested").String())
}

func TestToolResult_GetOK(t *testing.T) {
	result := NewToolResult(map[string]any{"key": "value"})

	v, ok := result.GetOK("key")
	assert.True(t, ok)
	assert.Equal(t, "value", v.String())

	v, ok = result.GetOK("nonexistent")
	assert.False(t, ok)
	assert.True(t, v.IsNil())
}

func TestToolResult_JSON(t *testing.T) {
	result := NewToolResult(nil)
	result.Set("content", "test")
	result.Set("lines", []any{1, 2, 3})

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var result2 ToolResult
	err = json.Unmarshal(data, &result2)
	require.NoError(t, err)

	assert.Equal(t, "test", result2.Get("content").String())
	arr := result2.Get("lines").Array()
	require.Len(t, arr, 3)
	assert.Equal(t, int64(2), arr[1].Int())
}

func TestToolResult_Raw(t *testing.T) {
	result := NewToolResult(map[string]any{"key": "value"})
	raw := result.Raw()
	assert.Equal(t, "value", raw["key"])
}

func TestToolCall_WithComplexArgs(t *testing.T) {
	call := ToolCall{
		ID:       "call-123",
		Function: "process_files",
		Arguments: NewToolArgs(map[string]any{
			"files":   []any{"/a.txt", "/b.txt"},
			"options": map[string]any{"recursive": true},
		}),
	}

	assert.Equal(t, "call-123", call.ID)
	assert.Equal(t, "process_files", call.Function)

	files := call.Arguments.Array("files")
	require.Len(t, files, 2)
	assert.Equal(t, "/a.txt", files[0].String())

	options := call.Arguments.Object("options")
	assert.Equal(t, true, options["recursive"].Bool())
}

func TestToolResponse_WithComplexResult(t *testing.T) {
	result := ToolResult{}
	result.Set("files", []any{
		map[string]any{"name": "a.txt", "size": 100},
		map[string]any{"name": "b.txt", "size": 200},
	})

	response := ToolResponse{
		ID:     "call-123",
		Error:  nil,
		Result: result,
		Done:   true,
	}

	assert.Equal(t, "call-123", response.ID)
	assert.NoError(t, response.Error)
	assert.True(t, response.Done)

	files := response.Result.Get("files").Array()
	require.Len(t, files, 2)
	assert.Equal(t, "a.txt", files[0].Get("name").String())
	assert.Equal(t, int64(100), files[0].Get("size").Int())
}

func TestPropertySchema_BasicTypes(t *testing.T) {
	tests := []struct {
		name       string
		schemaType SchemaType
		desc       string
	}{
		{"string", SchemaTypeString, "A string property"},
		{"number", SchemaTypeNumber, "A number property"},
		{"integer", SchemaTypeInteger, "An integer property"},
		{"boolean", SchemaTypeBoolean, "A boolean property"},
		{"array", SchemaTypeArray, "An array property"},
		{"object", SchemaTypeObject, "An object property"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop := PropertySchema{
				Type:        tt.schemaType,
				Description: tt.desc,
			}
			assert.Equal(t, tt.schemaType, prop.Type)
			assert.Equal(t, tt.desc, prop.Description)
		})
	}
}

func TestPropertySchema_WithEnum(t *testing.T) {
	prop := PropertySchema{
		Type:        SchemaTypeString,
		Description: "Unit of temperature",
		Enum:        []string{"celsius", "fahrenheit", "kelvin"},
	}

	assert.Equal(t, SchemaTypeString, prop.Type)
	assert.Len(t, prop.Enum, 3)
	assert.Contains(t, prop.Enum, "celsius")
	assert.Contains(t, prop.Enum, "fahrenheit")
	assert.Contains(t, prop.Enum, "kelvin")
}

func TestPropertySchema_ArrayWithItems(t *testing.T) {
	prop := PropertySchema{
		Type:        SchemaTypeArray,
		Description: "List of file paths",
		Items: &PropertySchema{
			Type:        SchemaTypeString,
			Description: "A file path",
		},
	}

	assert.Equal(t, SchemaTypeArray, prop.Type)
	require.NotNil(t, prop.Items)
	assert.Equal(t, SchemaTypeString, prop.Items.Type)
	assert.Equal(t, "A file path", prop.Items.Description)
}

func TestPropertySchema_NestedObject(t *testing.T) {
	prop := PropertySchema{
		Type:        SchemaTypeObject,
		Description: "Configuration options",
		Properties: map[string]PropertySchema{
			"timeout": {
				Type:        SchemaTypeInteger,
				Description: "Timeout in milliseconds",
			},
			"retries": {
				Type:        SchemaTypeInteger,
				Description: "Number of retry attempts",
			},
		},
		Required: []string{"timeout"},
	}

	assert.Equal(t, SchemaTypeObject, prop.Type)
	assert.Len(t, prop.Properties, 2)
	assert.Contains(t, prop.Properties, "timeout")
	assert.Contains(t, prop.Properties, "retries")
	assert.Equal(t, []string{"timeout"}, prop.Required)
}

func TestToolSchema_NewToolSchema(t *testing.T) {
	schema := NewToolSchema()

	assert.Equal(t, SchemaTypeObject, schema.Type)
	assert.NotNil(t, schema.Properties)
	assert.Len(t, schema.Properties, 0)
	assert.False(t, schema.AdditionalProperties)
}

func TestToolSchema_AddProperty(t *testing.T) {
	schema := NewToolSchema()

	schema.AddProperty("path", PropertySchema{
		Type:        SchemaTypeString,
		Description: "File path to read",
	}, true)

	schema.AddProperty("encoding", PropertySchema{
		Type:        SchemaTypeString,
		Description: "File encoding",
		Enum:        []string{"utf-8", "ascii", "latin-1"},
	}, false)

	assert.Len(t, schema.Properties, 2)
	assert.Contains(t, schema.Properties, "path")
	assert.Contains(t, schema.Properties, "encoding")
	assert.Equal(t, []string{"path"}, schema.Required)

	pathProp := schema.Properties["path"]
	assert.Equal(t, SchemaTypeString, pathProp.Type)
	assert.Equal(t, "File path to read", pathProp.Description)

	encodingProp := schema.Properties["encoding"]
	assert.Len(t, encodingProp.Enum, 3)
}

func TestToolSchema_ComplexSchema(t *testing.T) {
	// Test a complex schema similar to what LLM APIs expect
	schema := NewToolSchema()
	schema.Description = "Arguments for the edit_file tool"

	schema.AddProperty("file", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The path to the file to edit",
	}, true)

	schema.AddProperty("edits", PropertySchema{
		Type:        SchemaTypeArray,
		Description: "List of edits to apply",
		Items: &PropertySchema{
			Type:        SchemaTypeObject,
			Description: "A single edit operation",
			Properties: map[string]PropertySchema{
				"line": {
					Type:        SchemaTypeInteger,
					Description: "Line number to edit",
				},
				"text": {
					Type:        SchemaTypeString,
					Description: "New text for the line",
				},
			},
			Required: []string{"line", "text"},
		},
	}, true)

	schema.AddProperty("options", PropertySchema{
		Type:        SchemaTypeObject,
		Description: "Edit options",
		Properties: map[string]PropertySchema{
			"create_backup": {
				Type:        SchemaTypeBoolean,
				Description: "Whether to create a backup before editing",
			},
		},
	}, false)

	assert.Equal(t, SchemaTypeObject, schema.Type)
	assert.Len(t, schema.Properties, 3)
	assert.Equal(t, []string{"file", "edits"}, schema.Required)

	// Verify nested array items
	editsSchema := schema.Properties["edits"]
	assert.Equal(t, SchemaTypeArray, editsSchema.Type)
	require.NotNil(t, editsSchema.Items)
	assert.Equal(t, SchemaTypeObject, editsSchema.Items.Type)
	assert.Len(t, editsSchema.Items.Properties, 2)
}

func TestToolInfo_Complete(t *testing.T) {
	schema := NewToolSchema()
	schema.AddProperty("query", PropertySchema{
		Type:        SchemaTypeString,
		Description: "Search query string",
	}, true)
	schema.AddProperty("limit", PropertySchema{
		Type:        SchemaTypeInteger,
		Description: "Maximum number of results",
	}, false)

	info := ToolInfo{
		Name:        "search",
		Description: "Search for files matching a query",
		Schema:      schema,
	}

	assert.Equal(t, "search", info.Name)
	assert.Equal(t, "Search for files matching a query", info.Description)
	assert.Equal(t, SchemaTypeObject, info.Schema.Type)
	assert.Len(t, info.Schema.Properties, 2)
	assert.Equal(t, []string{"query"}, info.Schema.Required)
}

func TestToolSchema_JSON_Marshaling(t *testing.T) {
	schema := NewToolSchema()
	schema.AddProperty("location", PropertySchema{
		Type:        SchemaTypeString,
		Description: "City and country e.g. Bogotá, Colombia",
	}, true)
	schema.AddProperty("units", PropertySchema{
		Type:        SchemaTypeString,
		Description: "Temperature units",
		Enum:        []string{"celsius", "fahrenheit"},
	}, true)

	info := ToolInfo{
		Name:        "get_weather",
		Description: "Get current weather for a location",
		Schema:      schema,
	}

	// Marshal to JSON
	data, err := json.Marshal(info)
	require.NoError(t, err)

	// Unmarshal back
	var unmarshaled ToolInfo
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, info.Name, unmarshaled.Name)
	assert.Equal(t, info.Description, unmarshaled.Description)
	assert.Equal(t, info.Schema.Type, unmarshaled.Schema.Type)
	assert.Len(t, unmarshaled.Schema.Properties, 2)
	assert.Equal(t, info.Schema.Required, unmarshaled.Schema.Required)

	// Verify the enum was preserved
	unitsSchema := unmarshaled.Schema.Properties["units"]
	assert.Equal(t, []string{"celsius", "fahrenheit"}, unitsSchema.Enum)
}

func TestToolSchema_LLMAPICompatibleFormat(t *testing.T) {
	// Test that the schema can be marshaled to a format compatible with OpenAI/Anthropic/Ollama
	schema := NewToolSchema()
	schema.AddProperty("city", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The name of the city",
	}, true)

	info := ToolInfo{
		Name:        "get_current_weather",
		Description: "Get the current weather for a city",
		Schema:      schema,
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	// Parse as generic JSON to verify structure
	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Verify top-level structure
	assert.Equal(t, "get_current_weather", result["name"])
	assert.Equal(t, "Get the current weather for a city", result["description"])

	// Verify parameters structure (matches OpenAI/Ollama format)
	params, ok := result["parameters"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "object", params["type"])

	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)

	cityProp, ok := props["city"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", cityProp["type"])
	assert.Equal(t, "The name of the city", cityProp["description"])

	required, ok := params["required"].([]any)
	require.True(t, ok)
	assert.Contains(t, required, "city")
}

func TestLLMAPICompatibility(t *testing.T) {
	// Test parsing JSON as LLM APIs would send it
	t.Run("OpenAI style arguments", func(t *testing.T) {
		// OpenAI sends arguments as a JSON string
		argsJSON := `{"path": "/tmp/test.txt", "options": {"line_start": 1, "line_end": 100}}`
		args, err := NewToolArgsFromJSON(argsJSON)
		require.NoError(t, err)

		assert.Equal(t, "/tmp/test.txt", args.String("path"))
		options := args.Object("options")
		assert.Equal(t, int64(1), options["line_start"].Int())
		assert.Equal(t, int64(100), options["line_end"].Int())
	})

	t.Run("Anthropic/Ollama style arguments", func(t *testing.T) {
		// Anthropic and Ollama send arguments as parsed JSON object
		argsMap := map[string]any{
			"command": "ls -la",
			"env": map[string]any{
				"PATH": "/usr/bin",
				"HOME": "/home/user",
			},
			"timeout": 30000,
		}
		args := NewToolArgs(argsMap)

		assert.Equal(t, "ls -la", args.String("command"))
		assert.Equal(t, int64(30000), args.Int("timeout"))
		env := args.Object("env")
		assert.Equal(t, "/usr/bin", env["PATH"].String())
	})

	t.Run("Array of complex objects", func(t *testing.T) {
		argsJSON := `{
			"edits": [
				{"file": "a.go", "line": 10, "text": "new line"},
				{"file": "b.go", "line": 20, "text": "another line"}
			]
		}`
		args, err := NewToolArgsFromJSON(argsJSON)
		require.NoError(t, err)

		edits := args.Array("edits")
		require.Len(t, edits, 2)
		assert.Equal(t, "a.go", edits[0].Get("file").String())
		assert.Equal(t, int64(10), edits[0].Get("line").Int())
		assert.Equal(t, "new line", edits[0].Get("text").String())
	})
}
