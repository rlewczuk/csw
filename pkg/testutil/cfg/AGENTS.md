# Package `pkg/testutil/cfg` Overview

Package `pkg/testutil/cfg` provides integration test configuration helpers for managing `_integ` feature flags, reading configuration files, and creating temporary directories in `pkg/testutil/cfg`.

## Important files

* `integ.go` - Integration test configuration helpers.
* `integ_test.go` - Tests for helper functions.

## Important public API objects

* `Dir` - Returns absolute path to _integ directory.
* `ReadFile` - Reads trimmed _integ file content.
* `TestEnabled` - Checks feature or global enabled flag.
* `MkTempDir` - Creates temporary directory for tests.
