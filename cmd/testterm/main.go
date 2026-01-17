package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/codesnort/codesnort-swe/pkg/cswterm"
	"golang.org/x/term"
)

// DemoApp represents the demo application state.
type DemoApp struct {
	screen       *cswterm.ScreenBuffer
	renderer     *cswterm.ScreenRenderer
	eventReader  *cswterm.InputEventReader
	events       []cswterm.InputEvent
	scrollOffset int
	width        int
	height       int
	colors       []uint32
	colorIndex   int
	statusText   string
	borderStyle  int
	flashCount   int
}

// NewDemoApp creates a new demo application.
func NewDemoApp(width, height int) *DemoApp {
	screen := cswterm.NewMockScreen(width, height, 0)
	renderer := cswterm.NewScreenRenderer(screen, os.Stdout)

	return &DemoApp{
		screen:      screen,
		renderer:    renderer,
		width:       width,
		height:      height,
		events:      make([]cswterm.InputEvent, 0),
		colors:      []uint32{0xFF0000, 0x00FF00, 0x0000FF, 0xFFFF00, 0xFF00FF, 0x00FFFF, 0xFF8800, 0x8800FF},
		colorIndex:  0,
		statusText:  "Ready",
		borderStyle: 0,
	}
}

// Notify handles input events.
func (app *DemoApp) Notify(event cswterm.InputEvent) {
	// Add event to history
	app.events = append(app.events, event)

	// Handle specific keys to trigger visual changes
	if event.Type == cswterm.InputEventKey {
		// Handle arrow keys first (they have no modifiers from CSI sequences)
		if event.Key == 'A' && event.Modifiers == 0 {
			// Up arrow
			app.scrollOffset = max(0, app.scrollOffset-1)
			app.statusText = "Scrolled up"
		} else if event.Key == 'B' && event.Modifiers == 0 {
			// Down arrow
			maxScroll := max(0, len(app.events)-(app.height-8))
			app.scrollOffset = min(maxScroll, app.scrollOffset+1)
			app.statusText = "Scrolled down"
		} else if event.Key == 'P' && event.Modifiers == 0 {
			// Page Up
			app.scrollOffset = max(0, app.scrollOffset-10)
			app.statusText = "Scrolled up (page)"
		} else if event.Key == 'N' && event.Modifiers == 0 {
			// Page Down
			maxScroll := max(0, len(app.events)-(app.height-8))
			app.scrollOffset = min(maxScroll, app.scrollOffset+10)
			app.statusText = "Scrolled down (page)"
		} else {
			// Regular key handling
			switch event.Key {
			case 'q', 'Q':
				// Quit on 'q' or 'Q'
				app.cleanup()
				os.Exit(0)
			case 'c', 'C':
				// Change color on 'c' or 'C'
				app.colorIndex = (app.colorIndex + 1) % len(app.colors)
				app.statusText = fmt.Sprintf("Color changed to #%06X", app.colors[app.colorIndex])
			case 'b':
				// Change border style on 'b'
				app.borderStyle = (app.borderStyle + 1) % 3
				app.statusText = "Border style changed"
			case 'f', 'F':
				// Flash animation on 'f' or 'F'
				app.flashCount = (app.flashCount + 1) % 10
				app.statusText = fmt.Sprintf("Flash count: %d", app.flashCount)
			case 'r', 'R':
				// Reset on 'r' or 'R'
				app.events = make([]cswterm.InputEvent, 0)
				app.scrollOffset = 0
				app.colorIndex = 0
				app.borderStyle = 0
				app.flashCount = 0
				app.statusText = "Reset complete"
			}
		}
	} else if event.Type == cswterm.InputEventResize {
		// Update size
		app.width = int(event.X)
		app.height = int(event.Y)
		app.statusText = fmt.Sprintf("Resized to %dx%d", app.width, app.height)
	}

	// Render the updated screen
	app.render()
}

// render renders the current state to the screen.
func (app *DemoApp) render() {
	// Clear screen
	app.screen.Clear()

	// Draw header
	headerColor := app.colors[app.colorIndex]
	app.screen.PutText(0, 0, centerText("Terminal Input Event Demo", app.width),
		cswterm.AttrsWithColor(cswterm.AttrBold, headerColor, 0))

	// Draw instructions
	instructionColor := uint32(0xAAAAAA)
	app.screen.PutText(0, 1, centerText("Press keys to see events | q=quit c=color b=border f=flash r=reset", app.width),
		cswterm.AttrsWithColor(0, instructionColor, 0))

	// Draw top border
	app.drawBorder()

	// Draw event list
	app.drawEventList()

	// Draw status bar
	statusColor := uint32(0x00FFFF)
	statusBg := uint32(0x333333)
	statusLine := fmt.Sprintf(" Status: %s | Events: %d | Scroll: %d ",
		app.statusText, len(app.events), app.scrollOffset)
	app.screen.PutText(0, app.height-1, padRight(statusLine, app.width),
		cswterm.AttrsWithColor(cswterm.AttrBold, statusColor, statusBg))

	// Draw flash indicator if active
	if app.flashCount > 0 {
		flashColor := app.colors[(app.flashCount)%len(app.colors)]
		app.screen.PutText(app.width-10, 3, "* FLASH *",
			cswterm.AttrsWithColor(cswterm.AttrBold|cswterm.AttrBlink, flashColor, 0))
	}

	// Render to terminal
	app.renderer.Render()
}

// drawBorder draws the border based on current style.
func (app *DemoApp) drawBorder() {
	borderColor := uint32(0x888888)
	borderChars := []string{
		"─────────────────────────",
		"═════════════════════════",
		"▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀",
	}

	borderChar := borderChars[app.borderStyle]
	fullBorder := ""
	for i := 0; i < app.width; i++ {
		fullBorder += string([]rune(borderChar)[i%len([]rune(borderChar))])
	}

	app.screen.PutText(0, 2, fullBorder,
		cswterm.AttrsWithColor(0, borderColor, 0))
}

// drawEventList draws the scrollable event list.
func (app *DemoApp) drawEventList() {
	startY := 3
	endY := app.height - 2
	visibleLines := endY - startY

	if len(app.events) == 0 {
		emptyText := "No events yet. Press some keys!"
		app.screen.PutText((app.width-len(emptyText))/2, (startY+endY)/2, emptyText,
			cswterm.AttrsWithColor(cswterm.AttrItalic, 0x888888, 0))
		return
	}

	// Draw events from the scroll offset
	for i := 0; i < visibleLines; i++ {
		eventIdx := app.scrollOffset + i
		if eventIdx >= len(app.events) {
			break
		}

		event := app.events[eventIdx]
		eventText := formatEvent(eventIdx+1, event)

		// Truncate if too long
		if len(eventText) > app.width {
			eventText = eventText[:app.width-3] + "..."
		}

		// Use different colors for different event types
		var color uint32
		switch event.Type {
		case cswterm.InputEventKey:
			color = 0x00FF88
		case cswterm.InputEventMouse:
			color = 0xFF8800
		case cswterm.InputEventResize:
			color = 0x8800FF
		default:
			color = 0xFFFFFF
		}

		app.screen.PutText(0, startY+i, eventText,
			cswterm.AttrsWithColor(0, color, 0))
	}

	// Draw scroll indicator
	if len(app.events) > visibleLines {
		scrollColor := uint32(0xFFFF00)
		scrollIndicator := fmt.Sprintf("[%d/%d]", app.scrollOffset+1, len(app.events))
		app.screen.PutText(app.width-len(scrollIndicator), startY,
			scrollIndicator, cswterm.AttrsWithColor(cswterm.AttrBold, scrollColor, 0))
	}
}

// formatEvent formats an input event as a string.
func formatEvent(index int, event cswterm.InputEvent) string {
	typeStr := ""
	switch event.Type {
	case cswterm.InputEventKey:
		typeStr = "KEY"
	case cswterm.InputEventMouse:
		typeStr = "MOUSE"
	case cswterm.InputEventResize:
		typeStr = "RESIZE"
	default:
		typeStr = "OTHER"
	}

	details := ""
	if event.Type == cswterm.InputEventKey {
		keyName := formatKey(event.Key)
		modStr := formatModifiers(event.Modifiers)
		if modStr != "" {
			details = fmt.Sprintf("%s+%s", modStr, keyName)
		} else {
			details = keyName
		}
	} else if event.Type == cswterm.InputEventMouse {
		details = fmt.Sprintf("x=%d y=%d %s", event.X, event.Y, formatModifiers(event.Modifiers))
	} else if event.Type == cswterm.InputEventResize {
		details = fmt.Sprintf("w=%d h=%d", event.X, event.Y)
	}

	return fmt.Sprintf("%4d | %-6s | %s", index, typeStr, details)
}

// formatKey formats a key code as a string.
func formatKey(key rune) string {
	switch key {
	case 0x1B:
		return "ESC"
	case 0x09:
		return "TAB"
	case 0x0D:
		return "ENTER"
	case 0x0A:
		return "LF"
	case 'A':
		return "↑"
	case 'B':
		return "↓"
	case 'C':
		return "→"
	case 'D':
		return "←"
	case 'H':
		return "HOME"
	case 'F':
		return "END"
	case 'I':
		return "INSERT"
	case 'P':
		return "PGUP"
	case 'N':
		return "PGDN"
	default:
		if key >= 32 && key < 127 {
			return fmt.Sprintf("'%c'", key)
		}
		return fmt.Sprintf("0x%02X", key)
	}
}

// formatModifiers formats event modifiers as a string.
func formatModifiers(mods cswterm.EventModifiers) string {
	parts := []string{}
	if mods&cswterm.ModCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&cswterm.ModAlt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&cswterm.ModShift != 0 {
		parts = append(parts, "Shift")
	}
	if mods&cswterm.ModMeta != 0 {
		parts = append(parts, "Meta")
	}
	if mods&cswterm.ModFn != 0 {
		parts = append(parts, "Fn")
	}
	if mods&cswterm.ModPress != 0 {
		parts = append(parts, "Press")
	}
	if mods&cswterm.ModRelease != 0 {
		parts = append(parts, "Release")
	}
	if mods&cswterm.ModMove != 0 {
		parts = append(parts, "Move")
	}
	if mods&cswterm.ModScrollUp != 0 {
		parts = append(parts, "ScrollUp")
	}
	if mods&cswterm.ModScrollDown != 0 {
		parts = append(parts, "ScrollDown")
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "+"
		}
		result += part
	}
	return result
}

// centerText centers text in a given width.
func centerText(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	padding := (width - len(text)) / 2
	result := ""
	for i := 0; i < padding; i++ {
		result += " "
	}
	result += text
	for len(result) < width {
		result += " "
	}
	return result
}

// padRight pads text to the right.
func padRight(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	result := text
	for len(result) < width {
		result += " "
	}
	return result
}

// cleanup restores terminal state.
func (app *DemoApp) cleanup() {
	app.renderer.ShowCursor()
	fmt.Print("\x1b[?1049l") // Disable alternative buffer
	fmt.Print("\x1b[?25h")   // Show cursor
	fmt.Print("\x1b[?1000l") // Disable mouse tracking
	fmt.Print("\x1b[2J")     // Clear screen
	fmt.Print("\x1b[H")      // Move cursor to home
}

func main() {
	// Get terminal size
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		width, height = 80, 24
	}

	// Create demo app
	app := NewDemoApp(width, height)

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		app.cleanup()
		os.Exit(0)
	}()

	// Enable raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to enable raw mode: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Enable alternative buffer
	fmt.Print("\x1b[?1049h")
	defer fmt.Print("\x1b[?1049l")

	// Enable mouse tracking
	fmt.Print("\x1b[?1000h") // Enable mouse button tracking
	fmt.Print("\x1b[?1002h") // Enable mouse motion tracking
	fmt.Print("\x1b[?1015h") // Enable urxvt mouse mode
	fmt.Print("\x1b[?1006h") // Enable SGR mouse mode
	defer fmt.Print("\x1b[?1000l")

	// Hide cursor
	if err := app.renderer.HideCursor(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hide cursor: %v\n", err)
		os.Exit(1)
	}
	defer app.renderer.ShowCursor()

	// Create and start input event reader
	app.eventReader = cswterm.NewInputEventReader(os.Stdin, os.Stdout, app)
	if err := app.eventReader.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start event reader: %v\n", err)
		os.Exit(1)
	}
	defer app.eventReader.Stop()

	// Initial render
	app.render()

	// Wait forever (event reader runs in background)
	select {}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
