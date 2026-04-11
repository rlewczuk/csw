package main

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyRunDefaultsUsesResolverAndDefaults(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("model", "", "")
	cmd.Flags().String("worktree", "", "")
	cmd.Flags().Bool("merge", false, "")
	cmd.Flags().Bool("log-llm-requests", false, "")
	cmd.Flags().String("thinking", "", "")
	cmd.Flags().String("thinking-mode", "", "")
	cmd.Flags().String("lsp-server", "", "")
	cmd.Flags().String("git-user", "", "")
	cmd.Flags().String("git-email", "", "")
	cmd.Flags().Int("max-threads", 0, "")
	cmd.Flags().String("shadow-dir", "", "")
	cmd.Flags().Bool("allow-all-permissions", false, "")
	cmd.Flags().StringArray("vfs-allow", nil, "")

	model := ""
	worktree := ""
	merge := false
	logRequests := false
	thinking := ""
	lspServer := ""
	gitUser := ""
	gitEmail := ""
	maxThreads := 0
	shadowDir := ""
	allowAll := false
	vfsAllow := []string(nil)

	err := applyRunDefaults(func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		assert.Equal(t, "wd", params.WorkDir)
		assert.Equal(t, "shadow", params.ShadowDir)
		assert.Equal(t, "project", params.ProjectConfig)
		assert.Equal(t, "cfg", params.ConfigPath)
		return conf.RunDefaultsConfig{
			Model:               "provider/model",
			Worktree:            "feature/default",
			Merge:               true,
			LogLLMRequests:      true,
			Thinking:            "high",
			LSPServer:           "gopls",
			GitUserName:         "Config User",
			GitUserEmail:        "config@example.com",
			MaxThreads:          7,
			ShadowDir:           "shadow/config",
			AllowAllPermissions: true,
			VFSAllow:            []string{"/allow/one", "/allow/two"},
		}, nil
	}, cmd, "wd", "shadow", "project", "cfg", &model, &worktree, &merge, &logRequests, &thinking, &lspServer, &gitUser, &gitEmail, &maxThreads, &shadowDir, &allowAll, &vfsAllow)

	require.NoError(t, err)
	assert.Equal(t, "provider/model", model)
	assert.Equal(t, "feature/default", worktree)
	assert.True(t, merge)
	assert.True(t, logRequests)
	assert.Equal(t, "high", thinking)
	assert.Equal(t, "gopls", lspServer)
	assert.Equal(t, "Config User", gitUser)
	assert.Equal(t, "config@example.com", gitEmail)
	assert.Equal(t, 7, maxThreads)
	assert.Equal(t, "shadow/config", shadowDir)
	assert.True(t, allowAll)
	assert.Equal(t, []string{"/allow/one", "/allow/two"}, vfsAllow)
}

func TestApplyRunDefaultsValidatesMaxThreads(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("model", "", "")
	cmd.Flags().String("worktree", "", "")
	cmd.Flags().Bool("merge", false, "")
	cmd.Flags().Bool("log-llm-requests", false, "")
	cmd.Flags().String("thinking", "", "")
	cmd.Flags().String("thinking-mode", "", "")
	cmd.Flags().String("lsp-server", "", "")
	cmd.Flags().String("git-user", "", "")
	cmd.Flags().String("git-email", "", "")
	cmd.Flags().Int("max-threads", 0, "")
	cmd.Flags().String("shadow-dir", "", "")
	cmd.Flags().Bool("allow-all-permissions", false, "")
	cmd.Flags().StringArray("vfs-allow", nil, "")

	model := ""
	worktree := ""
	merge := false
	logRequests := false
	thinking := ""
	lspServer := ""
	gitUser := ""
	gitEmail := ""
	maxThreads := -1
	shadowDir := ""
	allowAll := false
	vfsAllow := []string(nil)

	err := applyRunDefaults(func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{}, nil
	}, cmd, "wd", "", "", "", &model, &worktree, &merge, &logRequests, &thinking, &lspServer, &gitUser, &gitEmail, &maxThreads, &shadowDir, &allowAll, &vfsAllow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--max-threads must be >= 0")
}
