# Package `pkg/shared` Overview

Package `pkg/shared` contains cross-cutting utility code reused across packages, including file copy helpers, UUIDv7 generation, message types, and HTML to Markdown conversion.

## Important files

* `fileutil.go` - Recursive directory and file copy utilities
* `messagetype.go` - User-facing status message type enum
* `util.go` - String formatting and template rendering utilities
* `uuid.go` - UUIDv7 generator for time-sortable unique IDs
* `godown/godown.go` - HTML to Markdown conversion logic

## Important public API objects

* `CopyDir` - Recursively copies a directory preserving permissions
* `CopyFile` - Copies a file preserving permissions
* `GenerateUUIDv7` - Generates time-sortable UUIDv7 string
* `MessageType` - Enum for user-facing message types (info, warning, error)
* `MessageTypeInfo` - Informational message type value
* `MessageTypeWarning` - Warning message type value
* `MessageTypeError` - Error message type value
* `FormatList` - Formats string slice as sorted comma-separated list
* `SortedList` - Returns sorted copy of string slice
* `NullValue` - Returns "-" for empty strings
* `RenderTextWithContext` - Renders text/template with context map
* `Convert` - Converts HTML from reader to Markdown writer
* `CovertStr` - Converts HTML string to Markdown string
* `Option` - Configuration options for HTML to Markdown conversion
* `CustomRule` - Interface for custom HTML element handlers
* `WalkFunc` - Function signature for HTML node traversal
