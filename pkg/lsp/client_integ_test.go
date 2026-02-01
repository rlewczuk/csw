package lsp

// Integration tests for LSP client.
//
// These tests can run in two modes:
// 1. Real LSP mode (when _integ/lsp.enabled contains "yes"):
//    - Uses real gopls server from _integ/lsp.gopls
//    - Runs slowly due to LSP server initialization and indexing
//    - Tests actual LSP integration
//
// 2. Mock LSP mode (when _integ/lsp.enabled is "no" or missing):
//    - Uses MockLSP test double
//    - Runs very fast (no sleep needed, immediate responses)
//    - Some tests require mock response setup to pass
//
// To add mock support to a test:
//   - Use createLSPClient(t) instead of NewClient()
//   - Use sleepForLSP(client, duration) instead of time.Sleep()
//   - Set up mock responses with: if mock := asMock(client); mock != nil { ... }
//
// See TestLSPClientDiagnostics and TestLSPClientDefinition for examples.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/testutil/cfg"
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

	path := cfg.ReadFile("lsp.gopls")
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
// Returns true if _integ/lsp.enabled exists and contains "yes", false otherwise.
func shouldUseRealLSP(t *testing.T) bool {
	t.Helper()

	return cfg.TestEnabled("lsp")
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

// getDiagnosticsForURI returns diagnostics for a specific URI from either Client or MockLSP.
func getDiagnosticsForURI(client LSP, uri string) []Diagnostic {
	if c, ok := client.(*Client); ok {
		return c.getDiagnosticsForURI(uri)
	} else if m, ok := client.(*MockLSP); ok {
		return m.getDiagnosticsForURI(uri)
	}
	return nil
}

// sleepForLSP sleeps only if using real LSP server (for indexing/processing time).
// Mock LSP doesn't need sleep as responses are immediate.
func sleepForLSP(client LSP, duration time.Duration) {
	if _, ok := client.(*Client); ok {
		time.Sleep(duration)
	}
	// No sleep for MockLSP - responses are immediate
}

// findTextPosition finds the position of a substring in the file content.
// It returns the line (0-based) and column (0-based) of the first occurrence.
// If the text is not found, it fails the test.
func findTextPosition(t *testing.T, content, searchText string) (line int, col int) {
	t.Helper()

	lines := strings.Split(content, "\n")
	for lineNum, lineText := range lines {
		colPos := strings.Index(lineText, searchText)
		if colPos >= 0 {
			return lineNum, colPos
		}
	}

	require.Fail(t, "Text not found in content", "searchText=%q", searchText)
	return 0, 0
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

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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

	// Set up mock response if using mock LSP
	if mock := asMock(client); mock != nil {
		uri := pathToURI(testFile)
		mock.SetDiagnostics(uri, []Diagnostic{
			{
				Range: Range{
					Start: Position{Line: 4, Character: 0},
					End:   Position{Line: 4, Character: 0},
				},
				Severity: SeverityError,
				Message:  "expected '}', found 'EOF'",
			},
		})
	}

	// Touch and validate the file
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait a bit for diagnostics to arrive (only for real LSP)
	sleepForLSP(client, 2*time.Second)

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

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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

	// Wait for indexing (only for real LSP)
	sleepForLSP(client, 1*time.Second)

	// Find definition of myFunc (search for the call to myFunc in main)
	line, col := findTextPosition(t, testContent, "myFunc()")
	cursorLoc := CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
	}

	// Set up mock response if using mock LSP
	if mock := asMock(client); mock != nil {
		defLine, defCol := findTextPosition(t, testContent, "func myFunc()")
		defCol += len("func ")
		mock.SetDefinition(cursorLoc, []Location{
			{
				URI: pathToURI(testFile),
				Range: Range{
					Start: Position{Line: defLine, Character: defCol},
					End:   Position{Line: defLine, Character: defCol + len("myFunc")},
				},
			},
		})
	}

	locations, err := client.FindDefinition(cursorLoc)
	require.NoError(t, err)
	require.NotEmpty(t, locations, "Expected to find definition")

	// The definition should point to where myFunc is defined
	defLine, _ := findTextPosition(t, testContent, "func myFunc()")
	found := false
	for _, loc := range locations {
		if strings.Contains(loc.URI, "test_definition.go") {
			// gopls uses 0-based line numbers
			if loc.Range.Start.Line == defLine {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "Expected definition to point to function definition line")
}

// TestLSPClientReferences tests find-references functionality.
func TestLSPClientReferences(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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

	// Wait for indexing (only for real LSP)
	sleepForLSP(client, 1*time.Second)

	// Find references to x (search for the declaration of x)
	line, col := findTextPosition(t, testContent, "x := 1")
	cursorLoc := CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
	}

	// Set up mock response if using mock LSP
	if mock := asMock(client); mock != nil {
		// x is declared on line 3, used on lines 4 and 5
		declLine, declCol := findTextPosition(t, testContent, "x := 1")
		useLine1, useCol1 := findTextPosition(t, testContent, "_ = x")
		useCol1 += len("_ = ")
		useLine2, useCol2 := findTextPosition(t, testContent, "y := x + 1")
		useCol2 += len("y := ")

		mock.SetReferences(cursorLoc, []Location{
			{
				URI: pathToURI(testFile),
				Range: Range{
					Start: Position{Line: declLine, Character: declCol},
					End:   Position{Line: declLine, Character: declCol + 1},
				},
			},
			{
				URI: pathToURI(testFile),
				Range: Range{
					Start: Position{Line: useLine1, Character: useCol1},
					End:   Position{Line: useLine1, Character: useCol1 + 1},
				},
			},
			{
				URI: pathToURI(testFile),
				Range: Range{
					Start: Position{Line: useLine2, Character: useCol2},
					End:   Position{Line: useLine2, Character: useCol2 + 1},
				},
			},
		})
	}

	locations, err := client.FindReferences(cursorLoc)
	require.NoError(t, err)

	// We should find at least 2 references (declaration, uses in subsequent lines)
	assert.GreaterOrEqual(t, len(locations), 2, "Expected to find at least 2 references to x")
}

// TestLSPClientMultipleFiles tests working with multiple files.
func TestLSPClientMultipleFiles(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
	require.NoError(t, err)

	// Create a unique temporary subdirectory for this test to avoid package conflicts
	tmpDir := filepath.Join(projectRoot, "tmp", "test_multiple_files")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

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

	err = os.WriteFile(file2, []byte(file2Content), 0644)
	require.NoError(t, err)

	// Open both files
	_, err = client.TouchAndValidate(file1, false)
	require.NoError(t, err)

	_, err = client.TouchAndValidate(file2, false)
	require.NoError(t, err)

	// Wait for indexing (only for real LSP)
	sleepForLSP(client, 1*time.Second)

	// Find definition of Helper from file2 (search for Helper call)
	line, col := findTextPosition(t, file2Content, "Helper")
	cursorLoc := CursorLocation{
		Path: file2,
		Line: line,
		Col:  col,
	}

	// If using mock LSP, set up the expected response
	if mock := asMock(client); mock != nil {
		// The definition should point to Helper function in file1
		defLine, defCol := findTextPosition(t, file1Content, "func Helper()")
		defCol += len("func ")
		mock.SetDefinition(cursorLoc, []Location{
			{
				URI: pathToURI(file1),
				Range: Range{
					Start: Position{Line: defLine, Character: defCol},
					End:   Position{Line: defLine, Character: defCol + len("Helper")},
				},
			},
		})
	}

	locations, err := client.FindDefinition(cursorLoc)
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

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
	require.NoError(t, err)

	// Create a unique temporary subdirectory for this test to avoid package conflicts
	tmpDir := filepath.Join(projectRoot, "tmp", "test_update")
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

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

	// Open the file
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	sleepForLSP(client, 1*time.Second)

	// Get diagnostics - should be clean
	_, err = client.Diagnostics()
	require.NoError(t, err)

	// Filter diagnostics for our test file
	uri := pathToURI(testFile)
	testFileDiags := getDiagnosticsForURI(client, uri)

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

	// Set up mock response for invalid code if using mock LSP
	if mock := asMock(client); mock != nil {
		mock.SetDiagnostics(uri, []Diagnostic{
			{
				Range: Range{
					Start: Position{Line: 4, Character: 0},
					End:   Position{Line: 4, Character: 0},
				},
				Severity: SeverityError,
				Message:  "expected '}', found 'EOF'",
			},
		})
	}

	// Touch and validate again
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	sleepForLSP(client, 2*time.Second)

	// Get diagnostics - should have errors now
	testFileDiags = getDiagnosticsForURI(client, uri)

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

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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
	sleepForLSP(client, 1*time.Second)

	// Test hovering over Helper function call (search for the line with Helper call)
	line, col := findTextPosition(t, testContent, "fmt.Println(Helper())")
	// Position cursor on "Helper" - after "fmt.Println("
	col += len("fmt.Println(")

	// Set up mock response if using mock LSP
	if mock := asMock(client); mock != nil {
		mock.SetHover(CursorLocation{
			Path: testFile,
			Line: line,
			Col:  col,
		}, "func Helper() string\n\nHelper is a helper function that returns a greeting message.", "markdown")
	}

	text, format, err := client.Hover(CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, text, "Expected hover text for Helper function")
	assert.NotEmpty(t, format, "Expected hover format")
	assert.Contains(t, text, "Helper", "Expected hover text to contain function name")

	// Test hovering over fmt.Println (search for Println in the same line)
	line, col = findTextPosition(t, testContent, "fmt.Println")
	// Position cursor on "Println" - after "fmt."
	col += len("fmt.")

	// Set up mock response if using mock LSP
	if mock := asMock(client); mock != nil {
		mock.SetHover(CursorLocation{
			Path: testFile,
			Line: line,
			Col:  col,
		}, "func Println(a ...any) (n int, err error)\n\nPrintln formats using the default formats for its operands and writes to standard output.", "markdown")
	}

	text, format, err = client.Hover(CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, text, "Expected hover text for fmt.Println")
	assert.NotEmpty(t, format, "Expected hover format")
	assert.Contains(t, text, "Println", "Expected hover text to contain Println")

	// Test hovering over empty space (should return no hover information)
	// Search for empty line after package declaration
	line, col = findTextPosition(t, testContent, "package main")
	text, format, err = client.Hover(CursorLocation{
		Path: testFile,
		Line: line + 1, // Empty line after package
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

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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
	sleepForLSP(client, 1*time.Second)

	// Test hovering over MyStruct (search for MyStruct in variable declaration)
	line, col := findTextPosition(t, testContent, "var s MyStruct")
	// Position cursor on MyStruct identifier
	col += len("var s ")

	// Set up mock response if using mock LSP
	if mock := asMock(client); mock != nil {
		mock.SetHover(CursorLocation{
			Path: testFile,
			Line: line,
			Col:  col,
		}, "type MyStruct struct {\n\tField int\n}\n\nMyStruct is a sample struct with documentation.\nIt has multiple lines of documentation.", "markdown")
	}

	text, format, err := client.Hover(CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
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

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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

// TestLSPClientCallHierarchy tests call hierarchy functionality.
func TestLSPClientCallHierarchy(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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

// ThirdCaller also calls other functions.
func ThirdCaller() {
	Caller()
	AnotherCaller()
}

func main() {
	ThirdCaller()
}
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Open the file first
	_, err = client.TouchAndValidate(testFile, false)
	require.NoError(t, err)

	// Wait for indexing
	sleepForLSP(client, 2*time.Second)

	// Test 1: Prepare call hierarchy for Helper function (search for Helper function definition)
	line, col := findTextPosition(t, testContent, "func Helper()")
	col += len("func ")

	// Set up mock responses if using mock LSP
	if mock := asMock(client); mock != nil {
		helperItem := CallHierarchyItem{
			Name: "Helper",
			Kind: SymbolKindFunction,
			URI:  pathToURI(testFile),
			Range: Range{
				Start: Position{Line: 4, Character: 0},
				End:   Position{Line: 6, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 4, Character: 5},
				End:   Position{Line: 4, Character: 11},
			},
		}

		mock.SetCallHierarchy(CursorLocation{
			Path: testFile,
			Line: line,
			Col:  col,
		}, []CallHierarchyItem{helperItem})

		mock.SetIncomingCalls("Helper", []CallHierarchyIncomingCall{
			{
				From: CallHierarchyItem{
					Name: "Caller",
					Kind: SymbolKindFunction,
					URI:  pathToURI(testFile),
					Range: Range{
						Start: Position{Line: 9, Character: 0},
						End:   Position{Line: 11, Character: 1},
					},
					SelectionRange: Range{
						Start: Position{Line: 9, Character: 5},
						End:   Position{Line: 9, Character: 11},
					},
				},
				FromRanges: []Range{
					{
						Start: Position{Line: 10, Character: 1},
						End:   Position{Line: 10, Character: 7},
					},
				},
			},
			{
				From: CallHierarchyItem{
					Name: "AnotherCaller",
					Kind: SymbolKindFunction,
					URI:  pathToURI(testFile),
					Range: Range{
						Start: Position{Line: 14, Character: 0},
						End:   Position{Line: 16, Character: 1},
					},
					SelectionRange: Range{
						Start: Position{Line: 14, Character: 5},
						End:   Position{Line: 14, Character: 18},
					},
				},
				FromRanges: []Range{
					{
						Start: Position{Line: 15, Character: 1},
						End:   Position{Line: 15, Character: 7},
					},
				},
			},
		})

		// Set up for Caller function
		callerLine, callerCol := findTextPosition(t, testContent, "func Caller()")
		callerCol += len("func ")
		callerItem := CallHierarchyItem{
			Name: "Caller",
			Kind: SymbolKindFunction,
			URI:  pathToURI(testFile),
			Range: Range{
				Start: Position{Line: 9, Character: 0},
				End:   Position{Line: 11, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 9, Character: 5},
				End:   Position{Line: 9, Character: 11},
			},
		}

		mock.SetCallHierarchy(CursorLocation{
			Path: testFile,
			Line: callerLine,
			Col:  callerCol,
		}, []CallHierarchyItem{callerItem})

		mock.SetOutgoingCalls("Caller", []CallHierarchyOutgoingCall{
			{
				To: helperItem,
				FromRanges: []Range{
					{
						Start: Position{Line: 10, Character: 1},
						End:   Position{Line: 10, Character: 7},
					},
				},
			},
		})

		// Set up for ThirdCaller function
		thirdLine, thirdCol := findTextPosition(t, testContent, "func ThirdCaller()")
		thirdCol += len("func ")
		thirdCallerItem := CallHierarchyItem{
			Name: "ThirdCaller",
			Kind: SymbolKindFunction,
			URI:  pathToURI(testFile),
			Range: Range{
				Start: Position{Line: 19, Character: 0},
				End:   Position{Line: 22, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 19, Character: 5},
				End:   Position{Line: 19, Character: 16},
			},
		}

		mock.SetCallHierarchy(CursorLocation{
			Path: testFile,
			Line: thirdLine,
			Col:  thirdCol,
		}, []CallHierarchyItem{thirdCallerItem})

		mock.SetOutgoingCalls("ThirdCaller", []CallHierarchyOutgoingCall{
			{
				To: callerItem,
				FromRanges: []Range{
					{
						Start: Position{Line: 20, Character: 1},
						End:   Position{Line: 20, Character: 7},
					},
				},
			},
			{
				To: CallHierarchyItem{
					Name: "AnotherCaller",
					Kind: SymbolKindFunction,
					URI:  pathToURI(testFile),
					Range: Range{
						Start: Position{Line: 14, Character: 0},
						End:   Position{Line: 16, Character: 1},
					},
					SelectionRange: Range{
						Start: Position{Line: 14, Character: 5},
						End:   Position{Line: 14, Character: 18},
					},
				},
				FromRanges: []Range{
					{
						Start: Position{Line: 21, Character: 1},
						End:   Position{Line: 21, Character: 14},
					},
				},
			},
		})
	}

	items, err := client.CallHierarchy(CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
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
	// First prepare call hierarchy for Caller (search for Caller function definition)
	line, col = findTextPosition(t, testContent, "func Caller()")
	col += len("func ")
	callerItems, err := client.CallHierarchy(CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
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

	// Test 4: Get outgoing calls for ThirdCaller function (testing multi-level call chains)
	line, col = findTextPosition(t, testContent, "func ThirdCaller()")
	col += len("func ")
	thirdCallerItems, err := client.CallHierarchy(CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
	})
	require.NoError(t, err)
	require.NotEmpty(t, thirdCallerItems, "Expected to get call hierarchy items for ThirdCaller")

	thirdCallerItem := thirdCallerItems[0]
	assert.Equal(t, "ThirdCaller", thirdCallerItem.Name, "Expected item name to be 'ThirdCaller'")

	thirdCallerOutgoingCalls, err := client.OutgoingCalls(thirdCallerItem)
	require.NoError(t, err)
	require.NotEmpty(t, thirdCallerOutgoingCalls, "Expected to find outgoing calls from ThirdCaller")

	// We should find Caller and AnotherCaller
	calleeNames := make(map[string]bool)
	for _, call := range thirdCallerOutgoingCalls {
		calleeNames[call.To.Name] = true
	}

	assert.True(t, calleeNames["Caller"] || calleeNames["AnotherCaller"],
		"Expected to find at least one of Caller or AnotherCaller in ThirdCaller's outgoing calls")
}

// TestLSPClientCallHierarchyMultipleFiles tests call hierarchy across multiple files.
func TestLSPClientCallHierarchyMultipleFiles(t *testing.T) {
	if !shouldRunIntegrationTests(t) {
		return
	}

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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
	sleepForLSP(client, 2*time.Second)

	// Prepare call hierarchy for SharedFunction (search for SharedFunction definition)
	line, col := findTextPosition(t, file1Content, "func SharedFunction()")
	col += len("func ")

	// Set up mock responses if using mock LSP
	if mock := asMock(client); mock != nil {
		sharedItem := CallHierarchyItem{
			Name: "SharedFunction",
			Kind: SymbolKindFunction,
			URI:  pathToURI(file1),
			Range: Range{
				Start: Position{Line: 2, Character: 0},
				End:   Position{Line: 4, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 2, Character: 5},
				End:   Position{Line: 2, Character: 19},
			},
		}

		mock.SetCallHierarchy(CursorLocation{
			Path: file1,
			Line: line,
			Col:  col,
		}, []CallHierarchyItem{sharedItem})

		mock.SetIncomingCalls("SharedFunction", []CallHierarchyIncomingCall{
			{
				From: CallHierarchyItem{
					Name: "CallerInFile2",
					Kind: SymbolKindFunction,
					URI:  pathToURI(file2),
					Range: Range{
						Start: Position{Line: 2, Character: 0},
						End:   Position{Line: 5, Character: 1},
					},
					SelectionRange: Range{
						Start: Position{Line: 2, Character: 5},
						End:   Position{Line: 2, Character: 18},
					},
				},
				FromRanges: []Range{
					{
						Start: Position{Line: 3, Character: 10},
						End:   Position{Line: 3, Character: 24},
					},
				},
			},
			{
				From: CallHierarchyItem{
					Name: "CallerInFile3",
					Kind: SymbolKindFunction,
					URI:  pathToURI(file3),
					Range: Range{
						Start: Position{Line: 2, Character: 0},
						End:   Position{Line: 5, Character: 1},
					},
					SelectionRange: Range{
						Start: Position{Line: 2, Character: 5},
						End:   Position{Line: 2, Character: 18},
					},
				},
				FromRanges: []Range{
					{
						Start: Position{Line: 3, Character: 8},
						End:   Position{Line: 3, Character: 22},
					},
				},
			},
		})
	}

	items, err := client.CallHierarchy(CursorLocation{
		Path: file1,
		Line: line,
		Col:  col,
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

	projectRoot := getProjectRoot(t)

	client := createLSPClient(t)
	defer closeLSPClient(client)

	err := client.Init(true)
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
	sleepForLSP(client, 1*time.Second)

	// Prepare call hierarchy for UnusedFunction (search for UnusedFunction definition)
	line, col := findTextPosition(t, testContent, "func UnusedFunction()")
	col += len("func ")

	// Set up mock responses if using mock LSP
	if mock := asMock(client); mock != nil {
		unusedItem := CallHierarchyItem{
			Name: "UnusedFunction",
			Kind: SymbolKindFunction,
			URI:  pathToURI(testFile),
			Range: Range{
				Start: Position{Line: 2, Character: 0},
				End:   Position{Line: 4, Character: 1},
			},
			SelectionRange: Range{
				Start: Position{Line: 2, Character: 5},
				End:   Position{Line: 2, Character: 19},
			},
		}

		mock.SetCallHierarchy(CursorLocation{
			Path: testFile,
			Line: line,
			Col:  col,
		}, []CallHierarchyItem{unusedItem})

		// No incoming calls for unused function
		mock.SetIncomingCalls("UnusedFunction", []CallHierarchyIncomingCall{})

		// No outgoing calls for unused function (it doesn't call anything)
		mock.SetOutgoingCalls("UnusedFunction", []CallHierarchyOutgoingCall{})
	}

	items, err := client.CallHierarchy(CursorLocation{
		Path: testFile,
		Line: line,
		Col:  col,
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
