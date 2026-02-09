package tool

import (
	"fmt"
	"strings"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBashTool_Execute_MissingCommand(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	tool := NewRunBashTool(mockRunner, nil)

	args := ToolCall{
		ID:        "test-id",
		Function:  "runBash",
		Arguments: NewToolValue(map[string]any{}),
	}

	response := tool.Execute(&args)

	assert.NotNil(t, response.Error)
	assert.Contains(t, response.Error.Error(), "missing required argument: command")
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_AllowedCommand(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("echo 'test'", "test\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		"echo.*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "echo 'test'",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, "test\n", response.Result.String("output"))
	assert.Equal(t, int64(0), response.Result.Int("exit_code"))
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_DeniedCommand(t *testing.T) {
	mockRunner := runner.NewMockRunner()

	privileges := map[string]conf.AccessFlag{
		"rm.*": conf.AccessDeny,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "rm -rf /",
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	_, ok := response.Error.(*RunCommandError)
	assert.True(t, ok, "Error should be RunCommandError")
	assert.Contains(t, response.Error.Error(), "permission denied")
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_AskPermission(t *testing.T) {
	mockRunner := runner.NewMockRunner()

	// No privileges means Ask by default
	tool := NewRunBashTool(mockRunner, nil)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "ls -la",
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	query, ok := response.Error.(*ToolPermissionsQuery)
	assert.True(t, ok, "Error should be ToolPermissionsQuery")
	assert.Equal(t, "Permission Required", query.Title)
	assert.Contains(t, query.Details, "ls -la")
	assert.Contains(t, query.Options, "Allow")
	assert.Contains(t, query.Options, "Deny")
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_ExplicitAskPermission(t *testing.T) {
	mockRunner := runner.NewMockRunner()

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAsk,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "cat file.txt",
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	query, ok := response.Error.(*ToolPermissionsQuery)
	assert.True(t, ok, "Error should be ToolPermissionsQuery")
	assert.Equal(t, "run", query.Meta["type"])
	assert.Equal(t, "cat file.txt", query.Meta["command"])
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_CommandWithNonZeroExitCode(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("exit 42", "", 42, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "exit 42",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, int64(42), response.Result.Int("exit_code"))
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_CommandWithError(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("timeout command", "", 124, fmt.Errorf("command timed out"))

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "timeout command",
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	assert.Contains(t, response.Error.Error(), "command timed out")
	assert.True(t, response.Done)
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_CheckPermission_MostSpecificMatch(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("echo test", "test\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*":       conf.AccessDeny,  // Deny all
		"echo.*":   conf.AccessAllow, // Allow echo commands (more specific)
		"echo foo": conf.AccessDeny,  // Deny "echo foo" specifically (most specific)
	}
	tool := NewRunBashTool(mockRunner, privileges)

	tests := []struct {
		name          string
		command       string
		wantAccess    conf.AccessFlag
		shouldExecute bool
	}{
		{
			name:          "most specific deny",
			command:       "echo foo",
			wantAccess:    conf.AccessDeny,
			shouldExecute: false,
		},
		{
			name:          "specific allow",
			command:       "echo test",
			wantAccess:    conf.AccessAllow,
			shouldExecute: true,
		},
		{
			name:          "general deny",
			command:       "ls -la",
			wantAccess:    conf.AccessDeny,
			shouldExecute: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner.Reset()
			if tt.shouldExecute {
				mockRunner.SetResponse(tt.command, "output\n", 0, nil)
			}

			args := ToolCall{
				ID:       "test-id",
				Function: "runBash",
				Arguments: NewToolValue(map[string]any{
					"command": tt.command,
				}),
			}

			response := tool.Execute(&args)

			if tt.shouldExecute {
				assert.NoError(t, response.Error)
				assert.Equal(t, 1, mockRunner.ExecutionCount())
			} else {
				assert.Error(t, response.Error)
				assert.Equal(t, 0, mockRunner.ExecutionCount())
			}
		})
	}
}

func TestRunBashTool_CheckPermission_NoPrivileges(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	tool := NewRunBashTool(mockRunner, nil)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "any command",
		}),
	}

	response := tool.Execute(&args)

	// Should ask for permission when no privileges are defined
	_, ok := response.Error.(*ToolPermissionsQuery)
	assert.True(t, ok, "Should return permission query")
}

func TestRunBashTool_PermissionQuery_Options(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	tool := NewRunBashTool(mockRunner, nil)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "test command",
		}),
	}

	response := tool.Execute(&args)

	query, ok := response.Error.(*ToolPermissionsQuery)
	require.True(t, ok)

	assert.Equal(t, "Permission Required", query.Title)
	assert.Contains(t, query.Details, "test command")
	assert.Contains(t, query.Options, "Allow")
	assert.Contains(t, query.Options, "Deny")
	assert.Contains(t, query.Options, "Allow and remember (add to privileges)")
	assert.True(t, query.AllowCustomResponse)
	assert.Equal(t, "run", query.Meta["type"])
	assert.Equal(t, "test command", query.Meta["command"])
}

func TestRunBashTool_Execute_WithRelativeWorkdir(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("pwd", "/project/root/subdir\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithRoot(mockRunner, privileges, "/project/root")

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "pwd",
			"workdir": "subdir",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, 1, mockRunner.ExecutionCount())

	// Verify that RunCommandWithOptions was called
	exec := mockRunner.GetLastExecution()
	assert.NotNil(t, exec)
	assert.Equal(t, "pwd", exec.Command)
}

func TestRunBashTool_Execute_WithAbsoluteWorkdir(t *testing.T) {
	mockRunner := runner.NewMockRunner()

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithRoot(mockRunner, privileges, "/project/root")

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "ls",
			"workdir": "/absolute/path",
		}),
	}

	response := tool.Execute(&args)

	// Should ask for permission due to absolute path
	assert.Error(t, response.Error)
	query, ok := response.Error.(*ToolPermissionsQuery)
	assert.True(t, ok, "Error should be ToolPermissionsQuery")
	assert.Equal(t, "Permission Required for Absolute Path", query.Title)
	assert.Contains(t, query.Details, "/absolute/path")
	assert.Contains(t, query.Details, "ls")
	assert.Equal(t, "run_absolute_workdir", query.Meta["type"])
	assert.Equal(t, "/absolute/path", query.Meta["workdir"])
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithTimeout(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("echo test", "test\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "echo test",
			"timeout": int64(30),
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, "test\n", response.Result.String("output"))
	assert.Equal(t, int64(0), response.Result.Int("exit_code"))
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithInvalidTimeout(t *testing.T) {
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
			"timeout": int64(-5),
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	assert.Contains(t, response.Error.Error(), "timeout must be positive")
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithWorkdirAndTimeout(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("ls", "file1.txt\nfile2.txt\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithRoot(mockRunner, privileges, "/project/root")

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "ls",
			"workdir": "test-dir",
			"timeout": int64(10),
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, "file1.txt\nfile2.txt\n", response.Result.String("output"))
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithoutProjectRoot(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("pwd", "subdir\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "pwd",
			"workdir": "subdir",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Render(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	tool := NewRunBashTool(mockRunner, nil)

	tests := []struct {
		name           string
		args           *ToolCall
		wantSummary    string
		wantDetails    string
		wantInSummary  string
		wantInDetails  string
	}{
		{
			name: "basic command",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "echo hello"}),
			},
			wantInSummary: "bash: echo hello",
			wantInDetails: "echo hello\n\n",
		},
		{
			name: "command with output",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "ls -la", "output": "file1.txt\nfile2.txt\n"}),
			},
			wantInSummary: "bash: ls -la",
			wantInDetails: "ls -la\n\nfile1.txt\nfile2.txt\n",
		},
		{
			name: "long command should be truncated in summary",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "this is a very long command that should be truncated in the summary because it exceeds the maximum length limit of 128 characters for sure"}),
			},
			wantInSummary: "...",
			wantInDetails: "this is a very long command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "bash: "), "Summary should start with 'bash: '")
			assert.Contains(t, summary, tt.wantInSummary, "Summary should contain expected text")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, details, tt.wantInDetails, "Details should contain expected text")
		})
	}
}

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
	// Should be truncated to 5 lines plus truncation message
	assert.Contains(t, output, "line1")
	assert.Contains(t, output, "line5")
	assert.NotContains(t, output, "line6")
	assert.Contains(t, output, "Output is truncated.")
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
	assert.NotContains(t, response.Result.String("output"), "Output is truncated.")
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
	assert.NotContains(t, response.Result.String("output"), "Output is truncated.")
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
	// Create output with 250 lines (more than default 200)
	var longOutput strings.Builder
	for i := 1; i <= 250; i++ {
		longOutput.WriteString(fmt.Sprintf("line%d\n", i))
	}
	mockRunner.SetResponse("seq 1 250", longOutput.String(), 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	// No limit specified - should use default of 200
	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "seq 1 250",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)

	output := response.Result.String("output")
	// Should be truncated to 200 lines plus truncation message
	assert.Contains(t, output, "line1")
	assert.Contains(t, output, "line200")
	assert.NotContains(t, output, "line201")
	assert.Contains(t, output, "Output is truncated.")
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithLimitAndOtherOptions(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	longOutput := "line1\nline2\nline3\nline4\nline5\n"
	mockRunner.SetResponse("seq 1 5", longOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithRoot(mockRunner, privileges, "/project/root")

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "seq 1 5",
			"workdir": "subdir",
			"timeout": int64(30),
			"limit":   int64(3),
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)

	output := response.Result.String("output")
	assert.Contains(t, output, "line1")
	assert.Contains(t, output, "line3")
	assert.NotContains(t, output, "line4")
	assert.Contains(t, output, "Output is truncated.")
	assert.Equal(t, 1, mockRunner.ExecutionCount())
}
