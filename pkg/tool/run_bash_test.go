package tool

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
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

	assert.Error(t, response.Error)
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

	exec := mockRunner.GetLastExecution()
	require.NotNil(t, exec)
	assert.Equal(t, 30*time.Second, exec.Timeout)
}

func TestRunBashTool_Execute_DefaultTimeoutApplied(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("echo test", "test\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges, 120*time.Second)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "echo test",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, "test\n", response.Result.String("output"))

	exec := mockRunner.GetLastExecution()
	require.NotNil(t, exec)
	assert.Equal(t, 120*time.Second, exec.Timeout)
}

func TestRunBashTool_Execute_TimeoutErrorIncludesPartialOutput(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("sleep 10", "line1\nline2\nline3\n", 124, fmt.Errorf("command timed out after 2s"))

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges, 2*time.Second)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "sleep 10",
			"limit":   int64(2),
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	assert.Contains(t, response.Error.Error(), "command terminated due to timeout")
	assert.Contains(t, response.Error.Error(), "Partial output:")
	assert.Contains(t, response.Error.Error(), "line2")
	assert.Contains(t, response.Error.Error(), "line3")
	assert.NotContains(t, response.Error.Error(), "line1")
	assert.True(t, response.Done)
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

func TestRunBashTool_Render(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	tool := NewRunBashTool(mockRunner, nil)

	tests := []struct {
		name          string
		args          *ToolCall
		wantSummary   string
		wantDetails   string
		wantInSummary string
		wantInDetails string
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
				Arguments: NewToolValue(map[string]any{"command": "ls -la", "stdout": "file1.txt\nfile2.txt\n"}),
			},
			wantInSummary: "bash: ls -la",
			wantInDetails: "ls -la\n\nSTDOUT:\nfile1.txt\nfile2.txt\n",
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
			summary, details, _, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "bash: "), "Summary should start with 'bash: '")
			assert.Contains(t, summary, tt.wantInSummary, "Summary should contain expected text")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, details, tt.wantInDetails, "Details should contain expected text")
		})
	}
}

func TestRunBashTool_Render_WithError(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	tool := NewRunBashTool(mockRunner, nil)

	tests := []struct {
		name            string
		args            *ToolCall
		wantInSummary   string
		wantInDetails   string
		wantNotInDetails string
	}{
		{
			name: "error with exit code only",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "exit 1", "exit_code": int64(1)}),
			},
			wantInSummary: "ERROR: exit code 1",
			wantInDetails: "ERROR: exit code 1\n",
		},
		{
			name: "error with single line stderr",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "cmd", "exit_code": int64(2), "stderr": "permission denied"}),
			},
			wantInSummary: "ERROR: exit code 2, permission denied",
			wantInDetails: "ERROR: exit code 2, permission denied\nSTDERR:\npermission denied",
		},
		{
			name: "error with multi-line stderr",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "cmd", "exit_code": int64(3), "stderr": "line1\nline2"}),
			},
			wantInSummary: "ERROR: exit code 3",
			wantInDetails: "ERROR: exit code 3\nSTDERR:\nline1\nline2",
		},
		{
			name: "error with output",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "cmd", "exit_code": int64(1), "output": "some output"}),
			},
			wantInSummary: "ERROR: exit code 1",
			wantInDetails: "ERROR: exit code 1\nsome output",
		},
		{
			name: "success with exit code 0 - no error prefix",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "echo hello", "exit_code": int64(0), "output": "hello"}),
			},
			wantInSummary: "bash: echo hello",
			wantInDetails: "echo hello\n\nhello",
			wantNotInDetails: "ERROR:",
		},
		{
			name: "permission denied error from tool response is rendered",
			args: &ToolCall{
				Function:  "runBash",
				Arguments: NewToolValue(map[string]any{"command": "go test", "error": "RunCommandError [run.go]: permission denied: go test"}),
			},
			wantInSummary: "ERROR: RunCommandError [run.go]: permission denied: go test",
			wantInDetails: "ERROR: RunCommandError [run.go]: permission denied: go test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, _, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, summary, tt.wantInSummary, "Summary should contain expected text")
			assert.Contains(t, details, tt.wantInDetails, "Details should contain expected text")
			if tt.wantNotInDetails != "" {
				assert.NotContains(t, details, tt.wantNotInDetails, "Details should not contain unexpected text")
			}
		})
	}
}
