# pkg/lsp

`pkg/lsp` provides a lightweight Language Server Protocol integration layer. It exposes an LSP interface used by tools, a concrete JSON-RPC stdio client, protocol DTOs, and a configurable in-memory mock for tests.

## Major files

- `lsp.go`: Public LSP API (`LSP` interface) for diagnostics, navigation, symbols, hover, and call hierarchy operations.
- `client.go`: Language server client implementation with process lifecycle, JSON-RPC request/response handling, and diagnostics caching.
- `dto.go`: LSP protocol data types and enums used across client and mock implementations.
- `mock.go`: In-memory `LSP` test double with configurable responses and request tracking.
