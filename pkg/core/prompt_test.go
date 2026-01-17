package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		wantOrder    int
		wantKind     string
		wantToolName string
		wantTag      string
		wantOk       bool
	}{
		{
			name:      "system fragment without tag",
			filename:  "10-system.md",
			wantOrder: 10,
			wantKind:  "system",
			wantTag:   "all",
			wantOk:    true,
		},
		{
			name:      "system fragment with tag",
			filename:  "20-system-anthropic.md",
			wantOrder: 20,
			wantKind:  "system",
			wantTag:   "anthropic",
			wantOk:    true,
		},
		{
			name:      "system fragment with multi-part tag",
			filename:  "20-system-anthropic-v2.md",
			wantOrder: 20,
			wantKind:  "system",
			wantTag:   "anthropic-v2",
			wantOk:    true,
		},
		{
			name:         "tools fragment without tag",
			filename:     "30-tools-read.md",
			wantOrder:    30,
			wantKind:     "tools",
			wantToolName: "read",
			wantTag:      "all",
			wantOk:       true,
		},
		{
			name:         "tools fragment with tag",
			filename:     "40-tools-write-anthropic.md",
			wantOrder:    40,
			wantKind:     "tools",
			wantToolName: "write",
			wantTag:      "anthropic",
			wantOk:       true,
		},
		{
			name:     "invalid no extension",
			filename: "10-system",
			wantOk:   false,
		},
		{
			name:     "invalid wrong extension",
			filename: "10-system.txt",
			wantOk:   false,
		},
		{
			name:     "invalid no number",
			filename: "system.md",
			wantOk:   false,
		},
		{
			name:     "invalid number",
			filename: "abc-system.md",
			wantOk:   false,
		},
		{
			name:     "invalid kind",
			filename: "10-unknown.md",
			wantOk:   false,
		},
		{
			name:     "invalid tools without toolname",
			filename: "10-tools.md",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, kind, toolName, tag, ok := parseFilename(tt.filename)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.wantOrder, order)
				assert.Equal(t, tt.wantKind, kind)
				assert.Equal(t, tt.wantToolName, toolName)
				assert.Equal(t, tt.wantTag, tag)
			}
		})
	}
}

func TestFileBasedPromptScanner_GetFragments(t *testing.T) {
	// Create a temporary test directory
	tmpDir := t.TempDir()
	rolesDir := filepath.Join(tmpDir, "roles")

	// Copy test roles to temp directory
	err := shared.CopyDir("../../testdata/conf/roles", rolesDir)
	require.NoError(t, err)

	// Create scanner
	scanner, err := NewFileBasedPromptScanner(rolesDir)
	require.NoError(t, err)
	defer scanner.Close()

	// Load test role
	role := conf.AgentRoleConfig{
		Name: "test1",
		ToolsAccess: map[string]conf.AccessFlag{
			"read":  conf.AccessAllow,
			"write": conf.AccessAllow,
			"bash":  conf.AccessAllow,
		},
	}

	tests := []struct {
		name     string
		tags     []string
		role     *conf.AgentRoleConfig
		wantKeys []string
	}{
		{
			name: "test1 with no specific tags",
			tags: []string{},
			role: &role,
			wantKeys: []string{
				"test1/10-system",     // overrides all/10-system.md
				"all/30-system",       // no tag, so tag="all"
				"all/40-tools-read",   // no tag, so tag="all"
				"test1/60-system",     // no tag, so tag="all"
				"test1/70-tools-bash", // no tag, so tag="all"
			},
		},
		{
			name: "test1 with anthropic tag",
			tags: []string{"anthropic", "all"},
			role: &role,
			wantKeys: []string{
				"test1/10-system",
				"all/20-system-anthropic",
				"all/30-system",
				"all/40-tools-read",
				"all/50-tools-write-anthropic",
				"test1/60-system",
				"test1/70-tools-bash",
			},
		},
		{
			name: "test1 with openai tag",
			tags: []string{"openai", "all"},
			role: &role,
			wantKeys: []string{
				"test1/10-system",
				"all/20-system-openai",
				"all/30-system",
				"all/40-tools-read",
				"test1/60-system",
				"test1/70-tools-bash",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragments, err := scanner.GetFragments(tt.tags, tt.role)
			require.NoError(t, err)

			// Check that we got the expected keys
			var gotKeys []string
			for key := range fragments {
				gotKeys = append(gotKeys, key)
			}

			// Sort for comparison (note: fragments are sorted in GetPrompt, not GetFragments)
			assert.ElementsMatch(t, tt.wantKeys, gotKeys)

			// Verify content is not empty
			for key, content := range fragments {
				assert.NotEmpty(t, content, "fragment %s should not be empty", key)
			}
		})
	}
}

func TestFileBasedPromptScanner_GetFragments_ToolsFiltering(t *testing.T) {
	// Create a temporary test directory
	tmpDir := t.TempDir()
	rolesDir := filepath.Join(tmpDir, "roles")

	// Copy test roles to temp directory
	err := shared.CopyDir("../../testdata/conf/roles", rolesDir)
	require.NoError(t, err)

	// Create scanner
	scanner, err := NewFileBasedPromptScanner(rolesDir)
	require.NoError(t, err)
	defer scanner.Close()

	// Test with role that only has read access
	role := conf.AgentRoleConfig{
		Name: "test2",
		ToolsAccess: map[string]conf.AccessFlag{
			"read": conf.AccessAllow,
		},
	}

	fragments, err := scanner.GetFragments([]string{"all"}, &role)
	require.NoError(t, err)

	// Should only have fragments for tools that are enabled
	for key := range fragments {
		if strings.Contains(key, "tools-write") {
			t.Errorf("unexpected write tool fragment: %s", key)
		}
		if strings.Contains(key, "tools-bash") {
			t.Errorf("unexpected bash tool fragment: %s", key)
		}
	}

	// Should have read tool fragment
	foundRead := false
	for key := range fragments {
		if strings.Contains(key, "tools-read") {
			foundRead = true
			break
		}
	}
	assert.True(t, foundRead, "should have read tool fragment")
}

func TestFSPromptGenerator_GetPrompt(t *testing.T) {
	// Create a temporary test directory
	tmpDir := t.TempDir()
	rolesDir := filepath.Join(tmpDir, "roles")

	// Copy test roles to temp directory
	err := shared.CopyDir("../../testdata/conf/roles", rolesDir)
	require.NoError(t, err)

	// Create scanner
	scanner, err := NewFileBasedPromptScanner(rolesDir)
	require.NoError(t, err)
	defer scanner.Close()

	// Create generator with scanner
	gen, err := NewFSPromptGenerator(scanner)
	require.NoError(t, err)

	// Load test role
	role := conf.AgentRoleConfig{
		Name: "test1",
		ToolsAccess: map[string]conf.AccessFlag{
			"read":  conf.AccessAllow,
			"write": conf.AccessAllow,
			"bash":  conf.AccessAllow,
		},
	}

	// Create agent state
	state := &AgentState{
		Info: AgentStateCommonInfo{
			WorkDir: "/test/dir",
		},
	}

	tests := []struct {
		name         string
		tags         []string
		wantContains []string
	}{
		{
			name: "anthropic tags",
			tags: []string{"anthropic", "all"},
			wantContains: []string{
				"/test/dir",                           // from template substitution
				"Test1 Core Instructions",             // from test1/10-system.md
				"Anthropic-Specific Instructions",     // from all/20-system-anthropic.md
				"General Guidelines",                  // from all/30-system.md
				"Read Tool Instructions",              // from all/40-tools-read.md
				"Write Tool Instructions (Anthropic)", // from all/50-tools-write-anthropic.md
				"Test1-Specific Guidelines",           // from test1/60-system.md
				"Bash Tool Instructions (Test1)",      // from test1/70-tools-bash.md
			},
		},
		{
			name: "openai tags",
			tags: []string{"openai", "all"},
			wantContains: []string{
				"/test/dir",
				"Test1 Core Instructions",
				"OpenAI-Specific Instructions", // from all/20-system-openai.md instead of anthropic
				"General Guidelines",
				"Read Tool Instructions",
				// Note: no write-anthropic fragment
				"Test1-Specific Guidelines",
				"Bash Tool Instructions (Test1)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := gen.GetPrompt(tt.tags, &role, state)
			require.NoError(t, err)
			assert.NotEmpty(t, prompt)

			for _, want := range tt.wantContains {
				assert.Contains(t, prompt, want, "prompt should contain: %s", want)
			}
		})
	}
}

func TestFSPromptGenerator_GetPrompt_Ordering(t *testing.T) {
	// Create a temporary test directory
	tmpDir := t.TempDir()
	rolesDir := filepath.Join(tmpDir, "roles")

	// Copy test roles to temp directory
	err := shared.CopyDir("../../testdata/conf/roles", rolesDir)
	require.NoError(t, err)

	// Create scanner
	scanner, err := NewFileBasedPromptScanner(rolesDir)
	require.NoError(t, err)
	defer scanner.Close()

	// Create generator with scanner
	gen, err := NewFSPromptGenerator(scanner)
	require.NoError(t, err)

	// Load test role
	role := conf.AgentRoleConfig{
		Name: "test1",
		ToolsAccess: map[string]conf.AccessFlag{
			"read":  conf.AccessAllow,
			"write": conf.AccessAllow,
			"bash":  conf.AccessAllow,
		},
	}

	// Create agent state
	state := &AgentState{
		Info: AgentStateCommonInfo{
			WorkDir: "/test/dir",
		},
	}

	prompt, err := gen.GetPrompt([]string{"anthropic", "all"}, &role, state)
	require.NoError(t, err)

	// Check ordering by finding positions
	markers := []string{
		"Test1 Core Instructions",             // 10
		"Anthropic-Specific Instructions",     // 20
		"General Guidelines",                  // 30
		"Read Tool Instructions",              // 40
		"Write Tool Instructions (Anthropic)", // 50
		"Test1-Specific Guidelines",           // 60
		"Bash Tool Instructions (Test1)",      // 70
	}

	lastPos := -1
	for _, marker := range markers {
		pos := strings.Index(prompt, marker)
		assert.NotEqual(t, -1, pos, "marker not found: %s", marker)
		assert.Greater(t, pos, lastPos, "marker %s should come after previous marker", marker)
		lastPos = pos
	}
}

func TestFileBasedPromptScanner_CacheReload(t *testing.T) {
	// Create a temporary test directory structure
	tmpDir := t.TempDir()
	rolesDir := filepath.Join(tmpDir, "roles")
	testRoleDir := filepath.Join(rolesDir, "testcache")

	// Create test role directory
	err := os.MkdirAll(testRoleDir, 0755)
	require.NoError(t, err)

	// Write initial file
	initialContent := "# Initial Content"
	err = os.WriteFile(filepath.Join(testRoleDir, "10-system.md"), []byte(initialContent), 0644)
	require.NoError(t, err)

	// Create scanner
	scanner, err := NewFileBasedPromptScanner(rolesDir)
	require.NoError(t, err)
	defer scanner.Close()

	// Create generator with scanner
	gen, err := NewFSPromptGenerator(scanner)
	require.NoError(t, err)

	// Create role
	role := conf.AgentRoleConfig{
		Name:        "testcache",
		ToolsAccess: map[string]conf.AccessFlag{},
	}

	// Get initial prompt
	state := &AgentState{Info: AgentStateCommonInfo{WorkDir: "/test"}}
	prompt1, err := gen.GetPrompt([]string{"all"}, &role, state)
	require.NoError(t, err)
	assert.Contains(t, prompt1, initialContent)

	// Modify file
	updatedContent := "# Updated Content"
	err = os.WriteFile(filepath.Join(testRoleDir, "10-system.md"), []byte(updatedContent), 0644)
	require.NoError(t, err)

	// Wait for fsnotify to detect the change
	time.Sleep(100 * time.Millisecond)

	// Get prompt again - should have updated content
	prompt2, err := gen.GetPrompt([]string{"all"}, &role, state)
	require.NoError(t, err)
	assert.Contains(t, prompt2, updatedContent)
	assert.NotContains(t, prompt2, initialContent)
}

func TestFilterDuplicates(t *testing.T) {
	tests := []struct {
		name      string
		fragments []promptFragment
		wantCount int
	}{
		{
			name: "no duplicates",
			fragments: []promptFragment{
				{order: 10, kind: "system", tag: "all", isAll: true},
				{order: 20, kind: "system", tag: "anthropic", isAll: true},
			},
			wantCount: 2,
		},
		{
			name: "role overrides all",
			fragments: []promptFragment{
				{order: 10, kind: "system", tag: "all", isAll: true},
				{order: 10, kind: "system", tag: "all", isAll: false}, // role-specific override
			},
			wantCount: 1, // only role-specific remains
		},
		{
			name: "mixed overrides",
			fragments: []promptFragment{
				{order: 10, kind: "system", tag: "all", isAll: true},
				{order: 10, kind: "system", tag: "all", isAll: false},
				{order: 20, kind: "system", tag: "anthropic", isAll: true},
				{order: 30, kind: "tools", toolName: "read", tag: "all", isAll: true},
			},
			wantCount: 3, // 10-system (role), 20-system-anthropic (all), 30-tools-read (all)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterDuplicates(tt.fragments)
			assert.Equal(t, tt.wantCount, len(result))
		})
	}
}

func TestFileBasedPromptScanner_HasChanged(t *testing.T) {
	// Create a temporary test directory structure
	tmpDir := t.TempDir()
	rolesDir := filepath.Join(tmpDir, "roles")
	testRoleDir := filepath.Join(rolesDir, "testchange")

	// Create test role directory
	err := os.MkdirAll(testRoleDir, 0755)
	require.NoError(t, err)

	// Write initial file
	initialContent := "# Initial Content"
	err = os.WriteFile(filepath.Join(testRoleDir, "10-system.md"), []byte(initialContent), 0644)
	require.NoError(t, err)

	// Create scanner
	scanner, err := NewFileBasedPromptScanner(rolesDir)
	require.NoError(t, err)
	defer scanner.Close()

	// HasChanged should be true initially (after loadCache in constructor)
	assert.True(t, scanner.HasChanged(), "should be true after initial load")

	// Second call should be false (flag gets reset)
	assert.False(t, scanner.HasChanged(), "should be false after reset")

	// Modify file
	updatedContent := "# Updated Content"
	err = os.WriteFile(filepath.Join(testRoleDir, "10-system.md"), []byte(updatedContent), 0644)
	require.NoError(t, err)

	// Wait for fsnotify to detect the change
	time.Sleep(100 * time.Millisecond)

	// HasChanged should be true now
	assert.True(t, scanner.HasChanged(), "should be true after file change")

	// Second call should be false again
	assert.False(t, scanner.HasChanged(), "should be false after reset")
}

func TestFSPromptGenerator_MultipleScanners(t *testing.T) {
	// Create two temporary directories
	tmpDir1 := t.TempDir()
	rolesDir1 := filepath.Join(tmpDir1, "roles")
	testRoleDir1 := filepath.Join(rolesDir1, "all")

	tmpDir2 := t.TempDir()
	rolesDir2 := filepath.Join(tmpDir2, "roles")
	testRoleDir2 := filepath.Join(rolesDir2, "all")

	// Create test role directories
	err := os.MkdirAll(testRoleDir1, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(testRoleDir2, 0755)
	require.NoError(t, err)

	// Write files to both directories
	// First scanner has 10-system.md
	content1 := "# Content from Scanner 1"
	err = os.WriteFile(filepath.Join(testRoleDir1, "10-system.md"), []byte(content1), 0644)
	require.NoError(t, err)

	// Second scanner also has 10-system.md (should override first)
	content2 := "# Content from Scanner 2"
	err = os.WriteFile(filepath.Join(testRoleDir2, "10-system.md"), []byte(content2), 0644)
	require.NoError(t, err)

	// Second scanner also has 20-system.md (unique)
	content3 := "# Additional Content from Scanner 2"
	err = os.WriteFile(filepath.Join(testRoleDir2, "20-system.md"), []byte(content3), 0644)
	require.NoError(t, err)

	// Create scanners
	scanner1, err := NewFileBasedPromptScanner(rolesDir1)
	require.NoError(t, err)
	defer scanner1.Close()

	scanner2, err := NewFileBasedPromptScanner(rolesDir2)
	require.NoError(t, err)
	defer scanner2.Close()

	// Create generator with both scanners
	gen, err := NewFSPromptGenerator(scanner1, scanner2)
	require.NoError(t, err)

	// Create test role
	role := conf.AgentRoleConfig{
		Name:        "test",
		ToolsAccess: map[string]conf.AccessFlag{},
	}

	// Get prompt
	state := &AgentState{Info: AgentStateCommonInfo{WorkDir: "/test"}}
	prompt, err := gen.GetPrompt([]string{"all"}, &role, state)
	require.NoError(t, err)

	// Should contain content from scanner 2 (overrides scanner 1)
	assert.Contains(t, prompt, content2, "should contain content from scanner 2")
	assert.NotContains(t, prompt, content1, "should not contain content from scanner 1 (overridden)")

	// Should also contain unique content from scanner 2
	assert.Contains(t, prompt, content3, "should contain unique content from scanner 2")
}
