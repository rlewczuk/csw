package runner

import (
	"fmt"
	"sync"
)

var newContainerRunnerFactory = NewContainerRunner

// SetNewContainerRunnerFactoryForTest overrides container runner constructor for tests.
func SetNewContainerRunnerFactoryForTest(fn func(config ContainerConfig) (ContainerRunner, error)) {
	newContainerRunnerFactory = fn
}

// NewContainerRunnerFactoryForTest returns current container runner constructor.
func NewContainerRunnerFactoryForTest() func(config ContainerConfig) (ContainerRunner, error) {
	return newContainerRunnerFactory
}

// lazyContainerRunner defers creating the real container runner until first command execution.
type lazyContainerRunner struct {
	mu       sync.Mutex
	config   ContainerConfig
	identity ContainerIdentity
	image    ContainerImageInfo
	runner   ContainerRunner
	closed   bool
}

// NewLazyContainerRunner creates a container runner that starts the container on first command execution.
func NewLazyContainerRunner(config ContainerConfig) (ContainerRunner, error) {
	if config.ImageName == "" {
		return nil, fmt.Errorf("NewLazyContainerRunner() [lazy_container.go]: image name cannot be empty")
	}
	if config.UID < 0 {
		return nil, fmt.Errorf("NewLazyContainerRunner() [lazy_container.go]: UID cannot be negative")
	}
	if config.GID < 0 {
		return nil, fmt.Errorf("NewLazyContainerRunner() [lazy_container.go]: GID cannot be negative")
	}
	if config.UID > 0 && config.GID > 0 {
		if config.UserName == "" {
			return nil, fmt.Errorf("NewLazyContainerRunner() [lazy_container.go]: user name cannot be empty when UID/GID are set")
		}
		if config.GroupName == "" {
			return nil, fmt.Errorf("NewLazyContainerRunner() [lazy_container.go]: group name cannot be empty when UID/GID are set")
		}
		if config.HomeDir == "" {
			return nil, fmt.Errorf("NewLazyContainerRunner() [lazy_container.go]: home directory cannot be empty when UID/GID are set")
		}
	}

	identity := ContainerIdentity{
		UID:       config.UID,
		GID:       config.GID,
		UserName:  config.UserName,
		GroupName: config.GroupName,
		HomeDir:   config.HomeDir,
	}
	if identity.UID == 0 && identity.GID == 0 {
		identity = ContainerIdentity{UID: 0, GID: 0, UserName: "root", GroupName: "root", HomeDir: "/root"}
	}

	return &lazyContainerRunner{
		config:   config,
		identity: identity,
		image:    parseContainerImageInfo(config.ImageName),
	}, nil
}

// Identity returns planned container identity without forcing container start.
func (r *lazyContainerRunner) Identity() ContainerIdentity {
	return r.identity
}

// ImageInfo returns parsed image information without forcing container start.
func (r *lazyContainerRunner) ImageInfo() ContainerImageInfo {
	return r.image
}

// RunCommand starts the container lazily and executes command.
func (r *lazyContainerRunner) RunCommand(command string) (string, int, error) {
	runner, err := r.ensureRunner()
	if err != nil {
		return "", 1, err
	}
	return runner.RunCommand(command)
}

// RunCommandWithOptions starts the container lazily and executes command with options.
func (r *lazyContainerRunner) RunCommandWithOptions(command string, options CommandOptions) (string, int, error) {
	runner, err := r.ensureRunner()
	if err != nil {
		return "", 1, err
	}
	return runner.RunCommandWithOptions(command, options)
}

// RunCommandWithOptionsDetailed starts the container lazily and executes command with options.
func (r *lazyContainerRunner) RunCommandWithOptionsDetailed(command string, options CommandOptions) (string, string, int, error) {
	runner, err := r.ensureRunner()
	if err != nil {
		return "", "", 1, err
	}
	return runner.RunCommandWithOptionsDetailed(command, options)
}

// Close closes underlying container runner if it was started.
func (r *lazyContainerRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true
	if r.runner == nil {
		return nil
	}

	return r.runner.Close()
}

func (r *lazyContainerRunner) ensureRunner() (ContainerRunner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, fmt.Errorf("lazyContainerRunner.ensureRunner() [lazy_container.go]: container is closed")
	}
	if r.runner != nil {
		return r.runner, nil
	}

	runner, err := newContainerRunnerFactory(r.config)
	if err != nil {
		return nil, fmt.Errorf("lazyContainerRunner.ensureRunner() [lazy_container.go]: failed to start container runner: %w", err)
	}
	r.runner = runner
	return r.runner, nil
}
