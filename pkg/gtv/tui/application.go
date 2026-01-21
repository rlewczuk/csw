package tui

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"golang.org/x/term"
)

// eventChannelAdapter is an adapter that implements InputEventHandler interface
// and forwards events to a channel. This is used internally in Run() method.
type eventChannelAdapter struct {
	eventCh chan gtv.InputEvent
}

// Notify implements the InputEventHandler interface.
func (a *eventChannelAdapter) Notify(event gtv.InputEvent) {
	a.eventCh <- event
}

// IApplication is an interface for the application that manages the main event loop,
// screen rendering, and input handling.
type IApplication interface {
	// Run starts the application main loop. This method blocks until the application exits.
	// It returns an error if initialization fails.
	Run() error

	// Quit signals the application to exit the main loop gracefully.
	Quit()

	// GetScreen returns the screen buffer for testing purposes.
	GetScreen() gtv.IScreenOutput

	// ExecuteOnUiThread executes the given function with the UI mutex locked.
	// This ensures that application state is not modified concurrently.
	ExecuteOnUiThread(f func())
}

// TApplication is the main application struct that manages the TUI event loop.
// It handles screen rendering, input events, signal handling, and screen state management.
type TApplication struct {
	// Main widget of the application
	mainWidget IWidget

	// Screen buffer for rendering
	screen gtv.IScreenOutput

	// Renderer for outputting to terminal (nil when not running)
	renderer *tio.ScreenRenderer

	// Input event reader for reading from terminal (nil when not running)
	eventReader *tio.InputEventReader

	// Quit channel signals the application to exit
	quitCh chan struct{}

	// Mutex for synchronizing access to application and widget state
	mu sync.Mutex

	// Terminal state management
	stdin           io.Reader
	stdout          io.Writer
	oldTermState    *term.State
	termFd          int
	savedScreen     []byte
	savedCursorX    int
	savedCursorY    int
	termInitialized bool
}

// NewApplication creates a new TApplication with the given main widget and screen buffer.
// The screen buffer size determines the initial widget size.
// Call Run() to start the application with real terminal I/O, or use Notify() and ExecuteOnUiThread()
// for testing without real terminal I/O.
func NewApplication(mainWidget IWidget, screen gtv.IScreenOutput) *TApplication {
	return NewApplicationWithTheme(mainWidget, screen, nil)
}

// NewApplicationWithTheme creates a new TApplication with the given main widget, screen buffer, and theme.
// If theme is not nil, a ThemeInterceptor is inserted into the screen output pipeline.
// The theme parameter is a map of theme tag strings to CellAttributes.
// The screen buffer size determines the initial widget size.
// Call Run() to start the application with real terminal I/O, or use Notify() and ExecuteOnUiThread()
// for testing without real terminal I/O.
func NewApplicationWithTheme(mainWidget IWidget, screen gtv.IScreenOutput, theme map[string]gtv.CellAttributes) *TApplication {
	// Wrap screen with theme interceptor if theme is provided
	var finalScreen gtv.IScreenOutput = screen
	if theme != nil {
		finalScreen = gtv.NewThemeInterceptor(screen, theme)
	}

	// Get screen size
	width, height := finalScreen.GetSize()

	// Set main widget size to fill screen
	mainWidget.HandleEvent(&TEvent{
		Type: TEventTypeResize,
		Rect: gtv.TRect{X: 0, Y: 0, W: uint16(width), H: uint16(height)},
	})

	app := &TApplication{
		mainWidget:  mainWidget,
		screen:      finalScreen,
		renderer:    nil,                    // Will be created in Run()
		eventReader: nil,                    // Will be created in Run()
		quitCh:      make(chan struct{}, 1), // Buffered to allow non-blocking Quit()
	}

	return app
}

// Run starts the application main loop with real terminal I/O.
// This method blocks until the application exits.
// It performs the following:
// - Saves terminal state
// - Sets up raw mode
// - Enables alternative screen buffer
// - Sets up signal handlers (SIGWINCH, SIGINT, SIGTERM)
// - Starts input event reader
// - Enters main event loop
// - Restores terminal state on exit
//
// stdin should be os.Stdin and stdout should be os.Stdout for production use.
func (app *TApplication) Run(stdin io.Reader, stdout io.Writer) error {
	// Get terminal file descriptor
	var termFd int = -1
	if f, ok := stdin.(*os.File); ok {
		termFd = int(f.Fd())
	}
	if termFd < 0 {
		return fmt.Errorf("TApplication.Run(): stdin is not a valid terminal file")
	}

	// Get terminal size
	width, height, err := term.GetSize(termFd)
	if err != nil {
		return fmt.Errorf("TApplication.Run(): failed to get terminal size: %w", err)
	}

	// Resize screen buffer to match terminal
	app.screen.SetSize(width, height)

	// Notify main widget of resize
	app.mainWidget.HandleEvent(&TEvent{
		Type: TEventTypeResize,
		Rect: gtv.TRect{X: 0, Y: 0, W: uint16(width), H: uint16(height)},
	})

	// Create renderer
	app.renderer = tio.NewScreenRenderer(app.screen, stdout)

	// Store terminal state
	app.stdin = stdin
	app.stdout = stdout
	app.termFd = termFd

	// Initialize terminal
	if err := app.initTerminal(); err != nil {
		return fmt.Errorf("TApplication.Run(): failed to initialize terminal: %w", err)
	}
	defer app.restoreTerminal()

	// Set up signal handlers
	app.setupSignalHandlers()

	// Create local event channel for the event reader
	eventCh := make(chan gtv.InputEvent, 100)

	// Create and start input event reader with local channel
	app.eventReader = tio.NewInputEventReader(app.stdin, app.stdout, &eventChannelAdapter{eventCh: eventCh})
	if err := app.eventReader.Start(); err != nil {
		return fmt.Errorf("TApplication.Run(): failed to start event reader: %w", err)
	}
	defer app.eventReader.Stop()

	// Draw initial frame
	app.mu.Lock()
	app.screen.SetCursorStyle(gtv.CursorStyleHidden)
	app.mainWidget.Draw(app.screen)
	app.mu.Unlock()
	if err := app.renderer.Render(); err != nil {
		return fmt.Errorf("TApplication.Run(): failed to render initial frame: %w", err)
	}

	// Main event loop
	for {
		select {
		case <-app.quitCh:
			return nil

		case event := <-eventCh:
			app.mu.Lock()
			app.handleEvent(event)
			app.mu.Unlock()
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
func (app *TApplication) GetScreen() gtv.IScreenOutput {
	return app.screen
}

// ExecuteOnUiThread executes the given function with the UI mutex locked.
// This ensures that application state is not modified concurrently.
func (app *TApplication) ExecuteOnUiThread(f func()) {
	app.mu.Lock()
	defer app.mu.Unlock()
	f()
}

// Notify handles input events from the InputEventReader.
// This implements the InputEventHandler interface.
// It calls handleEvent directly with the mutex locked.
func (app *TApplication) Notify(event gtv.InputEvent) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.handleEvent(event)
}

// handleEvent processes a single input event.
func (app *TApplication) handleEvent(event gtv.InputEvent) {
	// Handle special events
	switch event.Type {
	case gtv.InputEventResize:
		// Resize screen buffer
		app.screen.SetSize(int(event.X), int(event.Y))

		// Notify renderer of resize
		if app.renderer != nil {
			app.renderer.Reset()
		}

		// Notify main widget of resize
		app.mainWidget.HandleEvent(&TEvent{
			Type: TEventTypeResize,
			Rect: gtv.TRect{X: 0, Y: 0, W: event.X, H: event.Y},
		})

		// Trigger redraw
		app.mainWidget.HandleEvent(&TEvent{Type: TEventTypeRedraw})

		// Redraw screen
		app.screen.SetCursorStyle(gtv.CursorStyleHidden)
		app.mainWidget.Draw(app.screen)
		if app.renderer != nil {
			app.renderer.Render()
		}
		return

	case gtv.InputEventKey:
		// Handle Ctrl+C
		if event.Modifiers&gtv.ModCtrl != 0 && event.Key == 'c' {
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
	app.screen.SetCursorStyle(gtv.CursorStyleHidden)
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

	// Enable mouse tracking
	fmt.Fprint(app.stdout, "\x1b[?1000h") // Enable mouse button tracking
	fmt.Fprint(app.stdout, "\x1b[?1002h") // Enable mouse motion tracking
	fmt.Fprint(app.stdout, "\x1b[?1015h") // Enable urxvt mouse mode
	fmt.Fprint(app.stdout, "\x1b[?1006h") // Enable SGR mouse mode

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

	// Disable mouse tracking
	fmt.Fprint(app.stdout, "\x1b[?1000l") // Disable mouse button tracking
	fmt.Fprint(app.stdout, "\x1b[?1002l") // Disable mouse motion tracking
	fmt.Fprint(app.stdout, "\x1b[?1015l") // Disable urxvt mouse mode
	fmt.Fprint(app.stdout, "\x1b[?1006l") // Disable SGR mouse mode

	// Show cursor
	fmt.Fprint(app.stdout, "\x1b[?25h")

	// Reset cursor style to default (blinking block)
	fmt.Fprint(app.stdout, "\x1b[0 q")

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
