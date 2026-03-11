# Package `pkg/runner` Overview

Package `pkg/runner` provides command execution abstractions for running shell commands with support for bash execution, containerized environments, and testing mocks.

## Important files

* `runner.go` - Core interfaces and configuration types
* `bash.go` - Bash command runner implementation
* `container.go` - Container-based command runner using testcontainers
* `mock.go` - Mock runner for testing with programmable responses

## Important public API objects

* `CommandRunner` - Interface for running commands with output and exit codes
* `ContainerRunner` - Interface for container-based command execution
* `BashRunner` - Executes commands using local bash shell
* `MockRunner` - Test double with programmable responses and execution tracking
* `CommandOptions` - Optional parameters for command execution (workdir, timeout)
* `ContainerConfig` - Configuration for creating container runners
* `CommandExecution` - Record of a single command execution
* `NewBashRunner` - Creates a new BashRunner instance
* `NewContainerRunner` - Creates a new ContainerRunner instance
* `NewMockRunner` - Creates a new MockRunner instance
