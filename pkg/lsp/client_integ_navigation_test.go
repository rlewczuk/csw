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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_definition_*")
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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_references_*")
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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_multiple_files_*")

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
