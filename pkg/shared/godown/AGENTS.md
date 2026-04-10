# Package `pkg/shared/godown` Overview

Package `pkg/shared/godown` converts HTML into Markdown with customizable options and support for custom tag rules.

## Important files

* `godown.go` - HTML to Markdown conversion.
* `godown_test.go` - Conversion behavior tests.

## Important public API objects

* `WalkFunc` - HTML node walk function type.
* `CustomRule` - Interface for custom tag conversion.
* `Option` - Converter options for rendering behavior.
* `Clone` - Copies options without modifying original.
* `Convert` - Converts HTML reader to Markdown writer.
* `CovertStr` - Converts HTML string to Markdown string.
