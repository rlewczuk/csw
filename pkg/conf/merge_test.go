package conf

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolSelectionConfig_Merge(t *testing.T) {
	tests := []struct {
		name     string
		base     ToolSelectionConfig
		override ToolSelectionConfig
		expected ToolSelectionConfig
	}{
		{
			name: "merges defaults and tags with override precedence",
			base: ToolSelectionConfig{
				Default: map[string]bool{"runBash": false, "vfsRead": true},
				Tags: map[string]map[string]bool{
					"safe": {"runBash": false, "vfsDelete": false},
				},
			},
			override: ToolSelectionConfig{
				Default: map[string]bool{"runBash": true, "vfsEdit": false},
				Tags: map[string]map[string]bool{
					"safe":   {"runBash": true},
					"strict": {"vfsMove": false},
				},
			},
			expected: ToolSelectionConfig{
				Default: map[string]bool{"runBash": true, "vfsRead": true, "vfsEdit": false},
				Tags: map[string]map[string]bool{
					"safe":   {"runBash": true, "vfsDelete": false},
					"strict": {"vfsMove": false},
				},
			},
		},
		{
			name: "initializes nil maps",
			base: ToolSelectionConfig{},
			override: ToolSelectionConfig{
				Default: map[string]bool{"runBash": false},
				Tags: map[string]map[string]bool{
					"safe": {"vfsDelete": false},
				},
			},
			expected: ToolSelectionConfig{
				Default: map[string]bool{"runBash": false},
				Tags: map[string]map[string]bool{
					"safe": {"vfsDelete": false},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := tt.base
			base.Merge(tt.override)
			assert.Equal(t, tt.expected, base)
		})
	}
}

func TestContainerConfig_Merge(t *testing.T) {
	tests := []struct {
		name     string
		base     ContainerConfig
		override ContainerConfig
		expected ContainerConfig
	}{
		{
			name:     "replaces mounts env image and enables container",
			base:     ContainerConfig{Mounts: []string{"a"}, Env: []string{"A=1"}, Image: "img1", Enabled: false},
			override: ContainerConfig{Mounts: []string{"b"}, Env: []string{"B=2"}, Image: "img2", Enabled: true},
			expected: ContainerConfig{Mounts: []string{"b"}, Env: []string{"B=2"}, Image: "img2", Enabled: true},
		},
		{
			name:     "keeps existing values when override is zero-value",
			base:     ContainerConfig{Mounts: []string{"a"}, Env: []string{"A=1"}, Image: "img1", Enabled: true},
			override: ContainerConfig{},
			expected: ContainerConfig{Mounts: []string{"a"}, Env: []string{"A=1"}, Image: "img1", Enabled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := tt.base
			base.Merge(tt.override)
			assert.Equal(t, tt.expected, base)
		})
	}
}

func TestCLIDefaultsConfig_MergeFrom(t *testing.T) {
	base := RunParameters{
		DefaultProvider:   "provider1",
		DefaultRole:       "role1",
		Container:         &ContainerConfig{Image: "image1", Mounts: []string{"/a:/b"}, Env: []string{"A=1"}},
		Model:             "provider1/model",
		Workdir:           "/base/workdir",
		Worktree:          "feature/one",
		Merge:             false,
		NoCommit:          false,
		LogLLMRequests:    false,
		LogLLMRequestsRaw: false,
		Thinking:          "medium",
		LSPServer:         "first",
		GitUserName:       "Base User",
		GitUserEmail:      "base@example.com",
		MaxThreads:        4,
		TaskDir:           ".cswdata/tasks",
		ShadowDir:         "shadow/base",
		VFSAllow:          []string{"/base/allow"},
		RunBashMax:        intPtr(256),
		VfsReadLimit:      intPtr(384),
	}
	override := RunParameters{
		DefaultProvider:     "provider2",
		DefaultRole:         "role2",
		Container:           &ContainerConfig{Enabled: true, Image: "image2", Mounts: []string{"/c:/d"}, Env: []string{"B=2"}},
		Model:               "provider2/model",
		Workdir:             "/override/workdir",
		Merge:               true,
		NoCommit:            true,
		LogLLMRequests:      true,
		LogLLMRequestsRaw:   true,
		LSPServer:           "second",
		GitUserName:         "Override User",
		GitUserEmail:        "override@example.com",
		MaxThreads:          12,
		TaskDir:             "custom/tasks",
		ShadowDir:           "shadow/override",
		AllowAllPermissions: true,
		VFSAllow:            []string{"/override/allow1", "/override/allow2"},
		RunBashMax:          intPtr(0),
		VfsReadLimit:        intPtr(128),
	}

	base.MergeFrom(override)

	assert.Equal(t, RunParameters{
		DefaultProvider:     "provider2",
		DefaultRole:         "role2",
		Container:           &ContainerConfig{Enabled: true, Image: "image2", Mounts: []string{"/c:/d"}, Env: []string{"B=2"}},
		Model:               "provider2/model",
		Workdir:             "/override/workdir",
		Worktree:            "feature/one",
		Merge:               true,
		NoCommit:            true,
		LogLLMRequests:      true,
		LogLLMRequestsRaw:   true,
		Thinking:            "medium",
		LSPServer:           "second",
		GitUserName:         "Override User",
		GitUserEmail:        "override@example.com",
		MaxThreads:          12,
		TaskDir:             "custom/tasks",
		ShadowDir:           "shadow/override",
		AllowAllPermissions: true,
		VFSAllow:            []string{"/override/allow1", "/override/allow2"},
		RunBashMax:          intPtr(0),
		VfsReadLimit:        intPtr(128),
	}, base)
}

func TestCLIDefaultsConfig_MergeFrom_EmptyWorkdirKeepsBase(t *testing.T) {
	base := RunParameters{Workdir: "/base/workdir"}
	override := RunParameters{}

	base.MergeFrom(override)

	assert.Equal(t, "/base/workdir", base.Workdir)
}

func TestCLIDefaultsConfig_MergeFrom_GitIdentity(t *testing.T) {
	tests := []struct {
		name          string
		base          RunParameters
		override      RunParameters
		expectedName  string
		expectedEmail string
	}{
		{
			name:          "override sets git identity",
			base:          RunParameters{},
			override:      RunParameters{GitUserName: "New User", GitUserEmail: "new@example.com"},
			expectedName:  "New User",
			expectedEmail: "new@example.com",
		},
		{
			name:          "empty override keeps base git identity",
			base:          RunParameters{GitUserName: "Base User", GitUserEmail: "base@example.com"},
			override:      RunParameters{},
			expectedName:  "Base User",
			expectedEmail: "base@example.com",
		},
		{
			name:          "partial override only updates provided fields",
			base:          RunParameters{GitUserName: "Base User", GitUserEmail: "base@example.com"},
			override:      RunParameters{GitUserName: "New User"},
			expectedName:  "New User",
			expectedEmail: "base@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.MergeFrom(tt.override)
			assert.Equal(t, tt.expectedName, tt.base.GitUserName)
			assert.Equal(t, tt.expectedEmail, tt.base.GitUserEmail)
		})
	}
}

func TestGlobalConfig_Merge(t *testing.T) {
	base := &GlobalConfig{
		ModelTags: []ModelTagMapping{{Model: "gpt-.*", Tag: "openai"}},
		ToolSelection: ToolSelectionConfig{
			Default: map[string]bool{"runBash": false},
			Tags:    map[string]map[string]bool{"safe": {"vfsDelete": false}},
		},
		ContextCompactionThreshold: 0.7,
		LLMRetryMaxAttempts:        5,
		LLMRetryMaxBackoffSeconds:  30,
		Parameters: RunParameters{
			DefaultProvider: "provider1",
			DefaultRole:     "role1",
			MaxThreads:      3,
			Container:       &ContainerConfig{Image: "image1", Mounts: []string{"/a:/b"}, Env: []string{"A=1"}},
			Model:           "m1",
			Worktree:        "w1",
			Thinking:        "low",
			TaskDir:         ".cswdata/tasks",
			ShadowDir:       "shadow/base",
			VFSAllow:        []string{"/base/allow"},
			RunBashMax:      intPtr(128),
			VfsReadLimit:    intPtr(384),
		},
		ShadowPaths: []string{"AGENTS.md"},
	}

	override := &GlobalConfig{
		ModelTags: []ModelTagMapping{{Model: "claude-.*", Tag: "anthropic"}},
		ToolSelection: ToolSelectionConfig{
			Default: map[string]bool{"runBash": true, "vfsEdit": false},
			Tags:    map[string]map[string]bool{"safe": {"runBash": false}},
		},
		ContextCompactionThreshold: 0.95,
		LLMRetryMaxAttempts:        10,
		LLMRetryMaxBackoffSeconds:  60,
		Parameters: RunParameters{
			DefaultProvider:     "provider2",
			DefaultRole:         "role2",
			MaxThreads:          12,
			Container:           &ContainerConfig{Enabled: true, Image: "image2", Mounts: []string{"/c:/d"}, Env: []string{"B=2"}},
			Model:               "m2",
			Merge:               true,
			LogLLMRequests:      true,
			LogLLMRequestsRaw:   true,
			Thinking:            "high",
			LSPServer:           "lsp2",
			TaskDir:             "custom/tasks",
			ShadowDir:           "shadow/override",
			AllowAllPermissions: true,
			VFSAllow:            []string{"/override/allow"},
			RunBashMax:          intPtr(0),
			VfsReadLimit:        intPtr(256),
		},
		ShadowPaths: []string{".cswdata/**", ".agents/**"},
	}

	base.Merge(override)

	assert.Equal(t, []ModelTagMapping{{Model: "gpt-.*", Tag: "openai"}, {Model: "claude-.*", Tag: "anthropic"}}, base.ModelTags)
	assert.Equal(t, map[string]bool{"runBash": true, "vfsEdit": false}, base.ToolSelection.Default)
	assert.Equal(t, map[string]bool{"vfsDelete": false, "runBash": false}, base.ToolSelection.Tags["safe"])
	assert.Equal(t, 0.95, base.ContextCompactionThreshold)
	assert.Equal(t, "provider2", base.Parameters.DefaultProvider)
	assert.Equal(t, "role2", base.Parameters.DefaultRole)
	assert.Equal(t, 10, base.LLMRetryMaxAttempts)
	assert.Equal(t, 60, base.LLMRetryMaxBackoffSeconds)
	assert.Equal(t, 12, base.Parameters.MaxThreads)
	require.NotNil(t, base.Parameters.Container)
	assert.Equal(t, ContainerConfig{Enabled: true, Image: "image2", Mounts: []string{"/c:/d"}, Env: []string{"B=2"}}, *base.Parameters.Container)
	assert.Equal(t, RunParameters{DefaultProvider: "provider2", DefaultRole: "role2", MaxThreads: 12, Container: &ContainerConfig{Enabled: true, Image: "image2", Mounts: []string{"/c:/d"}, Env: []string{"B=2"}}, Model: "m2", Worktree: "w1", Merge: true, LogLLMRequests: true, LogLLMRequestsRaw: true, Thinking: "high", LSPServer: "lsp2", TaskDir: "custom/tasks", ShadowDir: "shadow/override", AllowAllPermissions: true, VFSAllow: []string{"/override/allow"}, RunBashMax: intPtr(0), VfsReadLimit: intPtr(256)}, base.Parameters)
	assert.Equal(t, []string{".cswdata/**", ".agents/**"}, base.ShadowPaths)
}

func TestGlobalConfig_Merge_ContainerFalseDoesNotOverride(t *testing.T) {
	base := &GlobalConfig{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"parameters": {
			"container": {
			"enabled": true,
			"image": "base-image",
			"env": ["A=1"],
			"mounts": ["/a:/b"]
			}
		}
	}`), base))

	override := &GlobalConfig{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"parameters": {
			"container": {
			"enabled": false
			}
		}
	}`), override))

	base.Merge(override)

	require.NotNil(t, base.Parameters.Container)
	assert.True(t, base.Parameters.Container.Enabled)
	assert.Equal(t, "base-image", base.Parameters.Container.Image)
	assert.Equal(t, []string{"A=1"}, base.Parameters.Container.Env)
	assert.Equal(t, []string{"/a:/b"}, base.Parameters.Container.Mounts)
}

func TestAgentRoleConfig_Merge(t *testing.T) {
	base := &AgentRoleConfig{
		Name:        "role",
		Description: "first",
		VFSPrivileges: map[string]FileAccess{
			"/": {Read: AccessAllow, Write: AccessDeny},
		},
		PromptFragments: map[string]string{
			"a": "a1",
			"b": "b1",
		},
		ToolFragments: map[string]string{
			"x.md": "x1",
		},
		HiddenPatterns: []string{".git/"},
	}

	override := &AgentRoleConfig{
		Name:        "role",
		Description: "second",
		VFSPrivileges: map[string]FileAccess{
			"/": {Read: AccessAllow, Write: AccessAllow},
		},
		PromptFragments: map[string]string{
			"a": "", // remove
			"c": "c2",
		},
		ToolFragments: map[string]string{
			"x.md": "   ", // remove
			"y.md": "y2",
		},
		HiddenPatterns: []string{"tmp/"},
	}

	base.Merge(override)

	assert.Equal(t, "second", base.Description)
	assert.Equal(t, AccessAllow, base.VFSPrivileges["/"].Write)
	assert.Equal(t, map[string]string{"b": "b1", "c": "c2"}, base.PromptFragments)
	assert.Equal(t, map[string]string{"y.md": "y2"}, base.ToolFragments)
	assert.Equal(t, []string{".git/", "tmp/"}, base.HiddenPatterns)
}

func TestConfigCloneMethods_DeepCopy(t *testing.T) {
	t.Run("global clone is deep copy", func(t *testing.T) {
		cfg := &GlobalConfig{
			ModelTags: []ModelTagMapping{{Model: ".*", Tag: "all"}},
			ToolSelection: ToolSelectionConfig{
				Default: map[string]bool{"runBash": true},
				Tags:    map[string]map[string]bool{"safe": {"vfsRead": true}},
			},
			Parameters: RunParameters{Container: &ContainerConfig{Mounts: []string{"a"}, Env: []string{"A=1"}}, VFSAllow: []string{"/a", "/b"}, RunBashMax: intPtr(512), VfsReadLimit: intPtr(384)},
		}

		clone := cfg.Clone()
		require.NotNil(t, clone)

		clone.ModelTags[0].Tag = "changed"
		clone.ToolSelection.Default["runBash"] = false
		clone.ToolSelection.Tags["safe"]["vfsRead"] = false
		require.NotNil(t, clone.Parameters.Container)
		require.NotNil(t, cfg.Parameters.Container)
		clone.Parameters.Container.Mounts[0] = "b"
		clone.Parameters.VFSAllow[0] = "/changed"
		*clone.Parameters.RunBashMax = 128
		*clone.Parameters.VfsReadLimit = 64

		assert.Equal(t, "all", cfg.ModelTags[0].Tag)
		assert.True(t, cfg.ToolSelection.Default["runBash"])
		assert.True(t, cfg.ToolSelection.Tags["safe"]["vfsRead"])
		assert.Equal(t, "a", cfg.Parameters.Container.Mounts[0])
		assert.Equal(t, "/a", cfg.Parameters.VFSAllow[0])
		require.NotNil(t, cfg.Parameters.RunBashMax)
		require.NotNil(t, cfg.Parameters.VfsReadLimit)
		assert.Equal(t, 512, *cfg.Parameters.RunBashMax)
		assert.Equal(t, 384, *cfg.Parameters.VfsReadLimit)
	})

	t.Run("agent role clone is deep copy", func(t *testing.T) {
		cfg := &AgentRoleConfig{
			VFSPrivileges:   map[string]FileAccess{"/": {Read: AccessAllow}},
			ToolsAccess:     map[string]AccessFlag{"vfsRead": AccessAllow},
			RunPrivileges:   map[string]AccessFlag{".*": AccessAsk},
			PromptFragments: map[string]string{"a": "1"},
			ToolFragments:   map[string]string{"x": "1"},
			HiddenPatterns:  []string{".git/"},
		}

		clone := cfg.Clone()
		require.NotNil(t, clone)

		clone.VFSPrivileges["/"] = FileAccess{Read: AccessDeny}
		clone.ToolsAccess["vfsRead"] = AccessDeny
		clone.RunPrivileges[".*"] = AccessDeny
		clone.PromptFragments["a"] = "2"
		clone.ToolFragments["x"] = "2"
		clone.HiddenPatterns[0] = "tmp/"

		assert.Equal(t, AccessAllow, cfg.VFSPrivileges["/"].Read)
		assert.Equal(t, AccessAllow, cfg.ToolsAccess["vfsRead"])
		assert.Equal(t, AccessAsk, cfg.RunPrivileges[".*"])
		assert.Equal(t, "1", cfg.PromptFragments["a"])
		assert.Equal(t, "1", cfg.ToolFragments["x"])
		assert.Equal(t, ".git/", cfg.HiddenPatterns[0])
	})

	t.Run("tool selection clone is deep copy", func(t *testing.T) {
		cfg := ToolSelectionConfig{
			Default: map[string]bool{"runBash": true},
			Tags:    map[string]map[string]bool{"safe": {"vfsRead": true}},
		}

		clone := cfg.Clone()
		clone.Default["runBash"] = false
		clone.Tags["safe"]["vfsRead"] = false

		assert.True(t, cfg.Default["runBash"])
		assert.True(t, cfg.Tags["safe"]["vfsRead"])
	})

}

func TestGlobalConfig_Merge_NilOverride(t *testing.T) {
	base := &GlobalConfig{Parameters: RunParameters{DefaultProvider: "provider"}}
	base.Merge(nil)
	assert.Equal(t, "provider", base.Parameters.DefaultProvider)
}

func intPtr(value int) *int {
	return &value
}

func TestAgentRoleConfig_Merge_NilOverride(t *testing.T) {
	base := &AgentRoleConfig{Name: "role", HiddenPatterns: []string{".git/"}}
	base.Merge(nil)
	assert.Equal(t, []string{".git/"}, base.HiddenPatterns)
}

func TestAgentRoleConfig_Merge_Aliases(t *testing.T) {
	base := &AgentRoleConfig{Name: "developer", Aliases: []string{"dev"}}
	override := &AgentRoleConfig{Name: "developer", Aliases: []string{"build", "coder"}}

	base.Merge(override)

	assert.Equal(t, []string{"build", "coder"}, base.Aliases)
}

func TestAgentRoleConfig_Clone_Aliases(t *testing.T) {
	original := &AgentRoleConfig{Name: "developer", Aliases: []string{"dev", "build"}}
	cloned := original.Clone()

	require.NotNil(t, cloned)
	assert.Equal(t, original.Aliases, cloned.Aliases)

	cloned.Aliases[0] = "changed"
	assert.Equal(t, []string{"dev", "build"}, original.Aliases)
}

func TestGlobalConfig_Clone_NilReceiver(t *testing.T) {
	var cfg *GlobalConfig
	assert.Nil(t, cfg.Clone())
}

func TestAgentRoleConfig_Clone_NilReceiver(t *testing.T) {
	var cfg *AgentRoleConfig
	assert.Nil(t, cfg.Clone())
}
