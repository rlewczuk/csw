package tui_test

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"
	"github.com/rlewczuk/csw/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMenuWidget_BasicCreation tests basic menu widget creation and initialization.
func TestMenuWidget_BasicCreation(t *testing.T) {
	// Create a menu widget
	menu := tui.NewMenuWidget(nil)
	require.NotNil(t, menu)

	// Verify initial state
	assert.Equal(t, 0, len(menu.GetItems()))
	assert.Equal(t, 0, menu.GetSelectedIndex())
	assert.False(t, menu.IsCustomInputEnabled())
	assert.Equal(t, "Menu", menu.GetTitle())
}

// TestMenuWidget_AddItems tests adding items to the menu.
func TestMenuWidget_AddItems(t *testing.T) {
	menu := tui.NewMenuWidget(nil)

	// Add some items
	handler1Called := false
	handler1 := func(text string) {
		handler1Called = true
	}
	idx1 := menu.AddItem("Item 1", handler1)
	assert.Equal(t, 0, idx1)

	handler2Called := false
	handler2 := func(text string) {
		handler2Called = true
	}
	idx2 := menu.AddItem("Item 2", handler2)
	assert.Equal(t, 1, idx2)

	// Verify items
	items := menu.GetItems()
	assert.Equal(t, 2, len(items))
	assert.Equal(t, "Item 1", items[0].Label)
	assert.Equal(t, "Item 2", items[1].Label)

	// Verify handlers are not called yet
	assert.False(t, handler1Called)
	assert.False(t, handler2Called)
}

// TestMenuWidget_Navigation tests arrow key navigation.
func TestMenuWidget_Navigation(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// Add items
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)
	menu.AddItem("Item 3", nil)

	// Focus the menu
	menu.Focus()

	// Initially selected index is 0
	assert.Equal(t, 0, menu.GetSelectedIndex())

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Navigate down
	mockInput.TypeKeysByName("Down")
	assert.Equal(t, 1, menu.GetSelectedIndex())

	// Navigate down again
	mockInput.TypeKeysByName("Down")
	assert.Equal(t, 2, menu.GetSelectedIndex())

	// Navigate down again (should wrap to 0)
	mockInput.TypeKeysByName("Down")
	assert.Equal(t, 0, menu.GetSelectedIndex())

	// Navigate up (should wrap to 2)
	mockInput.TypeKeysByName("Up")
	assert.Equal(t, 2, menu.GetSelectedIndex())

	// Navigate up again
	mockInput.TypeKeysByName("Up")
	assert.Equal(t, 1, menu.GetSelectedIndex())
}

// TestMenuWidget_Selection tests item selection with Enter key.
func TestMenuWidget_Selection(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// Add items with handlers
	item1Selected := false
	menu.AddItem("Item 1", func(text string) {
		item1Selected = true
		assert.Equal(t, "", text) // Regular items have empty text
	})

	item2Selected := false
	menu.AddItem("Item 2", func(text string) {
		item2Selected = true
		assert.Equal(t, "", text)
	})

	// Focus the menu
	menu.Focus()

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Select first item
	mockInput.TypeKeysByName("Enter")
	assert.True(t, item1Selected)
	assert.False(t, item2Selected)

	// Navigate to second item and select
	item1Selected = false
	mockInput.TypeKeysByName("Down")
	mockInput.TypeKeysByName("Enter")
	assert.False(t, item1Selected)
	assert.True(t, item2Selected)
}

// TestMenuWidget_CancelHandler tests the cancel handler with Esc key.
func TestMenuWidget_CancelHandler(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// Add items
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)

	// Set cancel handler
	cancelCalled := false
	menu.SetOnCancel(func() {
		cancelCalled = true
	})

	// Focus the menu
	menu.Focus()

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Press Esc
	mockInput.TypeKeysByName("Esc")
	assert.True(t, cancelCalled)
}

// TestMenuWidget_MouseClick tests mouse click selection.
func TestMenuWidget_MouseClick(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)
	menu := tui.NewMenuWidget(layout, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Add items (handlers not used in this test, just testing selection)
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)
	menu.AddItem("Item 3", nil)

	// Focus the menu
	menu.Focus()

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Initially at index 0
	assert.Equal(t, 0, menu.GetSelectedIndex())

	// Click on second item (Y=7)
	// Menu is at (10, 5) with size (30, 10)
	// Frame border is 1 cell, so inner area starts at (11, 6)
	// First item at Y=6, second item at Y=7
	mockInput.MouseClick(15, 7, 0)
	assert.Equal(t, 1, menu.GetSelectedIndex(), "Second item should be selected after click")

	// Click on third item (Y=8)
	mockInput.MouseClick(15, 8, 0)
	assert.Equal(t, 2, menu.GetSelectedIndex(), "Third item should be selected after click")

	// Click on first item (Y=6)
	mockInput.MouseClick(15, 6, 0)
	assert.Equal(t, 0, menu.GetSelectedIndex(), "First item should be selected after click")
}

// TestMenuWidget_CustomInput tests custom text input functionality.
func TestMenuWidget_CustomInput(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// Add regular items
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)

	// Enable custom input
	menu.EnableCustomInput(true, "Custom:")

	// Set custom input handler
	customInputText := ""
	menu.SetOnCustomInput(func(text string) {
		customInputText = text
	})

	// Focus the menu
	menu.Focus()

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Navigate to custom input item (should be at index 2)
	mockInput.TypeKeysByName("Down", "Down")
	assert.Equal(t, 2, menu.GetSelectedIndex())

	// Select custom input item (press Enter)
	mockInput.TypeKeysByName("Enter")

	// Now we should be in input mode
	// GetSelectedIndex should return -1 when in input mode
	assert.Equal(t, -1, menu.GetSelectedIndex())

	// Type custom text
	mockInput.TypeKeys("My Custom Text")

	// Confirm input with Enter
	mockInput.TypeKeysByName("Enter")

	// Verify custom input handler was called with the text
	assert.Equal(t, "My Custom Text", customInputText)

	// Should no longer be in input mode
	assert.NotEqual(t, -1, menu.GetSelectedIndex())
}

// TestMenuWidget_CustomInput_CancelWithEsc tests canceling custom input with Esc.
func TestMenuWidget_CustomInput_CancelWithEsc(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// Add regular items
	menu.AddItem("Item 1", nil)

	// Enable custom input
	menu.EnableCustomInput(true, "Custom:")

	// Set custom input handler
	customInputCalled := false
	menu.SetOnCustomInput(func(text string) {
		customInputCalled = true
	})

	// Focus the menu
	menu.Focus()

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Navigate to custom input item
	mockInput.TypeKeysByName("Down")

	// Select custom input item
	mockInput.TypeKeysByName("Enter")

	// Type some text
	mockInput.TypeKeys("Test")

	// Press Esc to cancel
	mockInput.TypeKeysByName("Esc")

	// Custom input handler should NOT be called
	assert.False(t, customInputCalled)

	// Should have navigated to previous item (item 0)
	assert.Equal(t, 0, menu.GetSelectedIndex())
}

// TestMenuWidget_CustomInput_NavigateWithArrows tests navigating away from input mode.
func TestMenuWidget_CustomInput_NavigateWithArrows(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// Add regular items
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)

	// Enable custom input
	menu.EnableCustomInput(true, "Custom:")

	// Focus the menu
	menu.Focus()

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Navigate to custom input item (index 2)
	mockInput.TypeKeysByName("Down", "Down")
	assert.Equal(t, 2, menu.GetSelectedIndex())

	// Enter input mode
	mockInput.TypeKeysByName("Enter")
	assert.Equal(t, -1, menu.GetSelectedIndex())

	// Press Up arrow to exit input mode
	mockInput.TypeKeysByName("Up")

	// Should have navigated to previous item (item 1)
	assert.Equal(t, 1, menu.GetSelectedIndex())
}

// TestMenuWidget_Rendering tests that menu renders correctly.
func TestMenuWidget_Rendering(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// Add items
	menu.AddItem("First Item", nil)
	menu.AddItem("Second Item", nil)
	menu.AddItem("Third Item", nil)

	// Set title
	menu.SetTitle("Test Menu")

	// Focus the menu
	menu.Focus()

	// Draw the menu (draw directly to screen, not themedScreen)
	menu.Draw(screen)

	// Verify screen content
	width, height, content := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Verify something was drawn (frame border and items)
	// Count non-space characters
	nonSpaceCount := 0
	for _, cell := range content {
		if cell.Rune != 0 && cell.Rune != ' ' {
			nonSpaceCount++
		}
	}
	assert.Greater(t, nonSpaceCount, 50, "Menu should render frame and items (border + text)")
}

// TestMenuWidget_SetItems tests replacing all items.
func TestMenuWidget_SetItems(t *testing.T) {
	menu := tui.NewMenuWidget(nil)

	// Add initial items
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)
	assert.Equal(t, 2, len(menu.GetItems()))

	// Replace with new items
	newItems := []tui.MenuItem{
		{Label: "New Item 1", Handler: nil},
		{Label: "New Item 2", Handler: nil},
		{Label: "New Item 3", Handler: nil},
	}
	menu.SetItems(newItems)

	// Verify new items
	items := menu.GetItems()
	assert.Equal(t, 3, len(items))
	assert.Equal(t, "New Item 1", items[0].Label)
	assert.Equal(t, "New Item 2", items[1].Label)
	assert.Equal(t, "New Item 3", items[2].Label)
}

// TestMenuWidget_SetSelectedIndex tests manually setting selected index.
func TestMenuWidget_SetSelectedIndex(t *testing.T) {
	menu := tui.NewMenuWidget(nil)

	// Add items
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)
	menu.AddItem("Item 3", nil)

	// Initially at index 0
	assert.Equal(t, 0, menu.GetSelectedIndex())

	// Set to index 2
	menu.SetSelectedIndex(2)
	assert.Equal(t, 2, menu.GetSelectedIndex())

	// Set to index 1
	menu.SetSelectedIndex(1)
	assert.Equal(t, 1, menu.GetSelectedIndex())

	// Set to invalid index (should clamp)
	menu.SetSelectedIndex(10)
	assert.Equal(t, 2, menu.GetSelectedIndex()) // Should be clamped to last item

	// Set to negative index (should clamp to 0)
	menu.SetSelectedIndex(-5)
	assert.Equal(t, 0, menu.GetSelectedIndex())
}

// TestMenuWidget_EmptyMenu tests behavior with no items.
func TestMenuWidget_EmptyMenu(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// No items added
	assert.Equal(t, 0, len(menu.GetItems()))

	// Focus the menu
	menu.Focus()

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Try to navigate (should not crash)
	mockInput.TypeKeysByName("Down")
	mockInput.TypeKeysByName("Up")

	// Try to select (should not crash)
	mockInput.TypeKeysByName("Enter")

	// Draw should not crash
	themedScreen := app.GetScreen()
	menu.Draw(themedScreen)
}

// TestMenuWidget_RoundRobinNavigation tests round-robin navigation at boundaries.
func TestMenuWidget_RoundRobinNavigation(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	menu := tui.NewMenuWidget(nil, tui.WithRectangle(10, 5, 30, 10))
	app := tui.NewApplication(menu, screen)
	require.NotNil(t, app)

	// Add 3 items
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)
	menu.AddItem("Item 3", nil)

	// Focus the menu
	menu.Focus()

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Start at index 0
	assert.Equal(t, 0, menu.GetSelectedIndex())

	// Press Up (should wrap to last item, index 2)
	mockInput.TypeKeysByName("Up")
	assert.Equal(t, 2, menu.GetSelectedIndex())

	// Press Down (should wrap to first item, index 0)
	mockInput.TypeKeysByName("Down")
	assert.Equal(t, 0, menu.GetSelectedIndex())

	// Navigate to last item
	menu.SetSelectedIndex(2)
	assert.Equal(t, 2, menu.GetSelectedIndex())

	// Press Down (should wrap to first item, index 0)
	mockInput.TypeKeysByName("Down")
	assert.Equal(t, 0, menu.GetSelectedIndex())
}

// TestMenuWidget_CustomInputDisabled tests that custom input can be disabled.
func TestMenuWidget_CustomInputDisabled(t *testing.T) {
	menu := tui.NewMenuWidget(nil)

	// Add items
	menu.AddItem("Item 1", nil)
	menu.AddItem("Item 2", nil)

	// Initially disabled
	assert.False(t, menu.IsCustomInputEnabled())

	// Enable custom input
	menu.EnableCustomInput(true, "Custom:")
	assert.True(t, menu.IsCustomInputEnabled())

	// Disable custom input
	menu.EnableCustomInput(false, "")
	assert.False(t, menu.IsCustomInputEnabled())
}
