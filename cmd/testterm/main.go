package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"golang.org/x/term"
)

// DemoApp represents the demo application state.
type DemoApp struct {
	screen       *tio.ScreenBuffer
	renderer     *tio.ScreenRenderer
	eventReader  *tio.InputEventReader
	lastEvent    *gtv.InputEvent
	eventCount   int
	width        int
	height       int
	colors       []uint32
	colorIndex   int
	statusText   string
	borderStyle  int
	flashCount   int
	oldTermState *term.State // Store terminal state for restoration
	cursorStyle  int         // Current cursor style index
}

// NewDemoApp creates a new demo application.
func NewDemoApp(width, height int) *DemoApp {
	screen := tio.NewScreenBuffer(width, height, 0)
	renderer := tio.NewScreenRenderer(screen, os.Stdout)

	return &DemoApp{
		screen:      screen,
		renderer:    renderer,
		width:       width,
		height:      height,
		lastEvent:   nil,
		eventCount:  0,
		colors:      []uint32{0xFF0000, 0x00FF00, 0x0000FF, 0xFFFF00, 0xFF00FF, 0x00FFFF, 0xFF8800, 0x8800FF},
		colorIndex:  0,
		statusText:  "Ready",
		borderStyle: 0,
	}
}

// Notify handles input events.
func (app *DemoApp) Notify(event gtv.InputEvent) {
	// Store the event
	app.lastEvent = &event
	app.eventCount++

	// Handle specific keys to trigger visual changes
	if event.Type == gtv.InputEventKey {
		// Handle function keys (ModFn set)
		if event.Modifiers&gtv.ModFn != 0 {
			switch event.Key {
			case 'I':
				// Insert key - rotate cursor style
				// First cycle through non-blinking styles, then blinking styles, then all combined
				app.cursorStyle = (app.cursorStyle + 1) % 13
				styles := []gtv.CursorStyle{
					gtv.CursorStyleDefault,
					gtv.CursorStyleBlock,
					gtv.CursorStyleUnderline,
					gtv.CursorStyleBar,
					gtv.CursorStyleHidden,
					gtv.CursorStyleDefault | gtv.CursorStyleBlinking,
					gtv.CursorStyleBlock | gtv.CursorStyleBlinking,
					gtv.CursorStyleUnderline | gtv.CursorStyleBlinking,
					gtv.CursorStyleBar | gtv.CursorStyleBlinking,
					gtv.CursorStyleDefault | gtv.CursorStyleBlinking,
					gtv.CursorStyleBlock | gtv.CursorStyleBlinking,
					gtv.CursorStyleUnderline | gtv.CursorStyleBlinking,
					gtv.CursorStyleBar | gtv.CursorStyleBlinking,
				}
				styleNames := []string{
					"Default",
					"Block",
					"Underline",
					"Bar",
					"Hidden",
					"Default Blinking",
					"Block Blinking",
					"Underline Blinking",
					"Bar Blinking",
					"Default Blinking",
					"Block Blinking",
					"Underline Blinking",
					"Bar Blinking",
				}
				app.screen.SetCursorStyle(styles[app.cursorStyle])
				// Set cursor to random position to demonstrate cursor positioning
				x := rand.Intn(app.width)
				y := rand.Intn(app.height)
				app.screen.MoveCursor(x, y)
				app.statusText = fmt.Sprintf("Cursor style: %s at (%d, %d)", styleNames[app.cursorStyle], x, y)
			case 'N':
				// PageDown key - move cursor to random location
				x := rand.Intn(app.width)
				y := rand.Intn(app.height)
				app.screen.MoveCursor(x, y)
				app.statusText = fmt.Sprintf("Cursor moved to (%d, %d)", x, y)
			}
		} else {
			switch event.Key {
			case 'q':
				// Quit on 'q' or Shift+Q
				app.cleanup()
				os.Exit(0)
			case 'c':
				// Change color on 'c' or Shift+C
				app.colorIndex = (app.colorIndex + 1) % len(app.colors)
				app.statusText = fmt.Sprintf("Color changed to #%06X", app.colors[app.colorIndex])
			case 'b':
				// Change border style on 'b'
				app.borderStyle = (app.borderStyle + 1) % 3
				app.statusText = "Border style changed"
			case 'f':
				// Flash animation on 'f' or Shift+F
				app.flashCount = (app.flashCount + 1) % 10
				app.statusText = fmt.Sprintf("Flash count: %d", app.flashCount)
			case 'r':
				// Reset on 'r' or Shift+R
				app.lastEvent = nil
				app.eventCount = 0
				app.colorIndex = 0
				app.borderStyle = 0
				app.flashCount = 0
				app.cursorStyle = 0
				app.screen.SetCursorStyle(gtv.CursorStyleDefault)
				app.screen.MoveCursor(0, 0)
				app.statusText = "Reset complete"
			}
		}
	} else if event.Type == gtv.InputEventResize {
		// Update size
		app.width = int(event.X)
		app.height = int(event.Y)
		app.statusText = fmt.Sprintf("Resized to %dx%d", app.width, app.height)

		// Create new screen buffer with new dimensions to avoid mangled frame
		//app.screen = cswterm.NewScreenBuffer(app.width, app.height, 0)
		//app.renderer = cswterm.NewScreenRenderer(app.screen, os.Stdout)
		app.renderer.Reset()
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
	app.screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, centerText("Terminal Input Event Demo", app.width),
		gtv.AttrsWithColor(gtv.AttrBold, headerColor, 0))

	// Draw instructions
	instructionColor := uint32(0xAAAAAA)
	app.screen.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, centerText("q=quit c=color b=border f=flash r=reset Ins=cursor PgDn=move", app.width),
		gtv.AttrsWithColor(0, instructionColor, 0))

	// Draw top border
	app.drawBorder()

	// Draw event list
	app.drawEventList()

	// Draw status bar
	statusColor := uint32(0x00FFFF)
	statusBg := uint32(0x333333)
	statusLine := fmt.Sprintf(" Status: %s | Events: %d ",
		app.statusText, app.eventCount)
	app.screen.PutText(gtv.TRect{X: 0, Y: uint16(app.height - 1), W: 0, H: 0}, padRight(statusLine, app.width),
		gtv.AttrsWithColor(gtv.AttrBold, statusColor, statusBg))

	// Draw flash indicator if active
	if app.flashCount > 0 {
		flashColor := app.colors[(app.flashCount)%len(app.colors)]
		app.screen.PutText(gtv.TRect{X: uint16(app.width - 10), Y: 3, W: 0, H: 0}, "* FLASH *",
			gtv.AttrsWithColor(gtv.AttrBold|gtv.AttrBlink, flashColor, 0))
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

	app.screen.PutText(gtv.TRect{X: 0, Y: 2, W: 0, H: 0}, fullBorder,
		gtv.AttrsWithColor(0, borderColor, 0))
}

// drawEventList draws the event details in a centered frame.
func (app *DemoApp) drawEventList() {
	startY := 3
	endY := app.height - 2

	if app.lastEvent == nil {
		emptyText := "No events yet. Press some keys!"
		app.screen.PutText(gtv.TRect{X: uint16((app.width - len(emptyText)) / 2), Y: uint16((startY + endY) / 2), W: 0, H: 0}, emptyText,
			gtv.AttrsWithColor(gtv.AttrItalic, 0x888888, 0))
		return
	}

	// Calculate frame dimensions
	frameWidth := min(app.width-4, 70)
	frameHeight := min(endY-startY-2, 20)
	frameX := (app.width - frameWidth) / 2
	frameY := startY + (endY-startY-frameHeight)/2

	// Draw frame border
	borderColor := uint32(0x00AAFF)
	// Top border
	app.screen.PutText(gtv.TRect{X: uint16(frameX), Y: uint16(frameY), W: 0, H: 0}, "┌"+padRight("", frameWidth-2)+"┐",
		gtv.AttrsWithColor(0, borderColor, 0))
	// Bottom border
	app.screen.PutText(gtv.TRect{X: uint16(frameX), Y: uint16(frameY + frameHeight - 1), W: 0, H: 0}, "└"+padRight("", frameWidth-2)+"┘",
		gtv.AttrsWithColor(0, borderColor, 0))
	// Side borders
	for i := 1; i < frameHeight-1; i++ {
		app.screen.PutText(gtv.TRect{X: uint16(frameX), Y: uint16(frameY + i), W: 0, H: 0}, "│",
			gtv.AttrsWithColor(0, borderColor, 0))
		app.screen.PutText(gtv.TRect{X: uint16(frameX + frameWidth - 1), Y: uint16(frameY + i), W: 0, H: 0}, "│",
			gtv.AttrsWithColor(0, borderColor, 0))
	}

	// Draw event details
	event := *app.lastEvent
	y := frameY + 1
	labelColor := uint32(0xAAAAAAA)
	valueColor := uint32(0xFFFFFF)

	// Event string representation
	if y < frameY+frameHeight-1 {
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 2), Y: uint16(y), W: 0, H: 0}, "Event:",
			gtv.AttrsWithColor(gtv.AttrBold, labelColor, 0))
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 12), Y: uint16(y), W: 0, H: 0}, event.String(),
			gtv.AttrsWithColor(0, valueColor, 0))
		y++
	}

	// Event type
	if y < frameY+frameHeight-1 {
		y++
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 2), Y: uint16(y), W: 0, H: 0}, "Type:",
			gtv.AttrsWithColor(gtv.AttrBold, labelColor, 0))
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 12), Y: uint16(y), W: 0, H: 0}, eventTypeName(event.Type),
			gtv.AttrsWithColor(0, valueColor, 0))
		y++
	}

	// Key value (hex and character)
	if y < frameY+frameHeight-1 {
		y++
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 2), Y: uint16(y), W: 0, H: 0}, "Key:",
			gtv.AttrsWithColor(gtv.AttrBold, labelColor, 0))
		keyStr := fmt.Sprintf("0x%04X", event.Key)
		if event.Key >= 32 && event.Key <= 126 {
			keyStr += fmt.Sprintf(" ('%c')", event.Key)
		}
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 12), Y: uint16(y), W: 0, H: 0}, keyStr,
			gtv.AttrsWithColor(0, valueColor, 0))
		y++
	}

	// X and Y coordinates
	if y < frameY+frameHeight-1 {
		y++
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 2), Y: uint16(y), W: 0, H: 0}, "X, Y:",
			gtv.AttrsWithColor(gtv.AttrBold, labelColor, 0))
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 12), Y: uint16(y), W: 0, H: 0}, fmt.Sprintf("%d, %d", event.X, event.Y),
			gtv.AttrsWithColor(0, valueColor, 0))
		y++
	}

	// Content (for copy/paste events)
	if event.Content != "" && y < frameY+frameHeight-1 {
		y++
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 2), Y: uint16(y), W: 0, H: 0}, "Content:",
			gtv.AttrsWithColor(gtv.AttrBold, labelColor, 0))
		content := event.Content
		maxLen := frameWidth - 14
		if len(content) > maxLen {
			content = content[:maxLen-3] + "..."
		}
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 12), Y: uint16(y), W: 0, H: 0}, content,
			gtv.AttrsWithColor(0, valueColor, 0))
		y++
	}

	// Modifiers
	if y < frameY+frameHeight-1 {
		y++
		app.screen.PutText(gtv.TRect{X: uint16(frameX + 2), Y: uint16(y), W: 0, H: 0}, "Modifiers:",
			gtv.AttrsWithColor(gtv.AttrBold, labelColor, 0))
		modNames := modifierNames(event.Modifiers)
		if len(modNames) == 0 {
			app.screen.PutText(gtv.TRect{X: uint16(frameX + 12), Y: uint16(y), W: 0, H: 0}, "(none)",
				gtv.AttrsWithColor(0, 0x888888, 0))
		} else {
			modY := y
			for i, name := range modNames {
				if modY < frameY+frameHeight-1 {
					app.screen.PutText(gtv.TRect{X: uint16(frameX + 12), Y: uint16(modY), W: 0, H: 0}, name,
						gtv.AttrsWithColor(0, valueColor, 0))
					modY++
				}
				if i == 0 {
					y = modY
				}
			}
			if modY > y {
				y = modY
			}
		}
	}
}

// eventTypeName returns the human-readable name of an event type.
func eventTypeName(t gtv.InputEventType) string {
	switch t {
	case gtv.InputEventKey:
		return "Key"
	case gtv.InputEventMouse:
		return "Mouse"
	case gtv.InputEventResize:
		return "Resize"
	case gtv.InputEventCopy:
		return "Copy"
	case gtv.InputEventPaste:
		return "Paste"
	case gtv.InputEventFocus:
		return "Focus"
	case gtv.InputEventBlur:
		return "Blur"
	default:
		return "Unknown"
	}
}

// modifierNames returns a list of modifier names from the modifiers bitfield.
func modifierNames(mods gtv.EventModifiers) []string {
	var names []string
	if mods&gtv.ModShift != 0 {
		names = append(names, "Shift")
	}
	if mods&gtv.ModAlt != 0 {
		names = append(names, "Alt")
	}
	if mods&gtv.ModCtrl != 0 {
		names = append(names, "Ctrl")
	}
	if mods&gtv.ModMeta != 0 {
		names = append(names, "Meta")
	}
	if mods&gtv.ModClick != 0 {
		names = append(names, "Click")
	}
	if mods&gtv.ModDoubleClick != 0 {
		names = append(names, "DoubleClick")
	}
	if mods&gtv.ModDrag != 0 {
		names = append(names, "Drag")
	}
	if mods&gtv.ModPress != 0 {
		names = append(names, "Press")
	}
	if mods&gtv.ModRelease != 0 {
		names = append(names, "Release")
	}
	if mods&gtv.ModMove != 0 {
		names = append(names, "Move")
	}
	if mods&gtv.ModScrollUp != 0 {
		names = append(names, "ScrollUp")
	}
	if mods&gtv.ModScrollDown != 0 {
		names = append(names, "ScrollDown")
	}
	if mods&gtv.ModFn != 0 {
		names = append(names, "Fn")
	}
	return names
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
	// Restore terminal to original state (this re-enables echo and line buffering)
	if app.oldTermState != nil {
		term.Restore(int(os.Stdin.Fd()), app.oldTermState)
	}

	app.renderer.ShowCursor()
	fmt.Print("\x1b[?1049l") // Disable alternative buffer
	fmt.Print("\x1b[?25h")   // Show cursor
	fmt.Print("\x1b[?1000l") // Disable mouse tracking
	fmt.Print("\x1b[?1002l") // Disable mouse motion tracking
	fmt.Print("\x1b[?1015l") // Disable urxvt mouse mode
	fmt.Print("\x1b[?1006l") // Disable SGR mouse mode
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

	// Set up signal handling for interrupt/terminate
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		app.cleanup()
		os.Exit(0)
	}()

	// Set up SIGWINCH handling for terminal resize
	winchCh := make(chan os.Signal, 1)
	signal.Notify(winchCh, syscall.SIGWINCH)
	go func() {
		for range winchCh {
			// Get new terminal size
			newWidth, newHeight, err := term.GetSize(int(os.Stdin.Fd()))
			if err == nil && app.eventReader != nil {
				app.eventReader.NotifyResize(newWidth, newHeight)
			}
		}
	}()

	// Enable raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to enable raw mode: %v\n", err)
		os.Exit(1)
	}
	app.oldTermState = oldState // Store for cleanup
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

	// Create and start input event reader
	app.eventReader = tio.NewInputEventReader(os.Stdin, os.Stdout, app)
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
