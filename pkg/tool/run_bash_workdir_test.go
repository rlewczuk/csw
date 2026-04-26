package tool

import (
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBashTool_Execute_WithRelativeWorkdir(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("pwd", "/project/root/subdir\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, "/project/root")

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

	exec := mockRunner.GetLastExecution()
	assert.NotNil(t, exec)
	assert.Equal(t, "pwd", exec.Command)
}

func TestRunBashTool_Execute_WithAbsoluteWorkdir(t *testing.T) {
	mockRunner := runner.NewMockRunner()

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, "/project/root")

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "ls",
			"workdir": "/absolute/path",
		}),
	}

	response := tool.Execute(&args)

	assert.Error(t, response.Error)
	assert.Contains(t, response.Error.Error(), "permission denied")
	assert.True(t, response.Done)
	assert.Equal(t, 0, mockRunner.ExecutionCount())
}

func TestRunBashTool_Execute_WithWorkdirAndTimeout(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("ls", "file1.txt\nfile2.txt\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, "/project/root")

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

func TestRunBashTool_Execute_WithSessionWorkdir_DefaultsToSessionWorkdir(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("pwd", "/session/workdir\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, "/session/workdir")

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "pwd",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, 1, mockRunner.ExecutionCount())

	exec := mockRunner.GetLastExecution()
	require.NotNil(t, exec)
	assert.Equal(t, "pwd", exec.Command)
	assert.Equal(t, "/session/workdir", exec.Workdir)
}

func TestRunBashTool_Execute_WithSessionWorkdir_ExplicitWorkdirOverrides(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("pwd", "/session/workdir/explicit\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, "/session/workdir")

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "pwd",
			"workdir": "explicit",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, 1, mockRunner.ExecutionCount())

	exec := mockRunner.GetLastExecution()
	require.NotNil(t, exec)
	assert.Equal(t, "pwd", exec.Command)
	assert.Equal(t, "/session/workdir/explicit", exec.Workdir)
}

func TestRunBashTool_Execute_WithoutSessionWorkdir_NoWorkdirSpecified(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("pwd", "output\n", 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashTool(mockRunner, privileges)

	args := ToolCall{
		ID:       "test-id",
		Function: "runBash",
		Arguments: NewToolValue(map[string]any{
			"command": "pwd",
		}),
	}

	response := tool.Execute(&args)

	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, 1, mockRunner.ExecutionCount())

	exec := mockRunner.GetLastExecution()
	require.NotNil(t, exec)
	assert.Equal(t, "pwd", exec.Command)
	assert.Equal(t, "", exec.Workdir)
}

func TestRunBashTool_Execute_WithLimitAndOtherOptions(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	longOutput := "line1\nline2\nline3\nline4\nline5\n"
	mockRunner.SetResponse("seq 1 5", longOutput, 0, nil)

	privileges := map[string]conf.AccessFlag{
		".*": conf.AccessAllow,
	}
	tool := NewRunBashToolWithSessionWorkdir(mockRunner, privileges, "/project/root")

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
	assert.NotContains(t, output, "line1")
	assert.Contains(t, output, "line3")
	assert.Contains(t, output, "line4")
	assert.Contains(t, output, "line5")
	assert.Equal(t, 1, mockRunner.ExecutionCount())

	exec := mockRunner.GetLastExecution()
	require.NotNil(t, exec)
	assert.Equal(t, 30*time.Second, exec.Timeout)
	assert.Equal(t, "/project/root/subdir", exec.Workdir)
}
