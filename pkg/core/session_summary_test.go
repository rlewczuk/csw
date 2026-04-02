package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmitSessionSummary validates summary behavior on success and error paths.
func TestEmitSessionSummary(t *testing.T) {
	tests := []struct {
		name                   string
		saveErr                error
		sessionRunErr          error
		expectErr              string
		expectInfoMessageCount int
		expectWarnMessageCount int
		expectWarningSubstring string
	}{
		{
			name:                   "session error still prints summary",
			sessionRunErr:          errors.New("agent failed"),
			expectErr:              "agent failed",
			expectInfoMessageCount: 1,
		},
		{
			name:                   "session error with summary save failure returns session error and warns",
			saveErr:                errors.New("disk full"),
			sessionRunErr:          errors.New("agent failed"),
			expectErr:              "agent failed",
			expectInfoMessageCount: 1,
			expectWarnMessageCount: 1,
			expectWarningSubstring: "Failed to save session summary: disk full",
		},
		{
			name:                   "no session error and summary save failure returns wrapped error",
			saveErr:                errors.New("disk full"),
			expectErr:              "EmitSessionSummary() [session_summary.go]: failed to save session summary: disk full",
			expectInfoMessageCount: 0,
			expectWarnMessageCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			originalSaveSummary := SaveSessionSummaryMarkdownFunc
			originalSaveSummaryJSON := SaveSessionSummaryJSONFunc
			t.Cleanup(func() {
				SaveSessionSummaryMarkdownFunc = originalSaveSummary
				SaveSessionSummaryJSONFunc = originalSaveSummaryJSON
			})

			SaveSessionSummaryMarkdownFunc = func(logsDir string, session *SweSession, sessionInfo string) error {
				return tc.saveErr
			}
			SaveSessionSummaryJSONFunc = func(logsDir string, session *SweSession, buildResult SessionSummaryBuildResult, startTime time.Time, endTime time.Time, baseCommitID string, headCommitID string) error {
				return nil
			}

			capture := &summaryMessageCapture{}
			err := EmitSessionSummary(
				time.Now().Add(-5*time.Second),
				time.Now(),
				nil,
				SessionSummaryBuildResult{LogsDir: "any"},
				capture.showMessage,
				tc.sessionRunErr,
				"",
				"",
			)

			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr)
			}

			assert.Len(t, capture.infoMessages, tc.expectInfoMessageCount)
			assert.Len(t, capture.warnMessages, tc.expectWarnMessageCount)
			if tc.expectInfoMessageCount > 0 {
				assert.Contains(t, capture.infoMessages[0], "Session completed in")
			}
			if tc.expectWarningSubstring != "" {
				assert.Contains(t, capture.warnMessages[0], tc.expectWarningSubstring)
			}
		})
	}
}

func TestBuildSessionSummaryMessage(t *testing.T) {
	t.Run("includes token and context stats", func(t *testing.T) {
		session := &SweSession{}
		summary := BuildSessionSummaryMessage(5*time.Second, session, SessionSummaryBuildResult{})

		assert.Contains(t, summary, "Session completed in 5s | tokens(input=0[cached=0,noncached=0], output=0, total=0) | context=0")
		assert.Contains(t, summary, "Model: -")
		assert.Contains(t, summary, "Thinking: -")
		assert.Contains(t, summary, "LSP server: -")
		assert.Contains(t, summary, "Container image: -")
		assert.Contains(t, summary, "Roles used: -")
		assert.Contains(t, summary, "Tools used: -")
		assert.Contains(t, summary, "Edited files:\n-")
	})

	t.Run("nil session returns base summary", func(t *testing.T) {
		assert.Equal(t,
			"Session completed in 5s",
			BuildSessionSummaryMessage(5*time.Second, nil, SessionSummaryBuildResult{}),
		)
	})
}

// TestFormatEditedFilesSummaryUsesWorktreeDir verifies edited files are collected from active worktree.
func TestFormatEditedFilesSummaryUsesWorktreeDir(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, runGitInDirForSessionSummary(repoDir, "init", "-b", "main"))
	require.NoError(t, runGitInDirForSessionSummary(repoDir, "config", "user.name", "Test User"))
	require.NoError(t, runGitInDirForSessionSummary(repoDir, "config", "user.email", "test@example.com"))

	targetFile := filepath.Join(repoDir, "test.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("old\n"), 0644))
	require.NoError(t, runGitInDirForSessionSummary(repoDir, "add", "test.txt"))
	require.NoError(t, runGitInDirForSessionSummary(repoDir, "commit", "-m", "initial"))

	require.NoError(t, runGitInDirForSessionSummary(repoDir, "branch", "feature/summary"))
	worktreeDir := filepath.Join(repoDir, ".cswdata", "work", "feature", "summary")
	require.NoError(t, os.MkdirAll(filepath.Dir(worktreeDir), 0755))
	require.NoError(t, runGitInDirForSessionSummary(repoDir, "worktree", "add", worktreeDir, "feature/summary"))

	require.NoError(t, os.WriteFile(filepath.Join(worktreeDir, "test.txt"), []byte("old\nnew\n"), 0644))

	summary := FormatEditedFilesSummary(repoDir, worktreeDir)
	assert.NotEqual(t, "-", summary)
	assert.Contains(t, summary, "test.txt")
}

// TestFormatEditedFilesSummaryIncludesUntrackedFiles verifies new untracked files are listed.
func TestFormatEditedFilesSummaryIncludesUntrackedFiles(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, runGitInDirForSessionSummary(repoDir, "init", "-b", "main"))

	newFile := filepath.Join(repoDir, "new.txt")
	require.NoError(t, os.WriteFile(newFile, []byte("content\n"), 0644))

	summary := FormatEditedFilesSummary(repoDir, repoDir)
	assert.NotEqual(t, "-", summary)
	assert.Contains(t, summary, "new.txt (new file)")
}

type summaryMessageCapture struct {
	infoMessages []string
	warnMessages []string
}

func (c *summaryMessageCapture) showMessage(message string, messageType shared.MessageType) {
	switch messageType {
	case shared.MessageTypeWarning:
		c.warnMessages = append(c.warnMessages, message)
	case shared.MessageTypeInfo:
		c.infoMessages = append(c.infoMessages, message)
	}
}

func runGitInDirForSessionSummary(workDir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("runGitInDirForSessionSummary() [session_summary_test.go]: git %v failed: %w: %s", args, err, string(output))
	}

	return nil
}
