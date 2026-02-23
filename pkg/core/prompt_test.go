package core

import (
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFragmentKey(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		hasExtension bool
		wantOrder    int
		wantKind     string
		wantToolName string
		wantTag      string
		wantOk       bool
	}{
		{
			name:         "system fragment without tag",
			filename:     "10-system.md",
			hasExtension: true,
			wantOrder:    10,
			wantKind:     "system",
			wantTag:      "all",
			wantOk:       true,
		},
		{
			name:         "system fragment with tag",
			filename:     "20-system-anthropic.md",
			hasExtension: true,
			wantOrder:    20,
			wantKind:     "system",
			wantTag:      "anthropic",
			wantOk:       true,
		},
		{
			name:         "system fragment with multi-part tag",
			filename:     "20-system-anthropic-v2.md",
			hasExtension: true,
			wantOrder:    20,
			wantKind:     "system",
			wantTag:      "anthropic-v2",
			wantOk:       true,
		},
		{
			name:         "tools fragment without tag",
			filename:     "30-tools-read.md",
			hasExtension: true,
			wantOrder:    30,
			wantKind:     "tools",
			wantToolName: "read",
			wantTag:      "all",
			wantOk:       true,
		},
		{
			name:         "tools fragment with tag",
			filename:     "40-tools-write-anthropic.md",
			hasExtension: true,
			wantOrder:    40,
			wantKind:     "tools",
			wantToolName: "write",
			wantTag:      "anthropic",
			wantOk:       true,
		},
		{
			name:         "invalid no extension",
			filename:     "10-system",
			hasExtension: true,
			wantOk:       false,
		},
		{
			name:         "invalid wrong extension",
			filename:     "10-system.txt",
			hasExtension: true,
			wantOk:       false,
		},
		{
			name:         "invalid no number",
			filename:     "system.md",
			hasExtension: true,
			wantOk:       false,
		},
		{
			name:         "invalid number",
			filename:     "abc-system.md",
			hasExtension: true,
			wantOk:       false,
		},
		{
			name:         "invalid kind",
			filename:     "10-unknown.md",
			hasExtension: true,
			wantOk:       false,
		},
		{
			name:         "invalid tools without toolname",
			filename:     "10-tools.md",
			hasExtension: true,
			wantOk:       false,
		},
		{
			name:         "no extension expected",
			filename:     "50-system",
			hasExtension: false,
			wantOrder:    50,
			wantKind:     "system",
			wantTag:      "all",
			wantOk:       true,
		},
		{
			name:         "extension rejected when not expected",
			filename:     "50-system.md",
			hasExtension: false,
			wantOk:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, kind, toolName, tag, ok := parseFragmentKey(tt.filename, tt.hasExtension)
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

func TestParseFragmentFromKey(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		content      string
		isAll        bool
		wantOrder    int
		wantKind     string
		wantToolName string
		wantTag      string
		wantIsAll    bool
		wantNil      bool
	}{
		{
			name:      "system fragment without tag",
			key:       "10-system",
			content:   "content",
			isAll:     true,
			wantOrder: 10,
			wantKind:  "system",
			wantTag:   "all",
			wantIsAll: true,
		},
		{
			name:      "system fragment with tag",
			key:       "20-system-anthropic",
			content:   "content",
			isAll:     true,
			wantOrder: 20,
			wantKind:  "system",
			wantTag:   "anthropic",
			wantIsAll: true,
		},
		{
			name:      "system fragment with multi-part tag",
			key:       "20-system-anthropic-v2",
			content:   "content",
			isAll:     false,
			wantOrder: 20,
			wantKind:  "system",
			wantTag:   "anthropic-v2",
			wantIsAll: false,
		},
		{
			name:         "tools fragment without tag",
			key:          "30-tools-read",
			content:      "content",
			isAll:        true,
			wantOrder:    30,
			wantKind:     "tools",
			wantToolName: "read",
			wantTag:      "all",
			wantIsAll:    true,
		},
		{
			name:         "tools fragment with tag",
			key:          "40-tools-write-anthropic",
			content:      "content",
			isAll:        false,
			wantOrder:    40,
			wantKind:     "tools",
			wantToolName: "write",
			wantTag:      "anthropic",
			wantIsAll:    false,
		},
		{
			name:    "invalid no number",
			key:     "system",
			content: "content",
			isAll:   true,
			wantNil: true,
		},
		{
			name:    "invalid number",
			key:     "abc-system",
			content: "content",
			isAll:   true,
			wantNil: true,
		},
		{
			name:    "invalid kind",
			key:     "10-unknown",
			content: "content",
			isAll:   true,
			wantNil: true,
		},
		{
			name:    "invalid tools without toolname",
			key:     "10-tools",
			content: "content",
			isAll:   true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragment := parseFragmentFromKey(tt.key, tt.content, tt.isAll)
			if tt.wantNil {
				assert.Nil(t, fragment)
			} else {
				require.NotNil(t, fragment)
				assert.Equal(t, tt.wantOrder, fragment.order)
				assert.Equal(t, tt.wantKind, fragment.kind)
				assert.Equal(t, tt.wantToolName, fragment.toolName)
				assert.Equal(t, tt.wantTag, fragment.tag)
				assert.Equal(t, tt.wantIsAll, fragment.isAll)
				assert.Equal(t, tt.content, fragment.content)
			}
		})
	}
}

func TestConfPromptGenerator_GetPrompt(t *testing.T) {
	// Import the mock config store
	mockStore := newMockConfigStoreWithFragments()

	// Create mock VFS
	mockVFS := vfs.NewMockVFS()

	// Create generator
	gen, err := NewConfPromptGenerator(mockStore, mockVFS)
	require.NoError(t, err)

	// Load test role
	role := conf.AgentRoleConfig{
		Name: "test1",
		ToolsAccess: map[string]conf.AccessFlag{
			"read":  conf.AccessAllow,
			"write": conf.AccessAllow,
			"bash":  conf.AccessAllow,
		},
		PromptFragments: map[string]string{
			"10-system":     "# Test1 Core Instructions (Overrides all/10-system.md)\n\nYou are an AI assistant for test1 role working in: {{.Info.WorkDir}}",
			"60-system":     "# Test1-Specific Guidelines\n\nFollow test1 guidelines.",
			"70-tools-bash": "# Bash Tool Instructions (Test1)\n\nUse bash carefully in test1.",
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
				"/test/dir",                       // from template expansion
				"Test1 Core Instructions",         // from test1/10-system.md
				"Anthropic-Specific Instructions", // from all/20-system-anthropic.md
				"General Guidelines",              // from all/30-system.md
				"Test1-Specific Guidelines",       // from test1/60-system.md
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
				"Test1-Specific Guidelines",
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
				"Test1-Specific Guidelines",
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

func TestConfPromptGenerator_GetPrompt_Ordering(t *testing.T) {
	// Import the mock config store
	mockStore := newMockConfigStoreWithFragments()

	// Create mock VFS
	mockVFS := vfs.NewMockVFS()

	// Create generator
	gen, err := NewConfPromptGenerator(mockStore, mockVFS)
	require.NoError(t, err)

	// Load test role
	role := conf.AgentRoleConfig{
		Name: "test1",
		ToolsAccess: map[string]conf.AccessFlag{
			"read":  conf.AccessAllow,
			"write": conf.AccessAllow,
			"bash":  conf.AccessAllow,
		},
		PromptFragments: map[string]string{
			"10-system":     "# Test1 Core Instructions (Overrides all/10-system.md)\n\nYou are an AI assistant for test1 role working in: {{.Info.WorkDir}}",
			"60-system":     "# Test1-Specific Guidelines\n\nFollow test1 guidelines.",
			"70-tools-bash": "# Bash Tool Instructions (Test1)\n\nUse bash carefully in test1.",
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

	// Check ordering by finding positions (tools fragments are now excluded)
	markers := []string{
		"Test1 Core Instructions",         // 10
		"Anthropic-Specific Instructions", // 20
		"General Guidelines",              // 30
		"Test1-Specific Guidelines",       // 60
	}

	lastPos := -1
	for _, marker := range markers {
		pos := strings.Index(prompt, marker)
		assert.NotEqual(t, -1, pos, "marker not found: %s", marker)
		assert.Greater(t, pos, lastPos, "marker %s should come after previous marker", marker)
		lastPos = pos
	}

	// Tool instructions should NOT be in the prompt anymore
	assert.NotContains(t, prompt, "Read Tool Instructions")
	assert.NotContains(t, prompt, "Write Tool Instructions")
	assert.NotContains(t, prompt, "Bash Tool Instructions")
}

func TestConfPromptGenerator_GetPrompt_ToolsFiltering(t *testing.T) {
	// Create mock store with fragments
	mockStore := newMockConfigStoreWithFragments()

	// Create mock VFS
	mockVFS := vfs.NewMockVFS()

	// Create generator
	gen, err := NewConfPromptGenerator(mockStore, mockVFS)
	require.NoError(t, err)

	// Test with role that only has read access
	role := conf.AgentRoleConfig{
		Name: "test2",
		ToolsAccess: map[string]conf.AccessFlag{
			"read": conf.AccessAllow,
		},
		PromptFragments: map[string]string{},
	}

	// Create agent state
	state := &AgentState{
		Info: AgentStateCommonInfo{
			WorkDir: "/test/dir",
		},
	}

	prompt, err := gen.GetPrompt([]string{"all"}, &role, state)
	require.NoError(t, err)

	// System prompt should NOT have tool instructions (they're now separate)
	assert.NotContains(t, prompt, "Read Tool Instructions")
	assert.NotContains(t, prompt, "Write Tool Instructions")
	assert.NotContains(t, prompt, "Bash Tool Instructions")

	// Should have core instructions
	assert.Contains(t, prompt, "All Core Instructions")
	assert.Contains(t, prompt, "General Guidelines")
}

// newMockConfigStoreWithFragments creates a mock config store with test fragment data.
func newMockConfigStoreWithFragments() *mockConfigStore {
	return &mockConfigStore{
		roleConfigs: map[string]*conf.AgentRoleConfig{
			"all": {
				Name: "all",
				PromptFragments: map[string]string{
					"10-system":                "# All Core Instructions\n\nYou are an AI assistant working in: {{.Info.WorkDir}}",
					"20-system-anthropic":      "# Anthropic-Specific Instructions\n\nUse Anthropic-specific features.",
					"20-system-openai":         "# OpenAI-Specific Instructions\n\nUse OpenAI-specific features.",
					"30-system":                "# General Guidelines\n\nFollow general guidelines.",
					"40-tools-read":            "# Read Tool Instructions\n\nUse read tool carefully.",
					"50-tools-write-anthropic": "# Write Tool Instructions (Anthropic)\n\nUse write tool with Anthropic.",
				},
			},
		},
	}
}

// mockConfigStore is a simple mock for testing ConfPromptGenerator.
type mockConfigStore struct {
	roleConfigs map[string]*conf.AgentRoleConfig
}

func (m *mockConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	return nil, nil
}

func (m *mockConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockConfigStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
	return m.roleConfigs, nil
}

func (m *mockConfigStore) LastAgentRoleConfigsUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	return nil, nil
}

func (m *mockConfigStore) LastGlobalConfigUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockConfigStore) GetAgentConfigFile(subdir, filename string) ([]byte, error) {
	return nil, nil
}

func TestConfPromptGenerator_GetAgentFiles(t *testing.T) {
	tests := []struct {
		name           string
		setupVFS       func(*vfs.MockVFS)
		dir            string
		wantFiles      map[string]string
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "AGENTS.md exists in directory",
			setupVFS: func(v *vfs.MockVFS) {
				err := v.WriteFile("subdir/AGENTS.md", []byte("# Agent Instructions\n\nSome guidelines here."))
				require.NoError(t, err)
			},
			dir: "subdir",
			wantFiles: map[string]string{
				"subdir/AGENTS.md": "# Agent Instructions\n\nSome guidelines here.",
			},
			wantErr: false,
		},
		{
			name: "AGENTS.md does not exist in directory",
			setupVFS: func(v *vfs.MockVFS) {
				// Don't create any files
			},
			dir:       "subdir",
			wantFiles: map[string]string{},
			wantErr:   false,
		},
		{
			name: "AGENTS.md exists at root",
			setupVFS: func(v *vfs.MockVFS) {
				err := v.WriteFile("AGENTS.md", []byte("# Root Agent Instructions"))
				require.NoError(t, err)
			},
			dir: ".",
			wantFiles: map[string]string{
				"AGENTS.md": "# Root Agent Instructions",
			},
			wantErr: false,
		},
		{
			name: "nested directory with AGENTS.md",
			setupVFS: func(v *vfs.MockVFS) {
				err := v.WriteFile("foo/bar/baz/AGENTS.md", []byte("# Nested Instructions"))
				require.NoError(t, err)
			},
			dir: "foo/bar/baz",
			wantFiles: map[string]string{
				"foo/bar/baz/AGENTS.md": "# Nested Instructions",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock VFS and setup
			mockVFS := vfs.NewMockVFS()
			if tt.setupVFS != nil {
				tt.setupVFS(mockVFS)
			}

			// Create mock store
			mockStore := newMockConfigStoreWithFragments()

			// Create generator
			gen, err := NewConfPromptGenerator(mockStore, mockVFS)
			require.NoError(t, err)

			// Call GetAgentFiles
			files, err := gen.GetAgentFiles(tt.dir)

			// Check error
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMessage != "" {
					assert.Contains(t, err.Error(), tt.wantErrMessage)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantFiles, files)
			}
		})
	}
}

func TestConfPromptGenerator_GetAgentFiles_NoVFS(t *testing.T) {
	// Create mock store
	mockStore := newMockConfigStoreWithFragments()

	// Create generator without VFS
	gen, err := NewConfPromptGenerator(mockStore, nil)
	require.NoError(t, err)

	// Call GetAgentFiles - should return error
	_, err = gen.GetAgentFiles(".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vfs is not initialized")
}

func TestConfPromptGenerator_GetPrompt_WithTopLevelAGENTS(t *testing.T) {
	// Create mock store
	mockStore := newMockConfigStoreWithFragments()

	// Create mock VFS with AGENTS.md at root
	mockVFS := vfs.NewMockVFS()
	err := mockVFS.WriteFile("AGENTS.md", []byte("# Top Level Agent Instructions\n\nThese are project-wide guidelines."))
	require.NoError(t, err)

	// Create generator
	gen, err := NewConfPromptGenerator(mockStore, mockVFS)
	require.NoError(t, err)

	// Load test role
	role := conf.AgentRoleConfig{
		Name: "test1",
		PromptFragments: map[string]string{
			"10-system": "# Test1 Core Instructions",
		},
	}

	// Create agent state
	state := &AgentState{
		Info: AgentStateCommonInfo{
			WorkDir: "/test/dir",
		},
	}

	// Get prompt
	prompt, err := gen.GetPrompt([]string{"all"}, &role, state)
	require.NoError(t, err)

	// Should contain both the role fragment and the AGENTS.md content
	assert.Contains(t, prompt, "Test1 Core Instructions")
	assert.Contains(t, prompt, "Top Level Agent Instructions")
	assert.Contains(t, prompt, "project-wide guidelines")
}

func TestConfPromptGenerator_GetPrompt_WithoutTopLevelAGENTS(t *testing.T) {
	// Create mock store
	mockStore := newMockConfigStoreWithFragments()

	// Create mock VFS without AGENTS.md
	mockVFS := vfs.NewMockVFS()

	// Create generator
	gen, err := NewConfPromptGenerator(mockStore, mockVFS)
	require.NoError(t, err)

	// Load test role
	role := conf.AgentRoleConfig{
		Name: "test1",
		PromptFragments: map[string]string{
			"10-system": "# Test1 Core Instructions",
		},
	}

	// Create agent state
	state := &AgentState{
		Info: AgentStateCommonInfo{
			WorkDir: "/test/dir",
		},
	}

	// Get prompt
	prompt, err := gen.GetPrompt([]string{"all"}, &role, state)
	require.NoError(t, err)

	// Should only contain the role fragment
	assert.Contains(t, prompt, "Test1 Core Instructions")
	assert.NotContains(t, prompt, "Top Level Agent Instructions")
}
