package tio

import (
	"sync"

	"github.com/rlewczuk/csw/pkg/gtv"
)

// listener represents a registered event listener with its event queue
type listener struct {
	ch    chan gtv.InputEvent
	queue []gtv.InputEvent
}

// ScreenBuffer is a test double implementation of IScreenOutput interface.
// It maintains an in-memory buffer and does not output to terminal.
type ScreenBuffer struct {
	width       int
	height      int
	buffer      []gtv.Cell
	listeners   map[chan gtv.InputEvent]*listener
	mu          sync.Mutex
	queueSize   int
	cursorX     int
	cursorY     int
	cursorStyle gtv.CursorStyle
}

// NewScreenBuffer creates a new ScreenBuffer with the specified dimensions.
// The queueSize parameter specifies the maximum number of events that can be
// queued for each listener when the listener's channel is full.
// If queueSize is 0, a default value of 100 is used.
func NewScreenBuffer(width, height int, queueSize int) *ScreenBuffer {
	buffer := make([]gtv.Cell, width*height)
	// Initialize with spaces
	for i := range buffer {
		buffer[i] = gtv.Cell{Rune: ' ', Attrs: gtv.CellAttributes{}}
	}
	if queueSize == 0 {
		queueSize = 100
	}
	return &ScreenBuffer{
		width:     width,
		height:    height,
		buffer:    buffer,
		listeners: make(map[chan gtv.InputEvent]*listener),
		queueSize: queueSize,
	}
}

// GetSize returns the size of the screen in characters.
func (m *ScreenBuffer) GetSize() (width int, height int) {
	return m.width, m.height
}

// SetSize changes the size of the screen in characters.
// When resizing, content is preserved:
// - horizontal expansion: new cells on the right are filled with spaces
// - vertical expansion: new rows at the bottom are filled with spaces
// - horizontal shrinking: leftmost columns are kept
// - vertical shrinking: topmost rows are kept
func (m *ScreenBuffer) SetSize(newWidth int, newHeight int) {
	if newWidth == m.width && newHeight == m.height {
		return
	}

	// Create new buffer
	newBuffer := make([]gtv.Cell, newWidth*newHeight)

	// Initialize all cells with spaces
	for i := range newBuffer {
		newBuffer[i] = gtv.Cell{Rune: ' ', Attrs: gtv.CellAttributes{}}
	}

	// Copy existing content, preserving as much as possible
	// We copy row by row, taking the minimum of old and new dimensions
	rowsToCopy := m.height
	if newHeight < rowsToCopy {
		rowsToCopy = newHeight
	}

	colsToCopy := m.width
	if newWidth < colsToCopy {
		colsToCopy = newWidth
	}

	for y := 0; y < rowsToCopy; y++ {
		for x := 0; x < colsToCopy; x++ {
			oldIdx := y*m.width + x
			newIdx := y*newWidth + x
			newBuffer[newIdx] = m.buffer[oldIdx]
		}
	}

	// Update the buffer and dimensions
	m.buffer = newBuffer
	m.width = newWidth
	m.height = newHeight
}

// GetContent returns the whole content of the screen.
// Returns width, height, and the internal buffer array.
// The content is a single dimensional array where index = y*width + x.
func (m *ScreenBuffer) GetContent() (width int, height int, content []gtv.Cell) {
	return m.width, m.height, m.buffer
}

// PutText puts text at the specified position with the specified attributes.
// The rect parameter specifies the position (X, Y) and optional clipping rectangle (W, H).
// If W and H are 0, the text is clipped only to screen boundaries.
// If W and H are non-zero, the text is clipped to both the rectangle and screen boundaries.
// Text is always rendered on a single line (Y coordinate from rect).
func (m *ScreenBuffer) PutText(rect gtv.TRect, text string, attrs gtv.CellAttributes) {
	x := int(rect.X)
	y := int(rect.Y)

	// Check if Y is within screen bounds
	if y < 0 || y >= m.height {
		return
	}

	// Check if X is within screen bounds
	if x < 0 || x >= m.width {
		return
	}

	// Determine the clipping rectangle
	var clipWidth int
	if rect.W == 0 {
		// Use full screen width
		clipWidth = m.width
	} else {
		// Use specified width, but clip to screen boundaries
		clipWidth = int(rect.X + rect.W)
		if clipWidth > m.width {
			clipWidth = m.width
		}
	}

	col := x
	for _, r := range text {
		// Check against clipping rectangle
		if col >= clipWidth {
			break
		}
		idx := y*m.width + col
		m.buffer[idx] = gtv.Cell{
			Rune:  r,
			Attrs: attrs,
		}
		col++
	}
}

// PutContent puts raw cell content at the specified position.
// The rect parameter specifies the position (X, Y) and optional clipping rectangle (W, H).
// If W and H are 0, the content is clipped only to screen boundaries.
// If W and H are non-zero, the content is clipped to both the rectangle and screen boundaries.
// Content is always rendered on a single line (Y coordinate from rect).
func (m *ScreenBuffer) PutContent(rect gtv.TRect, content []gtv.Cell) {
	x := int(rect.X)
	y := int(rect.Y)

	// Check if Y is within screen bounds
	if y < 0 || y >= m.height {
		return
	}

	// Check if X is within screen bounds
	if x < 0 || x >= m.width {
		return
	}

	// Determine the clipping rectangle
	var clipWidth int
	if rect.W == 0 {
		// Use full screen width
		clipWidth = m.width
	} else {
		// Use specified width, but clip to screen boundaries
		clipWidth = int(rect.X + rect.W)
		if clipWidth > m.width {
			clipWidth = m.width
		}
	}

	col := x
	for _, cell := range content {
		// Check against clipping rectangle
		if col >= clipWidth {
			break
		}
		idx := y*m.width + col
		m.buffer[idx] = cell
		col++
	}
}

// Clear resets all cells to spaces with default attributes.
func (m *ScreenBuffer) Clear() {
	for i := range m.buffer {
		m.buffer[i] = gtv.Cell{Rune: ' ', Attrs: gtv.CellAttributes{}}
	}
}

// MoveCursor moves the cursor to the specified position.
func (m *ScreenBuffer) MoveCursor(x int, y int) {
	m.cursorX = x
	m.cursorY = y
}

// SetCursorStyle sets the cursor style.
func (m *ScreenBuffer) SetCursorStyle(style gtv.CursorStyle) {
	m.cursorStyle = style
}

// GetCursorPosition returns the current cursor position.
func (m *ScreenBuffer) GetCursorPosition() (x int, y int) {
	return m.cursorX, m.cursorY
}

// GetCursorStyle returns the current cursor style.
func (m *ScreenBuffer) GetCursorStyle() gtv.CursorStyle {
	return m.cursorStyle
}

// Listen registers a channel to receive input events.
// The channel will be automatically unregistered when it is closed.
func (m *ScreenBuffer) Listen(ch chan gtv.InputEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Register the listener
	m.listeners[ch] = &listener{
		ch:    ch,
		queue: make([]gtv.InputEvent, 0),
	}
}

// Notify sends an input event to all registered listeners.
// It does not block if channels are full - instead, events are queued.
// Events are delivered in order. If queue overflows, oldest events are dropped.
func (m *ScreenBuffer) Notify(event gtv.InputEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Collect channels to remove (those that are closed)
	var toRemove []chan gtv.InputEvent

	// Iterate through all listeners
	for ch, l := range m.listeners {
		// First, try to deliver any queued events
		deliveredCount := 0
		for i := 0; i < len(l.queue); i++ {
			if m.trySend(ch, l.queue[i], &toRemove) {
				deliveredCount++
			} else {
				// Channel is full or closed
				break
			}
		}

		// Remove delivered events from queue
		if deliveredCount > 0 {
			l.queue = l.queue[deliveredCount:]
		}

		// Check if channel was marked for removal
		if m.isMarkedForRemoval(ch, toRemove) {
			continue
		}

		// Now try to send the new event
		if m.trySend(ch, event, &toRemove) {
			// Successfully sent, continue to next listener
			continue
		}

		// Check if channel was closed during send attempt
		if m.isMarkedForRemoval(ch, toRemove) {
			continue
		}

		// Channel is full, add event to queue
		l.queue = append(l.queue, event)

		// Check if queue has overflowed
		if len(l.queue) > m.queueSize {
			// Drop oldest event (first in queue)
			l.queue = l.queue[1:]
		}
	}

	// Remove closed listeners
	for _, ch := range toRemove {
		delete(m.listeners, ch)
	}
}

// trySend attempts to send an event to a channel without blocking.
// Returns true if sent successfully, false if channel is full or closed.
// If channel is closed, it's added to toRemove list.
func (m *ScreenBuffer) trySend(ch chan gtv.InputEvent, event gtv.InputEvent, toRemove *[]chan gtv.InputEvent) bool {
	defer func() {
		if r := recover(); r != nil {
			// Panic occurred, channel is closed
			*toRemove = append(*toRemove, ch)
		}
	}()

	select {
	case ch <- event:
		return true
	default:
		return false
	}
}

// isMarkedForRemoval checks if a channel is in the toRemove list
func (m *ScreenBuffer) isMarkedForRemoval(ch chan gtv.InputEvent, toRemove []chan gtv.InputEvent) bool {
	for _, c := range toRemove {
		if c == ch {
			return true
		}
	}
	return false
}
