package system

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIConfigDefaultsPropagation(t *testing.T) {
	defaults := conf.RunDefaultsConfig{
		Model:               "provider/default",
		Worktree:            "feature/default",
		Merge:               true,
		LogLLMRequests:      true,
		LogLLMRequestsRaw:   true,
		Thinking:            "high",
		LSPServer:           "gopls",
		GitUserName:         "Config User",
		GitUserEmail:        "config@example.com",
		MaxThreads:          13,
		TaskDir:             "default/tasks",
		ShadowDir:           "shadow/default",
		AllowAllPermissions: true,
		VFSAllow:            []string{"/allowed/one", "/allowed/two"},
	}

	originalRun := runCommandFunc
	originalDefaultsResolver := resolveRunDefaultsFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		resolveRunDefaultsFunc = originalDefaultsResolver
	})

	resolveRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return defaults, nil
	}

	captured := ""
	runCommandFunc = func(params *RunParams) error {
		taskDir := ""
		if params.Task != nil {
			taskDir = params.Task.TaskDir
		}
		vfsAllowJoined := ""
		if len(params.VFSAllow) > 0 {
			vfsAllowJoined = params.VFSAllow[0]
			for _, value := range params.VFSAllow[1:] {
				vfsAllowJoined += ";" + value
			}
		}
		captured = fmt.Sprintf(
			"model=%s,shadow=%s,worktree=%s,merge=%t,allowAll=%t,vfsAllow=%s,log=%t,logRaw=%t,thinking=%s,lsp=%s,gitUser=%s,gitEmail=%s,maxThreads=%d,taskDir=%s",
			params.ModelName,
			params.ShadowDir,
			params.WorktreeBranch,
			params.Merge,
			params.AllowAllPerms,
			vfsAllowJoined,
			params.LogLLMRequests,
			params.LogLLMRequestsRaw,
			params.Thinking,
			params.LSPServer,
			params.GitUserName,
			params.GitUserEmail,
			params.MaxThreads,
			taskDir,
		)
		return nil
	}

	cmd := RunCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"prompt"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, captured, "model=provider/default")
	assert.Contains(t, captured, "shadow=shadow/default")
	assert.Contains(t, captured, "worktree=feature/default")
	assert.Contains(t, captured, "merge=true")
	assert.Contains(t, captured, "allowAll=true")
	assert.Contains(t, captured, "vfsAllow=/allowed/one;/allowed/two")
	assert.Contains(t, captured, "log=true")
	assert.Contains(t, captured, "logRaw=true")
	assert.Contains(t, captured, "thinking=high")
	assert.Contains(t, captured, "lsp=gopls")
	assert.Contains(t, captured, "gitUser=Config User")
	assert.Contains(t, captured, "gitEmail=config@example.com")
	assert.Contains(t, captured, "maxThreads=13")
	assert.Contains(t, captured, "taskDir=")
}

func TestCLIFlagsOverrideConfigDefaults(t *testing.T) {
	defaults := conf.RunDefaultsConfig{
		Model:               "provider/default",
		Worktree:            "feature/default",
		Merge:               true,
		LogLLMRequests:      true,
		LogLLMRequestsRaw:   true,
		Thinking:            "high",
		LSPServer:           "gopls",
		GitUserName:         "Config User",
		GitUserEmail:        "config@example.com",
		MaxThreads:          13,
		TaskDir:             "default/tasks",
		ShadowDir:           "shadow/default",
		AllowAllPermissions: true,
		VFSAllow:            []string{"/allowed/one", "/allowed/two"},
	}

	originalRun := runCommandFunc
	originalDefaultsResolver := resolveRunDefaultsFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		resolveRunDefaultsFunc = originalDefaultsResolver
	})

	resolveRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return defaults, nil
	}

	captured := ""
	runCommandFunc = func(params *RunParams) error {
		taskDir := ""
		if params.Task != nil {
			taskDir = params.Task.TaskDir
		}
		vfsAllowJoined := ""
		if len(params.VFSAllow) > 0 {
			vfsAllowJoined = params.VFSAllow[0]
			for _, value := range params.VFSAllow[1:] {
				vfsAllowJoined += ";" + value
			}
		}
		captured = fmt.Sprintf(
			"model=%s,shadow=%s,worktree=%s,merge=%t,allowAll=%t,vfsAllow=%s,log=%t,logRaw=%t,thinking=%s,lsp=%s,gitUser=%s,gitEmail=%s,maxThreads=%d,taskDir=%s",
			params.ModelName,
			params.ShadowDir,
			params.WorktreeBranch,
			params.Merge,
			params.AllowAllPerms,
			vfsAllowJoined,
			params.LogLLMRequests,
			params.LogLLMRequestsRaw,
			params.Thinking,
			params.LSPServer,
			params.GitUserName,
			params.GitUserEmail,
			params.MaxThreads,
			taskDir,
		)
		return nil
	}

	cmd := RunCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"--model=provider/override",
		"--shadow-dir=shadow/override",
		"--worktree=feature/override",
		"--merge=false",
		"--allow-all-permissions=false",
		"--vfs-allow=/explicit/allow",
		"--log-llm-requests=false",
		"--log-llm-requests-raw",
		"--thinking=medium",
		"--lsp-server=custom-lsp",
		"--git-user=CLI User",
		"--git-email=cli@example.com",
		"--max-threads=2",
		"prompt",
	})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, captured, "model=provider/override")
	assert.Contains(t, captured, "shadow=shadow/override")
	assert.Contains(t, captured, "worktree=feature/override")
	assert.Contains(t, captured, "merge=false")
	assert.Contains(t, captured, "allowAll=false")
	assert.Contains(t, captured, "vfsAllow=/explicit/allow")
	assert.Contains(t, captured, "log=true")
	assert.Contains(t, captured, "logRaw=true")
	assert.Contains(t, captured, "thinking=medium")
	assert.Contains(t, captured, "lsp=custom-lsp")
	assert.Contains(t, captured, "gitUser=CLI User")
	assert.Contains(t, captured, "gitEmail=cli@example.com")
	assert.Contains(t, captured, "maxThreads=2")
	assert.Contains(t, captured, "taskDir=")
}

func TestCLINoMergeOverridesConfigDefaults(t *testing.T) {
	defaults := conf.RunDefaultsConfig{
		Worktree: "feature/default",
		Merge:    true,
	}

	originalRun := runCommandFunc
	originalDefaultsResolver := resolveRunDefaultsFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
		resolveRunDefaultsFunc = originalDefaultsResolver
	})

	resolveRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return defaults, nil
	}

	capturedMerge := true
	runCommandFunc = func(params *RunParams) error {
		capturedMerge = params.Merge
		return nil
	}

	cmd := RunCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--no-merge", "prompt"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.False(t, capturedMerge)
}

func TestCLIShadowDirFlagPropagation(t *testing.T) {
	originalRun := runCommandFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
	})

	captured := ""
	runCommandFunc = func(params *RunParams) error {
		captured = params.ShadowDir
		return nil
	}

	cmd := RunCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	shadowDir := t.TempDir()
	cmd.SetArgs([]string{"--shadow-dir=" + shadowDir, "prompt"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, shadowDir, captured)
}
