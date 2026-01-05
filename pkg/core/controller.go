package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// SessionController manages a session thread with input and context management.
// It ensures that SweSession is only called from a background thread and that
// at most one session loop is running at a time.
type SessionController struct {
	system        *SweSystem
	outputHandler ui.SessionOutputHandler

	mu               sync.Mutex
	session          *SweSession
	sessionRunning   bool
	backgroundCtx    context.Context
	cancelFunc       context.CancelFunc
	inputQueue       []string
	interruptPending bool
}

// NewSessionController creates a new SessionController with the given system and output handler.
func NewSessionController(system *SweSystem, outputHandler ui.SessionOutputHandler) *SessionController {
	return &SessionController{
		system:        system,
		outputHandler: outputHandler,
		inputQueue:    make([]string, 0),
	}
}

// GetSession returns the current session. Safe to call from any thread.
func (c *SessionController) GetSession() *SweSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session
}

// UserPrompt adds a user prompt to the session and starts processing if not already running.
// This method is non-blocking and thread-safe.
func (c *SessionController) UserPrompt(input string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Initialize session if not already created
	if c.session == nil {
		return fmt.Errorf("session not initialized, call StartSession first")
	}

	// Add to input queue
	c.inputQueue = append(c.inputQueue, input)

	// If session is not running, start it
	if !c.sessionRunning {
		c.startSessionLocked()
	}

	return nil
}

// Interrupt cancels the current running session.
// This method is non-blocking and thread-safe.
func (c *SessionController) Interrupt() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.sessionRunning {
		return fmt.Errorf("no session running to interrupt")
	}

	c.interruptPending = true
	if c.cancelFunc != nil {
		c.cancelFunc()
	}

	return nil
}

// StartSession initializes a new session with the given model.
// This method is non-blocking and thread-safe.
func (c *SessionController) StartSession(model string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		return fmt.Errorf("session already exists")
	}

	session, err := c.system.NewSession(model, c.outputHandler)
	if err != nil {
		return err
	}

	c.session = session
	return nil
}

// startSessionLocked starts the background session loop.
// Must be called with c.mu held.
func (c *SessionController) startSessionLocked() {
	if c.sessionRunning {
		return
	}

	c.sessionRunning = true
	c.backgroundCtx, c.cancelFunc = context.WithCancel(context.Background())

	go c.runSessionLoop()
}

// runSessionLoop is the background thread that processes the session.
func (c *SessionController) runSessionLoop() {
	ctx := c.backgroundCtx

	defer func() {
		c.mu.Lock()
		c.sessionRunning = false
		c.backgroundCtx = nil
		c.cancelFunc = nil
		c.mu.Unlock()
	}()

	for {
		// Get next input from queue
		c.mu.Lock()
		if len(c.inputQueue) == 0 {
			// No more input, stop the loop
			c.mu.Unlock()
			c.outputHandler.RunFinished(nil)
			return
		}

		input := c.inputQueue[0]
		c.inputQueue = c.inputQueue[1:]
		c.interruptPending = false
		c.mu.Unlock()

		// Add user prompt to session (this is safe because we're in the background thread)
		err := c.session.UserPrompt(input)
		if err != nil {
			c.outputHandler.RunFinished(err)
			return
		}

		// Run the session
		err = c.session.Run(ctx)

		// Check if we were interrupted
		c.mu.Lock()
		wasInterrupted := c.interruptPending
		c.mu.Unlock()

		if wasInterrupted {
			c.outputHandler.RunFinished(fmt.Errorf("interrupted"))
			return
		}

		if err != nil {
			c.outputHandler.RunFinished(err)
			return
		}
	}
}
