---
name: agents-update
description: Update AGENTS.md files across project
---

Analyze all packagfes of this project (`pkg/*`, `cmd/*`) and generate/update package-level `AGENTS.md` files with information for future agent operation:
* for each package containing actual code (`*.go` files), generate or update `AGENTS.md` file inside this package with following content:
    * section that contains short description (few lines) of the module:
        * it should describe package purpose and list main functionalities provided by this package
        * always mention exact path of the package (relative to project root) in section header and section content
    * subsection that contains list of important source files inside package,
        * each item containing file name and short one-liner describing its purpose (less than 10 words)
            * make sure one-liner is short summary (no more than 10 words), do not add extensive details nor any symbols/object names from the file
        * it should be formatted as a bullet list, each item in following format: "* `frobnicate.go` - implementation of frobnication algorithm"
            * DO NOT add any additional information, if in existing file there are additional things (eg. function count, symbols, references to other files/packages etc.), remove them
    * subsection that lists most important public objects (types, structs, interfaces, enums) exported by this package
      * all public interfaces should be listed
      * most important public structs and public constructor functions should be listed
      * public enum types should be listed; with short description and list of enum values in a single line
      * public functions that are used outside package should be listed
      * each listed item should be one-liner, with description no longer than 10 words, with following format: "* `IFrobnicator` - interface for frobnication"
* if `pkg/<dir>` contains subdirectories, also process them recursively and generate `AGENTS.md` files inside each subdirectory thatn contains actual code (`*.go` files)
* DO NOT use scripts to automate generation of AGENTS.md, manually analyze each package, including all its source files
    * descriptions for individual interfaces, functions, enums etc. should reflect business meaning obtained via deep analysis, not just generic statements like "business service"
    * do not do shortcuts, read all code carefully and perform deep analysis
* there may be existing package-level `AGENTS.md` files, read them and update their content to account for any changes in application code that occured since last update of those files
    * please keep changes focused, do not overhaul whole file if not absolutely necessary
    * if file format differs from what is described in this task, update files to match strictly above format
    * any additional sections/subsections other than ones described above (to be generated/updated) should be kept intact; DO NOT modify nor remove them
* after analyzing packages, look at top level `AGENTS.md` file, find and update section that lists all packages
  * it should contain list of all packages that contain actual code and have `AGENTS.md` file generated
  * each item in the list should be one-liner, with following format: "* `pkg/frob` - data frobnication, foozling and bar-ing algorithms"
  * other sections in this file should be kept intact
  * before updating top level `AGENTS.md` file, wait for all packages to be analyzed and updated
  * package description can contain one or two most important symbols/objects from the package, but make sure it is no longer than 20 words
  * reuse descriptions from package-level `AGENTS.md` files generated previously when generating descriptions for this list, DO NOT read all source code again
* there are a lot of packages in this project, use subagents make work parallel across packages

Example of package-level `AGENTS.md` file:

<example>

# Package `pkg/frob` Overview

Package `pkg/frob` contains data frobnication, foozling and bar-ing algorithms. Those algorithms are used for various purposes,
including data obfuscation, generating random test data, maintaining data uniqueness. Both reversible and irreversible 
algorithms are provided along with examples of how to use them. See test files for usage examples.

## Important files

* `frobnicate.go` - implementation of frobnication algorithm
* `foozle.go` - implementation of foozling algorithm
* `bar.go` - implementation of bar-ing algorithm
* `test_foozle.go` - example of how to use `foozle` algorithm
* `test_bar.go` - example of how to use `bar` algorithm
* `test_frobnicate.go` - example of how to use `frobnicate` algorithm

## Important public API objects

* `IFrobnicator` - interface for frobnication
* `Frobnicator` - implementation of frobnication algorithm
* `FrobnicatorConfig` - configuration for frobnication algorithm
* `Foozler` - implementation of foozling algorithm
* `FoozlerConfig` - configuration for foozling algorithm
* `FoozlerAlgorithm` - enum that describes foozling algorithm, with following values: `SHA256`, `MD5`, `BORK`;
* `Bar` - implementation of bar-ing algorithm
* `Frobnicate` - function that frobnicates data

# Additional code style guidelines

(this is user provided section that should be retained intact during update)

# Additional notes

(this is user provided section that should be retained intact during update)
</example>

