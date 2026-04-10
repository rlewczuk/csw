# Package `pkg/lsp` Overview

Package `pkg/lsp` provides a Language Server Protocol client, protocol DTOs, and an in-memory mock implementation for testing. It supports diagnostics, go-to-definition, find-references, hover, document/workspace symbols, and call hierarchy operations.

## Important files

* `lsp.go` - Core LSP interface.
* `client.go` - JSON-RPC stdio LSP client.
* `dto.go` - Protocol DTOs and enums.
* `mock.go` - In-memory LSP test double.

## Important public API objects

* `LSP` - Interface for diagnostics and navigation operations.
* `Client` - Stdio JSON-RPC LSP client implementation.
* `NewClient()` - Creates client from server and workdir.
* `MockLSP` - In-memory LSP mock implementation.
* `NewMockLSP()` - Creates `MockLSP` instance.
* `CursorLocation` - File path with line and column.
* `Diagnostic` - Diagnostic message with range and severity.
* `DiagnosticSeverity` - Enum: SeverityError=1 SeverityWarning=2 SeverityInformation=3 SeverityHint=4.
* `Location` - URI and source range location.
* `Position` - Zero-based line and character position.
* `Range` - Start and end positions.
* `DocumentSymbol` - Hierarchical symbol in a document.
* `WorkspaceSymbol` - Symbol result from workspace search.
* `CallHierarchyItem` - Symbol entry for call hierarchy.
* `CallHierarchyIncomingCall` - Incoming caller with ranges.
* `CallHierarchyOutgoingCall` - Outgoing callee with ranges.
* `SymbolKind` - Enum: File..TypeParameter symbolic kinds.
* `SymbolTag` - Enum: SymbolTagDeprecated=1.
* `MarkupKind` - Enum: PlainText Markdown.
* `Hover` - Hover response with content and range.
* `MarkupContent` - Markup value with format kind.
* `MarkedString` - Hover text or code block.
