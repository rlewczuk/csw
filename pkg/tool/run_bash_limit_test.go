package tool

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBashTool_Execute_WithMaxOutput_SavesLargeOutputToWorktmp(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	largeOutput := strings.Repeat("abcdef", 20)
	mockRunner.SetResponse("large output", largeOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	sessionWorkdir := t.TempDir()
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, sessionWorkdir)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command":    "large output",
			"max_output": int64(64),
		}),
	}

	response := tool.Execute(&args)

	require.NoError(t, response.Error)
	require.True(t, response.Done)
	assert.Equal(t, int64(0), response.Result.Int("exit_code"))
	assert.Equal(t, 1, mockRunner.ExecutionCount())

	output := response.Result.String("output")
	assert.Contains(t, output, "Output was too big")
	assert.Contains(t, output, "grepped or processed with tools/scripts")
	assert.Contains(t, output, "only partially read rather than read in its entirety")
	assert.NotContains(t, output, largeOutput)

	spilledPath := extractRunBashSpilledPath(t, output)
	assert.True(t, strings.HasPrefix(spilledPath, filepath.Join(sessionWorkdir, ".cswdata", "worktmp")))

	content, err := os.ReadFile(spilledPath)
	require.NoError(t, err)
	assert.Equal(t, largeOutput, string(content))
}

func TestRunBashTool_Execute_WithMaxOutput_DefaultSavesLargeOutput(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	largeOutput := strings.Repeat("x", defaultRunBashMaxOutputBytes+1)
	mockRunner.SetResponse("default large output", largeOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	sessionWorkdir := t.TempDir()
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, sessionWorkdir)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "default large output",
		}),
	}

	response := tool.Execute(&args)

	require.NoError(t, response.Error)
	output := response.Result.String("output")
	assert.Contains(t, output, "Output was too big")

	spilledPath := extractRunBashSpilledPath(t, output)
	content, err := os.ReadFile(spilledPath)
	require.NoError(t, err)
	assert.Equal(t, largeOutput, string(content))
}

func TestRunBashTool_Execute_WithMaxOutputZero_NoSafeguard(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	largeOutput := strings.Repeat("abcdef", 20)
	mockRunner.SetResponse("large output no safeguard", largeOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, t.TempDir())

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command":    "large output no safeguard",
			"max_output": int64(0),
		}),
	}

	response := tool.Execute(&args)

	require.NoError(t, response.Error)
	assert.Equal(t, largeOutput, response.Result.String("output"))
}

func TestRunBashTool_Execute_WithInvalidMaxOutput(t *testing.T) {
	mockRunner := runner.NewMockRunner()

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command":    "echo test",
			"max_output": int64(-1),
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	assert.Contains(t, response.Error.Error(), "max_output must be non-negative")
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func extractRunBashSpilledPath(t *testing.T, message string) string {
	t.Helper()

	re := regexp.MustCompile(`Saved full output to temporary file: (.+)`)
	matches := re.FindStringSubmatch(message)
	require.Len(t, matches, 2)
	return strings.TrimSpace(matches[1])
}
