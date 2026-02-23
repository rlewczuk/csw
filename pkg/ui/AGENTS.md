# pkg/ui

`pkg/ui` defines frontend-agnostic UI contracts and view models used by presenters and concrete interfaces (CLI/TUI/log rendering). It standardizes chat/app interaction APIs and message/tool/session data structures passed between presentation and UI layers.

## Major files

- `chat_ui.go`: Core chat UI contracts (`IChatView`, `IChatPresenter`) and major chat/session/tool DTOs for rendering state.
- `app_ui.go`: App-level contracts (`IAppView`, `IAppPresenter`, `IRetryPromptView`) and typed status message model.
