# Package `pkg/shared/godown` Overview

Package `pkg/shared/godown` contains HTML to Markdown converter with customizable options.

## Important files

* `godown.go` - HTML to Markdown conversion logic
* `godown_test.go` - Unit tests for conversion functionality

## Important public API objects

* `Convert` - Converts HTML from reader to Markdown writer
* `CovertStr` - Converts HTML string to Markdown string
* `Option` - Configuration options for conversion
* `CustomRule` - Interface for custom HTML element handlers
* `WalkFunc` - Function signature for HTML node traversal
* `Option.GuessLang` - Optional language detection function for code blocks
* `Option.Script` - Include script tags in output (bool)
* `Option.Style` - Include style tags in output (bool)
* `Option.TrimSpace` - Trim whitespace from text nodes (bool)
* `Option.CustomRules` - Custom HTML element handlers ([]CustomRule)
* `Option.IgnoreComments` - Skip HTML comments in output (bool)
* `Option.ItalicsAsterix` - Use * instead of _ for italics (bool)
