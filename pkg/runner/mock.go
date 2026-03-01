package runner

import (
	"fmt"
	"sync"
	"time"
)

// CommandExecution represents a single command execution record.
type CommandExecution struct {
	Command  string
	Output   string
	Stderr   string
	ExitCode int
	Error    error
	Workdir  string
	Timeout  time.Duration
}

// MockRunner implements the CommandRunner interface for testing.
type MockRunner struct {
	mu         sync.Mutex
	executions []CommandExecution
	responses  map[string]CommandExecution
	defaultRes CommandExecution
}

// NewMockRunner creates a new MockRunner instance.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		executions: make([]CommandExecution, 0),
		responses:  make(map[string]CommandExecution),
		defaultRes: CommandExecution{
			Output:   "",
			Stderr:   "",
			ExitCode: 0,
			Error:    nil,
		},
	}
}

// SetResponse sets the response for a specific command.
// When RunCommand is called with this command, it will return the specified output, exit code, and error.
func (m *MockRunner) SetResponse(command string, output string, exitCode int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[command] = CommandExecution{
		Command:  command,
		Output:   output,
		ExitCode: exitCode,
		Error:    err,
	}
}

// SetResponseDetailed sets the response with separate stdout and stderr for a specific command.
func (m *MockRunner) SetResponseDetailed(command string, stdout string, stderr string, exitCode int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[command] = CommandExecution{
		Command:  command,
		Output:   stdout,
		ExitCode: exitCode,
		Error:    err,
		Stderr:   stderr,
	}
}

// SetDefaultResponse sets the default response for commands that don't have a specific response.
func (m *MockRunner) SetDefaultResponse(output string, exitCode int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultRes = CommandExecution{
		Command:  "",
		Output:   output,
		Stderr:   "",
		ExitCode: exitCode,
		Error:    err,
	}
}

// RunCommand runs the given command and returns the output and exit code.
func (m *MockRunner) RunCommand(command string) (string, int, error) {
	return m.RunCommandWithOptions(command, CommandOptions{})
}

// RunCommandWithOptions runs the given command with options and returns the output and exit code.
func (m *MockRunner) RunCommandWithOptions(command string, options CommandOptions) (string, int, error) {
	stdout, stderr, exitCode, err := m.RunCommandWithOptionsDetailed(command, options)
	output := stdout
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr
	}
	return output, exitCode, err
}

// RunCommandWithOptionsDetailed runs command and returns stdout/stderr separately.
func (m *MockRunner) RunCommandWithOptionsDetailed(command string, options CommandOptions) (string, string, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if command == "" {
		return "", "", 1, fmt.Errorf("MockRunner.RunCommandWithOptionsDetailed() [mock.go]: command cannot be empty")
	}

	// Check if we have a specific response for this command
	var exec CommandExecution
	if resp, ok := m.responses[command]; ok {
		exec = resp
	} else {
		exec = m.defaultRes
	}

	// Record the execution
	m.executions = append(m.executions, CommandExecution{
		Command:  command,
		Output:   exec.Output,
		Stderr:   exec.Stderr,
		ExitCode: exec.ExitCode,
		Error:    exec.Error,
		Workdir:  options.Workdir,
		Timeout:  options.Timeout,
	})

	return exec.Output, exec.Stderr, exec.ExitCode, exec.Error
}

// GetExecutions returns all recorded command executions.
func (m *MockRunner) GetExecutions() []CommandExecution {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]CommandExecution, len(m.executions))
	copy(result, m.executions)
	return result
}

// GetLastExecution returns the last recorded command execution, or nil if no commands were executed.
func (m *MockRunner) GetLastExecution() *CommandExecution {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.executions) == 0 {
		return nil
	}
	exec := m.executions[len(m.executions)-1]
	return &exec
}

// Reset clears all recorded executions and responses.
func (m *MockRunner) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = make([]CommandExecution, 0)
	m.responses = make(map[string]CommandExecution)
	m.defaultRes = CommandExecution{
		Output:   "",
		ExitCode: 0,
		Error:    nil,
	}
}

// ExecutionCount returns the number of commands that have been executed.
func (m *MockRunner) ExecutionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.executions)
}

var _ CommandRunner = (*MockRunner)(nil)
