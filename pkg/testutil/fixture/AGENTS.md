# Package `pkg/testutil/fixture` Overview

Package `pkg/testutil/fixture` provides reusable test fixture helpers for locating the project root and managing temporary directories within the repository.

## Important files

* `paths.go` - Project root and temp path helpers.

## Important public API objects

* `ProjectRoot` - Returns repository root absolute path.
* `ProjectPath` - Joins path under repository root.
* `ProjectTmpDir` - Ensures and returns project tmp directory.
* `MkProjectTempDir` - Creates temporary directory under tmp.
