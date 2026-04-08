# Package `pkg/shared/godown` Overview

Package `pkg/shared/godown` implements HTML-to-Markdown conversion in package `pkg/shared/godown`.

## Important files

* `godown.go` - HTML to Markdown conversion.
* `godown_test.go` - Conversion behavior tests.

## Important public API objects

* `WalkFunc` - HTML node walk function type.
* `CustomRule` - Interface for custom tag conversion.
* `Option` - Converter options for rendering behavior.
* `Convert` - Converts HTML reader to Markdown writer.
* `CovertStr` - Converts HTML string to Markdown.
