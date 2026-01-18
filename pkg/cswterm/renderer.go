package cswterm

import (
	"bytes"
	"fmt"
	"io"
)

// ScreenRenderer renders a screen buffer to a terminal using ANSI escape sequences.
// It performs optimal differential rendering by tracking changes and only updating
// modified regions of the screen.
type ScreenRenderer struct {
	screen     ScreenOutput
	writer     io.Writer
	lastBuffer []Cell
	width      int
	height     int
	lastAttrs  CellAttributes
}

// NewScreenRenderer creates a new ScreenRenderer that renders the given screen
// to the specified writer.
func NewScreenRenderer(screen ScreenOutput, writer io.Writer) *ScreenRenderer {
	width, height := screen.GetSize()
	lastBuffer := make([]Cell, width*height)
	// Initialize with spaces to match ScreenBuffer initialization
	for i := range lastBuffer {
		lastBuffer[i] = Cell{Rune: ' ', Attrs: CellAttributes{}}
	}
	return &ScreenRenderer{
		screen:     screen,
		writer:     writer,
		lastBuffer: lastBuffer,
		width:      width,
		height:     height,
		lastAttrs:  CellAttributes{},
	}
}

// Render renders the current screen state to the terminal.
// It compares the current buffer with the last rendered buffer and only
// updates regions that have changed.
func (r *ScreenRenderer) Render() error {
	width, height, content := r.screen.GetContent()

	// Handle screen size changes
	if width != r.width || height != r.height {
		r.width = width
		r.height = height
		r.lastBuffer = make([]Cell, width*height)
		// Clear screen and reset cursor
		if err := r.clearScreen(); err != nil {
			return fmt.Errorf("ScreenRenderer.Render(): failed to clear screen: %w", err)
		}
	}

	// Find changed regions
	regions := r.findChangedRegions(content)

	// Merge close regions
	regions = r.mergeRegions(regions, 8)

	// Render each region
	buf := &bytes.Buffer{}
	for _, region := range regions {
		r.renderRegion(buf, content, region)
	}

	// Write all changes at once
	if buf.Len() > 0 {
		if _, err := r.writer.Write(buf.Bytes()); err != nil {
			return fmt.Errorf("ScreenRenderer.Render(): failed to write to terminal: %w", err)
		}
	}

	// Update last buffer
	copy(r.lastBuffer, content)

	return nil
}

// region represents a rectangular region on the screen
type region struct {
	x1, y1 int // top-left corner (inclusive)
	x2, y2 int // bottom-right corner (inclusive)
}

// findChangedRegions identifies all regions that have changed since last render
func (r *ScreenRenderer) findChangedRegions(content []Cell) []region {
	var regions []region

	for y := 0; y < r.height; y++ {
		x := 0
		for x < r.width {
			// Find start of changed region in this row
			for x < r.width {
				idx := y*r.width + x
				if !r.cellsEqual(content[idx], r.lastBuffer[idx]) {
					break
				}
				x++
			}

			if x >= r.width {
				break
			}

			// Found start of changed region
			startX := x

			// Find end of changed region in this row
			for x < r.width {
				idx := y*r.width + x
				if r.cellsEqual(content[idx], r.lastBuffer[idx]) {
					break
				}
				x++
			}

			// Add region for this row
			regions = append(regions, region{
				x1: startX,
				y1: y,
				x2: x - 1,
				y2: y,
			})
		}
	}

	return regions
}

// cellsEqual compares two cells for equality
func (r *ScreenRenderer) cellsEqual(a, b Cell) bool {
	return a.Rune == b.Rune &&
		a.Attrs.Attributes == b.Attrs.Attributes &&
		a.Attrs.TextColor == b.Attrs.TextColor &&
		a.Attrs.BackColor == b.Attrs.BackColor &&
		a.Attrs.StrikeColor == b.Attrs.StrikeColor
}

// mergeRegions merges regions that are close to each other
func (r *ScreenRenderer) mergeRegions(regions []region, threshold int) []region {
	if len(regions) == 0 {
		return regions
	}

	merged := []region{regions[0]}

	for i := 1; i < len(regions); i++ {
		curr := regions[i]
		last := &merged[len(merged)-1]

		// Check if regions are on the same row and close enough
		if curr.y1 == last.y1 && curr.x1-last.x2 <= threshold {
			// Merge by extending the last region
			last.x2 = curr.x2
		} else {
			// Add as separate region
			merged = append(merged, curr)
		}
	}

	return merged
}

// renderRegion renders a single region to the buffer
func (r *ScreenRenderer) renderRegion(buf *bytes.Buffer, content []Cell, reg region) {
	// Move cursor to start of region
	r.moveCursor(buf, reg.x1, reg.y1)

	// Reset attributes at start of each region
	currentAttrs := CellAttributes{}

	// Render each cell in the region
	for y := reg.y1; y <= reg.y2; y++ {
		if y > reg.y1 {
			// Move to next line
			r.moveCursor(buf, reg.x1, y)
		}

		for x := reg.x1; x <= reg.x2; x++ {
			idx := y*r.width + x
			cell := content[idx]

			// Update attributes if changed
			if !r.attrsEqual(cell.Attrs, currentAttrs) {
				r.setAttributes(buf, cell.Attrs)
				currentAttrs = cell.Attrs
			}

			// Write the character
			buf.WriteRune(cell.Rune)
		}
	}
}

// moveCursor writes ANSI sequence to move cursor to specified position
// Positions are 0-based, but ANSI uses 1-based positioning
func (r *ScreenRenderer) moveCursor(buf *bytes.Buffer, x, y int) {
	fmt.Fprintf(buf, "\x1b[%d;%dH", y+1, x+1)
}

// attrsEqual compares two CellAttributes for equality
func (r *ScreenRenderer) attrsEqual(a, b CellAttributes) bool {
	return a.Attributes == b.Attributes &&
		a.TextColor == b.TextColor &&
		a.BackColor == b.BackColor &&
		a.StrikeColor == b.StrikeColor
}

// setAttributes writes ANSI sequences to set cell attributes
func (r *ScreenRenderer) setAttributes(buf *bytes.Buffer, attrs CellAttributes) {
	// Build all SGR parameters together
	var params []int

	// Start with reset (0)
	params = append(params, 0)

	// Text attributes
	if attrs.Attributes&AttrBold != 0 {
		params = append(params, 1)
	}
	if attrs.Attributes&AttrDim != 0 {
		params = append(params, 2)
	}
	if attrs.Attributes&AttrItalic != 0 {
		params = append(params, 3)
	}
	if attrs.Attributes&AttrUnderline != 0 {
		params = append(params, 4)
	}
	if attrs.Attributes&AttrBlink != 0 {
		params = append(params, 5)
	}
	if attrs.Attributes&AttrReverse != 0 {
		params = append(params, 7)
	}
	if attrs.Attributes&AttrHidden != 0 {
		params = append(params, 8)
	}
	if attrs.Attributes&AttrStrikethrough != 0 {
		params = append(params, 9)
	}

	// Underline styles (need special handling)
	if attrs.Attributes&AttrDoubleUnderline != 0 {
		params = append(params, 21)
	}

	// Write all parameters in one SGR sequence
	buf.WriteString("\x1b[")
	for i, p := range params {
		if i > 0 {
			buf.WriteString(";")
		}
		fmt.Fprintf(buf, "%d", p)
	}
	buf.WriteString("m")

	// Foreground color (24-bit RGB) - separate sequence
	if attrs.TextColor != 0 {
		r := (attrs.TextColor >> 16) & 0xFF
		g := (attrs.TextColor >> 8) & 0xFF
		b := attrs.TextColor & 0xFF
		fmt.Fprintf(buf, "\x1b[38;2;%d;%d;%dm", r, g, b)
	}

	// Background color (24-bit RGB) - separate sequence
	if attrs.BackColor != 0 {
		r := (attrs.BackColor >> 16) & 0xFF
		g := (attrs.BackColor >> 8) & 0xFF
		b := attrs.BackColor & 0xFF
		fmt.Fprintf(buf, "\x1b[48;2;%d;%d;%dm", r, g, b)
	}
}

// clearScreen clears the entire screen and moves cursor to home position
func (r *ScreenRenderer) clearScreen() error {
	// ESC[2J clears entire screen, ESC[H moves cursor to home
	_, err := r.writer.Write([]byte("\x1b[2J\x1b[H"))
	if err != nil {
		return fmt.Errorf("ScreenRenderer.clearScreen(): failed to write clear sequence: %w", err)
	}
	return nil
}

// HideCursor hides the terminal cursor
func (r *ScreenRenderer) HideCursor() error {
	_, err := r.writer.Write([]byte("\x1b[?25l"))
	if err != nil {
		return fmt.Errorf("ScreenRenderer.HideCursor(): failed to write hide cursor sequence: %w", err)
	}
	return nil
}

// ShowCursor shows the terminal cursor
func (r *ScreenRenderer) ShowCursor() error {
	_, err := r.writer.Write([]byte("\x1b[?25h"))
	if err != nil {
		return fmt.Errorf("ScreenRenderer.ShowCursor(): failed to write show cursor sequence: %w", err)
	}
	return nil
}

// Reset resets the renderer state, clearing the last buffer
// This is useful for example when screen size changes.
func (r *ScreenRenderer) Reset() {
	width, height := r.screen.GetSize()
	r.width = width
	r.height = height
	r.lastBuffer = make([]Cell, width*height)
	r.lastAttrs = CellAttributes{}
}
