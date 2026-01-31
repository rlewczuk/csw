package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockLSPInitialization tests basic mock LSP initialization.
func TestMockLSPInitialization(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)
	require.NotNil(t, mock)

	// Should not be initialized yet
	assert.False(t, mock.initialized)

	// Initialize
	err = mock.Init(true)
	require.NoError(t, err)
	assert.True(t, mock.initialized)

	// Verify request was tracked
	requests := mock.GetReceivedRequests()
	assert.Len(t, requests, 1)
	assert.Equal(t, "initialize", requests[0].Method)
}

// TestMockLSPDiagnostics tests diagnostics functionality.
func TestMockLSPDiagnostics(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	// Set some diagnostics
	testURI := "file:///test/file.go"
	testDiags := []Diagnostic{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 10},
			},
			Severity: SeverityError,
			Message:  "Test error",
		},
		{
			Range: Range{
				Start: Position{Line: 1, Character: 0},
				End:   Position{Line: 1, Character: 5},
			},
			Severity: SeverityWarning,
			Message:  "Test warning",
		},
	}

	mock.SetDiagnostics(testURI, testDiags)

	// Get all diagnostics
	diags, err := mock.Diagnostics()
	require.NoError(t, err)
	assert.Len(t, diags, 2)

	// Verify diagnostics content
	assert.Equal(t, SeverityError, diags[0].Severity)
	assert.Equal(t, "Test error", diags[0].Message)
	assert.Equal(t, SeverityWarning, diags[1].Severity)
	assert.Equal(t, "Test warning", diags[1].Message)
}

// TestMockLSPTouchAndValidate tests file touch and validation.
func TestMockLSPTouchAndValidate(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	testPath := "/test/file.go"
	testURI := pathToURI(testPath)

	// Set diagnostics for the file
	testDiags := []Diagnostic{
		{
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
			Severity: SeverityError,
			Message:  "Syntax error",
		},
	}
	mock.SetDiagnostics(testURI, testDiags)

	// Touch file with sync=false should return nil
	diags, err := mock.TouchAndValidate(testPath, false)
	require.NoError(t, err)
	assert.Nil(t, diags)

	// Touch file with sync=true should return diagnostics
	diags, err = mock.TouchAndValidate(testPath, true)
	require.NoError(t, err)
	assert.Len(t, diags, 1)
	assert.Equal(t, "Syntax error", diags[0].Message)

	// Verify document was tracked as opened
	// Note: We called TouchAndValidate twice, so version should be 2
	mock.openDocsMu.RLock()
	version, isOpen := mock.openDocs[testURI]
	mock.openDocsMu.RUnlock()
	assert.True(t, isOpen)
	assert.Equal(t, 2, version)

	// Touch again should increment version
	_, err = mock.TouchAndValidate(testPath, false)
	require.NoError(t, err)

	mock.openDocsMu.RLock()
	version, _ = mock.openDocs[testURI]
	mock.openDocsMu.RUnlock()
	assert.Equal(t, 3, version)
}

// TestMockLSPFindDefinition tests definition finding.
func TestMockLSPFindDefinition(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	testLoc := CursorLocation{
		Path: "/test/file.go",
		Line: 10,
		Col:  5,
	}

	testLocations := []Location{
		{
			URI: "file:///test/file.go",
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
		},
	}

	mock.SetDefinition(testLoc, testLocations)

	// Find definition
	locs, err := mock.FindDefinition(testLoc)
	require.NoError(t, err)
	assert.Len(t, locs, 1)
	assert.Equal(t, "file:///test/file.go", locs[0].URI)
	assert.Equal(t, 5, locs[0].Range.Start.Line)

	// Find definition for unknown location should return empty
	unknownLoc := CursorLocation{Path: "/test/unknown.go", Line: 1, Col: 1}
	locs, err = mock.FindDefinition(unknownLoc)
	require.NoError(t, err)
	assert.Empty(t, locs)
}

// TestMockLSPFindReferences tests reference finding.
func TestMockLSPFindReferences(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	testLoc := CursorLocation{
		Path: "/test/file.go",
		Line: 3,
		Col:  1,
	}

	testLocations := []Location{
		{
			URI: "file:///test/file.go",
			Range: Range{
				Start: Position{Line: 3, Character: 1},
				End:   Position{Line: 3, Character: 5},
			},
		},
		{
			URI: "file:///test/file.go",
			Range: Range{
				Start: Position{Line: 5, Character: 10},
				End:   Position{Line: 5, Character: 14},
			},
		},
	}

	mock.SetReferences(testLoc, testLocations)

	// Find references
	locs, err := mock.FindReferences(testLoc)
	require.NoError(t, err)
	assert.Len(t, locs, 2)
	assert.Equal(t, 3, locs[0].Range.Start.Line)
	assert.Equal(t, 5, locs[1].Range.Start.Line)
}

// TestMockLSPHover tests hover functionality.
func TestMockLSPHover(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	testLoc := CursorLocation{
		Path: "/test/file.go",
		Line: 10,
		Col:  14,
	}

	mock.SetHover(testLoc, "func Helper() string", "markdown")

	// Get hover information
	text, format, err := mock.Hover(testLoc)
	require.NoError(t, err)
	assert.Equal(t, "func Helper() string", text)
	assert.Equal(t, "markdown", format)

	// Hover on unknown location should return empty
	unknownLoc := CursorLocation{Path: "/test/unknown.go", Line: 1, Col: 1}
	text, format, err = mock.Hover(unknownLoc)
	require.NoError(t, err)
	assert.Empty(t, text)
	assert.Empty(t, format)
}

// TestMockLSPDocumentSymbols tests document symbols.
func TestMockLSPDocumentSymbols(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	testPath := "/test/file.go"
	testSymbols := []DocumentSymbol{
		{
			Name: "MyStruct",
			Kind: SymbolKindStruct,
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 8, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 5, Character: 5},
				End:   Position{Line: 5, Character: 13},
			},
		},
		{
			Name: "MyFunction",
			Kind: SymbolKindFunction,
			Range: Range{
				Start: Position{Line: 10, Character: 0},
				End:   Position{Line: 12, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 10, Character: 5},
				End:   Position{Line: 10, Character: 15},
			},
		},
	}

	mock.SetDocumentSymbols(testPath, testSymbols)

	// Get document symbols
	symbols, err := mock.DocumentSymbols(testPath)
	require.NoError(t, err)
	assert.Len(t, symbols, 2)
	assert.Equal(t, "MyStruct", symbols[0].Name)
	assert.Equal(t, SymbolKindStruct, symbols[0].Kind)
	assert.Equal(t, "MyFunction", symbols[1].Name)
	assert.Equal(t, SymbolKindFunction, symbols[1].Kind)
}

// TestMockLSPWorkspaceSymbols tests workspace symbols.
func TestMockLSPWorkspaceSymbols(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	testQuery := "UniqueTest"
	testSymbols := []WorkspaceSymbol{
		{
			Name: "UniqueTestStruct",
			Kind: SymbolKindStruct,
			Location: Location{
				URI: "file:///test/file1.go",
				Range: Range{
					Start: Position{Line: 3, Character: 0},
					End:   Position{Line: 5, Character: 1},
				},
			},
		},
		{
			Name: "UniqueTestFunction",
			Kind: SymbolKindFunction,
			Location: Location{
				URI: "file:///test/file2.go",
				Range: Range{
					Start: Position{Line: 8, Character: 0},
					End:   Position{Line: 10, Character: 1},
				},
			},
		},
	}

	mock.SetWorkspaceSymbols(testQuery, testSymbols)

	// Search workspace symbols
	symbols, err := mock.WorkspaceSymbols(testQuery)
	require.NoError(t, err)
	assert.Len(t, symbols, 2)
	assert.Equal(t, "UniqueTestStruct", symbols[0].Name)
	assert.Equal(t, "UniqueTestFunction", symbols[1].Name)

	// Search for unknown query should return empty
	symbols, err = mock.WorkspaceSymbols("UnknownQuery")
	require.NoError(t, err)
	assert.Empty(t, symbols)
}

// TestMockLSPCallHierarchy tests call hierarchy functionality.
func TestMockLSPCallHierarchy(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	testLoc := CursorLocation{
		Path: "/test/file.go",
		Line: 5,
		Col:  5,
	}

	testItems := []CallHierarchyItem{
		{
			Name: "Helper",
			Kind: SymbolKindFunction,
			URI:  "file:///test/file.go",
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 7, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 5, Character: 5},
				End:   Position{Line: 5, Character: 11},
			},
		},
	}

	mock.SetCallHierarchy(testLoc, testItems)

	// Prepare call hierarchy
	items, err := mock.CallHierarchy(testLoc)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "Helper", items[0].Name)
	assert.Equal(t, SymbolKindFunction, items[0].Kind)
}

// TestMockLSPIncomingCalls tests incoming calls functionality.
func TestMockLSPIncomingCalls(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	helperItem := CallHierarchyItem{
		Name: "Helper",
		Kind: SymbolKindFunction,
		URI:  "file:///test/file.go",
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 7, Character: 1},
		},
		SelectionRange: Range{
			Start: Position{Line: 5, Character: 5},
			End:   Position{Line: 5, Character: 11},
		},
	}

	testCalls := []CallHierarchyIncomingCall{
		{
			From: CallHierarchyItem{
				Name: "Caller",
				Kind: SymbolKindFunction,
				URI:  "file:///test/file.go",
				Range: Range{
					Start: Position{Line: 10, Character: 0},
					End:   Position{Line: 12, Character: 1},
				},
				SelectionRange: Range{
					Start: Position{Line: 10, Character: 5},
					End:   Position{Line: 10, Character: 11},
				},
			},
			FromRanges: []Range{
				{
					Start: Position{Line: 11, Character: 1},
					End:   Position{Line: 11, Character: 7},
				},
			},
		},
	}

	mock.SetIncomingCalls("Helper", testCalls)

	// Get incoming calls
	calls, err := mock.IncomingCalls(helperItem)
	require.NoError(t, err)
	assert.Len(t, calls, 1)
	assert.Equal(t, "Caller", calls[0].From.Name)
	assert.Len(t, calls[0].FromRanges, 1)
}

// TestMockLSPOutgoingCalls tests outgoing calls functionality.
func TestMockLSPOutgoingCalls(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	callerItem := CallHierarchyItem{
		Name: "Caller",
		Kind: SymbolKindFunction,
		URI:  "file:///test/file.go",
		Range: Range{
			Start: Position{Line: 10, Character: 0},
			End:   Position{Line: 12, Character: 1},
		},
		SelectionRange: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 11},
		},
	}

	testCalls := []CallHierarchyOutgoingCall{
		{
			To: CallHierarchyItem{
				Name: "Helper",
				Kind: SymbolKindFunction,
				URI:  "file:///test/file.go",
				Range: Range{
					Start: Position{Line: 5, Character: 0},
					End:   Position{Line: 7, Character: 1},
				},
				SelectionRange: Range{
					Start: Position{Line: 5, Character: 5},
					End:   Position{Line: 5, Character: 11},
				},
			},
			FromRanges: []Range{
				{
					Start: Position{Line: 11, Character: 1},
					End:   Position{Line: 11, Character: 7},
				},
			},
		},
	}

	mock.SetOutgoingCalls("Caller", testCalls)

	// Get outgoing calls
	calls, err := mock.OutgoingCalls(callerItem)
	require.NoError(t, err)
	assert.Len(t, calls, 1)
	assert.Equal(t, "Helper", calls[0].To.Name)
	assert.Len(t, calls[0].FromRanges, 1)
}

// TestMockLSPRequestTracking tests request tracking functionality.
func TestMockLSPRequestTracking(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	err = mock.Init(true)
	require.NoError(t, err)

	// Clear init request
	mock.ClearReceivedRequests()

	// Make various requests
	testLoc := CursorLocation{Path: "/test/file.go", Line: 1, Col: 1}
	_, _ = mock.FindDefinition(testLoc)
	_, _ = mock.FindReferences(testLoc)
	_, _, _ = mock.Hover(testLoc)
	_, _ = mock.DocumentSymbols("/test/file.go")
	_, _ = mock.WorkspaceSymbols("test")

	// Verify all requests were tracked
	requests := mock.GetReceivedRequests()
	assert.Len(t, requests, 5)
	assert.Equal(t, "textDocument/definition", requests[0].Method)
	assert.Equal(t, "textDocument/references", requests[1].Method)
	assert.Equal(t, "textDocument/hover", requests[2].Method)
	assert.Equal(t, "textDocument/documentSymbol", requests[3].Method)
	assert.Equal(t, "workspace/symbol", requests[4].Method)

	// Clear and verify
	mock.ClearReceivedRequests()
	requests = mock.GetReceivedRequests()
	assert.Empty(t, requests)
}

// TestMockLSPUninitializedError tests that methods return error when not initialized.
func TestMockLSPUninitializedError(t *testing.T) {
	mock, err := NewMockLSP("/test/path")
	require.NoError(t, err)

	testLoc := CursorLocation{Path: "/test/file.go", Line: 1, Col: 1}

	// All methods should return error when not initialized
	_, err = mock.TouchAndValidate("/test/file.go", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = mock.Diagnostics()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = mock.FindDefinition(testLoc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = mock.FindReferences(testLoc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, _, err = mock.Hover(testLoc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = mock.DocumentSymbols("/test/file.go")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = mock.WorkspaceSymbols("test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = mock.CallHierarchy(testLoc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	item := CallHierarchyItem{Name: "test"}
	_, err = mock.IncomingCalls(item)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = mock.OutgoingCalls(item)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
