# Package `pkg/presenter` Overview

Package `pkg/presenter` connects UI events to `pkg/presenter` session workflows.

## Important files

* `app_presenter.go` - App presenter for session lifecycle
* `chat_presenter.go` - Chat presenter and thread output bridge

## Important public API objects

* `AppPresenter` - Application presenter implementation
* `ChatPresenter` - Chat presenter and output adapter
* `NewAppPresenter()` - Create app presenter
* `NewChatPresenter()` - Create chat presenter
