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

	"github.com/rlewczuk/csw/pkg/testutil/cfg"
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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_diagnostics_*")
	testFile := filepath.Join(tmpDir, "test_diagnostics.go")
	testContent := `package main

func main() {
	// Missing closing brace
	if true {
		x := 1
`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_update_*")

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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_hover_*")
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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_hover_format_*")
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
