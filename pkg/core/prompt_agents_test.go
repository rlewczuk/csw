package core

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			name: "includes AGENTS.md from parent directories excluding root",
			setupVFS: func(v *vfs.MockVFS) {
				require.NoError(t, v.WriteFile("foo/AGENTS.md", []byte("# Foo Instructions")))
				require.NoError(t, v.WriteFile("foo/bar/AGENTS.md", []byte("# Bar Instructions")))
				require.NoError(t, v.WriteFile("foo/bar/baz/AGENTS.md", []byte("# Baz Instructions")))
				require.NoError(t, v.WriteFile("AGENTS.md", []byte("# Root Instructions")))
			},
			dir: "foo/bar/baz",
			wantFiles: map[string]string{
				"foo/AGENTS.md":         "# Foo Instructions",
				"foo/bar/AGENTS.md":     "# Bar Instructions",
				"foo/bar/baz/AGENTS.md": "# Baz Instructions",
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
			name: "project root AGENTS.md is excluded",
			setupVFS: func(v *vfs.MockVFS) {
				err := v.WriteFile("AGENTS.md", []byte("# Root Agent Instructions"))
				require.NoError(t, err)
			},
			dir:       ".",
			wantFiles: map[string]string{},
			wantErr:   false,
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
