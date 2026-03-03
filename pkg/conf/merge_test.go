package conf

import (
	"testing"
	"time"

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
					"safe":  {"runBash": true},
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
			name: "replaces mounts env image and enables container",
			base: ContainerConfig{Mounts: []string{"a"}, Env: []string{"A=1"}, Image: "img1", Enabled: false},
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
	base := CLIDefaultsConfig{
		Model:          "provider1/model",
		Worktree:       "feature/one",
		Merge:          false,
		LogLLMRequests: false,
		Thinking:       "medium",
		LSPServer:      "first",
		GitUserName:    "Base User",
		GitUserEmail:   "base@example.com",
	}
	override := CLIDefaultsConfig{
		Model:          "provider2/model",
		Merge:          true,
		LogLLMRequests: true,
		LSPServer:      "second",
		GitUserName:    "Override User",
		GitUserEmail:   "override@example.com",
	}

	base.MergeFrom(override)

	assert.Equal(t, CLIDefaultsConfig{
		Model:          "provider2/model",
		Worktree:       "feature/one",
		Merge:          true,
		LogLLMRequests: true,
		Thinking:       "medium",
		LSPServer:      "second",
		GitUserName:    "Override User",
		GitUserEmail:   "override@example.com",
	}, base)
}

func TestCLIDefaultsConfig_MergeFrom_GitIdentity(t *testing.T) {
	tests := []struct {
		name           string
		base           CLIDefaultsConfig
		override       CLIDefaultsConfig
		expectedName   string
		expectedEmail  string
	}{
		{
			name:           "override sets git identity",
			base:           CLIDefaultsConfig{},
			override:       CLIDefaultsConfig{GitUserName: "New User", GitUserEmail: "new@example.com"},
			expectedName:   "New User",
			expectedEmail:  "new@example.com",
		},
		{
			name:           "empty override keeps base git identity",
			base:           CLIDefaultsConfig{GitUserName: "Base User", GitUserEmail: "base@example.com"},
			override:       CLIDefaultsConfig{},
			expectedName:   "Base User",
			expectedEmail:  "base@example.com",
		},
		{
			name:           "partial override only updates provided fields",
			base:           CLIDefaultsConfig{GitUserName: "Base User", GitUserEmail: "base@example.com"},
			override:       CLIDefaultsConfig{GitUserName: "New User"},
			expectedName:   "New User",
			expectedEmail:  "base@example.com",
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
		DefaultProvider:            "provider1",
		DefaultRole:                "role1",
		LLMRetryMaxAttempts:        5,
		LLMRetryMaxBackoffSeconds:  30,
		Container: ContainerConfig{
			Image:  "image1",
			Mounts: []string{"/a:/b"},
			Env:    []string{"A=1"},
		},
		Defaults: CLIDefaultsConfig{Model: "m1", Worktree: "w1", Thinking: "low"},
	}

	override := &GlobalConfig{
		ModelTags: []ModelTagMapping{{Model: "claude-.*", Tag: "anthropic"}},
		ToolSelection: ToolSelectionConfig{
			Default: map[string]bool{"runBash": true, "vfsEdit": false},
			Tags:    map[string]map[string]bool{"safe": {"runBash": false}},
		},
		ContextCompactionThreshold: 0.95,
		DefaultProvider:            "provider2",
		DefaultRole:                "role2",
		LLMRetryMaxAttempts:        10,
		LLMRetryMaxBackoffSeconds:  60,
		Container: ContainerConfig{
			Enabled: true,
			Image:   "image2",
			Mounts:  []string{"/c:/d"},
			Env:     []string{"B=2"},
		},
		Defaults: CLIDefaultsConfig{Model: "m2", Merge: true, LogLLMRequests: true, Thinking: "high", LSPServer: "lsp2"},
	}

	base.Merge(override)

	assert.Equal(t, []ModelTagMapping{{Model: "gpt-.*", Tag: "openai"}, {Model: "claude-.*", Tag: "anthropic"}}, base.ModelTags)
	assert.Equal(t, map[string]bool{"runBash": true, "vfsEdit": false}, base.ToolSelection.Default)
	assert.Equal(t, map[string]bool{"vfsDelete": false, "runBash": false}, base.ToolSelection.Tags["safe"])
	assert.Equal(t, 0.95, base.ContextCompactionThreshold)
	assert.Equal(t, "provider2", base.DefaultProvider)
	assert.Equal(t, "role2", base.DefaultRole)
	assert.Equal(t, 10, base.LLMRetryMaxAttempts)
	assert.Equal(t, 60, base.LLMRetryMaxBackoffSeconds)
	assert.Equal(t, ContainerConfig{Enabled: true, Image: "image2", Mounts: []string{"/c:/d"}, Env: []string{"B=2"}}, base.Container)
	assert.Equal(t, CLIDefaultsConfig{Model: "m2", Worktree: "w1", Merge: true, LogLLMRequests: true, Thinking: "high", LSPServer: "lsp2"}, base.Defaults)
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
			Container: ContainerConfig{Mounts: []string{"a"}, Env: []string{"A=1"}},
		}

		clone := cfg.Clone()
		require.NotNil(t, clone)

		clone.ModelTags[0].Tag = "changed"
		clone.ToolSelection.Default["runBash"] = false
		clone.ToolSelection.Tags["safe"]["vfsRead"] = false
		clone.Container.Mounts[0] = "b"

		assert.Equal(t, "all", cfg.ModelTags[0].Tag)
		assert.True(t, cfg.ToolSelection.Default["runBash"])
		assert.True(t, cfg.ToolSelection.Tags["safe"]["vfsRead"])
		assert.Equal(t, "a", cfg.Container.Mounts[0])
	})

	t.Run("provider clone is deep copy", func(t *testing.T) {
		streaming := true
		temperature := true
		experimental := true
		cfg := &ModelProviderConfig{
			Name:         "provider",
			ModelTags:    []ModelTagMapping{{Model: ".*", Tag: "x"}},
			Streaming:    &streaming,
			Temperature:  &temperature,
			Experimental: &experimental,
			Reasoning:    map[string]string{"low": "minimal"},
			Headers:      map[string]string{"X": "1"},
			QueryParams:  map[string]string{"q": "1"},
			Options:      map[string]any{"a": 1},
			Cost:         []ModelProviderCost{{Context: 0, Input: 1.0}, {Context: 200000, Input: 2.0}},
		}

		clone := cfg.Clone()
		require.NotNil(t, clone)

		clone.ModelTags[0].Tag = "y"
		*clone.Streaming = false
		*clone.Temperature = false
		*clone.Experimental = false
		clone.Reasoning["low"] = "changed"
		clone.Headers["X"] = "2"
		clone.QueryParams["q"] = "2"
		clone.Options["a"] = 2
		clone.Cost[0].Input = 99

		assert.Equal(t, "x", cfg.ModelTags[0].Tag)
		assert.True(t, *cfg.Streaming)
		assert.True(t, *cfg.Temperature)
		assert.True(t, *cfg.Experimental)
		assert.Equal(t, "minimal", cfg.Reasoning["low"])
		assert.Equal(t, "1", cfg.Headers["X"])
		assert.Equal(t, "1", cfg.QueryParams["q"])
		assert.Equal(t, 1, cfg.Options["a"])
		assert.Equal(t, 1.0, cfg.Cost[0].Input)
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
	base := &GlobalConfig{DefaultProvider: "provider"}
	base.Merge(nil)
	assert.Equal(t, "provider", base.DefaultProvider)
}

func TestAgentRoleConfig_Merge_NilOverride(t *testing.T) {
	base := &AgentRoleConfig{Name: "role", HiddenPatterns: []string{".git/"}}
	base.Merge(nil)
	assert.Equal(t, []string{".git/"}, base.HiddenPatterns)
}

func TestModelProviderConfig_Clone_NilReceiver(t *testing.T) {
	var cfg *ModelProviderConfig
	assert.Nil(t, cfg.Clone())
}

func TestGlobalConfig_Clone_NilReceiver(t *testing.T) {
	var cfg *GlobalConfig
	assert.Nil(t, cfg.Clone())
}

func TestAgentRoleConfig_Clone_NilReceiver(t *testing.T) {
	var cfg *AgentRoleConfig
	assert.Nil(t, cfg.Clone())
}

func TestModelProviderConfig_Clone_PreservesDurations(t *testing.T) {
	cfg := &ModelProviderConfig{
		ConnectTimeout:        5 * time.Second,
		RequestTimeout:        15 * time.Second,
		RateLimitBackoffScale: 2 * time.Second,
	}

	clone := cfg.Clone()
	require.NotNil(t, clone)
	assert.Equal(t, 5*time.Second, clone.ConnectTimeout)
	assert.Equal(t, 15*time.Second, clone.RequestTimeout)
	assert.Equal(t, 2*time.Second, clone.RateLimitBackoffScale)
}

func TestModelProviderConfig_Merge(t *testing.T) {
	streaming := true
	streamingOverride := false
	base := &ModelProviderConfig{
		Type:      "openai",
		Name:      "base",
		URL:       "https://base",
		ModelTags: []ModelTagMapping{{Model: "^gpt", Tag: "openai"}},
		Cost:      []ModelProviderCost{{Context: 0, Input: 1.0, Output: 2.0}},
		Reasoning: map[string]string{"low": "minimal"},
		Headers:   map[string]string{"X-A": "1"},
		Streaming: &streaming,
	}
	override := &ModelProviderConfig{
		URL:       "https://override",
		ModelTags: []ModelTagMapping{{Model: "^gpt", Tag: "general"}, {Model: "^o", Tag: "reasoning"}},
		Cost: []ModelProviderCost{
			{Context: 0, Input: 1.5},
			{Context: 200000, Input: 3.0, Output: 4.0},
		},
		Reasoning: map[string]string{"high": "deep"},
		Headers:   map[string]string{"X-B": "2"},
		Streaming: &streamingOverride,
	}

	base.Merge(override)

	assert.Equal(t, "https://override", base.URL)
	assert.Len(t, base.ModelTags, 2)
	assert.Equal(t, "general", base.ModelTags[0].Tag)
	assert.Equal(t, "reasoning", base.ModelTags[1].Tag)
	assert.Len(t, base.Cost, 2)
	assert.Equal(t, 0, base.Cost[0].Context)
	assert.Equal(t, 1.5, base.Cost[0].Input)
	assert.Equal(t, 200000, base.Cost[1].Context)
	assert.Equal(t, "minimal", base.Reasoning["low"])
	assert.Equal(t, "deep", base.Reasoning["high"])
	assert.Equal(t, "1", base.Headers["X-A"])
	assert.Equal(t, "2", base.Headers["X-B"])
	assert.False(t, *base.Streaming)
}
