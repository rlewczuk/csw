# Package `pkg/ui/mock` Overview

Package `pkg/ui/mock` contains mock implementations of UI interfaces for testing purposes. It provides test doubles for app and chat view/presenter interfaces with call recording and configurable error responses.

## Important files

* `app_mock.go` - Mock implementations for app UI interfaces
* `chat_mock.go` - Mock implementations for chat UI interfaces

## Important public API objects

* `MockAppView` - Mock implementation of ui.IAppView interface
* `MockAppPresenter` - Mock implementation of ui.IAppPresenter interface
* `MockChatView` - Mock implementation of ui.IChatView interface
* `MockChatPresenter` - Mock implementation of ui.IChatPresenter interface
* `MockAppMessageCall` - Stores one ShowMessage invocation record
* `NewMockAppView` - Creates a new MockAppView instance
* `NewMockAppPresenter` - Creates a new MockAppPresenter instance
* `NewMockChatView` - Creates a new MockChatView instance
* `NewMockChatPresenter` - Creates a new MockChatPresenter instance
