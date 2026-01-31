package lsp

type CursorLocation struct {
	Path string
	Line int
	Col  int
}

// LSP represents an interface for interacting with the language server protocol to analyze and validate source code files.
type LSP interface {
	// Init initializes the LSP client. If sync is true, it will block until the server is ready.
	Init(sync bool) error

	// TouchAndValidate touches the file at the given path and returns any diagnostics latest changes may have introduced.
	// If sync is true, it will block until the server responds with the validation results.
	// If sync is false, it will return immediately with nil results.
	// Result is the same as Diagnostics() but includes results from latest changes if sync is true.
	TouchAndValidate(path string, sync bool) ([]Diagnostic, error)

	// Diagnostics returns all current diagnostics known to the LSP client.
	// Client listens for textDocument/publishDiagnostics notifications and updates its internal state.
	// Client is responsible for managing diagnostics and their expiration.
	Diagnostics() ([]Diagnostic, error)

	// FindDefinition finds the definition of the symbol at the given location.
	// (calls textDocument/definition LSP endpoint)
	FindDefinition(loc CursorLocation) ([]Location, error)

	// FindReferences finds all references to the symbol at the given location.
	// (calls textDocument/references LSP endpoint)
	FindReferences(loc CursorLocation) ([]Location, error)

	// Hover returns hover information for the symbol at the given location.
	// (calls textDocument/hover LSP endpoint)
	// Returns documentation text, documentation format (e.g. "markdown", "plaintext"), and error.
	// Returns empty string and no error if there is no hover information available.
	Hover(loc CursorLocation) (string, string, error)

	// DocumentSymbols returns symbols for the given document.
	// (calls textDocument/documentSymbol LSP endpoint)
	DocumentSymbols(path string) ([]DocumentSymbol, error)

	// WorkspaceSymbols returns symbols for the given query in the current workspace.
	// (calls workspace/symbol LSP endpoint)
	WorkspaceSymbols(query string) ([]WorkspaceSymbol, error)

	// CallHierarchy prepares the call hierarchy for the symbol at the given location.
	// (calls textDocument/prepareCallHierarchy LSP endpoint)
	// Returns a list of CallHierarchyItem representing the symbol(s) at the location.
	CallHierarchy(loc CursorLocation) ([]CallHierarchyItem, error)

	// IncomingCalls returns incoming calls for the given call hierarchy item.
	// (calls callHierarchy/incomingCalls LSP endpoint)
	// Returns a list of CallHierarchyIncomingCall representing callers of the item.
	IncomingCalls(item CallHierarchyItem) ([]CallHierarchyIncomingCall, error)

	// OutgoingCalls returns outgoing calls for the given call hierarchy item.
	// (calls callHierarchy/outgoingCalls LSP endpoint)
	// Returns a list of CallHierarchyOutgoingCall representing callees from the item.
	OutgoingCalls(item CallHierarchyItem) ([]CallHierarchyOutgoingCall, error)
}
