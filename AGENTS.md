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

## Code Organization

- `cmd/csw` - Main CLI application with Cobra commands, CLI session execution, configuration management, and worktree cleanup functionality.
- `pkg/conf` - Configuration domain and config-store abstractions for global settings, model providers, agent roles, tool and file access policies.
- `pkg/conf/impl` - Configuration store implementations (local filesystem-based, embedded, composite, and mock config stores).
- `pkg/core` - Runtime orchestration layer for agent sessions, managing session lifecycle, prompt/tool assembly, role/model switching, and async session threading.
- `pkg/core/testfixture` - Core integration test fixtures for creating pre-configured SweSystem instances with mock dependencies.
- `pkg/logging` - Structured logging infrastructure for global runtime events and per-session logs.
- `pkg/lsp` - Language Server Protocol integration layer with JSON-RPC client, protocol DTOs, and in-memory mock.
- `pkg/models` - Model/provider abstraction layer for chat and embedding backends with provider registry, concrete clients, OAuth helpers, and tagging logic.
- `pkg/presenter` - Presenter layer connecting core session/system behavior to UI interfaces.
- `pkg/presenter/testfixture` - Presenter integration test fixtures.
- `pkg/runner` - Command execution abstractions for running shell commands with bash and containerized environments.
- `pkg/shared` - Cross-cutting utility code including patch parsing, file copy helpers, and UUIDv7 generation.
- `pkg/shared/godown` - HTML to Markdown converter with customizable options.
- `pkg/system` - Core system orchestration for managing sessions, models, tools, and CLI runtime initialization.
- `pkg/testutil` - Reusable test doubles and fixtures for integration and package tests.
- `pkg/testutil/cfg` - Integration test configuration helpers for managing test directories and feature flags.
- `pkg/testutil/fixture` - Reusable test fixture helpers for locating project paths and managing temporary directories.
- `pkg/tool` - Agent tool execution layer with tool interfaces, registry, permissions, and concrete tools for shell, todo, subagent, web fetch, and VFS operations.
- `pkg/ui` - Frontend-agnostic UI contracts and view models for presenters and interfaces.
- `pkg/ui/cli` - CLI implementations of UI interfaces for terminal-based interaction.
- `pkg/ui/cli/testfixture` - CLI integration test fixtures for setting up test environments.
- `pkg/ui/mock` - Mock implementations of UI interfaces for testing purposes.
- `pkg/vfs` - Filesystem and VCS abstractions with local/git implementations, access control, search/filter, patching, and mocks.
