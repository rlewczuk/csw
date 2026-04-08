# Package `pkg/core/testfixture` Overview

Package `pkg/core/testfixture` provides fixtures for testing `pkg/core/testfixture` integrations.

## Important files

* `fixture.go` - SweSystem fixture builders and options

## Important public API objects

* `SweSystemFixture` - Holds system fixture dependencies
* `SweSystemFixtureConfig` - Fixture configuration values
* `SweSystemFixtureOption` - Functional fixture option
* `StaticPromptGenerator` - Fixed prompt generator stub
* `NewSweSystemFixture()` - Build fixture with defaults
* `NewStaticPromptGenerator()` - Build fixed prompt generator
* `WithPromptGenerator()` - Override prompt generator
* `WithVFS()` - Override VFS implementation
* `WithTools()` - Override tool registry
* `WithModelProvider()` - Set default model provider
* `WithModelProviders()` - Set provider map
* `WithProviderName()` - Set provider key name
* `WithWorkDir()` - Set fixture work directory
* `WithSessionLoggerFactory()` - Set session logger factory
* `WithRoles()` - Set role registry
* `WithConfigStore()` - Set config store
* `WithLSP()` - Set LSP implementation
* `WithLogBaseDir()` - Set log base directory
* `WithLogLLMRequests()` - Toggle LLM request logging
* `WithoutVFSTools()` - Disable VFS tool registration
