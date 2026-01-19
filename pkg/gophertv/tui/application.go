package tui

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/codesnort/codesnort-swe/pkg/gophertv"
	"github.com/codesnort/codesnort-swe/pkg/gophertv/tio"
	"golang.org/x/term"
)

// IApplication is an interface for the application that manages the main event loop,
// screen rendering, and input handling.
type IApplication interface {
	// Run starts the application main loop. This method blocks until the application exits.
	// It returns an error if initialization fails.
	Run() error

	// Quit signals the application to exit the main loop gracefully.
	Quit()

	// GetScreen returns the screen buffer for testing purposes.
	GetScreen() gophertv.IScreenOutput

	// InjectEvent injects an input event for testing purposes.
	// This allows deterministic testing without involving real terminal I/O.
	InjectEvent(event gophertv.InputEvent)

	// ProcessEvents processes all pending events in the event queue.
	// This is useful for testing to ensure all events are processed synchronously.
	ProcessEvents()
}

// TApplication is the main application struct that manages the TUI event loop.
// It handles screen rendering, input events, signal handling, and screen state management.
type TApplication struct {
	// Main widget of the application
	mainWidget IWidget

	// Screen buffer for rendering
	screen gophertv.IScreenOutput

	// Renderer for outputting to terminal (nil in test mode)
	renderer *tio.ScreenRenderer

	// Input event reader for reading from terminal (nil in test mode)
	eventReader *tio.InputEventReader

	// Event channel for receiving input events
	eventCh chan gophertv.InputEvent

	// Quit channel signals the application to exit
	quitCh chan struct{}

	// Terminal state management
	stdin           io.Reader
	stdout          io.Writer
	oldTermState    *term.State
	termFd          int
	savedScreen     []byte
	savedCursorX    int
	savedCursorY    int
	termInitialized bool

	// Test mode flag - when true, Run() returns immediately after initialization
	// and events are processed synchronously via ProcessEvents()
	testMode bool
}

// NewApplication creates a new TApplication with the given main widget.
// For production use with real terminal I/O:
//   - stdin should be os.Stdin
//   - stdout should be os.Stdout
//
// For testing without real terminal I/O:
//   - use NewApplicationForTest instead
func NewApplication(mainWidget IWidget, stdin io.Reader, stdout io.Writer) (*TApplication, error) {
	// Get terminal size
	var width, height int
	if f, ok := stdin.(*os.File); ok {
		w, h, err := term.GetSize(int(f.Fd()))
		if err != nil {
			// Use default size if we can't get terminal size
			width, height = 80, 24
		} else {
			width, height = w, h
		}
	} else {
		// Not a real terminal, use default size
		width, height = 80, 24
	}

	// Create screen buffer
	screen := tio.NewScreenBuffer(width, height, 0)

	// Create renderer
	renderer := tio.NewScreenRenderer(screen, stdout)

	// Set main widget size to fill screen
	mainWidget.HandleEvent(&TEvent{
		Type: TEventTypeResize,
		Rect: gophertv.TRect{X: 0, Y: 0, W: uint16(width), H: uint16(height)},
	})

	// Create event channel
	eventCh := make(chan gophertv.InputEvent, 100)

	// Get terminal file descriptor if available
	var termFd int = -1
	if f, ok := stdin.(*os.File); ok {
		termFd = int(f.Fd())
	}

	app := &TApplication{
		mainWidget:  mainWidget,
		screen:      screen,
		renderer:    renderer,
		eventReader: nil, // Will be created in Run()
		eventCh:     eventCh,
		quitCh:      make(chan struct{}, 1), // Buffered to allow non-blocking Quit()
		stdin:       stdin,
		stdout:      stdout,
		termFd:      termFd,
		testMode:    false,
	}

	return app, nil
}

// NewApplicationForTest creates a new TApplication for testing without real terminal I/O.
// The application uses the provided screen buffer and processes events synchronously.
// Use InjectEvent() to inject events and ProcessEvents() to process them.
func NewApplicationForTest(mainWidget IWidget, screen *tio.ScreenBuffer) *TApplication {
	// Get screen size
	width, height := screen.GetSize()

	// Set main widget size to fill screen
	mainWidget.HandleEvent(&TEvent{
		Type: TEventTypeResize,
		Rect: gophertv.TRect{X: 0, Y: 0, W: uint16(width), H: uint16(height)},
	})

	// Create event channel
	eventCh := make(chan gophertv.InputEvent, 100)

	app := &TApplication{
		mainWidget: mainWidget,
		screen:     screen,
		renderer:   nil, // No renderer in test mode
		eventCh:    eventCh,
		quitCh:     make(chan struct{}, 1), // Buffered to allow non-blocking Quit()
		testMode:   true,
	}

	return app
}

// Run starts the application main loop.
// This method blocks until the application exits.
// It performs the following:
// - Saves terminal state
// - Sets up raw mode
// - Enables alternative screen buffer
// - Sets up signal handlers (SIGWINCH, SIGINT, SIGTERM)
// - Starts input event reader
// - Enters main event loop
// - Restores terminal state on exit
func (app *TApplication) Run() error {
	// In test mode, just do initial render and return
	if app.testMode {
		// Draw initial frame
		app.mainWidget.Draw(app.screen)
		return nil
	}

	// Initialize terminal
	if err := app.initTerminal(); err != nil {
		return fmt.Errorf("TApplication.Run(): failed to initialize terminal: %w", err)
	}
	defer app.restoreTerminal()

	// Set up signal handlers
	app.setupSignalHandlers()

	// Create and start input event reader
	app.eventReader = tio.NewInputEventReader(app.stdin, app.stdout, app)
	if err := app.eventReader.Start(); err != nil {
		return fmt.Errorf("TApplication.Run(): failed to start event reader: %w", err)
	}
	defer app.eventReader.Stop()

	// Draw initial frame
	app.mainWidget.Draw(app.screen)
	if err := app.renderer.Render(); err != nil {
		return fmt.Errorf("TApplication.Run(): failed to render initial frame: %w", err)
	}

	// Main event loop
	for {
		select {
		case <-app.quitCh:
			return nil

		case event := <-app.eventCh:
			app.handleEvent(event)
		}
	}
}

// Quit signals the application to exit the main loop gracefully.
func (app *TApplication) Quit() {
	// Use non-blocking send to avoid deadlock
	// In test mode, this allows synchronous testing
	select {
	case app.quitCh <- struct{}{}:
	default:
		// Channel already has a quit signal or is closed
	}
}

// GetScreen returns the screen buffer for testing purposes.
func (app *TApplication) GetScreen() gophertv.IScreenOutput {
	return app.screen
}

// InjectEvent injects an input event for testing purposes.
// This allows deterministic testing without involving real terminal I/O.
func (app *TApplication) InjectEvent(event gophertv.InputEvent) {
	app.eventCh <- event
}

// ProcessEvents processes all pending events in the event queue.
// This is useful for testing to ensure all events are processed synchronously.
func (app *TApplication) ProcessEvents() {
	for {
		select {
		case event := <-app.eventCh:
			app.handleEvent(event)
		default:
			// No more events
			return
		}
	}
}

// Notify handles input events from the InputEventReader.
// This implements the InputEventHandler interface.
func (app *TApplication) Notify(event gophertv.InputEvent) {
	app.eventCh <- event
}

// handleEvent processes a single input event.
func (app *TApplication) handleEvent(event gophertv.InputEvent) {
	// Handle special events
	switch event.Type {
	case gophertv.InputEventResize:
		// Resize screen buffer
		app.screen.SetSize(int(event.X), int(event.Y))

		// Notify renderer of resize
		if app.renderer != nil {
			app.renderer.Reset()
		}

		// Notify main widget of resize
		app.mainWidget.HandleEvent(&TEvent{
			Type: TEventTypeResize,
			Rect: gophertv.TRect{X: 0, Y: 0, W: event.X, H: event.Y},
		})

		// Trigger redraw
		app.mainWidget.HandleEvent(&TEvent{Type: TEventTypeRedraw})

		// Redraw screen
		app.mainWidget.Draw(app.screen)
		if app.renderer != nil {
			app.renderer.Render()
		}
		return

	case gophertv.InputEventKey:
		// Handle Ctrl+C
		if event.Modifiers&gophertv.ModCtrl != 0 && event.Key == 'c' {
			app.Quit()
			return
		}
	}

	// Pass event to main widget
	app.mainWidget.HandleEvent(&TEvent{
		Type:       TEventTypeInput,
		InputEvent: &event,
	})

	// Redraw screen after handling event
	app.mainWidget.Draw(app.screen)
	if app.renderer != nil {
		app.renderer.Render()
	}
}

// initTerminal initializes the terminal for TUI mode.
func (app *TApplication) initTerminal() error {
	if app.termFd < 0 {
		return fmt.Errorf("TApplication.initTerminal(): invalid terminal file descriptor")
	}

	// Save current terminal state
	oldState, err := term.MakeRaw(app.termFd)
	if err != nil {
		return fmt.Errorf("TApplication.initTerminal(): failed to enable raw mode: %w", err)
	}
	app.oldTermState = oldState

	// Enable alternative screen buffer
	fmt.Fprint(app.stdout, "\x1b[?1049h")

	// Hide cursor
	fmt.Fprint(app.stdout, "\x1b[?25l")

	// Save cursor position
	fmt.Fprint(app.stdout, "\x1b[s")

	// Clear screen
	fmt.Fprint(app.stdout, "\x1b[2J\x1b[H")

	app.termInitialized = true
	return nil
}

// restoreTerminal restores the terminal to its original state.
func (app *TApplication) restoreTerminal() {
	if !app.termInitialized {
		return
	}

	// Show cursor
	fmt.Fprint(app.stdout, "\x1b[?25h")

	// Restore cursor position
	fmt.Fprint(app.stdout, "\x1b[u")

	// Disable alternative screen buffer
	fmt.Fprint(app.stdout, "\x1b[?1049l")

	// Restore terminal state
	if app.oldTermState != nil && app.termFd >= 0 {
		term.Restore(app.termFd, app.oldTermState)
	}

	app.termInitialized = false
}

// setupSignalHandlers sets up signal handlers for SIGWINCH, SIGINT, and SIGTERM.
func (app *TApplication) setupSignalHandlers() {
	// Set up SIGWINCH handling for terminal resize
	winchCh := make(chan os.Signal, 1)
	signal.Notify(winchCh, syscall.SIGWINCH)
	go func() {
		for range winchCh {
			// Get new terminal size
			if app.termFd >= 0 {
				width, height, err := term.GetSize(app.termFd)
				if err == nil && app.eventReader != nil {
					app.eventReader.NotifyResize(width, height)
				}
			}
		}
	}()

	// Set up SIGINT and SIGTERM handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		app.Quit()
	}()
}
