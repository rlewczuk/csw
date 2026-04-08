# Package `pkg/runner` Overview

Package `pkg/runner` provides command runners and test doubles in `pkg/runner`.

## Important files

* `runner.go` - Shared runner interfaces and config types.
* `bash.go` - Local bash command runner implementation.
* `container.go` - Testcontainers-based container command runner.
* `mock.go` - Programmable mock command runner for tests.

## Important public API objects

* `CommandRunner` - Runs commands with basic and detailed outputs.
* `ContainerRunner` - Container command runner plus lifecycle methods.
* `CommandOptions` - Per-command workdir and timeout overrides.
* `ContainerConfig` - Container image, mounts, identity, environment settings.
* `ContainerImageInfo` - Parsed image reference name and tag details.
* `ContainerIdentity` - Effective UID/GID and user/group mapping.
* `CommandExecution` - Recorded mock execution data.
* `BashRunner` - Local bash-backed command runner.
* `MockRunner` - In-memory configurable command runner mock.
* `NewBashRunner` - Creates `BashRunner` with defaults.
* `NewContainerRunner` - Creates and starts `ContainerRunner`.
* `NewMockRunner` - Creates empty `MockRunner` instance.
