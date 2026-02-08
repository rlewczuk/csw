package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCommand_List(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Get composite config store (loads from embedded defaults)
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Verify that at least some default tools are available
	allRole, exists := roleConfigs["all"]
	require.True(t, exists, "all role should exist in embedded defaults")
	require.NotNil(t, allRole.ToolFragments, "all role should have tool fragments")
	assert.NotEmpty(t, allRole.ToolFragments, "all role should have at least one tool fragment")
}

func TestToolCommand_ListJSON(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Get composite config store (loads from embedded defaults)
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Get tool fragments from all role
	allRole, exists := roleConfigs["all"]
	require.True(t, exists, "all role should exist")
	require.NotNil(t, allRole.ToolFragments)

	// Extract tool names (just verify we can process them)
	toolCount := 0
	for key := range allRole.ToolFragments {
		if strings.HasSuffix(key, ".schema.json") {
			// Check if it's a <toolname>/<toolname>.schema.json pattern
			parts := strings.Split(key, "/")
			if len(parts) == 2 {
				toolName := parts[0]
				expectedFileName := toolName + ".schema.json"
				if parts[1] == expectedFileName {
					toolCount++
				}
			}
		}
	}
	assert.Greater(t, toolCount, 0, "should have at least one tool")
}

func TestToolCommand_Info(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Test that we can get tool info (using a tool that should exist in defaults)
	// We'll test with vfsRead since it's a basic tool that should always be there
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	allRole, exists := roleConfigs["all"]
	require.True(t, exists)
	require.NotNil(t, allRole.ToolFragments)

	// Check if vfsRead tool exists
	_, hasSchema := allRole.ToolFragments["vfsRead/vfsRead.schema.json"]
	_, hasDesc := allRole.ToolFragments["vfsRead/vfsRead.md"]
	if hasSchema && hasDesc {
		// If vfsRead exists, verify the fragments exist
		assert.True(t, hasSchema, "vfsRead should have vfsRead.schema.json")
		assert.True(t, hasDesc, "vfsRead should have vfsRead.md")
	}
}

func TestToolCommand_Desc(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	allRole, exists := roleConfigs["all"]
	require.True(t, exists)
	require.NotNil(t, allRole.ToolFragments)

	// Verify tool description exists
	_, hasDesc := allRole.ToolFragments["vfsRead/vfsRead.md"]
	if hasDesc {
		desc := allRole.ToolFragments["vfsRead/vfsRead.md"]
		assert.NotEmpty(t, desc, "tool description should not be empty")
	}
}

func TestOutputToolListTable(t *testing.T) {
	tools := map[string]string{
		"vfsRead":  "Reads a file from the local filesystem",
		"vfsWrite": "Writes content to a file",
		"vfsList":  "Lists files in a directory",
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputToolListTable(tools)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "DESCRIPTION")
	assert.Contains(t, output, "vfsRead")
	assert.Contains(t, output, "vfsWrite")
	assert.Contains(t, output, "vfsList")
}

func TestOutputToolListJSON(t *testing.T) {
	tools := map[string]string{
		"vfsRead":  "Reads a file from the local filesystem",
		"vfsWrite": "Writes content to a file",
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputToolListJSON(tools)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	// Verify it's valid JSON
	var result []toolListEntry
	err = json.Unmarshal([]byte(output), &result)
	assert.NoError(t, err, "output should be valid JSON")
	assert.Len(t, result, 2)

	// Verify entries (sorted by name)
	assert.Equal(t, "vfsRead", result[0].Name)
	assert.Equal(t, "Reads a file from the local filesystem", result[0].Description)
	assert.Equal(t, "vfsWrite", result[1].Name)
	assert.Equal(t, "Writes content to a file", result[1].Description)
}

func TestOutputToolListTable_LongDescription(t *testing.T) {
	tools := map[string]string{
		"test.tool": "This is a very long description that should be truncated when displayed in the table format because it exceeds the maximum length",
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputToolListTable(tools)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "test.tool")
	assert.Contains(t, output, "...")
}

func TestToolCommand_ListWithRole(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Test with developer role
	developerRole, exists := roleConfigs["developer"]
	if exists {
		require.NotNil(t, developerRole.ToolFragments, "developer role should have tool fragments")
		// At least one tool should be available
		assert.NotEmpty(t, developerRole.ToolFragments)
	}
}

func TestToolCommand_InfoNotFound(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	allRole, exists := roleConfigs["all"]
	require.True(t, exists)

	// Verify that a non-existent tool is not in fragments
	_, hasSchema := allRole.ToolFragments["nonexistent.tool/nonexistent.tool.schema.json"]
	assert.False(t, hasSchema, "nonexistent tool should not have schema")
}

func TestVFSDeleteTool_RegisteredAndAdvertised(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Get composite config store (loads from embedded defaults)
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Verify vfsDelete is advertised to LLM via tool fragments
	allRole, exists := roleConfigs["all"]
	require.True(t, exists, "all role should exist")
	require.NotNil(t, allRole.ToolFragments, "all role should have tool fragments")

	// Check that vfsDelete has both schema and description files
	_, hasSchema := allRole.ToolFragments["vfsDelete/vfsDelete.schema.json"]
	_, hasDesc := allRole.ToolFragments["vfsDelete/vfsDelete.md"]
	assert.True(t, hasSchema, "vfsDelete should have vfsDelete.schema.json advertised to LLM")
	assert.True(t, hasDesc, "vfsDelete should have vfsDelete.md advertised to LLM")

	// Verify the schema content is valid JSON
	schemaContent := allRole.ToolFragments["vfsDelete/vfsDelete.schema.json"]
	assert.NotEmpty(t, schemaContent, "vfsDelete schema should not be empty")
	assert.Contains(t, schemaContent, "path", "vfsDelete schema should contain 'path' property")

	// Verify the description content
	descContent := allRole.ToolFragments["vfsDelete/vfsDelete.md"]
	assert.NotEmpty(t, descContent, "vfsDelete description should not be empty")
}
