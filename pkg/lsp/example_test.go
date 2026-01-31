package lsp_test

import (
	"fmt"
	"log"

	"github.com/codesnort/codesnort-swe/pkg/lsp"
)

// ExampleClient demonstrates how to use the LSP client.
func ExampleClient() {
	// Create a new LSP client
	client, err := lsp.NewClient("/path/to/gopls", "/path/to/project")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Initialize the client (blocks until server is ready)
	if err := client.Init(true); err != nil {
		log.Fatal(err)
	}

	// Touch and validate a file
	diagnostics, err := client.TouchAndValidate("/path/to/file.go", true)
	if err != nil {
		log.Fatal(err)
	}

	for _, diag := range diagnostics {
		fmt.Printf("Diagnostic: %s (line %d)\n", diag.Message, diag.Range.Start.Line)
	}

	// Find definition of a symbol
	locations, err := client.FindDefinition(lsp.CursorLocation{
		Path: "/path/to/file.go",
		Line: 10,
		Col:  5,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, loc := range locations {
		fmt.Printf("Definition at: %s:%d:%d\n", loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
	}

	// Find references to a symbol
	references, err := client.FindReferences(lsp.CursorLocation{
		Path: "/path/to/file.go",
		Line: 10,
		Col:  5,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, ref := range references {
		fmt.Printf("Reference at: %s:%d:%d\n", ref.URI, ref.Range.Start.Line, ref.Range.Start.Character)
	}
}
