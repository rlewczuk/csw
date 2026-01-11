package ui

// AppView is an interface for rendering the main app window.
type AppView interface {
	// ShowChat shows the chat view.
	ShowChat(view ChatView)

	// ShowSettings shows the settings view.
	ShowSettings()
}

// AppPresenter is an interface for propagating user input from UI to the app.
type AppPresenter interface {
	// NewSession creates a new chat session.
	NewSession() error

	// OpenSession opens an existing chat session.
	OpenSession(id string) error

	// Exit exits the app.
	Exit() error
}
