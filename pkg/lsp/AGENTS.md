# Package `pkg/lsp` Overview

Package `pkg/lsp` provides a lightweight Language Server Protocol integration layer. It exposes an LSP interface used by tools, a concrete JSON-RPC stdio client, protocol DTOs, and a configurable in-memory mock for tests.

## Important files

* `lsp.go` - LSP interface for diagnostics, navigation, symbols, hover, and call hierarchy
* `client.go` - Language server client with JSON-RPC and process lifecycle management
* `dto.go` - LSP protocol data types and enums
* `mock.go` - In-memory LSP test double with configurable responses

## Important public API objects

* `LSP` - Interface for language server protocol operations
* `Client` - Concrete LSP client using JSON-RPC over stdio
* `NewClient()` - Creates a new LSP client instance
* `MockLSP` - Mock LSP implementation for testing
* `NewMockLSP()` - Creates a new mock LSP instance
* `CursorLocation` - Position in a file (path, line, column)
* `Diagnostic` - LSP diagnostic message with severity and range
* `Location` - URI and range location in a document
* `DocumentSymbol` - Hierarchical symbol in a document
* `WorkspaceSymbol` - Symbol found in workspace search
* `CallHierarchyItem` - Item for call hierarchy operations
* `Position` - Zero-based line and character position
* `Range` - Start and end positions in a document
