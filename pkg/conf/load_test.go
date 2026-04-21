package conf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCswConfigLoad(t *testing.T) {
	t.Parallel()

	t.Run("loads and merges global config from local directories", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()

		firstDir := filepath.Join(baseDir, "first")
		require.NoError(t, os.MkdirAll(firstDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(firstDir, "global.json"), []byte(`{
			"llm-retry-max-attempts": 11,
			"defaults": {
				"default-provider": "prov-a",
				"model": "prov-a/model-a"
			}
		}`), 0o644))

		secondDir := filepath.Join(baseDir, "second")
		require.NoError(t, os.MkdirAll(secondDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "global.json"), []byte(`{
			"llm-retry-max-attempts": 22,
			"defaults": {
				"model": "prov-b/model-b"
			}
		}`), 0o644))

		cfg, err := CswConfigLoad(firstDir + ":" + secondDir)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.GlobalConfig)
		require.Equal(t, 22, cfg.GlobalConfig.LLMRetryMaxAttempts)
		require.Equal(t, "prov-a", cfg.GlobalConfig.Defaults.DefaultProvider)
		require.Equal(t, "prov-b/model-b", cfg.GlobalConfig.Defaults.Model)
		require.Empty(t, cfg.AgentRoleConfigs)
		require.Empty(t, cfg.ModelProviderConfigs)
		require.Empty(t, cfg.AgentConfigFiles)
		require.Empty(t, cfg.ModelAliases)
	})

	t.Run("loads and overrides model providers by source order", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()

		firstDir := filepath.Join(baseDir, "first")
		require.NoError(t, os.MkdirAll(filepath.Join(firstDir, "models"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(firstDir, "models", "provider-a.json"), []byte(`{
			"type": "openai",
			"url": "https://first.example"
		}`), 0o644))

		secondDir := filepath.Join(baseDir, "second")
		require.NoError(t, os.MkdirAll(filepath.Join(secondDir, "models"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "models", "provider-a.json"), []byte(`{
			"type": "openai",
			"url": "https://second.example"
		}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "models", "provider-b.json"), []byte(`{
			"type": "anthropic",
			"url": "https://provider-b.example"
		}`), 0o644))

		cfg, err := CswConfigLoad(firstDir + ":" + secondDir)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Len(t, cfg.ModelProviderConfigs, 2)
		require.Equal(t, "https://second.example", cfg.ModelProviderConfigs["provider-a"].URL)
		require.Equal(t, "provider-a", cfg.ModelProviderConfigs["provider-a"].Name)
		require.Equal(t, "https://provider-b.example", cfg.ModelProviderConfigs["provider-b"].URL)
		require.Equal(t, "provider-b", cfg.ModelProviderConfigs["provider-b"].Name)
	})

	t.Run("loads and overrides model aliases from jsonl", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()

		firstDir := filepath.Join(baseDir, "first")
		require.NoError(t, os.MkdirAll(firstDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(firstDir, "model_aliases.jsonl"), []byte(`{
			"fast": "provider-a/model-a",
			"review": ["provider-a/model-b", "provider-a/model-c"]
		}`), 0o644))

		secondDir := filepath.Join(baseDir, "second")
		require.NoError(t, os.MkdirAll(secondDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "model_aliases.jsonl"), []byte(`{"fast":"provider-b/model-x"}`), 0o644))

		cfg, err := CswConfigLoad(firstDir + ":" + secondDir)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Len(t, cfg.ModelAliases, 2)
		require.Equal(t, []string{"provider-b/model-x"}, cfg.ModelAliases["fast"].Values)
		require.Equal(t, []string{"provider-a/model-b", "provider-a/model-c"}, cfg.ModelAliases["review"].Values)
	})

	t.Run("loads and merges agent roles with fragment and hidden-pattern behavior", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()

		firstDir := filepath.Join(baseDir, "first")
		require.NoError(t, os.MkdirAll(filepath.Join(firstDir, "roles", "dev"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(firstDir, "roles", "dev", "config.json"), []byte(`{
			"name": "dev",
			"description": "first",
			"hidden-patterns": ["first/**"]
		}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(firstDir, "roles", "dev", "base.md"), []byte("first-base"), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(firstDir, "tools", "vfsRead"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(firstDir, "tools", "vfsRead", "vfsRead.md"), []byte("first-tool"), 0o644))

		secondDir := filepath.Join(baseDir, "second")
		require.NoError(t, os.MkdirAll(filepath.Join(secondDir, "roles", "dev"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "roles", "dev", "config.json"), []byte(`{
			"name": "dev",
			"description": "second",
			"hidden-patterns": ["second/**"]
		}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "roles", "dev", "base.md"), []byte("second-base"), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(secondDir, "tools", "vfsRead"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "tools", "vfsRead", "vfsRead.md"), []byte("second-tool"), 0o644))

		cfg, err := CswConfigLoad(firstDir + ":" + secondDir)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Contains(t, cfg.AgentRoleConfigs, "dev")
		require.Equal(t, "second", cfg.AgentRoleConfigs["dev"].Description)
		require.Equal(t, "second-base", cfg.AgentRoleConfigs["dev"].PromptFragments["base"])
		require.Equal(t, "second-tool", cfg.AgentRoleConfigs["dev"].ToolFragments["vfsRead/vfsRead.md"])
		require.Equal(t, []string{"first/**", "second/**"}, cfg.AgentRoleConfigs["dev"].HiddenPatterns)
	})

	t.Run("loads and overrides agent config files by source order", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()

		firstDir := filepath.Join(baseDir, "first")
		require.NoError(t, os.MkdirAll(filepath.Join(firstDir, "agent", "commit"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(firstDir, "agent", "commit", "prompt.md"), []byte("first-commit-prompt"), 0o644))

		secondDir := filepath.Join(baseDir, "second")
		require.NoError(t, os.MkdirAll(filepath.Join(secondDir, "agent", "commit"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(secondDir, "agent", "review"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "agent", "commit", "prompt.md"), []byte("second-commit-prompt"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(secondDir, "agent", "review", "prompt.md"), []byte("review-prompt"), 0o644))

		cfg, err := CswConfigLoad(firstDir + ":" + secondDir)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Equal(t, "second-commit-prompt", cfg.AgentConfigFiles["commit"]["prompt.md"])
		require.Equal(t, "review-prompt", cfg.AgentConfigFiles["review"]["prompt.md"])
	})

	t.Run("loads defaults marker", func(t *testing.T) {
		t.Parallel()

		cfg, err := CswConfigLoad("@DEFAULTS")

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.GlobalConfig)
	})

	t.Run("loads embedded defaults and allows later override", func(t *testing.T) {
		t.Parallel()
		overrideDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "global.json"), []byte(`{
			"llm-retry-max-attempts": 123
		}`), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(overrideDir, "roles", "developer"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "roles", "developer", "config.json"), []byte(`{
			"name": "developer",
			"description": "custom developer"
		}`), 0o644))

		cfg, err := CswConfigLoad("@DEFAULTS:" + overrideDir)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.GlobalConfig)
		require.Equal(t, 123, cfg.GlobalConfig.LLMRetryMaxAttempts)
		require.Contains(t, cfg.AgentRoleConfigs, "developer")
		require.Equal(t, "custom developer", cfg.AgentRoleConfigs["developer"].Description)
	})
}

func TestParseConfigLoadPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		path  string
		want  []string
	}{
		{
			name: "defaults for empty path",
			path: "",
			want: []string{"@DEFAULTS", "~/.config/csw", ".csw"},
		},
		{
			name: "defaults for whitespace path",
			path: "   ",
			want: []string{"@DEFAULTS", "~/.config/csw", ".csw"},
		},
		{
			name: "splits and trims and skips empty entries",
			path: "@DEFAULTS:  /a/b  ::@PROJ/.csw/config:",
			want: []string{"@DEFAULTS", "/a/b", "@PROJ/.csw/config"},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := parseConfigLoadPath(testCase.path)

			require.Equal(t, testCase.want, got)
		})
	}
}
