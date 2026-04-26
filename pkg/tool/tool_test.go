package tool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		maxLines int
		expected string
	}{
		{
			name:     "no truncation needed - fewer lines than limit",
			output:   "line1\nline2\nline3",
			maxLines: 5,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "no truncation needed - exact limit",
			output:   "line1\nline2\nline3",
			maxLines: 3,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "truncation needed",
			output:   "line1\nline2\nline3\nline4\nline5",
			maxLines: 3,
			expected: "line1\nline2\nline3\nOutput is truncated.",
		},
		{
			name:     "zero limit means no limit",
			output:   "line1\nline2\nline3\nline4\nline5",
			maxLines: 0,
			expected: "line1\nline2\nline3\nline4\nline5",
		},
		{
			name:     "negative limit means no limit",
			output:   "line1\nline2\nline3",
			maxLines: -1,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "single line output",
			output:   "single line",
			maxLines: 5,
			expected: "single line",
		},
		{
			name:     "empty output",
			output:   "",
			maxLines: 5,
			expected: "",
		},
		{
			name:     "limit of 1",
			output:   "line1\nline2\nline3",
			maxLines: 1,
			expected: "line1\nOutput is truncated.",
		},
		{
			name:     "output with trailing newline truncated",
			output:   "line1\nline2\nline3\n",
			maxLines: 2,
			expected: "line1\nline2\nOutput is truncated.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateOutput(tt.output, tt.maxLines)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToolInfo_ShortDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "single line",
			description: "This is a short description",
			expected:    "This is a short description",
		},
		{
			name:        "multiple lines",
			description: "First line description\nSecond line\nThird line",
			expected:    "First line description",
		},
		{
			name:        "with leading whitespace",
			description: "   Leading spaces should be trimmed",
			expected:    "Leading spaces should be trimmed",
		},
		{
			name:        "with trailing whitespace",
			description: "Trailing spaces should be trimmed   ",
			expected:    "Trailing spaces should be trimmed",
		},
		{
			name:        "with empty lines before content",
			description: "\n\n  \nFirst non-empty line\nSecond line",
			expected:    "First non-empty line",
		},
		{
			name:        "markdown with multiple paragraphs",
			description: "This is the first paragraph.\n\nThis is the second paragraph.",
			expected:    "This is the first paragraph.",
		},
		{
			name:        "empty description",
			description: "",
			expected:    "",
		},
		{
			name:        "only whitespace",
			description: "   \n  \n   ",
			expected:    "",
		},
		{
			name: "complex markdown",
			description: `Read file contents from the filesystem.

This tool allows reading files with various options including line ranges and encoding.`,
			expected: "Read file contents from the filesystem.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolInfo := ToolInfo{
				Description: tt.description,
			}
			result := toolInfo.ShortDescription()
			assert.Equal(t, tt.expected, result)
		})
	}
}
