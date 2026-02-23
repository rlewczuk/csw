package ui

// MessageType defines a type of user-facing status message.
type MessageType string

const (
	// MessageTypeInfo indicates an informational message.
	MessageTypeInfo MessageType = "info"
	// MessageTypeWarning indicates a warning message.
	MessageTypeWarning MessageType = "warning"
	// MessageTypeError indicates an error message.
	MessageTypeError MessageType = "error"
)

// IAppView is an interface for rendering the main app window.
type IAppView interface {
	// ShowChat switches to the chat view with the given presenter.
	// Chat view can be created or reused but this is internal to implementation.
	ShowChat(presenter IChatPresenter) IChatView

	// ShowSettings shows the settings view.
	ShowSettings()

	// ShowMessage shows a status message to the user.
	ShowMessage(message string, messageType MessageType)
}

// IAppPresenter is an interface for propagating user input from UI to the app.
type IAppPresenter interface {
	// NewSession creates a new chat session
	NewSession() error

	// OpenSession reopens an existing chat session
	OpenSession(id string) error

	// Exit exits the app.
	Exit() error
}
