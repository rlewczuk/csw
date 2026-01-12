package tui

import (
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/stretchr/testify/assert"
)

func TestPermissionQueryWidget(t *testing.T) {
	t.Run("initial state and visibility", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Id:      "test-query",
			Title:   "Test Permission",
			Details: "Do you want to proceed?",
			Options: []string{"Yes", "No"},
		}

		callbackCalled := false
		widget := NewPermissionQueryWidget(query, func(response string) {
			callbackCalled = true
		})

		assert.False(t, widget.IsVisible(), "Widget should not be visible initially")
		assert.False(t, widget.IsClosed(), "Widget should not be closed initially")
		assert.Equal(t, query, widget.query)
		assert.Equal(t, 0, widget.cursor)
		assert.False(t, callbackCalled, "Callback should not be called initially")
	})

	t.Run("show and hide functionality", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test",
			Options: []string{"Option 1"},
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})

		widget.Show()
		assert.True(t, widget.IsVisible(), "Widget should be visible after Show()")

		widget.Hide()
		assert.False(t, widget.IsVisible(), "Widget should not be visible after Hide()")
	})

	t.Run("widget renders with title and details", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Details: "This is a test permission query with details.",
			Options: []string{"Allow", "Deny"},
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for widget to render
		assert.True(t, term.WaitForText("Test Permission", 1*time.Second), "Widget should display title")
		assert.True(t, term.WaitForText("This is a test permission query with details.", 1*time.Second), "Widget should display details")
		assert.True(t, term.WaitForText("Allow", 1*time.Second), "Widget should display first option")
		assert.True(t, term.WaitForText("Deny", 1*time.Second), "Widget should display second option")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("arrow key navigation", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Options: []string{"Option 1", "Option 2", "Option 3"},
		}

		callbackCalled := make(chan string, 1)
		widget := NewPermissionQueryWidget(query, func(response string) {
			callbackCalled <- response
		})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Option 1", 1*time.Second))

		// Navigate down
		term.SendKey("down")
		term.SendKey("down")
		term.SendKey("up")

		// Wait a bit for processing
		time.Sleep(50 * time.Millisecond)

		// Verify cursor moved
		assert.Equal(t, 1, widget.cursor, "Cursor should be at index 1 after navigation")

		// Cleanup - pressing esc will call callback with empty string
		term.SendKey("esc")

		// Wait for callback to be called
		select {
		case response := <-callbackCalled:
			assert.Equal(t, "", response, "Response should be empty when dismissed with esc")
		case <-time.After(1 * time.Second):
			t.Fatal("Callback should be called when widget is dismissed")
		}

		// Cleanup
		term.Close()
	})

	t.Run("enter key selects option", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Options: []string{"Option 1", "Option 2", "Option 3"},
		}

		callbackCalled := make(chan string, 1)
		widget := NewPermissionQueryWidget(query, func(response string) {
			callbackCalled <- response
		})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Option 1", 1*time.Second))

		// Navigate to second option
		term.SendKey("down")

		// Wait a bit for navigation to process
		time.Sleep(50 * time.Millisecond)

		// Select option
		term.SendKey("enter")

		// Wait for callback to be called
		select {
		case response := <-callbackCalled:
			assert.Equal(t, "Option 2", response, "Selected option should be Option 2")
		case <-time.After(1 * time.Second):
			t.Fatal("Callback should be called when option is selected")
		}

		// Cleanup
		term.Close()

		assert.True(t, widget.IsClosed(), "Widget should be closed after selection")
		assert.False(t, widget.IsVisible(), "Widget should not be visible after selection")
	})

	t.Run("esc key dismisses widget and calls callback with empty string", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Options: []string{"Option 1"},
		}

		callbackCalled := make(chan string, 1)
		widget := NewPermissionQueryWidget(query, func(response string) {
			callbackCalled <- response
		})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Test Permission", 1*time.Second))

		// Dismiss widget
		term.SendKey("esc")

		// Wait for callback to be called
		select {
		case response := <-callbackCalled:
			assert.Equal(t, "", response, "Response should be empty string when dismissed")
		case <-time.After(1 * time.Second):
			t.Fatal("Callback should be called when widget is dismissed")
		}

		// Cleanup
		term.Close()

		assert.True(t, widget.IsClosed(), "Widget should be closed after esc")
		assert.False(t, widget.IsVisible(), "Widget should not be visible after esc")
	})

	t.Run("custom response option appears when allowed", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:               "Test Permission",
			Options:             []string{"Yes", "No"},
			AllowCustomResponse: "Enter custom message",
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Test Permission", 1*time.Second))
		assert.True(t, term.WaitForText("Yes", 1*time.Second))
		assert.True(t, term.WaitForText("No", 1*time.Second))
		assert.True(t, term.WaitForText("Enter custom message", 1*time.Second), "Custom response option should be displayed")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("navigate to custom response option", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:               "Test Permission",
			Options:             []string{"Yes", "No"},
			AllowCustomResponse: "Enter custom message",
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Test Permission", 1*time.Second))

		// Navigate to custom response option
		term.SendKey("down")
		term.SendKey("down")

		// Wait a bit for navigation to process
		time.Sleep(50 * time.Millisecond)

		// Verify cursor is at custom input option (index 2)
		assert.Equal(t, 2, widget.cursor)

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("enter on custom response option activates text input", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:               "Test Permission",
			Options:             []string{"Yes", "No"},
			AllowCustomResponse: "Enter custom message",
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Test Permission", 1*time.Second))

		// Navigate to custom response option
		term.SendKey("down")
		term.SendKey("down")

		// Wait a bit for navigation to process
		time.Sleep(50 * time.Millisecond)

		// Press enter to activate text input
		term.SendKey("enter")

		// Wait a bit for text input to activate
		time.Sleep(50 * time.Millisecond)

		// Verify text input is focused
		assert.True(t, widget.textInput.Focused(), "Text input should be focused")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("enter custom text response", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:               "Test Permission",
			Options:             []string{"Yes", "No"},
			AllowCustomResponse: "Enter custom message",
		}

		callbackCalled := make(chan string, 1)
		widget := NewPermissionQueryWidget(query, func(response string) {
			callbackCalled <- response
		})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Test Permission", 1*time.Second))

		// Navigate to custom response option
		term.SendKey("down")
		term.SendKey("down")

		// Wait a bit for navigation to process
		time.Sleep(50 * time.Millisecond)

		// Press enter to activate text input
		term.SendKey("enter")

		// Wait a bit for text input to activate
		time.Sleep(50 * time.Millisecond)

		// Type custom message
		term.SendString("custom message here")

		// Wait a bit for text to be entered
		time.Sleep(50 * time.Millisecond)

		// Press enter to submit
		term.SendKey("enter")

		// Wait for callback to be called
		select {
		case response := <-callbackCalled:
			assert.Equal(t, "custom message here", response, "Response should be the custom message")
		case <-time.After(1 * time.Second):
			t.Fatal("Callback should be called when custom message is submitted")
		}

		// Cleanup
		term.Close()

		assert.True(t, widget.IsClosed(), "Widget should be closed after submission")
	})

	t.Run("esc in custom input mode cancels and calls callback with empty string", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:               "Test Permission",
			Options:             []string{"Yes", "No"},
			AllowCustomResponse: "Enter custom message",
		}

		callbackCalled := make(chan string, 1)
		widget := NewPermissionQueryWidget(query, func(response string) {
			callbackCalled <- response
		})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Test Permission", 1*time.Second))

		// Navigate to custom response option
		term.SendKey("down")
		term.SendKey("down")

		// Wait a bit for navigation to process
		time.Sleep(50 * time.Millisecond)

		// Press enter to activate text input
		term.SendKey("enter")

		// Wait a bit for text input to activate
		time.Sleep(50 * time.Millisecond)

		// Type partial message
		term.SendString("partial")

		// Wait a bit for text to be entered
		time.Sleep(50 * time.Millisecond)

		// Press esc to cancel
		term.SendKey("esc")

		// Wait for callback to be called
		select {
		case response := <-callbackCalled:
			assert.Equal(t, "", response, "Response should be empty string when cancelled")
		case <-time.After(1 * time.Second):
			t.Fatal("Callback should be called when cancelled")
		}

		// Cleanup
		term.Close()

		assert.True(t, widget.IsClosed(), "Widget should be closed after cancellation")
	})

	t.Run("navigation boundaries", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Options: []string{"Option 1", "Option 2", "Option 3"},
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Option 1", 1*time.Second))

		// Try to go up from first option (should stay at 0)
		term.SendKey("up")
		// Wait a bit for processing
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 0, widget.cursor)

		// Go to last option
		term.SendKey("down")
		term.SendKey("down")
		// Wait a bit for processing
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 2, widget.cursor)

		// Try to go down from last option (should stay at 2)
		term.SendKey("down")
		// Wait a bit for processing
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 2, widget.cursor)

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("navigation boundaries with custom response", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:               "Test Permission",
			Options:             []string{"Option 1", "Option 2"},
			AllowCustomResponse: "Custom",
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("Option 1", 1*time.Second))

		// Go to custom option (index 2)
		term.SendKey("down")
		term.SendKey("down")
		// Wait a bit for processing
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 2, widget.cursor)

		// Try to go down from custom option (should stay at 2)
		term.SendKey("down")
		// Wait a bit for processing
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 2, widget.cursor)

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("hidden widget does not respond to input", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Options: []string{"Option 1"},
		}

		callbackCalled := make(chan struct{})
		widget := NewPermissionQueryWidget(query, func(response string) {
			close(callbackCalled)
		})
		// Don't show the widget (keep it hidden)

		term := NewTerminalMock()
		term.Run(widget)

		// Try to interact with hidden widget
		term.SendKey("enter")

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Cleanup
		term.Close()

		select {
		case <-callbackCalled:
			t.Fatal("Hidden widget should not respond to input")
		case <-time.After(100 * time.Millisecond):
			// Callback was not called, which is expected
		}

		assert.False(t, widget.IsClosed(), "Hidden widget should not be closed by input")
	})

	t.Run("widget without title", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Details: "Details only",
			Options: []string{"Option 1"},
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for render (should show details and options even without title)
		assert.True(t, term.WaitForText("Details only", 1*time.Second))
		assert.True(t, term.WaitForText("Option 1", 1*time.Second))

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("widget without details", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Title only",
			Options: []string{"Option 1"},
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for render (should show title and options even without details)
		assert.True(t, term.WaitForText("Title only", 1*time.Second))
		assert.True(t, term.WaitForText("Option 1", 1*time.Second))

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("empty options list", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Options: []string{},
		}

		widget := NewPermissionQueryWidget(query, func(response string) {})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for render
		assert.True(t, term.WaitForText("Test Permission", 1*time.Second))

		// Should be able to dismiss empty widget
		term.SendKey("esc")

		// Wait a bit for processing
		time.Sleep(50 * time.Millisecond)

		// Cleanup
		term.Close()

		assert.True(t, widget.IsClosed(), "Empty widget should be dismissible")
	})

	t.Run("select first option", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Options: []string{"First", "Second", "Third"},
		}

		callbackCalled := make(chan string, 1)
		widget := NewPermissionQueryWidget(query, func(response string) {
			callbackCalled <- response
		})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("First", 1*time.Second))

		// Select first option (already at cursor 0)
		term.SendKey("enter")

		// Wait for callback to be called
		select {
		case response := <-callbackCalled:
			assert.Equal(t, "First", response, "Selected option should be First")
		case <-time.After(1 * time.Second):
			t.Fatal("Callback should be called when option is selected")
		}

		// Cleanup
		term.Close()

		assert.True(t, widget.IsClosed(), "Widget should be closed after selection")
	})

	t.Run("select last option", func(t *testing.T) {
		query := &ui.PermissionQueryUI{
			Title:   "Test Permission",
			Options: []string{"First", "Second", "Third"},
		}

		callbackCalled := make(chan string, 1)
		widget := NewPermissionQueryWidget(query, func(response string) {
			callbackCalled <- response
		})
		widget.Show()

		term := NewTerminalMock()
		term.Run(widget)

		// Wait for initial render
		assert.True(t, term.WaitForText("First", 1*time.Second))

		// Navigate to last option
		term.SendKey("down")
		term.SendKey("down")

		// Wait a bit for navigation to process
		time.Sleep(50 * time.Millisecond)

		// Select last option
		term.SendKey("enter")

		// Wait for callback to be called
		select {
		case response := <-callbackCalled:
			assert.Equal(t, "Third", response, "Selected option should be Third")
		case <-time.After(1 * time.Second):
			t.Fatal("Callback should be called when option is selected")
		}

		// Cleanup
		term.Close()

		assert.True(t, widget.IsClosed(), "Widget should be closed after selection")
	})
}
