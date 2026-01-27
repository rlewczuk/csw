package tui

import (
	"embed"
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

//go:embed themes/*.theme.json
var themesFS embed.FS

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

	// ExecuteOnUiThread executes the given function on the UI thread.
	// If the main loop is running, the function is sent to the main loop thread for execution.
	// If the main loop is not running, the function is executed immediately with the mutex locked.
	// Parameters:
	//   f: Function to execute, should return any value
	//   redraw: If true, triggers a redraw after function completes
	//   wait: If true, blocks until function completes and returns the result
	// Returns: The value returned by f() if wait=true, nil otherwise
	ExecuteOnUiThread(f func() any, redraw bool, wait bool) any

	// RequestRedraw requests a redraw of the UI from any thread.
	// This is safe to call from background threads.
	RequestRedraw()
}

// uiTask represents a function to execute on the UI thread
type uiTask struct {
	f      func() any
	redraw bool
	done   chan any
}

// TApplication is the main application struct that manages the TUI event loop.
// It handles screen rendering, input events, signal handling, and screen state management.
// TApplication extends TFlexLayout, allowing it to manage multiple children using flex layout.
type TApplication struct {
	TFlexLayout

	// Screen buffer for rendering
	screen gtv.IScreenOutput

	// Renderer for outputting to terminal (nil when not running)
	renderer *tio.ScreenRenderer

	// Input event reader for reading from terminal (nil when not running)
	eventReader *tio.InputEventReader

	// Quit channel signals the application to exit
	quitCh chan struct{}

	// Redraw request channel signals that UI needs to be redrawn
	redrawCh chan struct{}

	// UI task channel for executing functions on the main loop thread
	uiTaskCh chan uiTask

	// Mutex for synchronizing access to application and widget state
	mu sync.Mutex

	// Running flag indicates whether the main loop is running
	running bool

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
// This function loads the default theme automatically. Use NewApplicationWithTheme for custom themes.
// Call Run() to start the application with real terminal I/O, or use Notify() and ExecuteOnUiThread()
// for testing without real terminal I/O.
//
// The main widget is added as a child to the application's flex layout with flex-grow=1 to fill the screen.
func NewApplication(mainWidget IWidget, screen gtv.IScreenOutput) *TApplication {
	// Load default theme
	themeManager, err := gtv.NewThemeManager(themesFS)
	if err != nil {
		// If theme loading fails, fall back to no theme
		return NewApplicationWithTheme(mainWidget, screen, nil)
	}

	defaultTheme, err := themeManager.GetTheme("default")
	if err != nil {
		// If default theme not found, fall back to no theme
		return NewApplicationWithTheme(mainWidget, screen, nil)
	}

	return NewApplicationWithTheme(mainWidget, screen, defaultTheme)
}

// NewApplicationEmpty creates a new TApplication without any children.
// Use AddChild() to add widgets to the application.
// This function loads the default theme automatically. Use NewApplicationEmptyWithTheme for custom themes.
func NewApplicationEmpty(screen gtv.IScreenOutput) *TApplication {
	// Load default theme
	themeManager, err := gtv.NewThemeManager(themesFS)
	if err != nil {
		// If theme loading fails, fall back to no theme
		return NewApplicationEmptyWithTheme(screen, nil)
	}

	defaultTheme, err := themeManager.GetTheme("default")
	if err != nil {
		// If default theme not found, fall back to no theme
		return NewApplicationEmptyWithTheme(screen, nil)
	}

	return NewApplicationEmptyWithTheme(screen, defaultTheme)
}

// NewApplicationWithTheme creates a new TApplication with the given main widget, screen buffer, and theme.
// If theme is not nil, a ThemeInterceptor is inserted into the screen output pipeline.
// The theme parameter is a map of theme tag strings to CellAttributes.
// The screen buffer size determines the initial widget size.
// Call Run() to start the application with real terminal I/O, or use Notify() and ExecuteOnUiThread()
// for testing without real terminal I/O.
//
// The main widget is added as a child to the application's flex layout with flex-grow=1 to fill the screen.
func NewApplicationWithTheme(mainWidget IWidget, screen gtv.IScreenOutput, theme map[string]gtv.CellAttributes) *TApplication {
	app := NewApplicationEmptyWithTheme(screen, theme)

	// Add main widget as a child with flex-grow to fill screen
	app.AddChild(mainWidget)
	app.SetItemProperties(mainWidget, FlexItemProperties{
		FlexGrow:   1.0,
		FlexShrink: 1.0,
	})

	// Set main widget as active child so it receives keyboard events
	app.ActiveChild = mainWidget

	return app
}

// NewApplicationEmptyWithTheme creates a new TApplication without any children, using the specified theme.
// Use AddChild() to add widgets to the application.
func NewApplicationEmptyWithTheme(screen gtv.IScreenOutput, theme map[string]gtv.CellAttributes) *TApplication {
	// Wrap screen with theme interceptor if theme is provided
	var finalScreen gtv.IScreenOutput = screen
	if theme != nil {
		finalScreen = gtv.NewThemeInterceptor(screen, theme)
	}

	// Get screen size
	width, height := finalScreen.GetSize()

	// Create flex layout base for the application
	// Use column direction by default, no parent (it's the root)
	flexLayout := newFlexLayoutBase(nil, gtv.TRect{X: 0, Y: 0, W: uint16(width), H: uint16(height)}, FlexDirectionColumn)

	// Disable tab navigation at the application level since child layouts handle it
	flexLayout.TabOrderEnabled = false

	app := &TApplication{
		TFlexLayout: *flexLayout,
		screen:      finalScreen,
		renderer:    nil,                    // Will be created in Run()
		eventReader: nil,                    // Will be created in Run()
		quitCh:      make(chan struct{}, 1), // Buffered to allow non-blocking Quit()
		redrawCh:    make(chan struct{}, 1), // Buffered to coalesce multiple redraw requests
		uiTaskCh:    make(chan uiTask, 100), // Buffered to allow async task submission
		running:     false,                  // Main loop not running yet
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

	// Notify application (and its children) of resize
	app.HandleEvent(&TEvent{
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
	app.TFlexLayout.Draw(app.screen)
	app.running = true
	app.mu.Unlock()
	if err := app.renderer.Render(); err != nil {
		return fmt.Errorf("TApplication.Run(): failed to render initial frame: %w", err)
	}

	// Main event loop
	for {
		select {
		case <-app.quitCh:
			app.mu.Lock()
			app.running = false
			app.mu.Unlock()
			return nil

		case event := <-eventCh:
			app.mu.Lock()
			app.handleEvent(event)
			app.mu.Unlock()

		case <-app.redrawCh:
			// Redraw requested from background thread
			app.mu.Lock()
			app.screen.SetCursorStyle(gtv.CursorStyleHidden)
			app.TFlexLayout.Draw(app.screen)
			app.mu.Unlock()
			if app.renderer != nil {
				app.renderer.Render()
			}

		case task := <-app.uiTaskCh:
			// Execute UI task on main loop thread
			app.mu.Lock()
			result := task.f()
			if task.redraw {
				app.screen.SetCursorStyle(gtv.CursorStyleHidden)
				app.TFlexLayout.Draw(app.screen)
			}
			app.mu.Unlock()

			if app.renderer != nil && task.redraw {
				app.renderer.Render()
			}

			// Signal completion if task is waiting
			if task.done != nil {
				task.done <- result
			}
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

func (app *TApplication) ExecuteOnUiThread(f func() any, redraw bool, wait bool) any {
	app.mu.Lock()
	isRunning := app.running
	app.mu.Unlock()

	if isRunning {
		// Main loop is running - send task to main loop thread
		var done chan any
		if wait {
			done = make(chan any, 1)
		}

		task := uiTask{
			f:      f,
			redraw: redraw,
			done:   done,
		}

		app.uiTaskCh <- task

		if wait {
			return <-done
		}
		return nil
	} else {
		// Main loop is not running - execute immediately with mutex locked
		app.mu.Lock()
		result := f()
		if redraw {
			app.screen.SetCursorStyle(gtv.CursorStyleHidden)
			app.TFlexLayout.Draw(app.screen)
		}
		app.mu.Unlock()

		if app.renderer != nil && redraw {
			app.renderer.Render()
		}

		return result
	}
}

// RequestRedraw requests a redraw of the UI from any thread.
// This is safe to call from background threads.
// Multiple calls are coalesced - only one redraw will happen.
func (app *TApplication) RequestRedraw() {
	// Non-blocking send - if channel is full, a redraw is already pending
	select {
	case app.redrawCh <- struct{}{}:
	default:
		// Redraw already pending, no need to queue another
	}
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

		// Notify application (and its children through TFlexLayout) of resize
		app.TFlexLayout.HandleEvent(&TEvent{
			Type: TEventTypeResize,
			Rect: gtv.TRect{X: 0, Y: 0, W: event.X, H: event.Y},
		})

		// Trigger redraw
		app.TFlexLayout.HandleEvent(&TEvent{Type: TEventTypeRedraw})

		// Redraw screen
		app.screen.SetCursorStyle(gtv.CursorStyleHidden)
		app.TFlexLayout.Draw(app.screen)
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

	// Pass event to application (which will route to children through TFlexLayout)
	app.TFlexLayout.HandleEvent(&TEvent{
		Type:       TEventTypeInput,
		InputEvent: &event,
	})

	// Redraw screen after handling event
	app.screen.SetCursorStyle(gtv.CursorStyleHidden)
	app.TFlexLayout.Draw(app.screen)
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

// GetApplication returns this application instance.
// This is the base case for the recursive GetApplication() chain defined in TWidget.
// When a widget calls GetApplication(), it recursively calls GetApplication() on its
// parent until reaching TApplication, which returns itself.
func (app *TApplication) GetApplication() *TApplication {
	return app
}

// AddChild overrides TFlexLayout.AddChild to ensure children have their Parent
// field set to *TApplication (not *TWidget). This is necessary because Go's
// struct embedding means that the embedded TWidget.AddChild() method would
// store *TWidget as the parent type in the interface, losing the concrete
// TApplication type information needed for GetApplication() to work correctly.
func (app *TApplication) AddChild(child IWidget) {
	app.TFlexLayout.Children = append(app.TFlexLayout.Children, child)
	child.SetParent(app) // Pass app as *TApplication, not embedded *TWidget

	// Initialize flex item properties (copied from TFlexLayout.AddChild)
	if _, ok := app.itemProperties[child]; !ok {
		childPos := child.GetPos()
		props := app.defaultItemPadding
		if props.FlexBasis == 0 {
			if app.direction == FlexDirectionRow {
				props.FlexBasis = childPos.W
			} else {
				props.FlexBasis = childPos.H
			}
		}
		app.itemProperties[child] = props
	}
	app.performLayout()
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
