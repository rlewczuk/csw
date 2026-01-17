package cswterm

import "strings"

// ScreenVerifier provides methods to verify screen buffer content in tests.
type ScreenVerifier struct {
	width   int
	height  int
	content []Cell
}

// NewScreenVerifier creates a new ScreenVerifier instance.
// The content must have exactly width * height cells.
func NewScreenVerifier(width, height int, content []Cell) *ScreenVerifier {
	if len(content) != width*height {
		panic("NewScreenVerifier: len(content) must equal width * height")
	}
	return &ScreenVerifier{
		width:   width,
		height:  height,
		content: content,
	}
}

// GetCell returns the cell at the specified position.
// Returns a Cell with space rune if position is out of bounds.
func (v *ScreenVerifier) GetCell(x, y int) Cell {
	if x < 0 || x >= v.width || y < 0 || y >= v.height {
		return Cell{Rune: ' ', Attrs: CellAttributes{}}
	}
	idx := y*v.width + x
	return v.content[idx]
}

// GetText returns the text content in the specified rectangle.
// Rectangle is defined by top-left corner (x, y) and dimensions (width, height).
func (v *ScreenVerifier) GetText(x, y, width, height int) string {
	var sb strings.Builder
	for row := y; row < y+height && row < v.height; row++ {
		if row < 0 {
			continue
		}
		for col := x; col < x+width && col < v.width; col++ {
			if col < 0 {
				continue
			}
			idx := row*v.width + col
			sb.WriteRune(v.content[idx].Rune)
		}
		if row < y+height-1 && row < v.height-1 {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}

// HasText checks if the specified text is present in the given rectangle.
// Rectangle is defined by top-left corner (x, y) and dimensions (width, height).
func (v *ScreenVerifier) HasText(x, y, width, height int, text string) bool {
	actualText := v.GetText(x, y, width, height)
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
func (v *ScreenVerifier) HasTextWithAttrs(x, y, width, height int, text string, mask AttributeMask) bool {
	// Convert text to runes for proper indexing
	runes := []rune(text)

	// Calculate actual bounds
	startY := y
	endY := y + height
	if startY < 0 {
		startY = 0
	}
	if endY > v.height {
		endY = v.height
	}

	startX := x
	endX := x + width
	if startX < 0 {
		startX = 0
	}
	if endX > v.width {
		endX = v.width
	}

	// Track position in the text
	textIdx := 0

	for row := startY; row < endY; row++ {
		for col := startX; col < endX; col++ {
			if textIdx >= len(runes) {
				// All text matched
				return true
			}

			idx := row*v.width + col
			cell := v.content[idx]

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
