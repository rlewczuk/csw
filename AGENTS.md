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
- when running tests, set gocache to local tmp dir: `GOCACHE="$PWD/tmp/.gocache" GOMODCACHE="$PWD/tmp/.gomodcache"`

Other rules:
- When generating summary after performing a task, DO NOT save it to file, just put it in chat.

## Code Organization

- `cmd/csw`: Main CLI application (root command, bootstrap, CLI/TUI execution, provider/role/tool config commands).
- `pkg/conf`: Configuration domain and config-store implementations (local, embedded defaults, composite, mock).
- `pkg/core`: Session runtime orchestration (`SweSystem`, `SweSession`, threading, prompts, role registry, commit-message generation).
- `pkg/gtv`: Terminal UI primitives (screen/event contracts, themes, and screen test helpers).
- `pkg/logging`: Structured logging infrastructure for global/session/LLM logs and test log capture.
- `pkg/lsp`: LSP abstraction and client implementation with DTOs and in-memory mock.
- `pkg/models`: Model/provider interfaces, provider registry, concrete provider clients, OAuth helpers, tags, and mocks.
- `pkg/presenter`: Presenter layer connecting core session output/input with UI interfaces.
- `pkg/runner`: Command runner abstraction with bash implementation and mock.
- `pkg/sandbox`: Placeholder for sandbox-related logic (currently no non-test implementation files).
- `pkg/shared`: Shared utilities (custom patch parsing, file copy helpers, UUIDv7 generation).
- `pkg/testinteg`: Placeholder for integration-test support code (currently no non-test implementation files).
- `pkg/testutil`: Reusable test helpers and mocks (session output handler and mock HTTP server).
- `pkg/tool`: Tool contracts, registry, permissions, and concrete tools (bash, todo, VFS read/write/edit/patch/grep).
- `pkg/ui`: UI contracts and view models for app/chat presentation flows.
- `pkg/vfs`: Filesystem and VCS interfaces plus local/git implementations, access control, search/filter, patching, and mocks.
