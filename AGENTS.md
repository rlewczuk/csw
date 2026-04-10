# Agent Guidelines for Codesnort SWE

## Build & Test Commands
- Build: `go build ./cmd/csw`
- Test all: `go test ./...`
- Test single package: `go test ./test -v`
- Test single function: `go test ./test -v -run TestAgentCoreInitialization`
- Run main: `go run ./cmd/csw`
- Before running tests, execute `tmpclean.sh` file in project root.

When you need to generate and run a test script, or generate and use some temporary file, use `./tmp` directory inside project root, not `/tmp` or other locations.

## Code Style
- Module: `github.com/codesnort/codesnort-swe`
- Go version: 1.25.5
- Imports: Standard library first, then blank line, then external packages, then local packages
- Package structure: `cmd/` for binaries, `internal/` for private code, `pkg/` for public APIs, `test/` for tests
- Naming: Interface names end with interface purpose (e.g., `ChatModel`, `SweSystem`), not generic `I` prefix
- Error handling: Return errors explicitly, use `error` as last return value
- Comments: Document all exported types and functions with comments starting with the name
- Types: Use explicit types for constants (e.g., `type ChatRole string`)
- Context: Pass `context.Context` as first parameter for operations that may block or need cancellation
- Try to be straighforward, avoid generating extra wrapper functions if it is not really necessary (i.e. at least three or more call sites for wrapper);
- Use standard library as long as possible, avoid adding dependencies if it is not really necessary;
- DO NOT use scripts to edit files, always edit files manually; if there are too many changes, just edit bigger chunks manually;
- When returning errors, always add function name and file name to error message; if function is a receiver, add 'Type.Method()' as a function name;
- Before implementing any algorithm or utility function, always check if golang standard library already provides it;
- When adding new global object (eg. function, method, struct, interface, type, constant etc.), add short doc comment describing what it is and what it does;
  - comment should be compliant with godoc format (eg. start comment with object name)
  - when adding interface, also add godoc comment to each method describing what it is and what it does;
- Keep source files focused on a given functionality or task, do not split logic of given functionality into multiple files unless it is really necessary (i.e. functionality spans across several layers);
  - even in such case try to keep logic of given functionality in one file, by designing it in such way, that other files will be limited to contain plumbing code for this functionality
- Avoid adding code to very big files (more than 1000 lines)
  - If given wile would exceed size threshold, consider finding consistent piece of functionality inside this file that can be split to a new file

Rules regarding implementing tests:
- Testing: Use table-driven tests with `t.Run()` for subtests
- Use testify library for assertions, avoid using manual `if` statements
- Avoid using mock libraries, try using real implementation if it's possible to use them in clearly defined test fixture without external dependencies, use test doubles instead
- if test double implementation for given interface is not available, implement one;
- following test double implementations are available:
  - `vfs.VFS` test double is implemented in `pkg/vfs/mock.go`;
  - `models.ModelProvider` test double is implemented in `pkg/models/mock/mock.go`;
- always run tests with timeout of 60 seconds;
  - but when running integration tests, extend duration to 300 seconds;
- prefer writing test exposing issue being solved before fixing it;
- when asked to make integration test conditional based on `_integ/xxx.enabled` file, always use `TestEnabled` function from `pkg/testutil/cfg/integ.go`, never implement it manually;

Other rules:
- When generating summary after performing a task, DO NOT save it to file, just put it in chat.
- this is big project, to not list all existing files, always use scoped search and use hints from `AGENTS.md` files across project;

## Packages Overview

* `cmd/csw` - CSW CLI with RunCommand, ProviderCommand, and config helpers
* `pkg/apis` - VFS and VCS abstraction interfaces
* `pkg/commands` - Slash command parsing with Command and Invocation
* `pkg/conf` - Config models and ConfigStore/WritableConfigStore interfaces
* `pkg/conf/impl` - Config store implementations: composite, embedded, local, mock
* `pkg/conf/impl/conf/tools` - Embedded tool directory names constant
* `pkg/core` - Session orchestration with SweSession, SessionThread, TaskManager
* `pkg/core/testfixture` - SweSystem test fixtures with SweSystemFixture
* `pkg/io` - Text and JSONL session input/output adapters
* `pkg/logging` - Structured JSONL logging with session and LLM loggers
* `pkg/lsp` - LSP client, protocol DTOs, and MockLSP
* `pkg/mcp` - MCP clients, Manager, and tool bridge
* `pkg/models` - Model/provider abstractions with ChatModel, ProviderRegistry
* `pkg/runner` - BashRunner and ContainerRunner command executors
* `pkg/shared` - File copy, formatting, template, UUIDv7 helpers
* `pkg/shared/godown` - HTML to Markdown converter with custom rules
* `pkg/system` - SweSystem orchestration and runtime bootstrapping
* `pkg/testutil` - MockHTTPServer and MockSessionOutputHandler test utilities
* `pkg/testutil/cfg` - Integration test config and feature flag helpers
* `pkg/testutil/fixture` - Project root and temporary directory helpers
* `pkg/tool` - Tool registry and built-in VFS, bash, task tools
* `pkg/vcs` - GitVCS and NullVCS implementations
* `pkg/vfs` - LocalVFS, access control, glob/grep, patch, shadow

## Code Organization

- `cmd/csw` - Main CLI application with Cobra commands, subcommands for run, provider, role, tool, hook, mcp, task, and worktree cleanup.
- `pkg/apis` - Core VFS and VCS abstraction interfaces and common errors.
- `pkg/commands` - Slash command parsing, template expansion, and prompt file loading.
- `pkg/conf` - Configuration domain models, store interfaces, and merge helpers.
- `pkg/conf/impl` - Concrete config store implementations: composite, embedded, local, and mock.
- `pkg/conf/impl/conf/tools` - Embedded tool directory names for bundled configuration.
- `pkg/core` - Runtime orchestration layer for agent sessions, threads, prompts, hooks, tasks, and summaries.
- `pkg/core/testfixture` - Preconfigured SweSystem test fixtures for core integration tests.
- `pkg/io` - Text and JSONL input/output adapters bridging external streams to session threads.
- `pkg/logging` - Structured JSONL logging infrastructure for global runtime and per-session events.
- `pkg/lsp` - Language Server Protocol integration with JSON-RPC client, DTOs, and in-memory mock.
- `pkg/mcp` - MCP client for external MCP servers via stdio and HTTP transports.
- `pkg/models` - Model/provider abstraction layer for chat and embedding backends.
- `pkg/runner` - Command execution abstractions for bash and containerized environments.
- `pkg/shared` - Cross-cutting utilities including file copy, UUIDv7, and HTML-to-Markdown conversion.
- `pkg/shared/godown` - HTML to Markdown converter with customizable options and custom tag rules.
- `pkg/system` - Core system orchestration for building, running, and managing agent sessions.
- `pkg/testutil` - Reusable test doubles and fixtures for integration and package tests.
- `pkg/testutil/cfg` - Integration test configuration helpers for feature flags and temp directories.
- `pkg/testutil/fixture` - Reusable test fixture helpers for locating project paths and managing temp dirs.
- `pkg/tool` - Agent tool execution layer with registry, permissions, and concrete tools.
- `pkg/vcs` - VCS interface implementations for git repositories and null/no-op version control.
- `pkg/vfs` - Filesystem and VCS abstractions with local implementations, access control, search, and patching.
