package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// containerRunner implements the ContainerRunner interface using testcontainers.
type containerRunner struct {
	mu        sync.Mutex
	container testcontainers.Container
	ctx       context.Context
	cancel    context.CancelFunc
	closed    bool
	uid       int
	gid       int
	env       map[string]string
}

// NewContainerRunner creates a new ContainerRunner instance.
// It starts a container with the specified image and mount directories.
// The container will be removed when Close() is called.
func NewContainerRunner(config ContainerConfig) (ContainerRunner, error) {
	if config.ImageName == "" {
		return nil, fmt.Errorf("NewContainerRunner() [container.go]: image name cannot be empty")
	}

	if config.UID < 0 {
		return nil, fmt.Errorf("NewContainerRunner() [container.go]: UID cannot be negative")
	}

	if config.GID < 0 {
		return nil, fmt.Errorf("NewContainerRunner() [container.go]: GID cannot be negative")
	}

	if config.UID > 0 && config.GID > 0 {
		if config.UserName == "" {
			return nil, fmt.Errorf("NewContainerRunner() [container.go]: user name cannot be empty when UID/GID are set")
		}
		if config.GroupName == "" {
			return nil, fmt.Errorf("NewContainerRunner() [container.go]: group name cannot be empty when UID/GID are set")
		}
		if config.HomeDir == "" {
			return nil, fmt.Errorf("NewContainerRunner() [container.go]: home directory cannot be empty when UID/GID are set")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Build container request - always start as root to allow chown
	req := testcontainers.ContainerRequest{
		Image: config.ImageName,
		// Keep container running - we'll execute commands in it
		Cmd:        []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"echo", "ready"}),
	}
	if config.Workdir != "" {
		req.WorkingDir = config.Workdir
	}

	// Add mount directories
	for containerPath, hostPath := range config.MountDirs {
		req.Mounts = append(req.Mounts, testcontainers.ContainerMount{
			Source: testcontainers.DockerBindMountSource{
				HostPath: hostPath,
			},
			Target:   testcontainers.ContainerMountTarget(containerPath),
			ReadOnly: config.ReadOnlyMounts,
		})
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

	// If UID/GID are specified, mirror host user/group and chown the home directory.
	if config.UID > 0 && config.GID > 0 {
		createUserScript := fmt.Sprintf(
			"set -e; \n"+
				"if command -v getent >/dev/null 2>&1 && getent group %q >/dev/null 2>&1; then :; "+
				"elif command -v groupadd >/dev/null 2>&1; then groupadd -g %d %q; "+
				"elif command -v addgroup >/dev/null 2>&1; then addgroup -g %d %q; "+
				"else echo 'group creation utility not found' >&2; exit 1; fi; \n"+
				"if command -v id >/dev/null 2>&1 && id -u %q >/dev/null 2>&1; then :; "+
				"elif command -v useradd >/dev/null 2>&1; then useradd -m -u %d -g %d -d %q -s /bin/sh %q; "+
				"elif command -v adduser >/dev/null 2>&1; then adduser -D -u %d -G %q -h %q -s /bin/sh %q; "+
				"else echo 'user creation utility not found' >&2; exit 1; fi; \n"+
				"mkdir -p %q; chown -R %d:%d %q",
			config.GroupName,
			config.GID,
			config.GroupName,
			config.GID,
			config.GroupName,
			config.UserName,
			config.UID,
			config.GID,
			config.HomeDir,
			config.UserName,
			config.UID,
			config.GroupName,
			config.HomeDir,
			config.UserName,
			config.HomeDir,
			config.UID,
			config.GID,
			config.HomeDir,
		)

		exitCode, reader, err := c.Exec(ctx, []string{"/bin/sh", "-c", createUserScript})
		if err != nil || exitCode != 0 {
			// Read any error output
			var output bytes.Buffer
			if reader != nil {
				_, _ = io.Copy(&output, reader)
			}
			// Terminate container on error
			_ = c.Terminate(ctx)
			cancel()
			if err != nil {
				return nil, fmt.Errorf("NewContainerRunner() [container.go]: failed to create mapped user and prepare home directory: %w", err)
			}
			return nil, fmt.Errorf("NewContainerRunner() [container.go]: create mapped user/home preparation failed with exit code %d: %s", exitCode, output.String())
		}
	}

	return &containerRunner{
		container: c,
		ctx:       ctx,
		cancel:    cancel,
		closed:    false,
		uid:       config.UID,
		gid:       config.GID,
		env:       copyEnvMap(config.Env),
	}, nil
}

// RunCommand runs the given command in the container and returns the output and exit code.
func (r *containerRunner) RunCommand(command string) (string, int, error) {
	return r.RunCommandWithOptions(command, CommandOptions{})
}

// RunCommandWithOptions runs the given command with options in the container and returns the output and exit code.
func (r *containerRunner) RunCommandWithOptions(command string, options CommandOptions) (string, int, error) {
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

// RunCommandWithOptionsDetailed runs the given command with options in the container and returns stdout, stderr, exit code, and error separately.
func (r *containerRunner) RunCommandWithOptionsDetailed(command string, options CommandOptions) (string, string, int, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return "", "", 1, fmt.Errorf("ContainerRunner.RunCommandWithOptionsDetailed() [container.go]: container is closed")
	}
	r.mu.Unlock()

	if command == "" {
		return "", "", 1, fmt.Errorf("ContainerRunner.RunCommandWithOptionsDetailed() [container.go]: command cannot be empty")
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
	workDir := options.Workdir
	if workDir == "" {
		workDir = "."
	}

	commandWithEnv := command
	if len(r.env) > 0 {
		commandWithEnv = buildExportPrefix(r.env) + command
	}

	if workDir != "" {
		cmd = []string{"/bin/sh", "-c", fmt.Sprintf("cd %q && %s", workDir, commandWithEnv)}
	} else {
		cmd = []string{"/bin/sh", "-c", commandWithEnv}
	}

	// Build exec options - run as specified user if UID/GID are set
	var execOpts []tcexec.ProcessOption
	if r.uid > 0 && r.gid > 0 {
		execOpts = append(execOpts, tcexec.WithUser(fmt.Sprintf("%d:%d", r.uid, r.gid)))
	}

	// Execute command in container
	exitCode, reader, err := r.container.Exec(ctx, cmd, execOpts...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", "", 124, fmt.Errorf("ContainerRunner.RunCommandWithOptionsDetailed() [container.go]: command timed out after %v", timeout)
		}
		return "", "", 1, fmt.Errorf("ContainerRunner.RunCommandWithOptionsDetailed() [container.go]: failed to execute command: %w", err)
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
		return "", "", 124, fmt.Errorf("ContainerRunner.RunCommandWithOptionsDetailed() [container.go]: command timed out after %v", timeout)
	}

	// Container exec combines stdout and stderr, so we return all as stdout
	return output.String(), "", exitCode, nil
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

// copyEnvMap creates a shallow copy of environment map.
func copyEnvMap(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(env))
	for key, value := range env {
		cloned[key] = value
	}

	return cloned
}

// buildExportPrefix builds shell-safe export prefix for command execution.
func buildExportPrefix(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}

	lines := make([]string, 0, len(env))
	for key, value := range env {
		lines = append(lines, fmt.Sprintf("export %s=%s;", key, shellSingleQuote(value)))
	}

	return strings.Join(lines, " ") + " "
}

// shellSingleQuote single-quotes a value for shell export assignment.
func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

var _ ContainerRunner = (*containerRunner)(nil)
