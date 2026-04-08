# Package `pkg/ui/cli/testfixture` Overview

Package `pkg/ui/cli/testfixture` offers reusable CLI integration-test setup wrappers. It composes presenter fixtures for tests in `pkg/ui/cli/testfixture`.

## Important files

* `fixture.go` - CLI fixture wrapping presenter test fixture

## Important public API objects

* `CliFixture` - Shared CLI integration fixture container.
* `NewCliFixture` - Builds fixture with SweSystem options.
