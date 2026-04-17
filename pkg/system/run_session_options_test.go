package system

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/commands"
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
	cmd.Flags().Bool("log-llm-requests-raw", false, "")
	cmd.Flags().String("thinking", "", "")
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
	logRequestsRaw := false
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
			LogLLMRequestsRaw:   true,
			Thinking:            "high",
			LSPServer:           "gopls",
			GitUserName:         "Config User",
			GitUserEmail:        "config@example.com",
			MaxThreads:          7,
			ShadowDir:           "shadow/config",
			AllowAllPermissions: true,
			VFSAllow:            []string{"/allow/one", "/allow/two"},
		}, nil
	}, cmd, "wd", "shadow", "project", "cfg", &model, &worktree, &merge, &logRequests, &logRequestsRaw, &thinking, &lspServer, &gitUser, &gitEmail, &maxThreads, &shadowDir, &allowAll, &vfsAllow)

	require.NoError(t, err)
	assert.Equal(t, "provider/model", model)
	assert.Equal(t, "feature/default", worktree)
	assert.True(t, merge)
	assert.True(t, logRequests)
	assert.True(t, logRequestsRaw)
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
	cmd.Flags().Bool("log-llm-requests-raw", false, "")
	cmd.Flags().String("thinking", "", "")
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
	logRequestsRaw := false
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
	}, cmd, "wd", "", "", "", &model, &worktree, &merge, &logRequests, &logRequestsRaw, &thinking, &lspServer, &gitUser, &gitEmail, &maxThreads, &shadowDir, &allowAll, &vfsAllow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--max-threads must be >= 0")
}

func TestApplyCommandRunDefaultsAppliesZeroAndFalseValues(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("model", "", "")
	cmd.Flags().String("role", "", "")
	cmd.Flags().String("worktree", "", "")
	cmd.Flags().Bool("merge", false, "")
	cmd.Flags().Bool("log-llm-requests", false, "")
	cmd.Flags().String("thinking", "", "")
	cmd.Flags().String("lsp-server", "", "")
	cmd.Flags().String("git-user", "", "")
	cmd.Flags().String("git-email", "", "")
	cmd.Flags().Int("max-threads", 0, "")
	cmd.Flags().String("shadow-dir", "", "")
	cmd.Flags().Bool("allow-all-permissions", false, "")
	cmd.Flags().StringArray("vfs-allow", nil, "")
	cmd.Flags().Bool("container-enabled", false, "")
	cmd.Flags().Bool("container-disabled", false, "")
	cmd.Flags().String("container-image", "", "")
	cmd.Flags().StringArray("container-mount", nil, "")
	cmd.Flags().StringArray("container-env", nil, "")

	model := "provider/current"
	role := "developer"
	worktree := "feature/current"
	merge := true
	logRequests := true
	thinking := "high"
	lspServer := "gopls"
	gitUser := "Current User"
	gitEmail := "current@example.com"
	maxThreads := 7
	shadowDir := "shadow/current"
	allowAll := true
	vfsAllow := []string{"/existing"}
	containerOn := false
	containerOff := false
	containerImage := "image/current"
	containerMounts := []string{"/old:/container"}
	containerEnv := []string{"A=B"}

	commandEnabled, err := applyCommandRunDefaults(
		cmd,
		&commands.RunDefaultsMetadata{
			Model:               stringPointer(""),
			DefaultRole:         stringPointer(""),
			Worktree:            stringPointer(""),
			Merge:               boolPointer(false),
			LogLLMRequests:      boolPointer(false),
			Thinking:            stringPointer(""),
			LSPServer:           stringPointer(""),
			GitUserName:         stringPointer(""),
			GitUserEmail:        stringPointer(""),
			MaxThreads:          intPointer(0),
			ShadowDir:           stringPointer(""),
			AllowAllPermissions: boolPointer(false),
			VFSAllow:            &[]string{},
			Container: &commands.ContainerMetadata{
				Enabled: boolPointer(false),
				Image:   stringPointer(""),
				Mounts:  &[]string{},
				Env:     &[]string{},
			},
		},
		&model,
		&role,
		&worktree,
		&merge,
		&logRequests,
		&thinking,
		&lspServer,
		&gitUser,
		&gitEmail,
		&maxThreads,
		&shadowDir,
		&allowAll,
		&vfsAllow,
		&containerOn,
		&containerOff,
		&containerImage,
		&containerMounts,
		&containerEnv,
	)
	require.NoError(t, err)
	require.NotNil(t, commandEnabled)
	assert.False(t, *commandEnabled)
	assert.Equal(t, "", model)
	assert.Equal(t, "", role)
	assert.Equal(t, "", worktree)
	assert.False(t, merge)
	assert.False(t, logRequests)
	assert.Equal(t, "", thinking)
	assert.Equal(t, "", lspServer)
	assert.Equal(t, "", gitUser)
	assert.Equal(t, "", gitEmail)
	assert.Equal(t, 0, maxThreads)
	assert.Equal(t, "", shadowDir)
	assert.False(t, allowAll)
	assert.Empty(t, vfsAllow)
	assert.Equal(t, "", containerImage)
	assert.Empty(t, containerMounts)
	assert.Empty(t, containerEnv)
}

func stringPointer(value string) *string {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}

func intPointer(value int) *int {
	return &value
}
