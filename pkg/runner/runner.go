package runner

import "time"

// CommandOptions holds optional parameters for running commands.
type CommandOptions struct {
	// Workdir is the directory where the command will be executed.
	// If empty, the runner's default working directory is used.
	Workdir string
	// Timeout is the maximum duration for command execution.
	// If 0, the runner's default timeout is used.
	Timeout time.Duration
}

// CommandRunner is an interface for running commands.
type CommandRunner interface {
	// RunCommand runs the given command and returns the output and exit code.
	RunCommand(command string) (string, int, error)
	// RunCommandWithOptions runs the given command with options and returns the output and exit code.
	RunCommandWithOptions(command string, options CommandOptions) (string, int, error)
}
