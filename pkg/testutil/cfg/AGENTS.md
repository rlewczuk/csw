# Package `pkg/testutil/cfg` Overview

Package `pkg/testutil/cfg` provides integration config helpers in package `pkg/testutil/cfg`.

## Important files

* `integ.go` - Integration configuration and temp dirs.

## Important public API objects

* `Dir` - Returns `_integ` directory absolute path.
* `ReadFile` - Reads trimmed `_integ` file content.
* `TestEnabled` - Checks `<name>.enabled` or global flag.
* `MkTempDir` - Creates temporary test directory.
