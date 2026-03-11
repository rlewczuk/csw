# Package `pkg/shared/godown` Overview

Package `pkg/shared/godown` contains HTML to Markdown converter with customizable options.

## Important files

* `godown.go` - HTML to Markdown conversion logic

## Important public API objects

* `Convert` - Converts HTML from reader to Markdown writer
* `CovertStr` - Converts HTML string to Markdown string
* `Option` - Configuration options for conversion
* `CustomRule` - Interface for custom HTML element handlers
* `WalkFunc` - Function signature for HTML node traversal
* `Option.Clone` - Creates a copy of options
