# Package `pkg/core/testfixture` Overview

Package `pkg/core/testfixture` provides preconfigured test fixtures for building `system.SweSystem` instances used in `pkg/core` integration tests.

## Important files

* `fixture.go` - SweSystem fixture builders and options

## Important public API objects

* `SweSystemFixture` - Holds fixture system and dependencies
* `SweSystemFixtureConfig` - Fixture creation configuration values
* `SweSystemFixtureOption` - Functional fixture option type
* `StaticPromptGenerator` - Fixed prompt generator for tests
* `NewSweSystemFixture()` - Creates fixture with defaults
* `NewStaticPromptGenerator()` - Creates fixed prompt generator
* `WithPromptGenerator()` - Sets custom prompt generator
* `WithVFS()` - Sets custom VFS implementation
* `WithTools()` - Sets custom tool registry
* `WithModelProvider()` - Sets default model provider
* `WithModelProviders()` - Sets model provider map
* `WithProviderName()` - Sets provider key name
* `WithWorkDir()` - Sets system work directory
* `WithSessionLoggerFactory()` - Sets session logger factory
* `WithRoles()` - Sets role registry
* `WithConfigStore()` - Sets config store
* `WithLSP()` - Sets LSP implementation
* `WithLogBaseDir()` - Sets log base directory
* `WithLogLLMRequests()` - Toggles LLM request logging
* `WithoutVFSTools()` - Disables VFS tool registration
