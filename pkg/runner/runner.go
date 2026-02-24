package runner

import (
	"io"
	"time"
)

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

// ContainerConfig contains configuration for the ContainerRunner runner.
// It is used to create a new ContainerRunner runner.
type ContainerConfig struct {
	// ImageName is the name of the container image to use.
	ImageName string
	// MountDirs is a map of host directories to mount in the container.
	// Keys are paths in the container, values are paths on the host.
	MountDirs map[string]string
}

// ContainerRunner is a command runner that runs commands in a container.
type ContainerRunner interface {
	io.Closer
	CommandRunner
}
