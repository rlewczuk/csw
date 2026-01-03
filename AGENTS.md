# Agent Guidelines for Codesnort SWE

## Build & Test Commands
- Build: `go build ./cmd/csw`
- Test all: `go test ./...`
- Test single package: `go test ./test -v`
- Test single function: `go test ./test -v -run TestAgentCoreInitialization`
- Run main: `go run ./cmd/csw`

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
 
Packages implemented:
- `pkg/core`: Core agent logic
- `pkg/models`: Model provider and model interfaces
- `pkg/tool`: Tool interfaces and implementations
- `pkg/vfs`: Virtual filesystem interfaces and implementations

Rules regarding implementing tests:
- Testing: Use table-driven tests with `t.Run()` for subtests
- Use testify library for assertions, avoid using manual `if` statements
- Avoid using mock libraries, try using real implementation if it's possible to use them in clearly defined test fixture without external dependencies, use test doubles instead
- if test double implementation for given interface is not available, implement one;
- following test double implementations are available:
  - `vfs.VFS` test double is implemented in `pkg/vfs/mock.go`;
  - `models.ModelProvider` test double is implemented in `pkg/models/mock/mock.go`;
