package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// BashRunner implements the CommandRunner interface for running bash commands.
type BashRunner struct {
	workdir string
	timeout time.Duration
}

// NewBashRunner creates a new BashRunner instance.
// workdir is the directory where commands will be executed.
// timeout is the maximum duration for command execution (0 means no timeout).
func NewBashRunner(workdir string, timeout time.Duration) *BashRunner {
	return &BashRunner{
		workdir: workdir,
		timeout: timeout,
	}
}

// RunCommand runs the given command and returns the output and exit code.
func (r *BashRunner) RunCommand(command string) (string, int, error) {
	return r.RunCommandWithOptions(command, CommandOptions{})
}

// RunCommandWithOptions runs the given command with options and returns the output and exit code.
func (r *BashRunner) RunCommandWithOptions(command string, options CommandOptions) (string, int, error) {
	if command == "" {
		return "", 1, fmt.Errorf("BashRunner.RunCommandWithOptions() [bash.go]: command cannot be empty")
	}

	// Determine the working directory to use
	workdir := r.workdir
	if options.Workdir != "" {
		workdir = options.Workdir
	}

	// Determine the timeout to use
	timeout := r.timeout
	if options.Timeout > 0 {
		timeout = options.Timeout
	}

	var ctx context.Context
	var cancel context.CancelFunc

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = workdir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Combine stdout and stderr
	output := stdout.String()
	if stderr.Len() > 0 {
		if len(output) > 0 {
			output += "\n"
		}
		output += stderr.String()
	}

	// Get exit code
	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output, 124, fmt.Errorf("BashRunner.RunCommandWithOptions() [bash.go]: command timed out after %v", timeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Other errors (e.g., command not found)
			return output, 127, fmt.Errorf("BashRunner.RunCommandWithOptions() [bash.go]: %w", err)
		}
	}

	return output, exitCode, nil
}

var _ CommandRunner = (*BashRunner)(nil)
