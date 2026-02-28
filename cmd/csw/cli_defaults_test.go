package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIConfigDefaultsPropagation(t *testing.T) {
	mockStore := impl.NewMockConfigStore()
	mockStore.SetGlobalConfig(&conf.GlobalConfig{
		Defaults: conf.CLIDefaultsConfig{
			Model:          "provider/default",
			Worktree:       "feature/default",
			Merge:          true,
			LogLLMRequests: true,
			Thinking:       "high",
			LSPServer:      "gopls",
		},
	})

	originalRun := runCLIFunc
	originalConfigStoreBuilder := newCompositeConfigStoreFunc
	t.Cleanup(func() {
		runCLIFunc = originalRun
		newCompositeConfigStoreFunc = originalConfigStoreBuilder
	})

	newCompositeConfigStoreFunc = func(projDir string, configPath string) (conf.ConfigStore, error) {
		return mockStore, nil
	}

	captured := ""
	runCLIFunc = func(params *CLIParams) error {
		captured = fmt.Sprintf(
			"model=%s,worktree=%s,merge=%t,log=%t,thinking=%s,lsp=%s",
			params.ModelName,
			params.WorktreeBranch,
			params.Merge,
			params.LogLLMRequests,
			params.Thinking,
			params.LSPServer,
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
	assert.Contains(t, captured, "worktree=feature/default")
	assert.Contains(t, captured, "merge=true")
	assert.Contains(t, captured, "log=true")
	assert.Contains(t, captured, "thinking=high")
	assert.Contains(t, captured, "lsp=gopls")
}

func TestCLIFlagsOverrideConfigDefaults(t *testing.T) {
	mockStore := impl.NewMockConfigStore()
	mockStore.SetGlobalConfig(&conf.GlobalConfig{
		Defaults: conf.CLIDefaultsConfig{
			Model:          "provider/default",
			Worktree:       "feature/default",
			Merge:          true,
			LogLLMRequests: true,
			Thinking:       "high",
			LSPServer:      "gopls",
		},
	})

	originalRun := runCLIFunc
	originalConfigStoreBuilder := newCompositeConfigStoreFunc
	t.Cleanup(func() {
		runCLIFunc = originalRun
		newCompositeConfigStoreFunc = originalConfigStoreBuilder
	})

	newCompositeConfigStoreFunc = func(projDir string, configPath string) (conf.ConfigStore, error) {
		return mockStore, nil
	}

	captured := ""
	runCLIFunc = func(params *CLIParams) error {
		captured = fmt.Sprintf(
			"model=%s,worktree=%s,merge=%t,log=%t,thinking=%s,lsp=%s",
			params.ModelName,
			params.WorktreeBranch,
			params.Merge,
			params.LogLLMRequests,
			params.Thinking,
			params.LSPServer,
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
}
