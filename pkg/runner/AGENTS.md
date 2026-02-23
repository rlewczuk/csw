# pkg/runner

`pkg/runner` provides a small command-execution abstraction used by tools and other runtime components. It includes the public runner contract, a bash-based implementation, and a deterministic mock for testing.

## Major files

- `runner.go`: Public command runner API (`CommandRunner`, `CommandOptions`) for shell execution with workdir/timeout options.
- `bash.go`: Production bash runner implementation with context timeout handling and normalized exit-code behavior.
- `mock.go`: Thread-safe mock runner with programmable responses and execution-history inspection.
