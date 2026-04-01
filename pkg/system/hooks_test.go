package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatEditedFilesSummaryUsesWorktreeDir verifies edited files are collected from active worktree.
func TestFormatEditedFilesSummaryUsesWorktreeDir(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, runGitInDir(repoDir, "init", "-b", "main"))
	require.NoError(t, runGitInDir(repoDir, "config", "user.name", "Test User"))
	require.NoError(t, runGitInDir(repoDir, "config", "user.email", "test@example.com"))

	targetFile := filepath.Join(repoDir, "test.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("old\n"), 0644))
	require.NoError(t, runGitInDir(repoDir, "add", "test.txt"))
	require.NoError(t, runGitInDir(repoDir, "commit", "-m", "initial"))

	require.NoError(t, runGitInDir(repoDir, "branch", "feature/summary"))
	worktreeDir := filepath.Join(repoDir, ".cswdata", "work", "feature", "summary")
	require.NoError(t, os.MkdirAll(filepath.Dir(worktreeDir), 0755))
	require.NoError(t, runGitInDir(repoDir, "worktree", "add", worktreeDir, "feature/summary"))

	require.NoError(t, os.WriteFile(filepath.Join(worktreeDir, "test.txt"), []byte("old\nnew\n"), 0644))

	summary := FormatEditedFilesSummary(repoDir, worktreeDir)
	assert.NotEqual(t, "-", summary)
	assert.Contains(t, summary, "test.txt")
}

// TestFormatEditedFilesSummaryIncludesUntrackedFiles verifies new untracked files are listed.
func TestFormatEditedFilesSummaryIncludesUntrackedFiles(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, runGitInDir(repoDir, "init", "-b", "main"))

	newFile := filepath.Join(repoDir, "new.txt")
	require.NoError(t, os.WriteFile(newFile, []byte("content\n"), 0644))

	summary := FormatEditedFilesSummary(repoDir, repoDir)
	assert.NotEqual(t, "-", summary)
	assert.Contains(t, summary, "new.txt (new file)")
}

func runGitInDir(workDir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("runGitInDir() [hooks_test.go]: git %v failed: %w: %s", args, err, string(output))
	}

	return nil
}

func TestApplyHookOverridesToConfigs(t *testing.T) {
	tests := []struct {
		name         string
		configs      map[string]*conf.HookConfig
		overrides    []string
		expectErr    string
		assertResult func(t *testing.T, cfg map[string]*conf.HookConfig)
	}{
		{
			name: "hook name enables configured disabled hook",
			configs: map[string]*conf.HookConfig{
				"commit": {Name: "commit", Hook: "commit", Command: "echo before", Enabled: false, Type: conf.HookTypeShell, RunOn: conf.HookRunOnSandbox},
			},
			overrides: []string{"commit"},
			assertResult: func(t *testing.T, cfg map[string]*conf.HookConfig) {
				require.Contains(t, cfg, "commit")
				assert.True(t, cfg["commit"].Enabled)
				assert.Equal(t, "echo before", cfg["commit"].Command)
			},
		},
		{
			name: "disable override disables hook",
			configs: map[string]*conf.HookConfig{
				"commit": {Name: "commit", Hook: "commit", Command: "echo before", Enabled: true, Type: conf.HookTypeShell, RunOn: conf.HookRunOnSandbox},
			},
			overrides: []string{"commit:disable"},
			assertResult: func(t *testing.T, cfg map[string]*conf.HookConfig) {
				require.Contains(t, cfg, "commit")
				assert.False(t, cfg["commit"].Enabled)
			},
		},
		{
			name: "settings update hook and enable when disabled",
			configs: map[string]*conf.HookConfig{
				"commit": {Name: "commit", Hook: "commit", Command: "echo before", Enabled: false, Type: conf.HookTypeShell, RunOn: conf.HookRunOnSandbox},
			},
			overrides: []string{"commit:command=echo after,timeout=45,run-on=host"},
			assertResult: func(t *testing.T, cfg map[string]*conf.HookConfig) {
				require.Contains(t, cfg, "commit")
				assert.True(t, cfg["commit"].Enabled)
				assert.Equal(t, "echo after", cfg["commit"].Command)
				assert.Equal(t, 45*time.Second, cfg["commit"].Timeout)
				assert.Equal(t, conf.HookRunOnHost, cfg["commit"].RunOn)
			},
		},
		{
			name:      "adds new hook when mandatory fields are provided",
			configs:   map[string]*conf.HookConfig{},
			overrides: []string{"summary:hook=summary,command=echo summary"},
			assertResult: func(t *testing.T, cfg map[string]*conf.HookConfig) {
				require.Contains(t, cfg, "summary")
				assert.Equal(t, "summary", cfg["summary"].Name)
				assert.Equal(t, "summary", cfg["summary"].Hook)
				assert.Equal(t, "echo summary", cfg["summary"].Command)
				assert.True(t, cfg["summary"].Enabled)
				assert.Equal(t, conf.HookTypeShell, cfg["summary"].Type)
				assert.Equal(t, conf.HookRunOnSandbox, cfg["summary"].RunOn)
			},
		},
		{
			name:      "new hook without mandatory fields returns error",
			configs:   map[string]*conf.HookConfig{},
			overrides: []string{"summary:hook=summary"},
			expectErr: "requires setting \"command\"",
		},
		{
			name:      "new llm hook requires prompt",
			configs:   map[string]*conf.HookConfig{},
			overrides: []string{"summary:hook=summary,type=llm"},
			expectErr: "requires setting \"prompt\"",
		},
		{
			name:      "adds new llm hook when required fields are provided",
			configs:   map[string]*conf.HookConfig{},
			overrides: []string{"summary:hook=summary,type=llm,prompt=hello {{.user_prompt}},system_prompt=sys,model=mock/test,thinking=low,output_to=llm_result,error_to=llm_err"},
			assertResult: func(t *testing.T, cfg map[string]*conf.HookConfig) {
				require.Contains(t, cfg, "summary")
				assert.Equal(t, conf.HookTypeLLM, cfg["summary"].Type)
				assert.Equal(t, "hello {{.user_prompt}}", cfg["summary"].Prompt)
				assert.Equal(t, "sys", cfg["summary"].SystemPrompt)
				assert.Equal(t, "mock/test", cfg["summary"].Model)
				assert.Equal(t, "low", cfg["summary"].Thinking)
				assert.Equal(t, "llm_result", cfg["summary"].OutputTo)
				assert.Equal(t, "llm_err", cfg["summary"].ErrorTo)
			},
		},
		{
			name: "llm hook defaults to result output_to",
			configs: map[string]*conf.HookConfig{
				"summary": {Name: "summary", Hook: "summary", Type: conf.HookTypeLLM, Prompt: "p", Enabled: true},
			},
			overrides: []string{"summary"},
			assertResult: func(t *testing.T, cfg map[string]*conf.HookConfig) {
				require.Contains(t, cfg, "summary")
				assert.Equal(t, "result", cfg["summary"].OutputTo)
			},
		},
		{
			name:      "unknown hook with name only returns error",
			configs:   map[string]*conf.HookConfig{},
			overrides: []string{"missing"},
			expectErr: "is not configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ApplyHookOverridesToConfigs(tc.configs, tc.overrides)
			if tc.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			tc.assertResult(t, result)
		})
	}
}

func TestBuildRuntimeHookConfigStoreOverridesAreEphemeral(t *testing.T) {
	base := impl.NewMockConfigStore()
	base.SetHookConfigs(map[string]*conf.HookConfig{
		"commit":  {Name: "commit", Hook: "commit", Command: "echo before", Enabled: false, Type: conf.HookTypeShell, RunOn: conf.HookRunOnSandbox},
		"summary": {Name: "summary", Hook: "summary", Type: conf.HookTypeLLM, Prompt: "old", OutputTo: "result", Enabled: true},
	})

	runtimeStore, err := BuildRuntimeHookConfigStore(base, []string{"commit:command=echo after", "summary:prompt=new,output_to=hook_result"})
	require.NoError(t, err)

	runtimeHooks, err := runtimeStore.GetHookConfigs()
	require.NoError(t, err)
	require.Contains(t, runtimeHooks, "commit")
	assert.Equal(t, "echo after", runtimeHooks["commit"].Command)
	assert.True(t, runtimeHooks["commit"].Enabled)
	require.Contains(t, runtimeHooks, "summary")
	assert.Equal(t, "new", runtimeHooks["summary"].Prompt)
	assert.Equal(t, "hook_result", runtimeHooks["summary"].OutputTo)

	baseHooks, err := base.GetHookConfigs()
	require.NoError(t, err)
	require.Contains(t, baseHooks, "commit")
	assert.Equal(t, "echo before", baseHooks["commit"].Command)
	assert.False(t, baseHooks["commit"].Enabled)
	require.Contains(t, baseHooks, "summary")
	assert.Equal(t, "old", baseHooks["summary"].Prompt)
	assert.Equal(t, "result", baseHooks["summary"].OutputTo)
}

func TestNormalizeResumeTarget(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{name: "empty", input: "", expected: ""},
		{name: "last", input: "last", expected: "last"},
		{name: "last uppercase", input: "LAST", expected: "last"},
		{name: "valid uuid", input: "018F6E30-3ACB-7F24-BEDE-8D96CD157152", expected: "018f6e30-3acb-7f24-bede-8d96cd157152"},
		{name: "branch name", input: "feature/existing", expected: "feature/existing"},
		{name: "workdir name", input: "0145-resume-by-branch-dir", expected: "0145-resume-by-branch-dir"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := NormalizeResumeTarget(tc.input)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
