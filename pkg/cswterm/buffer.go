package cswterm

import (
	"sync"
)

// Cell represents a single character cell in the screen buffer.
type Cell struct {
	Rune  rune
	Attrs CellAttributes
}

// listener represents a registered event listener with its event queue
type listener struct {
	ch    chan InputEvent
	queue []InputEvent
}

// ScreenBuffer is a test double implementation of ScreenOutput interface.
// It maintains an in-memory buffer and does not output to terminal.
type ScreenBuffer struct {
	width     int
	height    int
	buffer    []Cell
	listeners map[chan InputEvent]*listener
	mu        sync.Mutex
	queueSize int
}

// NewMockScreen creates a new ScreenBuffer with the specified dimensions.
// The queueSize parameter specifies the maximum number of events that can be
// queued for each listener when the listener's channel is full.
// If queueSize is 0, a default value of 100 is used.
func NewMockScreen(width, height int, queueSize int) *ScreenBuffer {
	buffer := make([]Cell, width*height)
	// Initialize with spaces
	for i := range buffer {
		buffer[i] = Cell{Rune: ' ', Attrs: CellAttributes{}}
	}
	if queueSize == 0 {
		queueSize = 100
	}
	return &ScreenBuffer{
		width:     width,
		height:    height,
		buffer:    buffer,
		listeners: make(map[chan InputEvent]*listener),
		queueSize: queueSize,
	}
}

// Size returns the size of the screen in characters.
func (m *ScreenBuffer) Size() (width int, height int) {
	return m.width, m.height
}

// GetContent returns the whole content of the screen.
// Returns width, height, and the internal buffer array.
// The content is a single dimensional array where index = y*width + x.
func (m *ScreenBuffer) GetContent() (width int, height int, content []Cell) {
	return m.width, m.height, m.buffer
}

// PutText puts text at the specified position with the specified attributes.
// If the text is longer than the width of the screen, it is truncated.
func (m *ScreenBuffer) PutText(x int, y int, text string, attrs CellAttributes) {
	if y < 0 || y >= m.height {
		return
	}
	if x < 0 || x >= m.width {
		return
	}

	col := x
	for _, r := range text {
		if col >= m.width {
			break
		}
		idx := y*m.width + col
		m.buffer[idx] = Cell{
			Rune:  r,
			Attrs: attrs,
		}
		col++
	}
}

// Clear resets all cells to spaces with default attributes.
func (m *ScreenBuffer) Clear() {
	for i := range m.buffer {
		m.buffer[i] = Cell{Rune: ' ', Attrs: CellAttributes{}}
	}
}

// Listen registers a channel to receive input events.
// The channel will be automatically unregistered when it is closed.
func (m *ScreenBuffer) Listen(ch chan InputEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Register the listener
	m.listeners[ch] = &listener{
		ch:    ch,
		queue: make([]InputEvent, 0),
	}
}

// Notify sends an input event to all registered listeners.
// It does not block if channels are full - instead, events are queued.
// Events are delivered in order. If queue overflows, oldest events are dropped.
func (m *ScreenBuffer) Notify(event InputEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Collect channels to remove (those that are closed)
	var toRemove []chan InputEvent

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
func (m *ScreenBuffer) trySend(ch chan InputEvent, event InputEvent, toRemove *[]chan InputEvent) bool {
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
func (m *ScreenBuffer) isMarkedForRemoval(ch chan InputEvent, toRemove []chan InputEvent) bool {
	for _, c := range toRemove {
		if c == ch {
			return true
		}
	}
	return false
}
