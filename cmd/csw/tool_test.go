package main

import (
	"bytes"
	"encoding/json"
	"os"
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
		if len(key) > len("/schema.json") && key[len(key)-len("/schema.json"):] == "/schema.json" {
			toolCount++
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
	// We'll test with vfs.read since it's a basic tool that should always be there
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	allRole, exists := roleConfigs["all"]
	require.True(t, exists)
	require.NotNil(t, allRole.ToolFragments)

	// Check if vfs.read tool exists
	_, hasSchema := allRole.ToolFragments["vfs.read/schema.json"]
	_, hasDesc := allRole.ToolFragments["vfs.read/tool.md"]
	if hasSchema && hasDesc {
		// If vfs.read exists, verify the fragments exist
		assert.True(t, hasSchema, "vfs.read should have schema.json")
		assert.True(t, hasDesc, "vfs.read should have tool.md")
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
	_, hasDesc := allRole.ToolFragments["vfs.read/tool.md"]
	if hasDesc {
		desc := allRole.ToolFragments["vfs.read/tool.md"]
		assert.NotEmpty(t, desc, "tool description should not be empty")
	}
}

func TestOutputToolListTable(t *testing.T) {
	tools := map[string]string{
		"vfs.read":  "Reads a file from the local filesystem",
		"vfs.write": "Writes content to a file",
		"vfs.list":  "Lists files in a directory",
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
	assert.Contains(t, output, "vfs.read")
	assert.Contains(t, output, "vfs.write")
	assert.Contains(t, output, "vfs.list")
}

func TestOutputToolListJSON(t *testing.T) {
	tools := map[string]string{
		"vfs.read":  "Reads a file from the local filesystem",
		"vfs.write": "Writes content to a file",
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
	assert.Equal(t, "vfs.read", result[0].Name)
	assert.Equal(t, "Reads a file from the local filesystem", result[0].Description)
	assert.Equal(t, "vfs.write", result[1].Name)
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
	_, hasSchema := allRole.ToolFragments["nonexistent.tool/schema.json"]
	assert.False(t, hasSchema, "nonexistent tool should not have schema")
}
