---
description: Find biggest source code file and split it out into smaller files
agent: build
---
Call following command to find big code file to split:

```
python3 scripts/bfscan.py 640 1
```

Script will return file and number of lines in that file. If script returns nothing, there is no big file to split and you can stop here.

File listed by script is too big and part of its code needs to be split into smaller files. Analyze file and find biggest group of related secondary functions/definitions that can be moved to a separate file:
* note that main part of code in this file serves its  main purpose (eg. file defining class and all its methods) - it should be retained in original file
  * but if file contains definitions of several classes (with a bunch of methods each), one can be considered as a main class and all other classes can be considered as secondary definitions
* closely related helper code (functions, secondary structs etc.) which together form a group of related functions/definitions should be moved to a separate file
  * find biggest such group and split it out; process only one such group, leave remaining helper code in original file
  * if there are no significant secondary definitions and file consists mostly of methods of its main class, consider finding big enough group of related methods and splitting them out
* look through all tests for original file and also split them out if needed
* new file should be placed in the same directory as original file and named `<original-file>_<group-name>.go`, where group name is a slug indicating purpose of the group, for example functions for generating session summary when moved from `session.go` should land in `session_summary.go`
  * note that if file you are splitting out is a test file (ends with `_test.go`), then you need to keep naming convention for test files, i.e. `<original-file-sans-test-suffix>_<group-name>_test.go` in order for tests and build run properly
* look through other files in the same package (and possibly other packages) to find functions that should belong to the group that is split out
  * for each such function, also move it to the new file along with all its tests
  * if such function was in other package and has dependencies that conflict with the new file (eg. would create circular dependency), consider factoring out logic if this function belonging to the group and then move only those parts that are not dependent on the group to the new file
  * when factoring out logic as described above, be sure to implement unit tests for this logic and move them along with logic to the new file
* update AGENTS.md file in directory with new file to reflect change
* at the end be sure to run all tests to ensure that there are no regressions
