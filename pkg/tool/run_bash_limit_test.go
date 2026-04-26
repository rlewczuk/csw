package tool

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/stretchr/testify/assert"
)

func TestRunBashTool_Execute_WithLimit_TruncatesOutput(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	// Create output with 10 lines
	longOutput := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	mockRunner.SetResponse("seq 1 10", longOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "seq 1 10",
			"limit":   int64(5),
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, int64(0), response.Result.Int("exit_code"))

	output := response.Result.String("output")
	// Should keep only the last 5 lines
	assert.NotContains(t, output, "line5")
	assert.Contains(t, output, "line6")
	assert.Contains(t, output, "line10")
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithLimit_NoTruncationNeeded(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	shortOutput := "line1\nline2\nline3\n"
	mockRunner.SetResponse("echo test", shortOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "echo test",
			"limit":   int64(10),
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, "line1\nline2\nline3\n", response.Result.String("output"))
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithLimitZero_NoLimit(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	longOutput := "line1\nline2\nline3\nline4\nline5\n"
	mockRunner.SetResponse("seq 1 5", longOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "seq 1 5",
			"limit":   int64(0),
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, longOutput, response.Result.String("output"))
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithInvalidLimit(t *testing.T) {
	mockRunner := runner.NewMockRunner()

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "echo test",
			"limit":   int64(-1),
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	assert.Contains(t, response.Error.Error(), "limit must be non-negative")
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_DefaultLimit_Applied(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	// Create output with 550 lines (more than default 500)
	var longOutput strings.Builder
	for i := 1; i <= 550; i++ {
		longOutput.WriteString(fmt.Sprintf("line%d\n", i))
	}
	mockRunner.SetResponse("seq 1 550", longOutput.String(), 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	// No limit specified - should use default of 500
	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "seq 1 550",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)

	output := response.Result.String("output")
	// Should be truncated to the last 500 lines
	assert.NotContains(t, output, "line50\n")
	assert.Contains(t, output, "line51")
	assert.Contains(t, output, "line550")
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithLimit_LogsCropping(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	longOutput := "line1\nline2\nline3\nline4\nline5\n"
	mockRunner.SetResponse("seq 1 5", longOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	var logs bytes.Buffer
	tool.SetLogger(slog.New(slog.NewTextHandler(&logs, nil)))

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "seq 1 5",
			"limit":   int64(3),
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.Contains(t, logs.String(), "runBash_output_cropped")
	assert.Contains(t, logs.String(), "limit=3")
}
