package main

import (
	"fmt"
	"os"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/mdv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
)

func main() {
	// Check command-line arguments
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <markdown-file>\n", os.Args[0])
		os.Exit(1)
	}

	// Read the markdown file
	filePath := os.Args[1]
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "main(): error reading file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	// Create a screen buffer with initial size (will be resized to terminal size in Run())
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create markdown view widget that fills the entire screen
	// The width and height will be updated during app.Run()
	mdView := mdv.NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, string(content))

	// Create the application with the markdown view as root widget
	app := tui.NewApplication(mdView, screen)

	// Wrap the markdown view to intercept 'q' key for quitting
	wrapper := &markdownViewWrapper{
		mdView: mdView,
		app:    app,
	}

	// Replace the main widget with our wrapper
	// We need to create a new application with the wrapper
	app = tui.NewApplication(wrapper, screen)

	// Run the application
	if err := app.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "main(): error running application: %v\n", err)
		os.Exit(1)
	}
}

// markdownViewWrapper wraps the markdown view and handles quit events
type markdownViewWrapper struct {
	mdView *mdv.TMarkdownView
	app    *tui.TApplication
}

// HandleEvent intercepts events before passing them to the markdown view
func (w *markdownViewWrapper) HandleEvent(event *tui.TEvent) {
	// Handle quit key: 'q'
	if event.Type == tui.TEventTypeInput && event.InputEvent != nil {
		input := event.InputEvent
		if input.Type == gtv.InputEventKey {
			// Check for 'q' key
			if input.Key == 'q' || input.Key == 'Q' {
				w.app.Quit()
				return
			}
		}
	}

	// Pass the event to the markdown view
	w.mdView.HandleEvent(event)
}

// Draw delegates drawing to the markdown view
func (w *markdownViewWrapper) Draw(screen gtv.IScreenOutput) {
	w.mdView.Draw(screen)
}

// GetPos delegates to the markdown view
func (w *markdownViewWrapper) GetPos() gtv.TRect {
	return w.mdView.GetPos()
}

// GetAbsolutePos delegates to the markdown view
func (w *markdownViewWrapper) GetAbsolutePos() gtv.TRect {
	return w.mdView.GetAbsolutePos()
}

// AddChild delegates to the markdown view
func (w *markdownViewWrapper) AddChild(child tui.IWidget) {
	w.mdView.AddChild(child)
}

// GetParent delegates to the markdown view
func (w *markdownViewWrapper) GetParent() tui.IWidget {
	return w.mdView.GetParent()
}

// SetParent delegates to the markdown view
func (w *markdownViewWrapper) SetParent(parent tui.IWidget) {
	w.mdView.SetParent(parent)
}
