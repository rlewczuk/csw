# pkg/presenter

`pkg/presenter` connects core session/system behavior to UI interfaces. It translates user actions into session-thread operations and maps session outputs into chat/app view updates.

## Major files

- `app_presenter.go`: App-level presenter for session lifecycle actions (`NewSession`, `OpenSession`, `Exit`) and app-view wiring.
- `chat_presenter.go`: Chat presenter that bridges session-thread events to chat/app UI updates, including tool states and permission prompts.
