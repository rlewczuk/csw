package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestMenuWidget(t *testing.T) {
	t.Run("initial state and visibility", func(t *testing.T) {
		items := MenuItems{
			{Label: "Option 1", Action: func() {}},
			{Label: "Option 2", Action: func() {}},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())

		assert.False(t, menu.IsVisible(), "Menu should not be visible initially")
		assert.False(t, menu.IsClosed(), "Menu should not be closed initially")
		assert.Equal(t, "Test Menu", menu.title)
		assert.Equal(t, 2, len(menu.items))
		assert.Equal(t, 0, menu.cursor)
	})

	t.Run("show and hide functionality", func(t *testing.T) {
		items := MenuItems{
			{Label: "Option 1", Action: func() {}},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())

		menu.Show()
		assert.True(t, menu.IsVisible(), "Menu should be visible after Show()")

		menu.Hide()
		assert.False(t, menu.IsVisible(), "Menu should not be visible after Hide()")
	})

	t.Run("menu renders with title and items", func(t *testing.T) {
		items := MenuItems{
			{Label: "Item 1", Action: func() {}},
			{Label: "Item 2", Action: func() {}},
			{Label: "Item 3", Action: func() {}},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for menu to render
		assert.True(t, term.WaitForText("Test Menu", 1*time.Second), "Menu should display title")
		assert.True(t, term.WaitForText("Item 1", 1*time.Second), "Menu should display first item")
		assert.True(t, term.WaitForText("Item 2", 1*time.Second), "Menu should display second item")
		assert.True(t, term.WaitForText("Item 3", 1*time.Second), "Menu should display third item")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("arrow key navigation", func(t *testing.T) {
		actionCalled := make(chan struct{})
		items := MenuItems{
			{Label: "Item 1", Action: func() {}},
			{Label: "Item 2", Action: func() { close(actionCalled) }},
			{Label: "Item 3", Action: func() {}},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for initial render
		assert.True(t, term.WaitForText("Item 1", 1*time.Second))

		// Navigate down
		term.SendKey("down")
		term.SendKey("down")
		term.SendKey("up")
		term.SendKey("esc")
		term.Close()

		select {
		case <-actionCalled:
			t.Fatal("Action should not be called during navigation")
		case <-time.After(100 * time.Millisecond):
			// Action was not called, which is expected
		}
	})

	t.Run("enter key selects item", func(t *testing.T) {
		actionCalled := make(chan struct{})
		items := MenuItems{
			{Label: "Item 1", Action: func() {}},
			{Label: "Item 2", Action: func() { close(actionCalled) }},
			{Label: "Item 3", Action: func() {}},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for initial render
		assert.True(t, term.WaitForText("Item 1", 1*time.Second))

		// Navigate to second item
		term.SendKey("down")

		// Select item
		term.SendKey("enter")

		// Wait for action to be called
		select {
		case <-actionCalled:
			// Action was called
		case <-time.After(1 * time.Second):
			t.Fatal("Action should be called when item is selected")
		}

		// Cleanup
		term.Close()

		assert.True(t, menu.IsClosed(), "Menu should be closed after selection")
		assert.False(t, menu.IsVisible(), "Menu should not be visible after selection")
	})

	t.Run("esc key dismisses menu", func(t *testing.T) {
		actionCalled := make(chan struct{})
		items := MenuItems{
			{Label: "Item 1", Action: func() { close(actionCalled) }},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for initial render
		assert.True(t, term.WaitForText("Test Menu", 1*time.Second))

		// Dismiss menu
		term.SendKey("esc")

		// Cleanup
		term.Close()

		select {
		case <-actionCalled:
			t.Fatal("Action should not be called when menu is dismissed")
		case <-time.After(100 * time.Millisecond):
			// Action was not called, which is expected
		}

		assert.True(t, menu.IsClosed(), "Menu should be closed after esc")
		assert.False(t, menu.IsVisible(), "Menu should not be visible after esc")
	})

	t.Run("ctrl+c dismisses menu", func(t *testing.T) {
		actionCalled := make(chan struct{})
		items := MenuItems{
			{Label: "Item 1", Action: func() { close(actionCalled) }},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for initial render
		assert.True(t, term.WaitForText("Test Menu", 1*time.Second))

		// Dismiss menu with Ctrl+C
		term.SendKey("ctrl+c")

		// Cleanup
		term.Close()

		select {
		case <-actionCalled:
			t.Fatal("Action should not be called when menu is dismissed")
		case <-time.After(100 * time.Millisecond):
			// Action was not called, which is expected
		}

		assert.True(t, menu.IsClosed(), "Menu should be closed after ctrl+c")
		assert.False(t, menu.IsVisible(), "Menu should not be visible after ctrl+c")
	})

	t.Run("right key selects item", func(t *testing.T) {
		actionCalled := make(chan struct{})
		items := MenuItems{
			{Label: "Item 1", Action: func() {}},
			{Label: "Item 2", Action: func() { close(actionCalled) }},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for initial render
		assert.True(t, term.WaitForText("Item 1", 1*time.Second))

		// Navigate to second item
		term.SendKey("down")

		// Select item with right arrow
		term.SendKey("right")

		// Wait for action to be called
		select {
		case <-actionCalled:
			// Action was called
		case <-time.After(1 * time.Second):
			t.Fatal("Action should be called when item is selected with right arrow")
		}

		// Cleanup
		term.Close()
	})

	t.Run("custom colors", func(t *testing.T) {
		items := MenuItems{
			{Label: "Item 1", Action: func() {}},
		}

		customColors := MenuColors{
			Background:     lipgloss.Color("240"),
			Border:         lipgloss.Color("100"),
			Title:          lipgloss.Color("200"),
			SelectedItem:   lipgloss.Color("250"),
			UnselectedItem: lipgloss.Color("245"),
		}

		menu := NewMenuWidget("Test Menu", items, customColors)
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for render
		assert.True(t, term.WaitForText("Test Menu", 1*time.Second))

		// Cleanup
		term.SendKey("esc")
		term.Close()

		assert.Equal(t, customColors.Background, menu.colors.Background)
		assert.Equal(t, customColors.Border, menu.colors.Border)
	})

	t.Run("empty menu items", func(t *testing.T) {
		items := MenuItems{}

		menu := NewMenuWidget("Empty Menu", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for render
		assert.True(t, term.WaitForText("Empty Menu", 1*time.Second))

		// Should be able to dismiss empty menu
		term.SendKey("esc")

		// Wait a bit for processing
		for i := 0; i < 10; i++ {
			if menu.IsClosed() {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}

		// Cleanup
		term.Close()

		assert.True(t, menu.IsClosed(), "Empty menu should be dismissible")
	})

	t.Run("menu with no title", func(t *testing.T) {
		items := MenuItems{
			{Label: "Item 1", Action: func() {}},
		}

		menu := NewMenuWidget("", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for render (should show items even without title)
		assert.True(t, term.WaitForText("Item 1", 1*time.Second))

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("update menu properties", func(t *testing.T) {
		items := MenuItems{
			{Label: "Item 1", Action: func() {}},
		}

		menu := NewMenuWidget("Original Title", items, DefaultMenuColors())

		// Update title
		menu.SetTitle("New Title")
		assert.Equal(t, "New Title", menu.title)

		// Update items
		newItems := MenuItems{
			{Label: "New Item 1", Action: func() {}},
			{Label: "New Item 2", Action: func() {}},
		}
		menu.SetItems(newItems)
		assert.Equal(t, 2, len(menu.items))
		assert.Equal(t, 0, menu.cursor) // Cursor should reset

		// Update colors
		newColors := MenuColors{
			Background: lipgloss.Color("240"),
			Border:     lipgloss.Color("100"),
		}
		menu.SetColors(newColors)
		assert.Equal(t, newColors.Background, menu.colors.Background)
		assert.Equal(t, newColors.Border, menu.colors.Border)
	})

	t.Run("navigation boundaries", func(t *testing.T) {
		items := MenuItems{
			{Label: "Item 1", Action: func() {}},
			{Label: "Item 2", Action: func() {}},
			{Label: "Item 3", Action: func() {}},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())
		menu.Show()

		term := NewTerminalMock()
		term.Run(menu)

		// Wait for initial render
		assert.True(t, term.WaitForText("Item 1", 1*time.Second))

		// Try to go up from first item (should stay at 0)
		term.SendKey("up")
		// Wait a bit for processing
		for i := 0; i < 10; i++ {
			if menu.cursor == 0 {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}
		assert.Equal(t, 0, menu.cursor)

		// Go to last item
		term.SendKey("down")
		term.SendKey("down")
		// Wait a bit for processing
		for i := 0; i < 10; i++ {
			if menu.cursor == 2 {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}
		assert.Equal(t, 2, menu.cursor)

		// Try to go down from last item (should stay at 2)
		term.SendKey("down")
		// Wait a bit for processing
		for i := 0; i < 10; i++ {
			if menu.cursor == 2 {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}
		assert.Equal(t, 2, menu.cursor)

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("hidden menu does not respond to input", func(t *testing.T) {
		actionCalled := make(chan struct{})
		items := MenuItems{
			{Label: "Item 1", Action: func() { close(actionCalled) }},
		}

		menu := NewMenuWidget("Test Menu", items, DefaultMenuColors())
		// Don't show the menu (keep it hidden)

		term := NewTerminalMock()
		term.Run(menu)

		// Try to interact with hidden menu
		term.SendKey("enter")

		// Cleanup
		term.Close()

		select {
		case <-actionCalled:
			t.Fatal("Hidden menu should not respond to input")
		case <-time.After(100 * time.Millisecond):
			// Action was not called, which is expected
		}

		assert.False(t, menu.IsClosed(), "Hidden menu should not be closed by input")
	})
}
