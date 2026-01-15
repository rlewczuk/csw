package runner

// CommandRunner is an interface for running commands.
type CommandRunner interface {
	// RunCommand runs the given command and returns the output and exit code.
	RunCommand(command string) (string, int, error)
}
