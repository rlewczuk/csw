package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookCommand_ListAndInfo(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := t.TempDir()

	oldHome := os.Getenv("HOME")
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Chdir(oldDir)
	})
	require.NoError(t, os.Setenv("HOME", tmpHome))
	require.NoError(t, os.Chdir(tmpDir))

	hooksDir := filepath.Join(tmpDir, ".csw", "config", "hooks")
	require.NoError(t, os.MkdirAll(filepath.Join(hooksDir, "jira_ticket"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "jira_ticket", "jira_ticket.yml"),
		[]byte("name: jira_ticket\ndescription: Jira ticket verifier\nhook: pre_run\ntype: llm\nprompt: check {{.ticket}}\nenabled: true\n"),
		0o644,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(hooksDir, "disabled_hook"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "disabled_hook", "disabled_hook.yml"),
		[]byte("name: disabled_hook\ndescription: Disabled hook\nhook: pre_run\ntype: shell\ncommand: echo hi\nenabled: false\n"),
		0o644,
	))

	t.Run("list", func(t *testing.T) {
		cmd := HookCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"list"})

		err := cmd.Execute()
		require.NoError(t, err)

		out := stdout.String()
		assert.Contains(t, out, "NAME")
		assert.Contains(t, out, "HOOK")
		assert.Contains(t, out, "TYPE")
		assert.Contains(t, out, "DESCRIPTION")
		assert.Contains(t, out, "STATUS")
		assert.Contains(t, out, "jira_ticket")
		assert.Contains(t, out, "pre_run")
		assert.Contains(t, out, "llm")
		assert.Contains(t, out, "Jira ticket verifier")
		assert.Contains(t, out, "enabled")
		assert.Contains(t, out, "disabled_hook")
		assert.Contains(t, out, "disabled")
	})

	t.Run("info", func(t *testing.T) {
		cmd := HookCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"info", "jira_ticket"})

		err := cmd.Execute()
		require.NoError(t, err)

		out := stdout.String()
		assert.Contains(t, out, "\"name\": \"jira_ticket\"")
		assert.Contains(t, out, "\"description\": \"Jira ticket verifier\"")
		assert.Contains(t, out, "\"hook\": \"pre_run\"")
		assert.Contains(t, out, "\"type\": \"llm\"")
		assert.Contains(t, out, "\"prompt\": \"check {{.ticket}}\"")
	})
}

func TestHookCommand_Run(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := t.TempDir()

	oldHome := os.Getenv("HOME")
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Chdir(oldDir)
	})
	require.NoError(t, os.Setenv("HOME", tmpHome))
	require.NoError(t, os.Chdir(tmpDir))

	hooksDir := filepath.Join(tmpDir, ".csw", "config", "hooks")
	require.NoError(t, os.MkdirAll(filepath.Join(hooksDir, "llm_preview"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "llm_preview", "llm_preview.yml"),
		[]byte("name: llm_preview\ndescription: LLM preview hook\nhook: pre_run\ntype: llm\nprompt: ticket={{.ticket}}\nsystem-prompt: sys={{.env}}\noutput-to: llm_out\n"),
		0o644,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(hooksDir, "shell_ctx"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "shell_ctx", "shell_ctx.yml"),
		[]byte("name: shell_ctx\ndescription: shell context hook\nhook: pre_run\ntype: shell\nrun-on: host\ncommand: printf 'ticket=%s env=%s' \"$CSW_TICKET\" \"$CSW_ENV\"\n"),
		0o644,
	))

	ctxFile := filepath.Join(tmpDir, "env.txt")
	require.NoError(t, os.WriteFile(ctxFile, []byte("staging"), 0o644))

	t.Run("run llm without --run renders prompt", func(t *testing.T) {
		cmd := HookCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"run", "llm_preview", "--context", "ticket=ABC-123", "--context-from", "env=" + ctxFile})

		err := cmd.Execute()
		require.NoError(t, err)

		out := stdout.String()
		assert.Contains(t, out, "Hook: llm_preview")
		assert.Contains(t, out, "rendered_prompt_or_command:")
		assert.Contains(t, out, "ticket=ABC-123")
		assert.Contains(t, out, "session_context")
		assert.Contains(t, out, "\"env\": \"staging\"")
		assert.Contains(t, out, "\"ticket\": \"ABC-123\"")
	})

	t.Run("run shell executes and prints context+stdout", func(t *testing.T) {
		cmd := HookCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"run", "shell_ctx", "--context", "ticket=XYZ-9", "--context", "env=dev"})

		err := cmd.Execute()
		require.NoError(t, err)

		out := stdout.String()
		assert.Contains(t, out, "Hook: shell_ctx")
		assert.Contains(t, out, "stdout:")
		assert.Contains(t, out, "ticket=XYZ-9 env=dev")
		assert.Contains(t, out, "session_context")
		assert.Contains(t, out, "\"ticket\": \"XYZ-9\"")
		assert.Contains(t, out, "\"env\": \"dev\"")
	})
}
