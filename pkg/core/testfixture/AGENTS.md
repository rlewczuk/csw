# Package `pkg/core/testfixture` Overview

Package `pkg/core/testfixture` provides core integration test fixtures for creating pre-configured SweSystem instances with mock dependencies.

## Important files

* `fixture.go` - SweSystem test fixtures with configurable options

## Important public API objects

* `SweSystemFixture` - Holds configured SweSystem and dependencies
* `SweSystemFixtureConfig` - Configuration for SweSystemFixture creation
* `SweSystemFixtureOption` - Option function for fixture configuration
* `StaticPromptGenerator` - Fixed prompt generator for tests
* `NewSweSystemFixture()` - Creates SweSystemFixture with defaults
* `NewStaticPromptGenerator()` - Creates static prompt generator
* `WithPromptGenerator()` - Sets custom prompt generator
* `WithVFS()` - Sets custom VFS implementation
* `WithTools()` - Sets custom tool registry
* `WithModelProvider()` - Sets default model provider
* `WithModelProviders()` - Sets model providers map
* `WithProviderName()` - Sets provider name
* `WithWorkDir()` - Sets system work directory
* `WithSessionLoggerFactory()` - Sets session logger factory
* `WithRoles()` - Sets role registry
* `WithConfigStore()` - Sets config store
* `WithLSP()` - Sets LSP instance
* `WithLogBaseDir()` - Sets base directory for logs
* `WithLogLLMRequests()` - Sets LLM request logging
* `WithoutVFSTools()` - Disables VFS tool registration
