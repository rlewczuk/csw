package presenter

import (
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// AppPresenter implements ui.IAppPresenter.
// It manages the application-level interactions between the UI and the SweSystem.
type AppPresenter struct {
	mu           sync.Mutex
	system       *core.SweSystem
	view         ui.IAppView
	defaultModel string
}

// NewAppPresenter creates a new AppPresenter with the given system and default model.
func NewAppPresenter(system *core.SweSystem, defaultModel string) *AppPresenter {
	return &AppPresenter{
		system:       system,
		defaultModel: defaultModel,
	}
}

// SetView sets the view for the app presenter.
func (p *AppPresenter) SetView(view ui.IAppView) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.view = view
	return nil
}

// NewSession creates a new chat session.
func (p *AppPresenter) NewSession() error {
	p.mu.Lock()
	system := p.system
	view := p.view
	defaultModel := p.defaultModel
	p.mu.Unlock()

	// Create new SessionThread (will set output handler when creating ChatPresenter)
	thread := core.NewSessionThread(system, nil)

	// Start the session with default model
	if err := thread.StartSession(defaultModel); err != nil {
		return err
	}

	// Create ChatPresenter for this session
	// This will set the presenter as the output handler for the thread
	chatPresenter := NewChatPresenter(system, thread)

	if view != nil {
		// Show the chat view with the presenter
		chatView := view.ShowChat(chatPresenter)

		// Set the view on the presenter so it can update the view
		if err := chatPresenter.SetView(chatView); err != nil {
			return err
		}
	}

	return nil
}

// OpenSession reopens an existing chat session.
func (p *AppPresenter) OpenSession(id string) error {
	p.mu.Lock()
	system := p.system
	view := p.view
	p.mu.Unlock()

	// Get or create the thread for this session
	thread, err := system.GetSessionThread(id)
	if err != nil {
		return err
	}

	// Create ChatPresenter for this session
	// This will set the presenter as the output handler for the thread
	chatPresenter := NewChatPresenter(system, thread)

	if view != nil {
		// Show the chat view with the presenter
		chatView := view.ShowChat(chatPresenter)

		// Set the view on the presenter so it can update the view
		if err := chatPresenter.SetView(chatView); err != nil {
			return err
		}
	}

	return nil
}

// Exit exits the app and shuts down the system.
func (p *AppPresenter) Exit() error {
	p.mu.Lock()
	system := p.system
	p.mu.Unlock()

	system.Shutdown()
	return nil
}
