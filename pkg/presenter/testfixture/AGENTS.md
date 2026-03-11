# Package `pkg/presenter/testfixture` Overview

Package `pkg/presenter/testfixture` provides presenter integration test fixtures.

## Important files

* `fixture.go` - Presenter test fixture with helper methods

## Important public API objects

* `PresenterFixture` - Shared setup for presenter integration tests
* `NewPresenterFixture` - Creates presenter fixture with SweSystem
* `NewSessionThread` - Creates session thread for fixture system
* `NewSessionThreadWithSession` - Creates thread with existing session
* `NewAppPresenter` - Creates AppPresenter for fixture system
* `NewChatPresenter` - Creates ChatPresenter for fixture system
