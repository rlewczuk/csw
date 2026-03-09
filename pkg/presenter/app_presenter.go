package presenter

import (
	"sync"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/ui"
)

type sessionThreadProvider interface {
	core.SessionFactory
	GetSessionThread(id string) (*core.SessionThread, error)
	Shutdown()
}

// AppPresenter implements ui.IAppPresenter.
// It manages the application-level interactions between the UI and the SweSystem.
type AppPresenter struct {
	mu           sync.Mutex
	system       sessionThreadProvider
	view         ui.IAppView
	defaultModel string
	defaultRole  string
}

// NewAppPresenter creates a new AppPresenter with the given system, default model, and default role.
func NewAppPresenter(system sessionThreadProvider, defaultModel string, defaultRole string) *AppPresenter {
	return &AppPresenter{
		system:       system,
		defaultModel: defaultModel,
		defaultRole:  defaultRole,
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
	defaultRole := p.defaultRole
	p.mu.Unlock()

	// Create new SessionThread (will set output handler when creating ChatPresenter)
	thread := core.NewSessionThread(system, nil)

	// Start the session with default model
	if err := thread.StartSession(defaultModel); err != nil {
		return err
	}

	// Set the default role if provided
	if defaultRole != "" {
		session := thread.GetSession()
		if session != nil {
			if err := session.SetRole(defaultRole); err != nil {
				return err
			}
		}
	}

	// Create ChatPresenter for this session
	// This will set the presenter as the output handler for the thread
	chatPresenter := NewChatPresenter(system, thread)
	chatPresenter.SetAppView(view)

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
	chatPresenter.SetAppView(view)

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
