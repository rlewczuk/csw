package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleCommand_List(t *testing.T) {
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

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Verify that at least some default roles are present
	// The embedded defaults should include "developer" and "all" roles
	assert.GreaterOrEqual(t, len(configs), 1, "at least one role should be loaded from defaults")
	assert.Contains(t, configs, "developer", "developer role should be in embedded defaults")
}

func TestRoleCommand_Show(t *testing.T) {
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

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Test with the embedded "developer" role
	config, exists := configs["developer"]
	require.True(t, exists, "developer role should exist in embedded defaults")
	assert.NotEmpty(t, config.Description, "developer role should have a description")
	assert.NotNil(t, config.VFSPrivileges, "developer role should have VFS privileges")
}

func TestOutputRoleList(t *testing.T) {
	configs := map[string]*conf.AgentRoleConfig{
		"developer": {
			Name:        "developer",
			Description: "Developer role",
		},
		"reviewer": {
			Name:        "reviewer",
			Description: "Code reviewer role",
		},
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRoleList(configs)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "developer")
	assert.Contains(t, output, "reviewer")
	assert.Contains(t, output, "Developer role")
	assert.Contains(t, output, "Code reviewer role")
}

func TestOutputRoleDetails(t *testing.T) {
	config := &conf.AgentRoleConfig{
		Name:        "test-role",
		Aliases:     []string{"tester", "qa"},
		Description: "Test role for details",
		VFSPrivileges: map[string]conf.FileAccess{
			"**": {
				Read:   conf.AccessAllow,
				Write:  conf.AccessAsk,
				Delete: conf.AccessDeny,
				List:   conf.AccessAllow,
				Find:   conf.AccessAllow,
				Move:   conf.AccessDeny,
			},
		},
		ToolsAccess: map[string]conf.AccessFlag{
			"read":  conf.AccessAllow,
			"write": conf.AccessAsk,
			"run":   conf.AccessDeny,
		},
		RunPrivileges: map[string]conf.AccessFlag{
			"^git.*":  conf.AccessAllow,
			"^npm.*":  conf.AccessAsk,
			"^rm -rf": conf.AccessDeny,
		},
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRoleDetails(config)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "test-role")
	assert.Contains(t, output, "tester, qa")
	assert.Contains(t, output, "Test role for details")
	assert.Contains(t, output, "VFS Privileges")
	assert.Contains(t, output, "Tool Access")
	assert.Contains(t, output, "Run Privileges")
	assert.Contains(t, output, "allow")
	assert.Contains(t, output, "ask")
	assert.Contains(t, output, "deny")
}

func TestFindRoleConfigByName(t *testing.T) {
	configs := map[string]*conf.AgentRoleConfig{
		"developer": {
			Name:        "developer",
			Description: "Developer",
			Aliases:     []string{"dev", "build"},
		},
	}

	byName, ok := findRoleConfigByName(configs, "developer")
	require.True(t, ok)
	require.NotNil(t, byName)
	assert.Equal(t, "developer", byName.Name)

	byAlias, ok := findRoleConfigByName(configs, "dev")
	require.True(t, ok)
	require.NotNil(t, byAlias)
	assert.Equal(t, "developer", byAlias.Name)

	byAliasCaseInsensitive, ok := findRoleConfigByName(configs, "BUILD")
	require.True(t, ok)
	require.NotNil(t, byAliasCaseInsensitive)
	assert.Equal(t, "developer", byAliasCaseInsensitive.Name)

	missing, ok := findRoleConfigByName(configs, "unknown")
	assert.False(t, ok)
	assert.Nil(t, missing)
}

func TestFormatAccessFlag(t *testing.T) {
	tests := []struct {
		name     string
		flag     conf.AccessFlag
		expected string
	}{
		{
			name:     "allow flag",
			flag:     conf.AccessAllow,
			expected: "allow",
		},
		{
			name:     "deny flag",
			flag:     conf.AccessDeny,
			expected: "deny",
		},
		{
			name:     "ask flag",
			flag:     conf.AccessAsk,
			expected: "ask",
		},
		{
			name:     "unknown flag",
			flag:     conf.AccessFlag("unknown"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAccessFlag(tt.flag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoleComposite(t *testing.T) {
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

	// Get composite store - should load embedded defaults
	compositeStore, err := GetCompositeConfigStore()
	require.NoError(t, err)

	configs, err := compositeStore.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Should have at least the developer role from embedded defaults
	assert.GreaterOrEqual(t, len(configs), 1, "should have at least one role from embedded defaults")
	assert.Contains(t, configs, "developer", "developer role should be loaded from embedded defaults")

	// Verify developer role has expected fields
	developerConfig, exists := configs["developer"]
	require.True(t, exists)
	assert.NotEmpty(t, developerConfig.Description, "developer role should have a description")
}

func TestOutputSystemPrompt(t *testing.T) {
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

	// Get composite config store (uses embedded defaults)
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	config, exists := configs["developer"]
	require.True(t, exists, "developer role should exist in embedded defaults")

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = outputSystemPrompt(store, config, "", false)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)

	// Should succeed (the exact content depends on embedded prompts, may be empty if no prompts defined)
	assert.NoError(t, err)
}

func TestOutputSystemPromptWithModel(t *testing.T) {
	// This test verifies that model parsing works correctly
	// We test with an invalid model format to ensure proper error handling
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

	// Get composite config store
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	config, exists := configs["developer"]
	require.True(t, exists)

	// Test with invalid model format - should return error
	err = outputSystemPrompt(store, config, "invalid-model-format", false)
	assert.Error(t, err, "invalid model format should return error")
	assert.Contains(t, err.Error(), "invalid model format")
}

func TestOutputSystemPromptJSON(t *testing.T) {
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

	// Get composite config store (uses embedded defaults)
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	config, exists := configs["developer"]
	require.True(t, exists, "developer role should exist in embedded defaults")

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = outputSystemPrompt(store, config, "", true)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	// Verify it's valid JSON
	var result map[string]string
	err = json.Unmarshal([]byte(output), &result)
	assert.NoError(t, err, "output should be valid JSON")
	assert.Contains(t, result, "system_prompt", "JSON should contain system_prompt field")
}

func compileModelTagPattern(pattern string) (*regexp.Regexp, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compileModelTagPattern() [role.go]: invalid regexp %q: %w", pattern, err)
	}
	return compiled, nil
}

func TestCompileModelTagPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		shouldError bool
	}{
		{
			name:        "valid pattern",
			pattern:     "gpt-4.*",
			shouldError: false,
		},
		{
			name:        "another valid pattern",
			pattern:     ".*turbo.*",
			shouldError: false,
		},
		{
			name:        "invalid pattern",
			pattern:     "[invalid",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := compileModelTagPattern(tt.pattern)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
