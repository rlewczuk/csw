# Package `pkg/presenter` Overview

Package `pkg/presenter` connects core session/system behavior to UI interfaces. It translates user actions into session-thread operations and maps session outputs into chat/app view updates.

## Important files

* `app_presenter.go` - App-level presenter for session lifecycle actions
* `chat_presenter.go` - Chat presenter bridging session events to UI updates

## Important public API objects

* `AppPresenter` - App-level presenter implementing ui.IAppPresenter
* `NewAppPresenter` - Creates AppPresenter with system and defaults
* `ChatPresenter` - Chat presenter implementing ui.IChatPresenter
* `NewChatPresenter` - Creates ChatPresenter for a session thread
