package mdv

import (
	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
)

// MarkdownViewFactory is a factory for creating markdown widgets.
// It can be used to create widgets rendering markdown content or its fragments.
// It is possible that markdown widget uses factories implementing this interface to create child widgets
// (eg. headers, paragraphs, tables, code blocks etc.).
type MarkdownViewFactory interface {
	// NewMarkdownView creates a new widget rendering the given chunk of markdown content.
	// If rect width or height is not defined (0), the widget will be auto-sized to fit the content.
	// If rect width and height are defined, the widget will be created with the given size and content will be clipped if it doesn't fit.
	// If parent is not nil, the widget will be added as a child of the parent.
	// Depending on settings, view may clip content if rect is too narrow or it may wrap text to multiple lines.
	// Depending on settings, view may clip content if rect is too short or it may add scrollbars and let it scroll.
	NewMarkdownView(parent tui.IWidget, rect gtv.TRect, content string) IMarkdownView
}

// IMarkdownView is an interface for markdown view widgets
type IMarkdownView interface {
	tui.IWidget

	// SetContent sets the markdown content to render
	SetContent(content string)

	// GetScrollOffset returns the current scroll offset
	GetScrollOffset() int

	// SetScrollOffset sets the scroll offset
	SetScrollOffset(offset int)

	// ScrollUp scrolls up by the specified number of lines
	ScrollUp(lines int)

	// ScrollDown scrolls down by the specified number of lines
	ScrollDown(lines int)

	// PageUp scrolls up by one page
	PageUp()

	// PageDown scrolls down by one page
	PageDown()
}

// TMarkdownView is a widget that renders markdown content with vertical scrolling support
type TMarkdownView struct {
	tui.TResizable

	// Parsed markdown blocks
	blocks []Block

	// Factory for creating child widgets
	factory MarkdownViewFactory

	// Scroll state
	scrollOffset   int // Current scroll offset in lines
	contentHeight  int // Total height of rendered content in lines
	viewportHeight int // Height of visible viewport

	// Rendered child widgets
	renderedWidgets []tui.IWidget
}

// NewMarkdownView creates a new markdown view widget
func NewMarkdownView(parent tui.IWidget, rect gtv.TRect, content string) *TMarkdownView {
	factory := NewDefaultMarkdownViewFactory()
	return newMarkdownViewWithFactory(parent, rect, content, factory)
}

// newMarkdownViewWithFactory creates a new markdown view with a custom factory
func newMarkdownViewWithFactory(parent tui.IWidget, rect gtv.TRect, content string, factory MarkdownViewFactory) *TMarkdownView {
	// Parse the markdown content
	blocks := ParseMarkdown(content)

	// Create the widget
	view := &TMarkdownView{
		TResizable: tui.TResizable{
			TWidget: tui.TWidget{
				Position: rect,
				Parent:   parent,
			},
		},
		blocks:          blocks,
		factory:         factory,
		scrollOffset:    0,
		contentHeight:   0,
		viewportHeight:  int(rect.H),
		renderedWidgets: []tui.IWidget{},
	}

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(view)
	}

	// Render blocks into widgets
	view.renderBlocks()

	return view
}

// SetContent sets the markdown content to render
func (m *TMarkdownView) SetContent(content string) {
	m.blocks = ParseMarkdown(content)
	m.scrollOffset = 0
	m.renderBlocks()
}

// renderBlocks renders all markdown blocks into child widgets
func (m *TMarkdownView) renderBlocks() {
	// Clear existing children
	m.Children = []tui.IWidget{}
	m.renderedWidgets = []tui.IWidget{}

	if m.factory == nil {
		return
	}

	defaultFactory, ok := m.factory.(*DefaultMarkdownViewFactory)
	if !ok {
		return
	}

	width := int(m.Position.W)
	if width == 0 {
		width = 80 // Default width
	}

	yOffset := 0
	for _, block := range m.blocks {
		widgets := defaultFactory.renderBlock(m, block, &yOffset, width)
		m.renderedWidgets = append(m.renderedWidgets, widgets...)
	}

	m.contentHeight = yOffset
}

// GetScrollOffset returns the current scroll offset
func (m *TMarkdownView) GetScrollOffset() int {
	return m.scrollOffset
}

// SetScrollOffset sets the scroll offset
func (m *TMarkdownView) SetScrollOffset(offset int) {
	maxOffset := m.contentHeight - m.viewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	m.scrollOffset = offset
}

// ScrollUp scrolls up by the specified number of lines
func (m *TMarkdownView) ScrollUp(lines int) {
	m.SetScrollOffset(m.scrollOffset - lines)
}

// ScrollDown scrolls down by the specified number of lines
func (m *TMarkdownView) ScrollDown(lines int) {
	m.SetScrollOffset(m.scrollOffset + lines)
}

// PageUp scrolls up by one page
func (m *TMarkdownView) PageUp() {
	m.ScrollUp(m.viewportHeight)
}

// PageDown scrolls down by one page
func (m *TMarkdownView) PageDown() {
	m.ScrollDown(m.viewportHeight)
}

// Draw draws the markdown view on the screen
func (m *TMarkdownView) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if m.Flags&tui.WidgetFlagHidden != 0 {
		return
	}

	absPos := m.GetAbsolutePos()

	// Draw only visible children based on scroll offset
	for _, child := range m.Children {
		childPos := child.GetPos()
		childY := int(childPos.Y)

		// Check if child is within visible viewport
		if childY >= m.scrollOffset && childY < m.scrollOffset+m.viewportHeight {
			// Adjust child position to account for scroll offset
			adjustedY := childY - m.scrollOffset

			// Temporarily modify child position for rendering
			originalY := childPos.Y
			childPos.Y = uint16(adjustedY)

			// Create a resize event to update child position
			child.HandleEvent(&tui.TEvent{
				Type: tui.TEventTypeResize,
				Rect: childPos,
			})

			// Draw the child
			child.Draw(screen)

			// Restore original position
			childPos.Y = originalY
			child.HandleEvent(&tui.TEvent{
				Type: tui.TEventTypeResize,
				Rect: childPos,
			})
		}
	}

	// Note: We don't call TWidget.Draw(screen) because we're handling children manually
	_ = absPos // Suppress unused variable warning
}

// HandleEvent handles events for the markdown view
func (m *TMarkdownView) HandleEvent(event *tui.TEvent) {
	// Handle resize events
	if event.Type == tui.TEventTypeResize {
		m.Position = event.Rect
		m.viewportHeight = int(event.Rect.H)
		// Re-render blocks with new width
		m.renderBlocks()
		return
	}

	// Handle input events for scrolling
	if event.Type == tui.TEventTypeInput && event.InputEvent != nil {
		input := event.InputEvent

		// Handle keyboard scrolling
		if input.Type == gtv.InputEventKey {
			switch {
			case input.Key == 'G' && input.Modifiers&gtv.ModFn != 0: // PageUp
				m.PageUp()
				return
			case input.Key == 'N' && input.Modifiers&gtv.ModFn != 0: // PageDown
				m.PageDown()
				return
			case input.Key == 'A' && input.Modifiers&gtv.ModFn != 0: // Up arrow
				m.ScrollUp(1)
				return
			case input.Key == 'B' && input.Modifiers&gtv.ModFn != 0: // Down arrow
				m.ScrollDown(1)
				return
			}
		}

		// Handle mouse scrolling
		if input.Type == gtv.InputEventMouse {
			if input.Modifiers&gtv.ModScrollUp != 0 {
				m.ScrollUp(3) // Scroll 3 lines at a time
				return
			}
			if input.Modifiers&gtv.ModScrollDown != 0 {
				m.ScrollDown(3)
				return
			}
		}
	}

	// Delegate other events to base widget
	m.TResizable.HandleEvent(event)
}
