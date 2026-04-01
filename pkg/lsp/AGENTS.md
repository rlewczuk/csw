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
* `DiagnosticSeverity` - Enum: SeverityError=1, SeverityWarning=2, SeverityInformation=3, SeverityHint=4
* `Location` - URI and range location in a document
* `DocumentSymbol` - Hierarchical symbol in a document
* `WorkspaceSymbol` - Symbol found in workspace search
* `CallHierarchyItem` - Item for call hierarchy operations
* `CallHierarchyIncomingCall` - Incoming call to a symbol
* `CallHierarchyOutgoingCall` - Outgoing call from a symbol
* `Position` - Zero-based line and character position
* `Range` - Start and end positions in a document
* `SymbolKind` - Enum: File=1, Module=2, Namespace=3, Package=4, Class=5, Method=6, Property=7, Field=8, Constructor=9, Enum=10, Interface=11, Function=12, Variable=13, Constant=14, String=15, Number=16, Boolean=17, Array=18, Object=19, Key=20, Null=21, EnumMember=22, Struct=23, Event=24, Operator=25, TypeParameter=26
* `SymbolTag` - Enum: SymbolTagDeprecated=1
* `MarkupKind` - Enum: PlainText="plaintext", Markdown="markdown"
* `Hover` - Hover information with contents and range
* `MarkupContent` - Formatted content with kind and value
* `MarkedString` - Code block or markdown string for hover
