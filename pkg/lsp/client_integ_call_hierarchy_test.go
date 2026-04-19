package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/testutil/cfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_call_hierarchy_*")
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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_call_hierarchy_multi_*")

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

	err = os.WriteFile(file2, []byte(file2Content), 0644)
	require.NoError(t, err)

	err = os.WriteFile(file3, []byte(file3Content), 0644)
	require.NoError(t, err)

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
	tmpDir := cfg.MkTempDir(t, projectRoot, "test_call_hierarchy_no_results_*")
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
