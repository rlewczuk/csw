# Main Components

## Agent Core

Contains core logic and workflow utilities, including:

* task management:
  * saving and restoring task state;
  * running tasks;
  * splitting tasks (along with configurable prompts that do it)
* session management (saving)
* role management
* rule management
* blueprint management
* recipe management
* action management
* core loop;
* API provider for user interfaces;


## LLM Providers

Unified interface for accessint LLMs provides following functionalities:
 
* model listing and selection;
* model querying with streaming;
* rate limiting;
* error handling;
* authentication;
* model configuration;
* model cost tracking;

## Runtime Management

Managed sandboxed (or non-sandboxed) runtimes for building, testing, running and debugging applications.
Currently two variants of runtime are supported:
* local runtime (local process);
* containerized runtime (using docker);


## Virtual Filesystem

Virtual system provides access:
* access to local and remote files and directories;
  * 
* versioning and checkpointing;
* access to build and test artifacts;

## Tool Management


## Text User Interface



## Code Navigation

This module presents an interface that unifies all means to search and navigate over codebase, including:
* LSP-based navigation;
* AI-assisted navigation;
* Symbol search;
* Dependency graph;
* Code structure view;
* Pattern search (grep);

