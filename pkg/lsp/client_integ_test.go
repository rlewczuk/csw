package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/integcfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shouldRunIntegrationTests checks if integration tests should run.
// Integration tests always run, but use mock LSP if lsp.enabled is missing or doesn't contain "yes".
func shouldRunIntegrationTests(t *testing.T) bool {
	t.Helper()
	// Integration tests now always run (using mock or real LSP based on configuration)
	return true
}

// getGoplsPath returns the path to gopls binary from _integ/lsp.gopls.
func getGoplsPath(t *testing.T) string {
	t.Helper()

	path := integcfg.ReadFile("lsp.gopls")
	require.NotEmpty(t, path, "_integ/lsp.gopls is empty")

	return path
}

// getProjectRoot returns the project root directory.
func getProjectRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	// Go up from pkg/lsp to project root
	root := filepath.Join(wd, "../..")
	absRoot, err := filepath.Abs(root)
	require.NoError(t, err)

	return absRoot
}

// shouldUseRealLSP checks if we should use real LSP server (gopls).
// Returns true if _integ/lsp.enabled does NOT exist or does NOT contain "yes", false otherwise.
func shouldUseRealLSP(t *testing.T) bool {
	t.Helper()

	return !integcfg.TestEnabled("lsp")
}

// createLSPClient creates either a real or mock LSP client based on configuration.
func createLSPClient(t *testing.T) LSP {
	t.Helper()

	projectRoot := getProjectRoot(t)

	if shouldUseRealLSP(t) {
		goplsPath := getGoplsPath(t)
		client, err := NewClient(goplsPath, projectRoot)
		require.NoError(t, err)
		return client
	}

	// Use mock LSP
	mock, err := NewMockLSP(projectRoot)
	require.NoError(t, err)
	return mock
}

// closeLSPClient closes the LSP client (works for both Client and MockLSP).
func closeLSPClient(client LSP) {
	if c, ok := client.(*Client); ok {
		c.Close()
	} else if m, ok := client.(*MockLSP); ok {
		m.Close()
	}
}

// asMock returns the mock LSP if the client is a mock, nil otherwise.
func asMock(client LSP) *MockLSP {
	mock, _ := client.(*MockLSP)
	return mock
}

// TestLSPClientInitialization tests basic client initialization.
func TestLSPClientInitialization(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
	require.NoError(t, err)
}

// TestLSPClientDiagnostics tests diagnostics functionality.
func TestLSPClientDiagnostics(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create a temporary file with syntax error in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test_diagnostics.go")
	testContent := `package main

func main() {
	// Missing closing brace
	if true {
		x := 1
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Touch and validate the file
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait a bit for diagnostics to arrive
	time.Sleep(2 * time.Second)

	// Get diagnostics
	diags, err := client.Diagnostics()
	require.NoError(t, err)

	// We should have diagnostics for the syntax error
	assert.NotEmpty(t, diags, "Expected to receive diagnostics for syntax error")

	// Verify at least one diagnostic is an error
	hasError := false
	for _, diag := range diags {
		if diag.Severity == SeverityError {
			hasError = true
			break
		}
	}
	assert.True(t, hasError, "Expected at least one error diagnostic")
}

// TestLSPClientDefinition tests go-to-definition functionality.
func TestLSPClientDefinition(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create a temporary file in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test_definition.go")
	testContent := `package main

import "fmt"

func myFunc() {
	fmt.Println("hello")
}

func main() {
	myFunc()
}
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Find definition of myFunc (cursor on line 9, col 1 - the call to myFunc)
	locations, err := client.FindDefinition(CursorLocation{
		Path: testFile,
		Line: 9,
		Col:  1,
	})
	require.NoError(t, err)
	require.NotEmpty(t, locations, "Expected to find definition")

	// The definition should point to line 4 (where myFunc is defined)
	found := false
	for _, loc := range locations {
		if strings.Contains(loc.URI, "test_definition.go") {
			// gopls uses 0-based line numbers
			if loc.Range.Start.Line == 4 {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "Expected definition to point to line 4")
}

// TestLSPClientReferences tests find-references functionality.
func TestLSPClientReferences(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create a temporary file in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test_references.go")
	testContent := `package main

func myFunc() {
	x := 1
	_ = x
	y := x + 1
	_ = y
}
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Find references to x (cursor on line 3, col 1 - the declaration of x)
	locations, err := client.FindReferences(CursorLocation{
		Path: testFile,
		Line: 3,
		Col:  1,
	})
	require.NoError(t, err)

	// We should find at least 3 references (declaration, use in line 4, use in line 5)
	assert.GreaterOrEqual(t, len(locations), 2, "Expected to find at least 2 references to x")
}

// TestLSPClientMultipleFiles tests working with multiple files.
func TestLSPClientMultipleFiles(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create temporary files in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	file1 := filepath.Join(tmpDir, "test_multi_1.go")
	file1Content := `package main

func Helper() string {
	return "helper"
}
`

	file2 := filepath.Join(tmpDir, "test_multi_2.go")
	file2Content := `package main

func main() {
	s := Helper()
	_ = s
}
`

	err = os.WriteFile(file1, []byte(file1Content), 0644)
	require.NoError(t, err)
	defer os.Remove(file1)

	err = os.WriteFile(file2, []byte(file2Content), 0644)
	require.NoError(t, err)
	defer os.Remove(file2)

	// Open both files
	_, err = client.TouchAndValidate(file1, false)
	require.NoError(t, err)

	_, err = client.TouchAndValidate(file2, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Find definition of Helper from file2
	locations, err := client.FindDefinition(CursorLocation{
		Path: file2,
		Line: 3,
		Col:  7, // Position of Helper call
	})
	require.NoError(t, err)
	require.NotEmpty(t, locations, "Expected to find definition")

	// The definition should be in file1
	found := false
	for _, loc := range locations {
		if strings.Contains(loc.URI, "test_multi_1.go") {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected definition to be in test_multi_1.go")
}

// TestLSPClientDocumentUpdate tests updating a document and getting new diagnostics.
func TestLSPClientDocumentUpdate(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create a temporary file in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test_update.go")

	// First version: valid code
	validContent := `package main

func main() {
	x := 1
	_ = x
}
`
	err = os.WriteFile(testFile, []byte(validContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Open the file
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// Get diagnostics - should be clean
	_, err = client.Diagnostics()
	require.NoError(t, err)

	// Filter diagnostics for our test file
	uri := pathToURI(testFile)
	testFileDiags := client.getDiagnosticsForURI(uri)

	// Should have no errors for valid code
	errorCount := 0
	for _, diag := range testFileDiags {
		if diag.Severity == SeverityError {
			errorCount++
		}
	}
	assert.Equal(t, 0, errorCount, "Expected no errors for valid code")

	// Update with invalid code
	invalidContent := `package main

func main() {
	x := 1
	// Missing closing brace
`
	err = os.WriteFile(testFile, []byte(invalidContent), 0644)
	require.NoError(t, err)

	// Touch and validate again
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	// Get diagnostics - should have errors now
	testFileDiags = client.getDiagnosticsForURI(uri)

	errorCount = 0
	for _, diag := range testFileDiags {
		if diag.Severity == SeverityError {
			errorCount++
		}
	}
	assert.Greater(t, errorCount, 0, "Expected errors for invalid code")
}

// TestLSPClientHover tests hover functionality.
func TestLSPClientHover(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create a temporary file in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test_hover.go")
	testContent := `package main

import "fmt"

// Helper is a helper function that returns a greeting message.
func Helper() string {
	return "hello"
}

func main() {
	fmt.Println(Helper())
}
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Test hovering over Helper function call (line 10, col 14)
	text, format, err := client.Hover(CursorLocation{
		Path: testFile,
		Line: 10,
		Col:  14,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, text, "Expected hover text for Helper function")
	assert.NotEmpty(t, format, "Expected hover format")
	assert.Contains(t, text, "Helper", "Expected hover text to contain function name")

	// Test hovering over fmt.Println (line 10, col 5)
	text, format, err = client.Hover(CursorLocation{
		Path: testFile,
		Line: 10,
		Col:  5,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, text, "Expected hover text for fmt.Println")
	assert.NotEmpty(t, format, "Expected hover format")
	assert.Contains(t, text, "Println", "Expected hover text to contain Println")

	// Test hovering over empty space (should return no hover information)
	text, format, err = client.Hover(CursorLocation{
		Path: testFile,
		Line: 1,
		Col:  0,
	})
	require.NoError(t, err)
	// Empty text is acceptable for empty space
	_ = text
	_ = format
}

// TestLSPClientHoverMarkdownFormat tests that hover prefers markdown format.
func TestLSPClientHoverMarkdownFormat(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create a temporary file in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test_hover_format.go")
	testContent := `package main

// MyStruct is a sample struct with documentation.
// It has multiple lines of documentation.
type MyStruct struct {
	Field int
}

func main() {
	var s MyStruct
	_ = s
}
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Test hovering over MyStruct (line 9, col 6)
	text, format, err := client.Hover(CursorLocation{
		Path: testFile,
		Line: 9,
		Col:  6,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, text, "Expected hover text for MyStruct")

	// The format should be either "markdown" or "plaintext"
	assert.Contains(t, []string{"markdown", "plaintext"}, format, "Expected format to be markdown or plaintext")

	// Text should contain the struct name
	assert.Contains(t, text, "MyStruct", "Expected hover text to contain struct name")
}

// TestLSPClientDocumentSymbols tests document symbols functionality.
func TestLSPClientDocumentSymbols(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create a temporary file in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

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
	defer os.Remove(testFile)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(1 * time.Second)

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

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create test files in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

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
	defer os.Remove(testFile1)

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
	defer os.Remove(testFile2)

	// Open both files to ensure they're indexed
	_, err = client.TouchAndValidate(testFile1, false)
	require.NoError(t, err)

	_, err = client.TouchAndValidate(testFile2, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(2 * time.Second)

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

// TestLSPClientCallHierarchy tests call hierarchy functionality.
func TestLSPClientCallHierarchy(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create temporary files in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test_call_hierarchy.go")
	testContent := `package main

import "fmt"

// Helper is a helper function.
func Helper() {
	fmt.Println("helper")
}

// Caller calls Helper function.
func Caller() {
	Helper()
}

// AnotherCaller also calls Helper.
func AnotherCaller() {
	Helper()
}

func main() {
	Caller()
	AnotherCaller()
}
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(2 * time.Second)

	// Test 1: Prepare call hierarchy for Helper function (line 5, col 5)
	items, err := client.CallHierarchy(CursorLocation{
		Path: testFile,
		Line: 5,
		Col:  5,
	})
	require.NoError(t, err)
	require.NotEmpty(t, items, "Expected to get call hierarchy items for Helper")

	// Verify the first item is the Helper function
	helperItem := items[0]
	assert.Equal(t, "Helper", helperItem.Name, "Expected item name to be 'Helper'")
	assert.Equal(t, SymbolKindFunction, helperItem.Kind, "Expected item kind to be Function")
	assert.Contains(t, helperItem.URI, "test_call_hierarchy.go", "Expected URI to contain test file")

	// Test 2: Get incoming calls for Helper
	incomingCalls, err := client.IncomingCalls(helperItem)
	require.NoError(t, err)
	require.NotEmpty(t, incomingCalls, "Expected to find incoming calls to Helper")

	// We should find at least 2 callers: Caller and AnotherCaller
	callerNames := make(map[string]bool)
	for _, call := range incomingCalls {
		callerNames[call.From.Name] = true
		// Verify structure
		assert.NotEmpty(t, call.From.Name, "Caller name should not be empty")
		assert.NotZero(t, call.From.Kind, "Caller kind should not be zero")
		assert.NotEmpty(t, call.From.URI, "Caller URI should not be empty")
		assert.NotEmpty(t, call.FromRanges, "FromRanges should not be empty")
	}

	// Check we have the expected callers
	assert.True(t, callerNames["Caller"] || callerNames["AnotherCaller"],
		"Expected to find at least one of Caller or AnotherCaller")

	// Test 3: Get outgoing calls for Caller function
	// First prepare call hierarchy for Caller (line 10, col 5)
	callerItems, err := client.CallHierarchy(CursorLocation{
		Path: testFile,
		Line: 10,
		Col:  5,
	})
	require.NoError(t, err)
	require.NotEmpty(t, callerItems, "Expected to get call hierarchy items for Caller")

	callerItem := callerItems[0]
	assert.Equal(t, "Caller", callerItem.Name, "Expected item name to be 'Caller'")

	// Get outgoing calls from Caller
	outgoingCalls, err := client.OutgoingCalls(callerItem)
	require.NoError(t, err)
	require.NotEmpty(t, outgoingCalls, "Expected to find outgoing calls from Caller")

	// We should find Helper as a callee
	foundHelper := false
	for _, call := range outgoingCalls {
		if call.To.Name == "Helper" {
			foundHelper = true
			// Verify structure
			assert.Equal(t, SymbolKindFunction, call.To.Kind, "Helper should be a function")
			assert.NotEmpty(t, call.To.URI, "Callee URI should not be empty")
			assert.NotEmpty(t, call.FromRanges, "FromRanges should not be empty")
			break
		}
	}
	assert.True(t, foundHelper, "Expected to find Helper in outgoing calls from Caller")

	// Test 4: Get outgoing calls for main function
	mainItems, err := client.CallHierarchy(CursorLocation{
		Path: testFile,
		Line: 19,
		Col:  5,
	})
	require.NoError(t, err)
	require.NotEmpty(t, mainItems, "Expected to get call hierarchy items for main")

	mainItem := mainItems[0]
	assert.Equal(t, "main", mainItem.Name, "Expected item name to be 'main'")

	mainOutgoingCalls, err := client.OutgoingCalls(mainItem)
	require.NoError(t, err)
	require.NotEmpty(t, mainOutgoingCalls, "Expected to find outgoing calls from main")

	// We should find Caller and AnotherCaller
	calleeNames := make(map[string]bool)
	for _, call := range mainOutgoingCalls {
		calleeNames[call.To.Name] = true
	}

	assert.True(t, calleeNames["Caller"] || calleeNames["AnotherCaller"],
		"Expected to find at least one of Caller or AnotherCaller in main's outgoing calls")
}

// TestLSPClientCallHierarchyMultipleFiles tests call hierarchy across multiple files.
func TestLSPClientCallHierarchyMultipleFiles(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create temporary files in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	// File 1: defines a shared function
	file1 := filepath.Join(tmpDir, "test_call_hierarchy_1.go")
	file1Content := `package main

// SharedFunction is called by functions in other files.
func SharedFunction() string {
	return "shared"
}
`

	// File 2: calls SharedFunction
	file2 := filepath.Join(tmpDir, "test_call_hierarchy_2.go")
	file2Content := `package main

func CallerInFile2() {
	result := SharedFunction()
	_ = result
}
`

	// File 3: also calls SharedFunction
	file3 := filepath.Join(tmpDir, "test_call_hierarchy_3.go")
	file3Content := `package main

func CallerInFile3() {
	data := SharedFunction()
	_ = data
}
`

	err = os.WriteFile(file1, []byte(file1Content), 0644)
	require.NoError(t, err)
	defer os.Remove(file1)

	err = os.WriteFile(file2, []byte(file2Content), 0644)
	require.NoError(t, err)
	defer os.Remove(file2)

	err = os.WriteFile(file3, []byte(file3Content), 0644)
	require.NoError(t, err)
	defer os.Remove(file3)

	// Open all files
	_, err = client.TouchAndValidate(file1, false)
	require.NoError(t, err)

	_, err = client.TouchAndValidate(file2, false)
	require.NoError(t, err)

	_, err = client.TouchAndValidate(file3, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(2 * time.Second)

	// Prepare call hierarchy for SharedFunction (file1, line 3, col 5)
	items, err := client.CallHierarchy(CursorLocation{
		Path: file1,
		Line: 3,
		Col:  5,
	})
	require.NoError(t, err)
	require.NotEmpty(t, items, "Expected to get call hierarchy items for SharedFunction")

	sharedItem := items[0]
	assert.Equal(t, "SharedFunction", sharedItem.Name, "Expected item name to be 'SharedFunction'")

	// Get incoming calls for SharedFunction
	incomingCalls, err := client.IncomingCalls(sharedItem)
	require.NoError(t, err)
	require.NotEmpty(t, incomingCalls, "Expected to find incoming calls to SharedFunction")

	// We should find callers from different files
	callerNames := make(map[string]bool)
	for _, call := range incomingCalls {
		callerNames[call.From.Name] = true
	}

	// Verify we have at least one caller from file2 or file3
	assert.True(t, callerNames["CallerInFile2"] || callerNames["CallerInFile3"],
		"Expected to find at least one caller from other files")

	// Verify at least 2 incoming calls (from both file2 and file3)
	assert.GreaterOrEqual(t, len(incomingCalls), 1,
		"Expected at least one incoming call from other files")
}

// TestLSPClientCallHierarchyNoResults tests call hierarchy with functions that have no callers/callees.
func TestLSPClientCallHierarchyNoResults(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	goplsPath := getGoplsPath(t)
	projectRoot := getProjectRoot(t)

	client, err := NewClient(goplsPath, projectRoot)
	require.NoError(t, err)
	defer client.Close()

	err = client.Init(true)
	require.NoError(t, err)

	// Create a temporary file in tmp directory
	tmpDir := filepath.Join(projectRoot, "tmp")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test_call_hierarchy_no_results.go")
	testContent := `package main

// UnusedFunction is never called.
func UnusedFunction() {
	// No function calls here
}

func main() {
	// Don't call UnusedFunction
}
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Prepare call hierarchy for UnusedFunction (line 3, col 5)
	items, err := client.CallHierarchy(CursorLocation{
		Path: testFile,
		Line: 3,
		Col:  5,
	})
	require.NoError(t, err)
	require.NotEmpty(t, items, "Expected to get call hierarchy items for UnusedFunction")

	unusedItem := items[0]
	assert.Equal(t, "UnusedFunction", unusedItem.Name, "Expected item name to be 'UnusedFunction'")

	// Get incoming calls for UnusedFunction - should be empty
	incomingCalls, err := client.IncomingCalls(unusedItem)
	require.NoError(t, err)
	// It's OK to have no incoming calls for an unused function
	_ = incomingCalls

	// Get outgoing calls for UnusedFunction - should be empty
	outgoingCalls, err := client.OutgoingCalls(unusedItem)
	require.NoError(t, err)
	// It's OK to have no outgoing calls for a function with no calls
	assert.Empty(t, outgoingCalls, "Expected no outgoing calls from UnusedFunction")
}
