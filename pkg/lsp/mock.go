package lsp

import (
	"fmt"
	"sync"
)

// MockLSP is a mock implementation of the LSP interface for testing purposes.
// It does not use external binaries and returns predefined responses.
type MockLSP struct {
	// Configuration
	workingDir string

	// State
	initialized bool
	initMu      sync.Mutex

	// Open documents tracking
	openDocs   map[string]int // URI -> version
	openDocsMu sync.RWMutex

	// Predefined responses
	diagnosticsResponses map[string][]Diagnostic                // URI -> diagnostics
	definitionResponses  map[string][]Location                  // key -> locations
	referencesResponses  map[string][]Location                  // key -> locations
	hoverResponses       map[string]*HoverResponse              // key -> hover response
	documentSymbolsResp  map[string][]DocumentSymbol            // path -> symbols
	workspaceSymbolsResp map[string][]WorkspaceSymbol           // query -> symbols
	callHierarchyResp    map[string][]CallHierarchyItem         // key -> items
	incomingCallsResp    map[string][]CallHierarchyIncomingCall // key -> calls
	outgoingCallsResp    map[string][]CallHierarchyOutgoingCall // key -> calls

	responsesMu sync.RWMutex

	// Request tracking for verification
	receivedRequests []ReceivedRequest
	requestsMu       sync.RWMutex
}

// HoverResponse represents a hover response with text and format.
type HoverResponse struct {
	Text   string
	Format string
}

// ReceivedRequest represents a request received by the mock LSP server.
type ReceivedRequest struct {
	Method string
	Params interface{}
}

// NewMockLSP creates a new mock LSP server.
func NewMockLSP(workingDir string) (*MockLSP, error) {
	if workingDir == "" {
		return nil, fmt.Errorf("NewMockLSP: workingDir is empty")
	}

	return &MockLSP{
		workingDir:           workingDir,
		openDocs:             make(map[string]int),
		diagnosticsResponses: make(map[string][]Diagnostic),
		definitionResponses:  make(map[string][]Location),
		referencesResponses:  make(map[string][]Location),
		hoverResponses:       make(map[string]*HoverResponse),
		documentSymbolsResp:  make(map[string][]DocumentSymbol),
		workspaceSymbolsResp: make(map[string][]WorkspaceSymbol),
		callHierarchyResp:    make(map[string][]CallHierarchyItem),
		incomingCallsResp:    make(map[string][]CallHierarchyIncomingCall),
		outgoingCallsResp:    make(map[string][]CallHierarchyOutgoingCall),
		receivedRequests:     make([]ReceivedRequest, 0),
	}, nil
}

// Init initializes the mock LSP client.
func (m *MockLSP) Init(sync bool) error {
	m.initMu.Lock()
	defer m.initMu.Unlock()

	m.initialized = true
	m.trackRequest("initialize", nil)

	return nil
}

// TouchAndValidate touches a file and returns predefined diagnostics.
func (m *MockLSP) TouchAndValidate(path string, sync bool) ([]Diagnostic, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.TouchAndValidate: client not initialized")
	}

	uri := pathToURI(path)

	// Track document opening/updating
	m.openDocsMu.Lock()
	version, isOpen := m.openDocs[uri]
	if !isOpen {
		m.openDocs[uri] = 1
		m.trackRequest("textDocument/didOpen", map[string]interface{}{"uri": uri})
	} else {
		m.openDocs[uri] = version + 1
		m.trackRequest("textDocument/didChange", map[string]interface{}{"uri": uri, "version": version + 1})
	}
	m.openDocsMu.Unlock()

	if !sync {
		return nil, nil
	}

	// Return predefined diagnostics for this URI
	m.responsesMu.RLock()
	diags, ok := m.diagnosticsResponses[uri]
	m.responsesMu.RUnlock()

	if !ok {
		return []Diagnostic{}, nil
	}

	return diags, nil
}

// Diagnostics returns all predefined diagnostics.
func (m *MockLSP) Diagnostics() ([]Diagnostic, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.Diagnostics: client not initialized")
	}

	m.trackRequest("diagnostics", nil)

	m.responsesMu.RLock()
	defer m.responsesMu.RUnlock()

	var all []Diagnostic
	for _, diags := range m.diagnosticsResponses {
		all = append(all, diags...)
	}

	return all, nil
}

// FindDefinition returns predefined definition locations.
func (m *MockLSP) FindDefinition(loc CursorLocation) ([]Location, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.FindDefinition: client not initialized")
	}

	key := formatLocationKey(loc)
	m.trackRequest("textDocument/definition", loc)

	m.responsesMu.RLock()
	locations, ok := m.definitionResponses[key]
	m.responsesMu.RUnlock()

	if !ok {
		return []Location{}, nil
	}

	return locations, nil
}

// FindReferences returns predefined reference locations.
func (m *MockLSP) FindReferences(loc CursorLocation) ([]Location, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.FindReferences: client not initialized")
	}

	key := formatLocationKey(loc)
	m.trackRequest("textDocument/references", loc)

	m.responsesMu.RLock()
	locations, ok := m.referencesResponses[key]
	m.responsesMu.RUnlock()

	if !ok {
		return []Location{}, nil
	}

	return locations, nil
}

// Hover returns predefined hover information.
func (m *MockLSP) Hover(loc CursorLocation) (string, string, error) {
	if !m.initialized {
		return "", "", fmt.Errorf("MockLSP.Hover: client not initialized")
	}

	key := formatLocationKey(loc)
	m.trackRequest("textDocument/hover", loc)

	m.responsesMu.RLock()
	hover, ok := m.hoverResponses[key]
	m.responsesMu.RUnlock()

	if !ok || hover == nil {
		return "", "", nil
	}

	return hover.Text, hover.Format, nil
}

// DocumentSymbols returns predefined document symbols.
func (m *MockLSP) DocumentSymbols(path string) ([]DocumentSymbol, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.DocumentSymbols: client not initialized")
	}

	m.trackRequest("textDocument/documentSymbol", path)

	m.responsesMu.RLock()
	symbols, ok := m.documentSymbolsResp[path]
	m.responsesMu.RUnlock()

	if !ok {
		return []DocumentSymbol{}, nil
	}

	return symbols, nil
}

// WorkspaceSymbols returns predefined workspace symbols.
func (m *MockLSP) WorkspaceSymbols(query string) ([]WorkspaceSymbol, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.WorkspaceSymbols: client not initialized")
	}

	m.trackRequest("workspace/symbol", query)

	m.responsesMu.RLock()
	symbols, ok := m.workspaceSymbolsResp[query]
	m.responsesMu.RUnlock()

	if !ok {
		return []WorkspaceSymbol{}, nil
	}

	return symbols, nil
}

// CallHierarchy returns predefined call hierarchy items.
func (m *MockLSP) CallHierarchy(loc CursorLocation) ([]CallHierarchyItem, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.CallHierarchy: client not initialized")
	}

	key := formatLocationKey(loc)
	m.trackRequest("textDocument/prepareCallHierarchy", loc)

	m.responsesMu.RLock()
	items, ok := m.callHierarchyResp[key]
	m.responsesMu.RUnlock()

	if !ok {
		return []CallHierarchyItem{}, nil
	}

	return items, nil
}

// IncomingCalls returns predefined incoming calls.
func (m *MockLSP) IncomingCalls(item CallHierarchyItem) ([]CallHierarchyIncomingCall, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.IncomingCalls: client not initialized")
	}

	key := item.Name
	m.trackRequest("callHierarchy/incomingCalls", item)

	m.responsesMu.RLock()
	calls, ok := m.incomingCallsResp[key]
	m.responsesMu.RUnlock()

	if !ok {
		return []CallHierarchyIncomingCall{}, nil
	}

	return calls, nil
}

// OutgoingCalls returns predefined outgoing calls.
func (m *MockLSP) OutgoingCalls(item CallHierarchyItem) ([]CallHierarchyOutgoingCall, error) {
	if !m.initialized {
		return nil, fmt.Errorf("MockLSP.OutgoingCalls: client not initialized")
	}

	key := item.Name
	m.trackRequest("callHierarchy/outgoingCalls", item)

	m.responsesMu.RLock()
	calls, ok := m.outgoingCallsResp[key]
	m.responsesMu.RUnlock()

	if !ok {
		return []CallHierarchyOutgoingCall{}, nil
	}

	return calls, nil
}

// SetDiagnostics sets predefined diagnostics for a URI.
func (m *MockLSP) SetDiagnostics(uri string, diags []Diagnostic) {
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.diagnosticsResponses[uri] = diags
}

// SetDefinition sets predefined definition response for a location.
func (m *MockLSP) SetDefinition(loc CursorLocation, locations []Location) {
	key := formatLocationKey(loc)
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.definitionResponses[key] = locations
}

// SetReferences sets predefined references response for a location.
func (m *MockLSP) SetReferences(loc CursorLocation, locations []Location) {
	key := formatLocationKey(loc)
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.referencesResponses[key] = locations
}

// SetHover sets predefined hover response for a location.
func (m *MockLSP) SetHover(loc CursorLocation, text, format string) {
	key := formatLocationKey(loc)
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.hoverResponses[key] = &HoverResponse{
		Text:   text,
		Format: format,
	}
}

// SetDocumentSymbols sets predefined document symbols for a path.
func (m *MockLSP) SetDocumentSymbols(path string, symbols []DocumentSymbol) {
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.documentSymbolsResp[path] = symbols
}

// SetWorkspaceSymbols sets predefined workspace symbols for a query.
func (m *MockLSP) SetWorkspaceSymbols(query string, symbols []WorkspaceSymbol) {
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.workspaceSymbolsResp[query] = symbols
}

// SetCallHierarchy sets predefined call hierarchy items for a location.
func (m *MockLSP) SetCallHierarchy(loc CursorLocation, items []CallHierarchyItem) {
	key := formatLocationKey(loc)
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.callHierarchyResp[key] = items
}

// SetIncomingCalls sets predefined incoming calls for an item name.
func (m *MockLSP) SetIncomingCalls(itemName string, calls []CallHierarchyIncomingCall) {
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.incomingCallsResp[itemName] = calls
}

// SetOutgoingCalls sets predefined outgoing calls for an item name.
func (m *MockLSP) SetOutgoingCalls(itemName string, calls []CallHierarchyOutgoingCall) {
	m.responsesMu.Lock()
	defer m.responsesMu.Unlock()
	m.outgoingCallsResp[itemName] = calls
}

// GetReceivedRequests returns all received requests for verification.
func (m *MockLSP) GetReceivedRequests() []ReceivedRequest {
	m.requestsMu.RLock()
	defer m.requestsMu.RUnlock()

	// Return a copy to prevent concurrent modification
	result := make([]ReceivedRequest, len(m.receivedRequests))
	copy(result, m.receivedRequests)
	return result
}

// ClearReceivedRequests clears the received requests history.
func (m *MockLSP) ClearReceivedRequests() {
	m.requestsMu.Lock()
	defer m.requestsMu.Unlock()
	m.receivedRequests = make([]ReceivedRequest, 0)
}

// trackRequest tracks a received request.
func (m *MockLSP) trackRequest(method string, params interface{}) {
	m.requestsMu.Lock()
	defer m.requestsMu.Unlock()
	m.receivedRequests = append(m.receivedRequests, ReceivedRequest{
		Method: method,
		Params: params,
	})
}

// Close closes the mock LSP client (no-op for mock).
func (m *MockLSP) Close() error {
	return nil
}

// getDiagnosticsForURI returns diagnostics for a specific URI.
// This method is for test compatibility with Client.getDiagnosticsForURI.
func (m *MockLSP) getDiagnosticsForURI(uri string) []Diagnostic {
	m.responsesMu.RLock()
	defer m.responsesMu.RUnlock()

	diags, ok := m.diagnosticsResponses[uri]
	if !ok {
		return nil
	}

	return diags
}

// formatLocationKey formats a cursor location into a key for lookups.
func formatLocationKey(loc CursorLocation) string {
	return fmt.Sprintf("%s:%d:%d", loc.Path, loc.Line, loc.Col)
}
