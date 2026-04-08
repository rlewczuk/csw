# Package `pkg/ui/mock` Overview

Package `pkg/ui/mock` provides UI test doubles with call recording and errors. It covers app and chat contracts for tests under `pkg/ui/mock`.

## Important files

* `app_mock.go` - App view/presenter mocks with call tracking
* `chat_mock.go` - Chat view/presenter mocks with auto-permission support

## Important public API objects

* `MockAppView` - App view mock with recorded method calls.
* `MockAppPresenter` - App presenter mock with configurable errors.
* `MockChatView` - Chat view mock with optional auto responses.
* `MockChatPresenter` - Chat presenter mock recording user actions.
* `MockAppMessageCall` - Captures one ShowMessage call payload.
* `NewMockAppView` - Creates initialized app view mock.
* `NewMockAppPresenter` - Creates app presenter mock instance.
* `NewMockChatView` - Creates empty chat view mock.
* `NewMockChatPresenter` - Creates chat presenter mock instance.
