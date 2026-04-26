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

func TestToolValue_AsString(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		v := NewToolValue("hello")
		assert.Equal(t, "hello", v.AsString())
		s, ok := v.AsStringOK()
		assert.True(t, ok)
		assert.Equal(t, "hello", s)
	})

	t.Run("not a string", func(t *testing.T) {
		v := NewToolValue(42)
		assert.Equal(t, "", v.AsString())
		s, ok := v.AsStringOK()
		assert.False(t, ok)
		assert.Equal(t, "", s)
	})
}

func TestToolValue_AsBool(t *testing.T) {
	t.Run("valid bool", func(t *testing.T) {
		v := NewToolValue(true)
		assert.Equal(t, true, v.AsBool())
		b, ok := v.AsBoolOK()
		assert.True(t, ok)
		assert.Equal(t, true, b)
	})

	t.Run("not a bool", func(t *testing.T) {
		v := NewToolValue("true")
		assert.Equal(t, false, v.AsBool())
		b, ok := v.AsBoolOK()
		assert.False(t, ok)
		assert.Equal(t, false, b)
	})
}

func TestToolValue_AsInt(t *testing.T) {
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
			assert.Equal(t, tt.want, v.AsInt())
			i, ok := v.AsIntOK()
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, i)
		})
	}
}

func TestToolValue_AsFloat(t *testing.T) {
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
			assert.Equal(t, tt.want, v.AsFloat())
			f, ok := v.AsFloatOK()
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
		assert.Equal(t, "a", result[0].AsString())
		assert.Equal(t, "b", result[1].AsString())
		assert.Equal(t, "c", result[2].AsString())

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
		assert.Equal(t, int64(20), v.Index(1).AsInt())
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
		assert.Equal(t, "test", result["name"].AsString())
		assert.Equal(t, int64(30), result["age"].AsInt())

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
		assert.Equal(t, "value", v.Get("key").AsString())
		assert.True(t, v.Get("nonexistent").IsNil())

		val, ok := v.GetOK("key")
		assert.True(t, ok)
		assert.Equal(t, "value", val.AsString())

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

	user := v.Get("user")
	assert.Equal(t, "object", user.Type())
	assert.Equal(t, "John", user.Get("name").AsString())

	tags := user.Get("tags")
	assert.Equal(t, "array", tags.Type())
	assert.Equal(t, 2, tags.Len())
	assert.Equal(t, "admin", tags.Index(0).AsString())

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

		assert.Equal(t, "test", v.Get("name").AsString())
		assert.Equal(t, int64(42), v.Get("count").AsInt())
		assert.Equal(t, true, v.Get("active").AsBool())
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
	assert.Equal(t, "a", arr[0].AsString())
	assert.Equal(t, "b", arr[1].AsString())
}

func TestToolValue_FromToolValueMap(t *testing.T) {
	m := map[string]ToolValue{
		"key": NewToolValue("value"),
	}
	v := NewToolValue(m)
	assert.Equal(t, "value", v.Get("key").AsString())
}

func TestToolValue_ObjectOperations(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		v := NewToolValue(map[string]any{
			"path":    "/tmp/test.txt",
			"content": "hello world",
			"lines":   100,
			"verbose": true,
		})

		assert.Equal(t, 4, v.Len())
		assert.True(t, v.Has("path"))
		assert.False(t, v.Has("nonexistent"))

		keys := v.Keys()
		assert.Len(t, keys, 4)
	})

	t.Run("typed access by key", func(t *testing.T) {
		v := NewToolValue(map[string]any{
			"name":   "test",
			"count":  42,
			"rate":   3.14,
			"active": true,
		})

		assert.Equal(t, "test", v.String("name"))
		s, ok := v.StringOK("name")
		assert.True(t, ok)
		assert.Equal(t, "test", s)

		assert.Equal(t, "", v.String("nonexistent"))
		s, ok = v.StringOK("nonexistent")
		assert.False(t, ok)
		assert.Equal(t, "", s)

		assert.Equal(t, int64(42), v.Int("count"))
		i, ok := v.IntOK("count")
		assert.True(t, ok)
		assert.Equal(t, int64(42), i)

		assert.Equal(t, int64(0), v.Int("nonexistent"))
		i, ok = v.IntOK("nonexistent")
		assert.False(t, ok)
		assert.Equal(t, int64(0), i)

		assert.Equal(t, 3.14, v.Float("rate"))
		f, ok := v.FloatOK("rate")
		assert.True(t, ok)
		assert.Equal(t, 3.14, f)

		assert.Equal(t, true, v.Bool("active"))
		b, ok := v.BoolOK("active")
		assert.True(t, ok)
		assert.Equal(t, true, b)
	})

	t.Run("complex types", func(t *testing.T) {
		v := NewToolValue(map[string]any{
			"files": []any{"/a.txt", "/b.txt"},
			"config": map[string]any{
				"timeout": 30,
				"retry":   true,
			},
		})

		files := v.Get("files").Array()
		require.Len(t, files, 2)
		assert.Equal(t, "/a.txt", files[0].AsString())
		assert.Equal(t, "/b.txt", files[1].AsString())

		config := v.Get("config").Object()
		require.NotNil(t, config)
		assert.Equal(t, int64(30), config["timeout"].AsInt())
		assert.Equal(t, true, config["retry"].AsBool())

		configVal := v.Get("config")
		assert.Equal(t, "object", configVal.Type())
		assert.Equal(t, int64(30), configVal.Get("timeout").AsInt())
	})
}

func TestToolValue_Set(t *testing.T) {
	t.Run("set on empty value", func(t *testing.T) {
		var v ToolValue
		v.Set("content", "file contents")
		v.Set("size", 1024)
		v.Set("exists", true)

		assert.Equal(t, 3, v.Len())
		assert.True(t, v.Has("content"))
		assert.False(t, v.Has("nonexistent"))

		assert.Equal(t, "file contents", v.Get("content").AsString())
		assert.Equal(t, int64(1024), v.Get("size").AsInt())
		assert.Equal(t, true, v.Get("exists").AsBool())

		keys := v.Keys()
		assert.Len(t, keys, 3)
	})

	t.Run("set complex values", func(t *testing.T) {
		var v ToolValue
		v.Set("files", []any{"/a.txt", "/b.txt"})
		v.Set("metadata", map[string]any{
			"created": "2024-01-01",
			"size":    100,
		})

		files := v.Get("files").Array()
		require.Len(t, files, 2)
		assert.Equal(t, "/a.txt", files[0].AsString())

		metadata := v.Get("metadata")
		assert.Equal(t, "2024-01-01", metadata.Get("created").AsString())
		assert.Equal(t, int64(100), metadata.Get("size").AsInt())
	})

	t.Run("set ToolValue", func(t *testing.T) {
		var v ToolValue
		tv := NewToolValue(map[string]any{"nested": "value"})
		v.Set("data", tv)

		assert.Equal(t, "value", v.Get("data").Get("nested").AsString())
	})
}

func TestToolValue_FromJSON(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		jsonStr := `{"path": "/tmp/test.txt", "mode": 493}`
		v, err := NewToolValueFromJSON(jsonStr)
		require.NoError(t, err)

		assert.Equal(t, "/tmp/test.txt", v.String("path"))
		assert.Equal(t, int64(493), v.Int("mode"))
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := NewToolValueFromJSON("not json")
		assert.Error(t, err)
	})

	t.Run("not an object", func(t *testing.T) {
		_, err := NewToolValueFromJSON(`["array", "not", "object"]`)
		assert.Error(t, err)
	})
}

func TestToolValue_JSONRoundtrip(t *testing.T) {
	v := NewToolValue(map[string]any{
		"path":  "/tmp/test.txt",
		"lines": 100,
	})

	data, err := json.Marshal(v)
	require.NoError(t, err)

	var v2 ToolValue
	err = json.Unmarshal(data, &v2)
	require.NoError(t, err)

	assert.Equal(t, "/tmp/test.txt", v2.String("path"))
	assert.Equal(t, int64(100), v2.Int("lines"))
}

func TestToolValue_NilObject(t *testing.T) {
	v := NewToolValue(nil)
	assert.Equal(t, 0, v.Len())
	assert.False(t, v.Has("any"))
	assert.Equal(t, "", v.String("any"))
	assert.Nil(t, v.Keys())
}

func TestToolCall_WithComplexArgs(t *testing.T) {
	call := ToolCall{
		ID:       "call-123",
		Function: "process_files",
		Arguments: NewToolValue(map[string]any{
			"files":   []any{"/a.txt", "/b.txt"},
			"options": map[string]any{"recursive": true},
		}),
	}

	assert.Equal(t, "call-123", call.ID)
	assert.Equal(t, "process_files", call.Function)

	files := call.Arguments.Get("files").Array()
	require.Len(t, files, 2)
	assert.Equal(t, "/a.txt", files[0].AsString())

	options := call.Arguments.Get("options").Object()
	assert.Equal(t, true, options["recursive"].AsBool())
}

func TestToolResponse_WithComplexResult(t *testing.T) {
	var result ToolValue
	result.Set("files", []any{
		map[string]any{"name": "a.txt", "size": 100},
		map[string]any{"name": "b.txt", "size": 200},
	})

	response := ToolResponse{
		Call:   &ToolCall{ID: "call-123", Function: "list_files"},
		Error:  nil,
		Result: result,
		Done:   true,
	}

	assert.Equal(t, "call-123", response.Call.ID)
	assert.NoError(t, response.Error)
	assert.True(t, response.Done)

	files := response.Result.Get("files").Array()
	require.Len(t, files, 2)
	assert.Equal(t, "a.txt", files[0].Get("name").AsString())
	assert.Equal(t, int64(100), files[0].Get("size").AsInt())
}

func TestLLMAPICompatibility(t *testing.T) {
	t.Run("OpenAI style arguments", func(t *testing.T) {
		argsJSON := `{"path": "/tmp/test.txt", "options": {"line_start": 1, "line_end": 100}}`
		args, err := NewToolValueFromJSON(argsJSON)
		require.NoError(t, err)

		assert.Equal(t, "/tmp/test.txt", args.String("path"))
		options := args.Get("options").Object()
		assert.Equal(t, int64(1), options["line_start"].AsInt())
		assert.Equal(t, int64(100), options["line_end"].AsInt())
	})

	t.Run("Anthropic/Ollama style arguments", func(t *testing.T) {
		argsMap := map[string]any{
			"command": "ls -la",
			"env": map[string]any{
				"PATH": "/usr/bin",
				"HOME": "/home/user",
			},
			"timeout": 30000,
		}
		args := NewToolValue(argsMap)

		assert.Equal(t, "ls -la", args.String("command"))
		assert.Equal(t, int64(30000), args.Int("timeout"))
		env := args.Get("env").Object()
		assert.Equal(t, "/usr/bin", env["PATH"].AsString())
	})

	t.Run("Array of complex objects", func(t *testing.T) {
		argsJSON := `{
			"edits": [
				{"file": "a.go", "line": 10, "text": "new line"},
				{"file": "b.go", "line": 20, "text": "another line"}
			]
		}`
		args, err := NewToolValueFromJSON(argsJSON)
		require.NoError(t, err)

		edits := args.Get("edits").Array()
		require.Len(t, edits, 2)
		assert.Equal(t, "a.go", edits[0].Get("file").AsString())
		assert.Equal(t, int64(10), edits[0].Get("line").AsInt())
		assert.Equal(t, "new line", edits[0].Get("text").AsString())
	})
}
