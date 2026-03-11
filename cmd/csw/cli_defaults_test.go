package main

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
	defaults := conf.CLIDefaultsConfig{
		Model:          "provider/default",
		Worktree:       "feature/default",
		Merge:          true,
		LogLLMRequests: true,
		Thinking:       "high",
		LSPServer:      "gopls",
		GitUserName:    "Config User",
		GitUserEmail:   "config@example.com",
		MaxThreads:     13,
	}

	originalRun := runCLIFunc
	originalDefaultsResolver := resolveCLIDefaultsFunc
	t.Cleanup(func() {
		runCLIFunc = originalRun
		resolveCLIDefaultsFunc = originalDefaultsResolver
	})

	resolveCLIDefaultsFunc = func(params system.ResolveCLIDefaultsParams) (conf.CLIDefaultsConfig, error) {
		_ = params
		return defaults, nil
	}

	captured := ""
	runCLIFunc = func(params *CLIParams) error {
		captured = fmt.Sprintf(
			"model=%s,shadow=%s,worktree=%s,merge=%t,log=%t,thinking=%s,lsp=%s,gitUser=%s,gitEmail=%s,maxThreads=%d",
			params.ModelName,
			params.ShadowDir,
			params.WorktreeBranch,
			params.Merge,
			params.LogLLMRequests,
			params.Thinking,
			params.LSPServer,
			params.GitUserName,
			params.GitUserEmail,
			params.MaxThreads,
		)
		return nil
	}

	cmd := CliCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"prompt"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, captured, "model=provider/default")
	assert.Contains(t, captured, "shadow=")
	assert.Contains(t, captured, "worktree=feature/default")
	assert.Contains(t, captured, "merge=true")
	assert.Contains(t, captured, "log=true")
	assert.Contains(t, captured, "thinking=high")
	assert.Contains(t, captured, "lsp=gopls")
	assert.Contains(t, captured, "gitUser=Config User")
	assert.Contains(t, captured, "gitEmail=config@example.com")
	assert.Contains(t, captured, "maxThreads=13")
}

func TestCLIFlagsOverrideConfigDefaults(t *testing.T) {
	defaults := conf.CLIDefaultsConfig{
		Model:          "provider/default",
		Worktree:       "feature/default",
		Merge:          true,
		LogLLMRequests: true,
		Thinking:       "high",
		LSPServer:      "gopls",
		GitUserName:    "Config User",
		GitUserEmail:   "config@example.com",
		MaxThreads:     13,
	}

	originalRun := runCLIFunc
	originalDefaultsResolver := resolveCLIDefaultsFunc
	t.Cleanup(func() {
		runCLIFunc = originalRun
		resolveCLIDefaultsFunc = originalDefaultsResolver
	})

	resolveCLIDefaultsFunc = func(params system.ResolveCLIDefaultsParams) (conf.CLIDefaultsConfig, error) {
		_ = params
		return defaults, nil
	}

	captured := ""
	runCLIFunc = func(params *CLIParams) error {
		captured = fmt.Sprintf(
			"model=%s,shadow=%s,worktree=%s,merge=%t,log=%t,thinking=%s,lsp=%s,gitUser=%s,gitEmail=%s,maxThreads=%d",
			params.ModelName,
			params.ShadowDir,
			params.WorktreeBranch,
			params.Merge,
			params.LogLLMRequests,
			params.Thinking,
			params.LSPServer,
			params.GitUserName,
			params.GitUserEmail,
			params.MaxThreads,
		)
		return nil
	}

	cmd := CliCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"--model=provider/override",
		"--worktree=feature/override",
		"--merge=false",
		"--log-llm-requests=false",
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
	assert.Contains(t, captured, "worktree=feature/override")
	assert.Contains(t, captured, "merge=false")
	assert.Contains(t, captured, "log=false")
	assert.Contains(t, captured, "thinking=medium")
	assert.Contains(t, captured, "lsp=custom-lsp")
	assert.Contains(t, captured, "gitUser=CLI User")
	assert.Contains(t, captured, "gitEmail=cli@example.com")
	assert.Contains(t, captured, "maxThreads=2")
}

func TestCLIShadowDirFlagPropagation(t *testing.T) {
	originalRun := runCLIFunc
	t.Cleanup(func() {
		runCLIFunc = originalRun
	})

	captured := ""
	runCLIFunc = func(params *CLIParams) error {
		captured = params.ShadowDir
		return nil
	}

	cmd := CliCommand()
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
