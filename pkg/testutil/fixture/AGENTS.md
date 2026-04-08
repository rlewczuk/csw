# Package `pkg/testutil/fixture` Overview

Package `pkg/testutil/fixture` provides path fixtures in package `pkg/testutil/fixture`.

## Important files

* `paths.go` - Project root and temp path helpers.

## Important public API objects

* `ProjectRoot` - Returns repository root absolute path.
* `ProjectPath` - Joins path under repository root.
* `ProjectTmpDir` - Ensures and returns project tmp directory.
* `MkProjectTempDir` - Creates temporary directory under tmp.
