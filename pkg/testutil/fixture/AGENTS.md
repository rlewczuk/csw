# Package `pkg/testutil/fixture` Overview

Package `pkg/testutil/fixture` provides reusable test fixture helpers for locating project paths and managing temporary directories during tests.

## Important files

* `paths.go` - project path utilities and temp directory helpers

## Important public API objects

* `ProjectRoot` - returns absolute path to repository root
* `ProjectPath` - returns absolute path under repository root
* `ProjectTmpDir` - returns projectRoot/tmp and ensures it exists
* `MkProjectTempDir` - creates temporary directory inside projectRoot/tmp
