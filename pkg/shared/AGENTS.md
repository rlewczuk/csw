# Package `pkg/shared` Overview

Package `pkg/shared` provides shared helpers used across package `pkg/shared` consumers.

## Important files

* `fileutil.go` - Directory and file copy helpers.
* `messagetype.go` - Message type enum definitions.
* `util.go` - Sorting and template rendering helpers.
* `uuid.go` - UUIDv7 generation helper.

## Important public API objects

* `MessageType` - Enum values: info, warning, error.
* `CopyDir` - Recursively copies directory contents.
* `CopyFile` - Copies file preserving permissions.
* `FormatList` - Returns sorted comma-joined list.
* `SortedList` - Returns sorted copy of values.
* `NullValue` - Returns dash for empty strings.
* `RenderTextWithContext` - Renders text template with context.
* `GenerateUUIDv7` - Generates time-sortable UUIDv7 string.
