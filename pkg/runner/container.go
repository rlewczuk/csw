package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// containerRunner implements the ContainerRunner interface using testcontainers.
type containerRunner struct {
	mu       sync.Mutex
	container testcontainers.Container
	ctx      context.Context
	cancel   context.CancelFunc
	closed   bool
}

// NewContainerRunner creates a new ContainerRunner instance.
// It starts a container with the specified image and mount directories.
// The container will be removed when Close() is called.
func NewContainerRunner(config ContainerConfig) (ContainerRunner, error) {
	if config.ImageName == "" {
		return nil, fmt.Errorf("NewContainerRunner() [container.go]: image name cannot be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Build container request
	req := testcontainers.ContainerRequest{
		Image: config.ImageName,
		// Keep container running - we'll execute commands in it
		Cmd:        []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"echo", "ready"}),
	}

	// Add mount directories
	for containerPath, hostPath := range config.MountDirs {
		req.Mounts = append(req.Mounts, testcontainers.BindMount(hostPath, testcontainers.ContainerMountTarget(containerPath)))
	}

	// Create and start container
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("NewContainerRunner() [container.go]: failed to create container: %w", err)
	}

	return &containerRunner{
		container: c,
		ctx:       ctx,
		cancel:    cancel,
		closed:    false,
	}, nil
}

// RunCommand runs the given command in the container and returns the output and exit code.
func (r *containerRunner) RunCommand(command string) (string, int, error) {
	return r.RunCommandWithOptions(command, CommandOptions{})
}

// RunCommandWithOptions runs the given command with options in the container and returns the output and exit code.
func (r *containerRunner) RunCommandWithOptions(command string, options CommandOptions) (string, int, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return "", 1, fmt.Errorf("ContainerRunner.RunCommandWithOptions() [container.go]: container is closed")
	}
	r.mu.Unlock()

	if command == "" {
		return "", 1, fmt.Errorf("ContainerRunner.RunCommandWithOptions() [container.go]: command cannot be empty")
	}

	// Determine the timeout to use
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second // Default timeout
	}

	// Create context with timeout for this specific command
	ctx, cancel := context.WithTimeout(r.ctx, timeout)
	defer cancel()

	// Build the command - if workdir is specified, wrap with cd
	var cmd []string
	if options.Workdir != "" {
		cmd = []string{"/bin/sh", "-c", fmt.Sprintf("cd %s && %s", options.Workdir, command)}
	} else {
		cmd = []string{"/bin/sh", "-c", command}
	}

	// Execute command in container
	exitCode, reader, err := r.container.Exec(ctx, cmd)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", 124, fmt.Errorf("ContainerRunner.RunCommandWithOptions() [container.go]: command timed out after %v", timeout)
		}
		return "", 1, fmt.Errorf("ContainerRunner.RunCommandWithOptions() [container.go]: failed to execute command: %w", err)
	}

	// Read output with context awareness
	var output bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&output, reader)
	}()

	select {
	case <-done:
		// Output reading completed
	case <-ctx.Done():
		// Context timed out
		return "", 124, fmt.Errorf("ContainerRunner.RunCommandWithOptions() [container.go]: command timed out after %v", timeout)
	}

	return output.String(), exitCode, nil
}

// Close stops and removes the container.
// It is safe to call Close multiple times.
func (r *containerRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	// Cancel the context to signal any running operations to stop
	r.cancel()

	// Terminate the container (this will remove it)
	if r.container != nil {
		// Use a separate context for termination since r.ctx is cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Use a short stop timeout to avoid waiting for graceful shutdown
		if err := r.container.Terminate(ctx, testcontainers.StopTimeout(1*time.Second)); err != nil {
			return fmt.Errorf("ContainerRunner.Close() [container.go]: failed to terminate container: %w", err)
		}
	}

	return nil
}

var _ ContainerRunner = (*containerRunner)(nil)
