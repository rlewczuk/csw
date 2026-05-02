package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
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
	stdout, stderr, exitCode, err := r.RunCommandWithOptionsDetailed(command, options)
	output := stdout
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr
	}
	return output, exitCode, err
}

// RunCommandWithOptionsDetailed runs a command and returns stdout/stderr separately.
func (r *BashRunner) RunCommandWithOptionsDetailed(command string, options CommandOptions) (string, string, int, error) {
	if command == "" {
		return "", "", 1, fmt.Errorf("BashRunner.RunCommandWithOptionsDetailed() [bash.go]: command cannot be empty")
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

	// Get exit code
	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout.String(), stderr.String(), 124, fmt.Errorf("BashRunner.RunCommandWithOptionsDetailed() [bash.go]: command timed out after %v", timeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Other errors (e.g., command not found)
			return stdout.String(), stderr.String(), 127, fmt.Errorf("BashRunner.RunCommandWithOptionsDetailed() [bash.go]: %w", err)
		}
	}

	return stdout.String(), stderr.String(), exitCode, nil
}

// RunCommandWithOptionsBackgroundDetailed runs command in background mode and returns partial/full output.
func (r *BashRunner) RunCommandWithOptionsBackgroundDetailed(command string, options CommandOptions, background time.Duration) (string, string, int, int, bool, error) {
	if command == "" {
		return "", "", 1, 0, false, fmt.Errorf("BashRunner.RunCommandWithOptionsBackgroundDetailed() [bash.go]: command cannot be empty")
	}

	workdir := r.workdir
	if options.Workdir != "" {
		workdir = options.Workdir
	}

	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = workdir

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", 1, 0, false, fmt.Errorf("BashRunner.RunCommandWithOptionsBackgroundDetailed() [bash.go]: stdout pipe failed: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", 1, 0, false, fmt.Errorf("BashRunner.RunCommandWithOptionsBackgroundDetailed() [bash.go]: stderr pipe failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", "", 127, 0, false, fmt.Errorf("BashRunner.RunCommandWithOptionsBackgroundDetailed() [bash.go]: command start failed: %w", err)
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stdoutBuf, stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stderrBuf, stderrPipe)
	}()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	if background <= 0 {
		return stdoutBuf.String(), stderrBuf.String(), 0, pid, true, nil
	}

	select {
	case waitErr := <-done:
		wg.Wait()
		exitCode := 0
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				return stdoutBuf.String(), stderrBuf.String(), exitCode, pid, false, nil
			}
			return stdoutBuf.String(), stderrBuf.String(), 127, pid, false, fmt.Errorf("BashRunner.RunCommandWithOptionsBackgroundDetailed() [bash.go]: %w", waitErr)
		}
		return stdoutBuf.String(), stderrBuf.String(), exitCode, pid, false, nil
	case <-time.After(background):
		return stdoutBuf.String(), stderrBuf.String(), 0, pid, true, nil
	}
}

var _ CommandRunner = (*BashRunner)(nil)
