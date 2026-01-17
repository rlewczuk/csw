package cswterm

// Cell represents a single character cell in the screen buffer.
type Cell struct {
	Rune  rune
	Attrs CellAttributes
}

// ScreenBuffer is a test double implementation of Screen interface.
// It maintains an in-memory buffer and does not output to terminal.
type ScreenBuffer struct {
	width  int
	height int
	buffer []Cell
}

// NewMockScreen creates a new ScreenBuffer with the specified dimensions.
func NewMockScreen(width, height int) *ScreenBuffer {
	buffer := make([]Cell, width*height)
	// Initialize with spaces
	for i := range buffer {
		buffer[i] = Cell{Rune: ' ', Attrs: CellAttributes{}}
	}
	return &ScreenBuffer{
		width:  width,
		height: height,
		buffer: buffer,
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
func (m *ScreenBuffer) PutText(x int, y int, text string, attrs TextAttributes) {
	if y < 0 || y >= m.height {
		return
	}
	if x < 0 || x >= m.width {
		return
	}

	cellAttrs := CellAttributes{Attributes: attrs}
	col := x
	for _, r := range text {
		if col >= m.width {
			break
		}
		idx := y*m.width + col
		m.buffer[idx] = Cell{
			Rune:  r,
			Attrs: cellAttrs,
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
