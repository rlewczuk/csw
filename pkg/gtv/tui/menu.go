package tui

import (
	"github.com/codesnort/codesnort-swe/pkg/gtv"
)

// MenuItemHandler is a callback function that is called when a menu item is selected.
// For regular items, text is empty.
// For custom input items, text contains the user-entered text.
type MenuItemHandler func(text string)

// MenuItem represents a single item in the menu.
type MenuItem struct {
	// Label is the text displayed for this menu item
	Label string
	// Handler is called when the item is selected (Enter key or mouse click)
	Handler MenuItemHandler
}

// IMenuWidget is an interface for menu widgets that allow selecting from a list.
// It extends IFocusable with menu-specific methods.
type IMenuWidget interface {
	IFocusable

	// AddItem adds a menu item with the specified label and handler.
	// Returns the index of the added item.
	AddItem(label string, handler MenuItemHandler) int

	// SetItems replaces all menu items with the provided items.
	SetItems(items []MenuItem)

	// GetItems returns all menu items.
	GetItems() []MenuItem

	// EnableCustomInput enables or disables the custom input option.
	// When enabled, an additional item appears at the end of the list
	// that allows the user to enter custom text.
	EnableCustomInput(enabled bool, prompt string)

	// IsCustomInputEnabled returns true if custom input is enabled.
	IsCustomInputEnabled() bool

	// SetOnCancel sets the handler for cancel action (Esc key).
	SetOnCancel(handler func())

	// GetSelectedIndex returns the currently selected item index.
	// Returns -1 if custom input is active or no item is selected.
	GetSelectedIndex() int

	// SetSelectedIndex sets the currently selected item index.
	SetSelectedIndex(index int)

	// GetTitle returns the menu title.
	GetTitle() string

	// SetTitle sets the menu title.
	SetTitle(title string)
}

// TMenuWidget is a widget that displays a menu with selectable items.
// It extends TFocusable and implements IMenuWidget interface.
//
// Features:
// - Display a list of items with highlighted selection
// - Navigate using arrow keys (round-robin: wraps at edges)
// - Select items using Enter key or mouse click
// - Optional custom text input (appears as last item)
// - Cancel with Esc key (calls cancel handler if defined)
// - Border with title using TFrame
// - Theme support for normal, selected, and input states
type TMenuWidget struct {
	TFocusable

	// Menu items
	items []MenuItem

	// Currently selected item index (0-based)
	selectedIndex int

	// Custom input settings
	customInputEnabled bool
	customInputPrompt  string

	// Input box for custom text (created when custom input item is selected)
	inputBox *TInputBox
	// True when user is entering custom text
	isInputActive bool

	// Frame for border and title
	frame *TFrame

	// Cancel handler (called when Esc is pressed)
	onCancel func()

	// Custom input handler (called when custom text is confirmed)
	onCustomInput MenuItemHandler

	// Theme tags for different states
	itemAttrs         gtv.CellAttributes // Normal item
	selectedItemAttrs gtv.CellAttributes // Selected item
	inputPromptAttrs  gtv.CellAttributes // Custom input prompt
}

// NewMenuWidget creates a new menu widget with the specified parent and options.
// The parent parameter is optional (can be nil for root widgets).
// Options can be used to configure position, attributes, and other properties.
//
// Default values:
// - Items: empty
// - SelectedIndex: 0
// - CustomInputEnabled: false
// - Position: gtv.TRect{X: 0, Y: 0, W: 20, H: 10}
// - ItemAttrs: gtv.CellTag("menu-item")
// - SelectedItemAttrs: gtv.CellTag("menu-item-selected")
// - InputPromptAttrs: gtv.CellTag("menu-input-prompt")
//
// Available options:
// - WithRectangle(X, Y, W, H) - sets position and size
// - WithPosition(X, Y) - sets position only
func NewMenuWidget(parent IWidget, opts ...gtv.Option) *TMenuWidget {
	menu := &TMenuWidget{
		TFocusable:         *newFocusableBase(parent, opts...),
		items:              make([]MenuItem, 0),
		selectedIndex:      0,
		customInputEnabled: false,
		customInputPrompt:  "Enter custom text:",
		inputBox:           nil,
		isInputActive:      false,
		onCancel:           nil,
		itemAttrs:          gtv.CellTag("menu-item"),
		selectedItemAttrs:  gtv.CellTag("menu-item-selected"),
		inputPromptAttrs:   gtv.CellTag("menu-input-prompt"),
	}

	// Apply options
	for _, opt := range opts {
		opt(menu)
	}

	// Set default size if not specified
	if menu.Position.W == 0 {
		menu.Position.W = 20
	}
	if menu.Position.H == 0 {
		menu.Position.H = 10
	}

	// Create frame for border and title
	// Frame position is relative to menu (fills entire menu area)
	frameRect := gtv.TRect{X: 0, Y: 0, W: menu.Position.W, H: menu.Position.H}
	menu.frame = NewFrame(
		nil, // We'll set parent in Draw
		frameRect,
		BorderStyleSingle,
		gtv.CellTag("menu-frame"),
		gtv.CellTag("menu-frame-focused"),
	)
	menu.frame.SetTitle("Menu")

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(menu)
	}

	return menu
}

// AddItem adds a menu item with the specified label and handler.
// Returns the index of the added item.
func (m *TMenuWidget) AddItem(label string, handler MenuItemHandler) int {
	m.items = append(m.items, MenuItem{
		Label:   label,
		Handler: handler,
	})
	return len(m.items) - 1
}

// SetItems replaces all menu items with the provided items.
func (m *TMenuWidget) SetItems(items []MenuItem) {
	m.items = items
	if m.selectedIndex >= len(m.items) {
		m.selectedIndex = 0
	}
}

// GetItems returns all menu items.
func (m *TMenuWidget) GetItems() []MenuItem {
	return m.items
}

// EnableCustomInput enables or disables the custom input option.
func (m *TMenuWidget) EnableCustomInput(enabled bool, prompt string) {
	m.customInputEnabled = enabled
	m.customInputPrompt = prompt
	if !enabled {
		m.isInputActive = false
		m.inputBox = nil
	}
}

// SetOnCustomInput sets the handler for custom text input.
// This handler is called when the user confirms custom text entry.
func (m *TMenuWidget) SetOnCustomInput(handler MenuItemHandler) {
	m.onCustomInput = handler
}

// IsCustomInputEnabled returns true if custom input is enabled.
func (m *TMenuWidget) IsCustomInputEnabled() bool {
	return m.customInputEnabled
}

// SetOnCancel sets the handler for cancel action (Esc key).
func (m *TMenuWidget) SetOnCancel(handler func()) {
	m.onCancel = handler
}

// GetSelectedIndex returns the currently selected item index.
// Returns -1 if custom input is active or no item is selected.
func (m *TMenuWidget) GetSelectedIndex() int {
	if m.isInputActive {
		return -1
	}
	return m.selectedIndex
}

// SetSelectedIndex sets the currently selected item index.
func (m *TMenuWidget) SetSelectedIndex(index int) {
	if index < 0 {
		index = 0
	}
	totalItems := len(m.items)
	if m.customInputEnabled {
		totalItems++
	}
	if index >= totalItems {
		index = totalItems - 1
	}
	m.selectedIndex = index
	m.isInputActive = false
}

// GetTitle returns the menu title.
func (m *TMenuWidget) GetTitle() string {
	return m.frame.GetTitle()
}

// SetTitle sets the menu title.
func (m *TMenuWidget) SetTitle(title string) {
	m.frame.SetTitle(title)
}

// getTotalItems returns the total number of visible items including custom input option
func (m *TMenuWidget) getTotalItems() int {
	total := len(m.items)
	if m.customInputEnabled {
		total++
	}
	return total
}

// isCustomInputItem returns true if the given index is the custom input item
func (m *TMenuWidget) isCustomInputItem(index int) bool {
	return m.customInputEnabled && index == len(m.items)
}

// Draw draws the menu widget on the screen.
func (m *TMenuWidget) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if m.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Get absolute position
	absPos := m.GetAbsolutePos()

	// Update frame position to match menu's absolute position
	// The frame has no parent, so its Position is absolute
	m.frame.Position = absPos
	m.frame.Parent = nil

	// Update focus state
	if m.IsFocused() && !m.isInputActive {
		m.frame.focused = true
	} else {
		m.frame.focused = false
	}

	// Draw frame (border and title)
	m.frame.Draw(screen)

	// Calculate inner area (inside frame border)
	innerRect := gtv.TRect{
		X: absPos.X + 1,
		Y: absPos.Y + 1,
		W: absPos.W - 2,
		H: absPos.H - 2,
	}

	// Draw menu items
	if m.isInputActive && m.inputBox != nil {
		// Draw input box for custom text entry
		m.drawInputBox(screen, innerRect)
	} else {
		// Draw regular menu items
		m.drawMenuItems(screen, innerRect)
	}

	// Draw children (if any)
	m.TFocusable.Draw(screen)
}

// drawMenuItems draws the menu items in the inner area
func (m *TMenuWidget) drawMenuItems(screen gtv.IScreenOutput, innerRect gtv.TRect) {
	totalItems := m.getTotalItems()
	visibleHeight := int(innerRect.H)

	// Calculate scroll offset to keep selected item visible
	scrollOffset := 0
	if m.selectedIndex >= visibleHeight {
		scrollOffset = m.selectedIndex - visibleHeight + 1
	}

	// Draw visible items
	for i := 0; i < visibleHeight && (scrollOffset+i) < totalItems; i++ {
		itemIndex := scrollOffset + i
		y := innerRect.Y + uint16(i)

		// Determine item label and attributes
		var label string
		var attrs gtv.CellAttributes

		if m.isCustomInputItem(itemIndex) {
			// Custom input prompt item
			label = m.customInputPrompt
			attrs = m.inputPromptAttrs
		} else {
			// Regular menu item
			label = m.items[itemIndex].Label
			attrs = m.itemAttrs
		}

		// Highlight selected item
		if itemIndex == m.selectedIndex && m.IsFocused() {
			attrs = m.selectedItemAttrs
		}

		// Truncate or pad label to fit width
		labelRunes := []rune(label)
		width := int(innerRect.W)

		cells := make([]gtv.Cell, width)
		for j := 0; j < width; j++ {
			if j < len(labelRunes) {
				cells[j] = gtv.Cell{Rune: labelRunes[j], Attrs: attrs}
			} else {
				cells[j] = gtv.Cell{Rune: ' ', Attrs: attrs}
			}
		}

		// Draw the item
		screen.PutContent(gtv.TRect{X: innerRect.X, Y: y, W: innerRect.W, H: 1}, cells)
	}
}

// drawInputBox draws the input box for custom text entry
func (m *TMenuWidget) drawInputBox(screen gtv.IScreenOutput, innerRect gtv.TRect) {
	if m.inputBox == nil {
		return
	}

	// Draw prompt label
	promptLabel := m.customInputPrompt
	promptRunes := []rune(promptLabel)
	promptCells := make([]gtv.Cell, len(promptRunes))
	for i, r := range promptRunes {
		promptCells[i] = gtv.Cell{Rune: r, Attrs: m.inputPromptAttrs}
	}
	screen.PutContent(gtv.TRect{X: innerRect.X, Y: innerRect.Y, W: uint16(len(promptRunes)), H: 1}, promptCells)

	// Position input box below prompt
	inputBoxY := innerRect.Y + 2
	if inputBoxY < innerRect.Y+innerRect.H {
		// Update input box position
		m.inputBox.Position = gtv.TRect{
			X: 0, // Relative to parent (the menu widget at innerRect)
			Y: 0,
			W: innerRect.W,
			H: 1,
		}

		// Create event to update absolute position
		inputAbsPos := gtv.TRect{
			X: innerRect.X,
			Y: inputBoxY,
			W: innerRect.W,
			H: 1,
		}
		m.inputBox.HandleEvent(&TEvent{
			Type: TEventTypeResize,
			Rect: inputAbsPos,
		})

		// Draw input box
		m.inputBox.Draw(screen)
	}
}

// HandleEvent handles events for the menu widget.
func (m *TMenuWidget) HandleEvent(event *TEvent) {
	// Handle input events only if focused
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// If input box is active, forward events to it
		if m.isInputActive && m.inputBox != nil {
			// Handle Esc to exit input mode
			if inputEvent.Type == gtv.InputEventKey && inputEvent.Key == 0x1B {
				m.exitInputMode(false)
				return
			}

			// Handle Enter to confirm input
			if inputEvent.Type == gtv.InputEventKey && inputEvent.Key == '\r' {
				m.confirmInput()
				return
			}

			// Handle arrow keys to exit input mode and navigate
			if inputEvent.Type == gtv.InputEventKey && inputEvent.Modifiers&gtv.ModFn != 0 {
				switch inputEvent.Key {
				case 'A': // Up arrow
					m.isInputActive = false
					if m.inputBox != nil {
						m.inputBox.Blur()
					}
					m.navigateUp()
					return
				case 'B': // Down arrow
					m.isInputActive = false
					if m.inputBox != nil {
						m.inputBox.Blur()
					}
					m.navigateDown()
					return
				}
			}

			// Forward other events to input box
			m.inputBox.HandleEvent(event)
			return
		}

		// Handle keyboard events when focused
		if m.IsFocused() && inputEvent.Type == gtv.InputEventKey {
			m.handleKeyEvent(inputEvent)
			return
		}

		// Handle mouse events
		if inputEvent.Type == gtv.InputEventMouse {
			m.handleMouseEvent(inputEvent)
			return
		}
	}

	// Handle resize events
	if event.Type == TEventTypeResize {
		m.Position = event.Rect
		m.frame.Position = event.Rect
		return
	}

	// Delegate other events to base widget
	m.TFocusable.HandleEvent(event)
}

// handleKeyEvent handles keyboard events for menu navigation
func (m *TMenuWidget) handleKeyEvent(event *gtv.InputEvent) {
	// Handle Esc key
	if event.Key == 0x1B {
		if m.onCancel != nil {
			m.onCancel()
		}
		return
	}

	// Handle arrow keys
	if event.Modifiers&gtv.ModFn != 0 {
		switch event.Key {
		case 'A': // Up arrow
			m.navigateUp()
			return
		case 'B': // Down arrow
			m.navigateDown()
			return
		}
	}

	// Handle Enter key
	if event.Key == '\r' {
		m.selectCurrentItem()
		return
	}
}

// navigateUp moves selection up (with round-robin wrapping)
func (m *TMenuWidget) navigateUp() {
	totalItems := m.getTotalItems()
	if totalItems == 0 {
		return
	}

	m.selectedIndex--
	if m.selectedIndex < 0 {
		m.selectedIndex = totalItems - 1
	}
	m.isInputActive = false
}

// navigateDown moves selection down (with round-robin wrapping)
func (m *TMenuWidget) navigateDown() {
	totalItems := m.getTotalItems()
	if totalItems == 0 {
		return
	}

	m.selectedIndex++
	if m.selectedIndex >= totalItems {
		m.selectedIndex = 0
	}
	m.isInputActive = false
}

// selectCurrentItem activates the currently selected item
func (m *TMenuWidget) selectCurrentItem() {
	// Check if custom input item is selected
	if m.isCustomInputItem(m.selectedIndex) {
		m.enterInputMode()
		return
	}

	// Call handler for regular item
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
		if m.items[m.selectedIndex].Handler != nil {
			m.items[m.selectedIndex].Handler("")
		}
	}
}

// enterInputMode enters custom text input mode
func (m *TMenuWidget) enterInputMode() {
	m.isInputActive = true

	// Create input box if it doesn't exist
	if m.inputBox == nil {
		m.inputBox = NewInputBox(
			nil, // We'll manage positioning manually
			WithText(""),
			WithRectangle(0, 0, int(m.Position.W)-2, 1),
		)
	} else {
		// Clear previous text
		m.inputBox.SetText("")
	}

	// Focus the input box
	m.inputBox.Focus()
}

// exitInputMode exits custom text input mode
func (m *TMenuWidget) exitInputMode(keepSelection bool) {
	m.isInputActive = false
	if m.inputBox != nil {
		m.inputBox.Blur()
	}
	if !keepSelection {
		// Move selection to previous item
		m.navigateUp()
	}
}

// confirmInput confirms the custom text input
func (m *TMenuWidget) confirmInput() {
	if m.inputBox == nil {
		return
	}

	text := m.inputBox.GetText()
	m.exitInputMode(true)

	// Call custom input handler if defined
	if m.onCustomInput != nil {
		m.onCustomInput(text)
	}
}

// handleMouseEvent handles mouse events for item selection
func (m *TMenuWidget) handleMouseEvent(event *gtv.InputEvent) {
	absPos := m.GetAbsolutePos()

	// Check if click is within menu area (inside frame)
	innerRect := gtv.TRect{
		X: absPos.X + 1,
		Y: absPos.Y + 1,
		W: absPos.W - 2,
		H: absPos.H - 2,
	}

	if !innerRect.Contains(event.X, event.Y) {
		return
	}

	// Calculate which item was clicked
	relativeY := int(event.Y - innerRect.Y)
	totalItems := m.getTotalItems()

	// Calculate scroll offset (same as in drawMenuItems)
	visibleHeight := int(innerRect.H)
	scrollOffset := 0
	if m.selectedIndex >= visibleHeight {
		scrollOffset = m.selectedIndex - visibleHeight + 1
	}

	clickedIndex := scrollOffset + relativeY
	if clickedIndex >= 0 && clickedIndex < totalItems {
		// Handle mouse press - select item
		if event.Modifiers&gtv.ModPress != 0 || event.Modifiers&gtv.ModClick != 0 {
			// Focus the menu if not already focused
			if !m.IsFocused() {
				m.Focus()
			}

			m.selectedIndex = clickedIndex

			// If custom input item clicked, enter input mode
			if m.isCustomInputItem(clickedIndex) {
				m.enterInputMode()
			} else {
				m.isInputActive = false
			}
		}
	}
}
