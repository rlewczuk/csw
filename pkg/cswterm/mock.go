package cswterm

import "strings"

// Cell represents a single character cell in the screen buffer.
type Cell struct {
	Rune  rune
	Attrs CellAttributes
}

// MockScreen is a test double implementation of Screen interface.
// It maintains an in-memory buffer and does not output to terminal.
type MockScreen struct {
	width  int
	height int
	buffer [][]Cell
}

// NewMockScreen creates a new MockScreen with the specified dimensions.
func NewMockScreen(width, height int) *MockScreen {
	buffer := make([][]Cell, height)
	for i := range buffer {
		buffer[i] = make([]Cell, width)
		// Initialize with spaces
		for j := range buffer[i] {
			buffer[i][j] = Cell{Rune: ' ', Attrs: CellAttributes{}}
		}
	}
	return &MockScreen{
		width:  width,
		height: height,
		buffer: buffer,
	}
}

// Size returns the size of the screen in characters.
func (m *MockScreen) Size() (width int, height int) {
	return m.width, m.height
}

// PutText puts text at the specified position with the specified attributes.
// If the text is longer than the width of the screen, it is truncated.
func (m *MockScreen) PutText(x int, y int, text string, attrs TextAttributes) {
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
		m.buffer[y][col] = Cell{
			Rune:  r,
			Attrs: cellAttrs,
		}
		col++
	}
}

// GetCell returns the cell at the specified position.
// Returns a Cell with space rune if position is out of bounds.
func (m *MockScreen) GetCell(x, y int) Cell {
	if x < 0 || x >= m.width || y < 0 || y >= m.height {
		return Cell{Rune: ' ', Attrs: CellAttributes{}}
	}
	return m.buffer[y][x]
}

// GetText returns the text content in the specified rectangle.
// Rectangle is defined by top-left corner (x, y) and dimensions (width, height).
func (m *MockScreen) GetText(x, y, width, height int) string {
	var sb strings.Builder
	for row := y; row < y+height && row < m.height; row++ {
		if row < 0 {
			continue
		}
		for col := x; col < x+width && col < m.width; col++ {
			if col < 0 {
				continue
			}
			sb.WriteRune(m.buffer[row][col].Rune)
		}
		if row < y+height-1 && row < m.height-1 {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}

// HasText checks if the specified text is present in the given rectangle.
// Rectangle is defined by top-left corner (x, y) and dimensions (width, height).
func (m *MockScreen) HasText(x, y, width, height int, text string) bool {
	actualText := m.GetText(x, y, width, height)
	return actualText == text
}

// AttributeMask represents which attributes to check when comparing cells.
type AttributeMask struct {
	CheckAttributes  bool
	CheckTextColor   bool
	CheckBackColor   bool
	CheckStrikeColor bool
	// Expected values (only used if corresponding Check* is true)
	Attributes  TextAttributes
	TextColor   uint32
	BackColor   uint32
	StrikeColor uint32
}

// matchesAttributes checks if a cell's attributes match the mask.
func (mask *AttributeMask) matchesAttributes(attrs CellAttributes) bool {
	if mask.CheckAttributes {
		if attrs.Attributes != mask.Attributes {
			return false
		}
	}
	if mask.CheckTextColor {
		if attrs.TextColor != mask.TextColor {
			return false
		}
	}
	if mask.CheckBackColor {
		if attrs.BackColor != mask.BackColor {
			return false
		}
	}
	if mask.CheckStrikeColor {
		if attrs.StrikeColor != mask.StrikeColor {
			return false
		}
	}
	return true
}

// HasTextWithAttrs checks if the specified text with attributes is present in the given rectangle.
// Rectangle is defined by top-left corner (x, y) and dimensions (width, height).
// The mask parameter specifies which attributes to check.
func (m *MockScreen) HasTextWithAttrs(x, y, width, height int, text string, mask AttributeMask) bool {
	// Convert text to runes for proper indexing
	runes := []rune(text)

	// Calculate actual bounds
	startY := y
	endY := y + height
	if startY < 0 {
		startY = 0
	}
	if endY > m.height {
		endY = m.height
	}

	startX := x
	endX := x + width
	if startX < 0 {
		startX = 0
	}
	if endX > m.width {
		endX = m.width
	}

	// Track position in the text
	textIdx := 0

	for row := startY; row < endY; row++ {
		for col := startX; col < endX; col++ {
			if textIdx >= len(runes) {
				// All text matched
				return true
			}

			cell := m.buffer[row][col]

			// Check if rune matches
			if cell.Rune != runes[textIdx] {
				return false
			}

			// Check if attributes match (according to mask)
			if !mask.matchesAttributes(cell.Attrs) {
				return false
			}

			textIdx++
		}

		// Handle newline in text
		if textIdx < len(runes) && runes[textIdx] == '\n' {
			textIdx++
		}
	}

	// Check if we matched all the text
	return textIdx >= len(runes)
}

// Clear resets all cells to spaces with default attributes.
func (m *MockScreen) Clear() {
	for i := range m.buffer {
		for j := range m.buffer[i] {
			m.buffer[i][j] = Cell{Rune: ' ', Attrs: CellAttributes{}}
		}
	}
}
