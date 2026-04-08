# Package `pkg/ui` Overview

Package `pkg/ui` defines frontend-agnostic UI contracts and view models. It standardizes presenter-to-view communication for package path `pkg/ui`.

## Important files

* `app_ui.go` - App-level interfaces and message types
* `chat_ui.go` - Chat interfaces, models, and tool view types
* `composite.go` - Composite widget notifications and parent wiring
* `permissions_ui.go` - Permission query payload for UI prompts

## Important public API objects

* `IAppView` - Renders app shell, settings, and diagnostics.
* `IRetryPromptView` - App view with retry confirmation prompt.
* `IAppPresenter` - Handles app session lifecycle commands.
* `IChatView` - Renders and updates chat state.
* `IChatPresenter` - Handles chat actions and control commands.
* `CompositeWidget` - Composable widget notification contract.
* `MessageType` - Enum: info, warning, error.
* `ToolStatusUI` - Enum: started, executing, succeeded, failed.
* `ChatRoleUI` - Enum: assistant, user.
* `CompositeNotification` - Enum: refresh.
* `ToolUI` - Tool call render state model.
* `ChatMessageUI` - Chat message render model.
* `ChatSessionUI` - Chat session render snapshot.
* `PermissionQueryUI` - Permission prompt model with options.
