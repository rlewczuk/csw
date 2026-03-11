# Package `pkg/testutil/cfg` Overview

Package `pkg/testutil/cfg` provides integration test configuration helpers for managing test directories and feature flags via `_integ` directory files.

## Important files

* `integ.go` - integration test configuration helpers

## Important public API objects

* `Dir` - returns path to `_integ` directory at project root
* `ReadFile` - reads and returns trimmed content from `_integ` file
* `TestEnabled` - checks if feature is enabled via `.enabled` file
* `MkTempDir` - creates temp directory with automatic cleanup
