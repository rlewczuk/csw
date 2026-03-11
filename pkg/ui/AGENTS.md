# Package `pkg/ui` Overview

Package `pkg/ui` contains frontend-agnostic UI contracts and view models for presenters and interfaces. It standardizes chat/app interaction APIs and data structures between presentation and UI layers.

## Important files

* `app_ui.go` - App-level UI contracts and message types
* `chat_ui.go` - Chat UI contracts and view models
* `composite.go` - Composite widget interface for composition
* `permissions_ui.go` - Permission query UI structure

## Important public API objects

* `IAppView` - Interface for rendering main app window
* `IRetryPromptView` - Extends app view with retry prompt support
* `IAppPresenter` - Interface for propagating user input to app
* `IChatView` - Interface for rendering chat conversation
* `IChatPresenter` - Interface for propagating user input to chat
* `CompositeWidget` - Interface for composable widgets
* `MessageType` - Type for user-facing status messages
* `ToolStatusUI` - Represents status of a tool call
* `ChatRoleUI` - Represents message sender role
* `ToolUI` - State of a tool call for UI rendering
* `ChatMessageUI` - Chat message for UI rendering
* `ChatSessionUI` - Chat session for UI rendering
* `PermissionQueryUI` - Structure for querying user permissions
* `CompositeNotification` - Notification type for composite widgets
