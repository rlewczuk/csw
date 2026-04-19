package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/testutil/cfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLSPClientDocumentSymbols tests document symbols functionality.
func TestLSPClientDocumentSymbols(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
	require.NoError(t, err)

	// Create a temporary file in tmp directory
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_symbols_*")
	testFile := filepath.Join(tmpDir, "test_symbols.go")
	testContent := `package main

import "fmt"

// MyStruct is a sample struct.
type MyStruct struct {
	Field1 int
	Field2 string
}

// MyMethod is a method on MyStruct.
func (m *MyStruct) MyMethod() {
	fmt.Println("method")
}

// MyFunction is a standalone function.
func MyFunction() {
	fmt.Println("function")
}

const MyConstant = 42

var MyVariable = "hello"

func main() {
	s := MyStruct{Field1: 1, Field2: "test"}
	s.MyMethod()
	MyFunction()
}
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	sleepForLSP(client, 1*time.Second)

	// Set up mock response if using mock LSP
	if mock := asMock(client); mock != nil {
		mock.SetDocumentSymbols(testFile, []DocumentSymbol{
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
				Children: []DocumentSymbol{
					{
						Name: "Field1",
						Kind: SymbolKindField,
						Range: Range{
							Start: Position{Line: 6, Character: 1},
							End:   Position{Line: 6, Character: 14},
						},
						SelectionRange: Range{
							Start: Position{Line: 6, Character: 1},
							End:   Position{Line: 6, Character: 7},
						},
					},
					{
						Name: "Field2",
						Kind: SymbolKindField,
						Range: Range{
							Start: Position{Line: 7, Character: 1},
							End:   Position{Line: 7, Character: 17},
						},
						SelectionRange: Range{
							Start: Position{Line: 7, Character: 1},
							End:   Position{Line: 7, Character: 7},
						},
					},
				},
			},
			{
				Name: "MyMethod",
				Kind: SymbolKindMethod,
				Range: Range{
					Start: Position{Line: 11, Character: 0},
					End:   Position{Line: 13, Character: 1},
				},
				SelectionRange: Range{
					Start: Position{Line: 11, Character: 19},
					End:   Position{Line: 11, Character: 27},
				},
			},
			{
				Name: "MyFunction",
				Kind: SymbolKindFunction,
				Range: Range{
					Start: Position{Line: 16, Character: 0},
					End:   Position{Line: 18, Character: 1},
				},
				SelectionRange: Range{
					Start: Position{Line: 16, Character: 5},
					End:   Position{Line: 16, Character: 15},
				},
			},
			{
				Name: "MyConstant",
				Kind: SymbolKindConstant,
				Range: Range{
					Start: Position{Line: 20, Character: 0},
					End:   Position{Line: 20, Character: 22},
				},
				SelectionRange: Range{
					Start: Position{Line: 20, Character: 6},
					End:   Position{Line: 20, Character: 16},
				},
			},
			{
				Name: "MyVariable",
				Kind: SymbolKindVariable,
				Range: Range{
					Start: Position{Line: 22, Character: 0},
					End:   Position{Line: 22, Character: 23},
				},
				SelectionRange: Range{
					Start: Position{Line: 22, Character: 4},
					End:   Position{Line: 22, Character: 14},
				},
			},
			{
				Name: "main",
				Kind: SymbolKindFunction,
				Range: Range{
					Start: Position{Line: 24, Character: 0},
					End:   Position{Line: 28, Character: 1},
				},
				SelectionRange: Range{
					Start: Position{Line: 24, Character: 5},
					End:   Position{Line: 24, Character: 9},
				},
			},
		})
	}

	// Get document symbols
	symbols, err := client.DocumentSymbols(testFile)
	require.NoError(t, err)
	require.NotEmpty(t, symbols, "Expected to find symbols in document")

	// Helper function to find symbol by name (recursive)
	var findSymbol func(symbols []DocumentSymbol, name string) *DocumentSymbol
	findSymbol = func(symbols []DocumentSymbol, name string) *DocumentSymbol {
		for i := range symbols {
			if symbols[i].Name == name {
				return &symbols[i]
			}
			// Search in children
			if len(symbols[i].Children) > 0 {
				if found := findSymbol(symbols[i].Children, name); found != nil {
					return found
				}
			}
		}
		return nil
	}

	// Verify we have expected symbols
	// We should at least find: MyStruct, MyFunction, MyConstant, MyVariable, main
	expectedSymbols := []string{"MyStruct", "MyFunction", "MyConstant", "MyVariable", "main"}
	for _, expected := range expectedSymbols {
		sym := findSymbol(symbols, expected)
		assert.NotNil(t, sym, "Expected to find symbol: %s", expected)
		if sym != nil {
			assert.NotEmpty(t, sym.Name, "Symbol name should not be empty")
			assert.NotZero(t, sym.Kind, "Symbol kind should not be zero")
		}
	}

	// Note: gopls may return flat or hierarchical symbols depending on configuration
	// We verify that if MyStruct is present, it has the correct kind
	structSym := findSymbol(symbols, "MyStruct")
	if structSym != nil {
		assert.Equal(t, SymbolKindStruct, structSym.Kind, "MyStruct should be a struct")
		// If children are provided (hierarchical), verify they are valid
		if len(structSym.Children) > 0 {
			for _, child := range structSym.Children {
				assert.NotEmpty(t, child.Name, "Child symbol should have a name")
				assert.NotZero(t, child.Kind, "Child symbol should have a kind")
			}
		}
	}

	// Verify symbol kinds
	funcSym := findSymbol(symbols, "MyFunction")
	if funcSym != nil {
		assert.Equal(t, SymbolKindFunction, funcSym.Kind, "MyFunction should be a function")
	}

	constSym := findSymbol(symbols, "MyConstant")
	if constSym != nil {
		assert.Equal(t, SymbolKindConstant, constSym.Kind, "MyConstant should be a constant")
	}

	varSym := findSymbol(symbols, "MyVariable")
	if varSym != nil {
		assert.Equal(t, SymbolKindVariable, varSym.Kind, "MyVariable should be a variable")
	}

	// Verify that symbols have valid ranges
	for i := range symbols {
		assert.NotNil(t, symbols[i].Range, "Symbol should have a range")
		assert.NotNil(t, symbols[i].SelectionRange, "Symbol should have a selection range")
		// SelectionRange should be contained within Range
		assert.GreaterOrEqual(t, symbols[i].SelectionRange.Start.Line, symbols[i].Range.Start.Line)
		assert.LessOrEqual(t, symbols[i].SelectionRange.End.Line, symbols[i].Range.End.Line)
	}
}

// TestLSPClientWorkspaceSymbols tests workspace symbols functionality.
func TestLSPClientWorkspaceSymbols(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
	require.NoError(t, err)

	// Create test files in tmp directory
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_ws_symbols_*")

	// Create first test file with a unique type
	testFile1 := filepath.Join(tmpDir, "test_ws_symbols_1.go")
	testContent1 := `package main

// UniqueTestStruct is a unique test structure for workspace symbol search.
type UniqueTestStruct struct {
	Field int
}

// UniqueTestFunction is a unique test function for workspace symbol search.
func UniqueTestFunction() {
}

const UniqueTestConstant = 42
`

	err = os.WriteFile(testFile1, []byte(testContent1), 0644)
	require.NoError(t, err)

	// Create second test file with another unique type
	testFile2 := filepath.Join(tmpDir, "test_ws_symbols_2.go")
	testContent2 := `package main

// AnotherUniqueType is another unique type for workspace symbol search.
type AnotherUniqueType struct {
	Value string
}

func helper() {
}
`

	err = os.WriteFile(testFile2, []byte(testContent2), 0644)
	require.NoError(t, err)

	// Open both files to ensure they're indexed
	_, err = client.TouchAndValidate(testFile1, false)
	require.NoError(t, err)

	_, err = client.TouchAndValidate(testFile2, false)
	require.NoError(t, err)

	// Wait for indexing
	sleepForLSP(client, 2*time.Second)

	// Set up mock responses if using mock LSP
	if mock := asMock(client); mock != nil {
		mock.SetWorkspaceSymbols("UniqueTest", []WorkspaceSymbol{
			{
				Name: "UniqueTestStruct",
				Kind: SymbolKindStruct,
				Location: Location{
					URI: pathToURI(testFile1),
					Range: Range{
						Start: Position{Line: 2, Character: 5},
						End:   Position{Line: 2, Character: 21},
					},
				},
			},
			{
				Name: "UniqueTestFunction",
				Kind: SymbolKindFunction,
				Location: Location{
					URI: pathToURI(testFile1),
					Range: Range{
						Start: Position{Line: 7, Character: 5},
						End:   Position{Line: 7, Character: 23},
					},
				},
			},
			{
				Name: "UniqueTestConstant",
				Kind: SymbolKindConstant,
				Location: Location{
					URI: pathToURI(testFile1),
					Range: Range{
						Start: Position{Line: 10, Character: 6},
						End:   Position{Line: 10, Character: 24},
					},
				},
			},
		})

		mock.SetWorkspaceSymbols("AnotherUnique", []WorkspaceSymbol{
			{
				Name: "AnotherUniqueType",
				Kind: SymbolKindStruct,
				Location: Location{
					URI: pathToURI(testFile2),
					Range: Range{
						Start: Position{Line: 2, Character: 5},
						End:   Position{Line: 2, Character: 22},
					},
				},
			},
		})
	}

	// Test 1: Search for UniqueTest* symbols
	symbols, err := client.WorkspaceSymbols("UniqueTest")
	require.NoError(t, err)
	assert.NotEmpty(t, symbols, "Expected to find workspace symbols matching 'UniqueTest'")

	// Helper function to check if symbol exists by name
	hasSymbol := func(symbols []WorkspaceSymbol, name string) bool {
		for _, sym := range symbols {
			if sym.Name == name {
				return true
			}
		}
		return false
	}

	// We should find at least UniqueTestStruct, UniqueTestFunction, or UniqueTestConstant
	hasAnyUnique := hasSymbol(symbols, "UniqueTestStruct") ||
		hasSymbol(symbols, "UniqueTestFunction") ||
		hasSymbol(symbols, "UniqueTestConstant")
	assert.True(t, hasAnyUnique, "Expected to find at least one UniqueTest* symbol")

	// Verify symbol structure for found symbols
	for _, sym := range symbols {
		if strings.Contains(sym.Name, "UniqueTest") {
			assert.NotEmpty(t, sym.Name, "Symbol name should not be empty")
			assert.NotZero(t, sym.Kind, "Symbol kind should not be zero")
			assert.NotEmpty(t, sym.Location.URI, "Symbol location URI should not be empty")
			// Verify location points to one of our test files
			hasValidLocation := strings.Contains(sym.Location.URI, "test_ws_symbols_1.go") ||
				strings.Contains(sym.Location.URI, "test_ws_symbols_2.go")
			assert.True(t, hasValidLocation, "Symbol location should point to test files")
		}
	}

	// Test 2: Search for AnotherUniqueType
	symbols2, err := client.WorkspaceSymbols("AnotherUnique")
	require.NoError(t, err)

	// Should find AnotherUniqueType
	hasAnotherUnique := hasSymbol(symbols2, "AnotherUniqueType")
	assert.True(t, hasAnotherUnique, "Expected to find AnotherUniqueType symbol")

	// Test 3: Empty query behavior (LSP servers may handle empty queries differently)
	// Some servers return all symbols, others may return empty
	allSymbols, err := client.WorkspaceSymbols("")
	require.NoError(t, err)
	// Note: Different LSP servers handle empty queries differently:
	// - Some return all symbols in the workspace
	// - Some return empty results
	// We just verify the request succeeds
	_ = allSymbols

	// Test 4: Search for non-existent symbol
	noSymbols, err := client.WorkspaceSymbols("ThisSymbolDefinitelyDoesNotExistInWorkspace12345")
	require.NoError(t, err)
	// It's OK to return empty results for non-existent symbols
	_ = noSymbols
}
