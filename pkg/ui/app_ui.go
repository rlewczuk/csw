package ui

// IAppView is an interface for rendering the main app window.
type IAppView interface {
	// ShowChat switches to the chat view with the given presenter.
	// Chat view can be created or reused but this is internal to implementation.
	ShowChat(presenter IChatPresenter) IChatView

	// ShowSettings shows the settings view.
	ShowSettings()
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
