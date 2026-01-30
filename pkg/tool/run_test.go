package tool

import (
	"fmt"
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
		Function:  "run.bash",
		Arguments: NewToolValue(map[string]any{}),
	}

	response := tool.Execute(args)

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
		Function: "run.bash",
		Arguments: NewToolValue(map[string]any{
			"command": "echo 'test'",
		}),
	}

	response := tool.Execute(args)

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
		Function: "run.bash",
		Arguments: NewToolValue(map[string]any{
			"command": "rm -rf /",
		}),
	}

	response := tool.Execute(args)

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
		Function: "run.bash",
		Arguments: NewToolValue(map[string]any{
			"command": "ls -la",
		}),
	}

	response := tool.Execute(args)

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
		Function: "run.bash",
		Arguments: NewToolValue(map[string]any{
			"command": "cat file.txt",
		}),
	}

	response := tool.Execute(args)

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
		Function: "run.bash",
		Arguments: NewToolValue(map[string]any{
			"command": "exit 42",
		}),
	}

	response := tool.Execute(args)

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
		Function: "run.bash",
		Arguments: NewToolValue(map[string]any{
			"command": "timeout command",
		}),
	}

	response := tool.Execute(args)

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
				Function: "run.bash",
				Arguments: NewToolValue(map[string]any{
					"command": tt.command,
				}),
			}

			response := tool.Execute(args)

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
		Function: "run.bash",
		Arguments: NewToolValue(map[string]any{
			"command": "any command",
		}),
	}

	response := tool.Execute(args)

	// Should ask for permission when no privileges are defined
	_, ok := response.Error.(*ToolPermissionsQuery)
	assert.True(t, ok, "Should return permission query")
}

func TestRunBashTool_PermissionQuery_Options(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	tool := NewRunBashTool(mockRunner, nil)

	args := ToolCall{
		ID:       "test-id",
		Function: "run.bash",
		Arguments: NewToolValue(map[string]any{
			"command": "test command",
		}),
	}

	response := tool.Execute(args)

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
