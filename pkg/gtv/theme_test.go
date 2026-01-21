package gtv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockScreen is a simple mock implementation of IScreenOutput for testing
type mockScreen struct {
	width       int
	height      int
	buffer      []Cell
	cursorX     int
	cursorY     int
	cursorStyle CursorStyle
}

func newMockScreen(width, height int) *mockScreen {
	buffer := make([]Cell, width*height)
	for i := range buffer {
		buffer[i] = Cell{Rune: ' ', Attrs: CellAttributes{}}
	}
	return &mockScreen{
		width:  width,
		height: height,
		buffer: buffer,
	}
}

func (m *mockScreen) GetSize() (width int, height int) {
	return m.width, m.height
}

func (m *mockScreen) SetSize(width int, height int) {
	if width == m.width && height == m.height {
		return
	}
	newBuffer := make([]Cell, width*height)
	for i := range newBuffer {
		newBuffer[i] = Cell{Rune: ' ', Attrs: CellAttributes{}}
	}
	rowsToCopy := m.height
	if height < rowsToCopy {
		rowsToCopy = height
	}
	colsToCopy := m.width
	if width < colsToCopy {
		colsToCopy = width
	}
	for y := 0; y < rowsToCopy; y++ {
		for x := 0; x < colsToCopy; x++ {
			oldIdx := y*m.width + x
			newIdx := y*width + x
			newBuffer[newIdx] = m.buffer[oldIdx]
		}
	}
	m.buffer = newBuffer
	m.width = width
	m.height = height
}

func (m *mockScreen) GetContent() (width int, height int, content []Cell) {
	return m.width, m.height, m.buffer
}

func (m *mockScreen) PutText(rect TRect, text string, attrs CellAttributes) {
	x := int(rect.X)
	y := int(rect.Y)
	if y < 0 || y >= m.height || x < 0 || x >= m.width {
		return
	}
	clipWidth := m.width
	if rect.W != 0 {
		clipWidth = int(rect.X + rect.W)
		if clipWidth > m.width {
			clipWidth = m.width
		}
	}
	col := x
	for _, r := range text {
		if col >= clipWidth {
			break
		}
		idx := y*m.width + col
		m.buffer[idx] = Cell{Rune: r, Attrs: attrs}
		col++
	}
}

func (m *mockScreen) PutContent(rect TRect, content []Cell) {
	x := int(rect.X)
	y := int(rect.Y)
	if y < 0 || y >= m.height || x < 0 || x >= m.width {
		return
	}
	clipWidth := m.width
	if rect.W != 0 {
		clipWidth = int(rect.X + rect.W)
		if clipWidth > m.width {
			clipWidth = m.width
		}
	}
	col := x
	for _, cell := range content {
		if col >= clipWidth {
			break
		}
		idx := y*m.width + col
		m.buffer[idx] = cell
		col++
	}
}

func (m *mockScreen) MoveCursor(x int, y int) {
	m.cursorX = x
	m.cursorY = y
}

func (m *mockScreen) SetCursorStyle(style CursorStyle) {
	m.cursorStyle = style
}

func (m *mockScreen) GetCursorPosition() (x int, y int) {
	return m.cursorX, m.cursorY
}

func (m *mockScreen) GetCursorStyle() CursorStyle {
	return m.cursorStyle
}

func TestNewThemeInterceptor(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {TextColor: 0xFF0000, BackColor: 0x00FF00},
	}

	interceptor := NewThemeInterceptor(screen, theme)

	assert.NotNil(t, interceptor, "NewThemeInterceptor() at theme.go")
	assert.Equal(t, screen, interceptor.output, "NewThemeInterceptor() output at theme.go")
	assert.Equal(t, theme, interceptor.theme, "NewThemeInterceptor() theme at theme.go")
}

func TestThemeInterceptor_GetSize(t *testing.T) {
	screen := newMockScreen(80, 24)
	interceptor := NewThemeInterceptor(screen, nil)

	width, height := interceptor.GetSize()

	assert.Equal(t, 80, width, "ThemeInterceptor.GetSize() width at theme.go")
	assert.Equal(t, 24, height, "ThemeInterceptor.GetSize() height at theme.go")
}

func TestThemeInterceptor_SetSize(t *testing.T) {
	screen := newMockScreen(80, 24)
	interceptor := NewThemeInterceptor(screen, nil)

	interceptor.SetSize(100, 30)

	width, height := screen.GetSize()
	assert.Equal(t, 100, width, "ThemeInterceptor.SetSize() width at theme.go")
	assert.Equal(t, 30, height, "ThemeInterceptor.SetSize() height at theme.go")
}

func TestThemeInterceptor_GetContent(t *testing.T) {
	screen := newMockScreen(80, 24)
	interceptor := NewThemeInterceptor(screen, nil)

	width, height, content := interceptor.GetContent()

	assert.Equal(t, 80, width, "ThemeInterceptor.GetContent() width at theme.go")
	assert.Equal(t, 24, height, "ThemeInterceptor.GetContent() height at theme.go")
	assert.Equal(t, 80*24, len(content), "ThemeInterceptor.GetContent() content length at theme.go")
}

func TestThemeInterceptor_MoveCursor(t *testing.T) {
	screen := newMockScreen(80, 24)
	interceptor := NewThemeInterceptor(screen, nil)

	interceptor.MoveCursor(10, 5)

	x, y := screen.GetCursorPosition()
	assert.Equal(t, 10, x, "ThemeInterceptor.MoveCursor() x at theme.go")
	assert.Equal(t, 5, y, "ThemeInterceptor.MoveCursor() y at theme.go")
}

func TestThemeInterceptor_SetCursorStyle(t *testing.T) {
	screen := newMockScreen(80, 24)
	interceptor := NewThemeInterceptor(screen, nil)

	interceptor.SetCursorStyle(CursorStyleBlock)

	style := screen.GetCursorStyle()
	assert.Equal(t, CursorStyleBlock, style, "ThemeInterceptor.SetCursorStyle() at theme.go")
}

func TestThemeInterceptor_PutText_NoThemeTag(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {TextColor: 0xFF0000, BackColor: 0x00FF00},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Attributes without theme tag
	attrs := CellAttributes{
		Attributes: AttrBold,
		TextColor:  0x0000FF,
		BackColor:  0xFFFF00,
	}

	interceptor.PutText(TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", attrs)

	// Verify content
	_, _, content := screen.GetContent()
	assert.Equal(t, 'H', content[0].Rune, "ThemeInterceptor.PutText() rune at theme.go")
	assert.Equal(t, attrs, content[0].Attrs, "ThemeInterceptor.PutText() attrs at theme.go")
}

func TestThemeInterceptor_PutText_WithThemeTag_AppliesColors(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {
			Attributes:  AttrItalic,
			TextColor:   0xFF0000,
			BackColor:   0x00FF00,
			StrikeColor: 0x0000FF,
		},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Attributes with theme tag but zero colors
	attrs := CellAttributes{
		ThemeTag: 1,
	}

	interceptor.PutText(TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", attrs)

	// Verify content - should have theme colors
	_, _, content := screen.GetContent()
	assert.Equal(t, 'H', content[0].Rune, "ThemeInterceptor.PutText() rune at theme.go")
	assert.Equal(t, uint32(1), content[0].Attrs.ThemeTag, "ThemeInterceptor.PutText() theme tag at theme.go")
	assert.Equal(t, AttrItalic, content[0].Attrs.Attributes, "ThemeInterceptor.PutText() attributes at theme.go")
	assert.Equal(t, uint32(0xFF0000), content[0].Attrs.TextColor, "ThemeInterceptor.PutText() text color at theme.go")
	assert.Equal(t, uint32(0x00FF00), content[0].Attrs.BackColor, "ThemeInterceptor.PutText() back color at theme.go")
	assert.Equal(t, uint32(0x0000FF), content[0].Attrs.StrikeColor, "ThemeInterceptor.PutText() strike color at theme.go")
}

func TestThemeInterceptor_PutText_WithThemeTag_PreservesExplicitColors(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {
			Attributes:  AttrItalic,
			TextColor:   0xFF0000,
			BackColor:   0x00FF00,
			StrikeColor: 0x0000FF,
		},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Attributes with theme tag and explicit colors
	attrs := CellAttributes{
		ThemeTag:    1,
		Attributes:  AttrBold,
		TextColor:   0xAAAAAA,
		BackColor:   0xBBBBBB,
		StrikeColor: 0xCCCCCC,
	}

	interceptor.PutText(TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", attrs)

	// Verify content - should preserve explicit colors
	_, _, content := screen.GetContent()
	assert.Equal(t, 'H', content[0].Rune, "ThemeInterceptor.PutText() rune at theme.go")
	assert.Equal(t, uint32(1), content[0].Attrs.ThemeTag, "ThemeInterceptor.PutText() theme tag at theme.go")
	assert.Equal(t, AttrBold, content[0].Attrs.Attributes, "ThemeInterceptor.PutText() attributes at theme.go")
	assert.Equal(t, uint32(0xAAAAAA), content[0].Attrs.TextColor, "ThemeInterceptor.PutText() text color at theme.go")
	assert.Equal(t, uint32(0xBBBBBB), content[0].Attrs.BackColor, "ThemeInterceptor.PutText() back color at theme.go")
	assert.Equal(t, uint32(0xCCCCCC), content[0].Attrs.StrikeColor, "ThemeInterceptor.PutText() strike color at theme.go")
}

func TestThemeInterceptor_PutText_WithThemeTag_PartialExplicitColors(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {
			TextColor:   0xFF0000,
			BackColor:   0x00FF00,
			StrikeColor: 0x0000FF,
		},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Attributes with theme tag and only text color explicit
	attrs := CellAttributes{
		ThemeTag:  1,
		TextColor: 0xAAAAAA,
	}

	interceptor.PutText(TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", attrs)

	// Verify content - should preserve text color, apply theme back/strike colors
	_, _, content := screen.GetContent()
	assert.Equal(t, 'H', content[0].Rune, "ThemeInterceptor.PutText() rune at theme.go")
	assert.Equal(t, uint32(0xAAAAAA), content[0].Attrs.TextColor, "ThemeInterceptor.PutText() text color at theme.go")
	assert.Equal(t, uint32(0x00FF00), content[0].Attrs.BackColor, "ThemeInterceptor.PutText() back color at theme.go")
	assert.Equal(t, uint32(0x0000FF), content[0].Attrs.StrikeColor, "ThemeInterceptor.PutText() strike color at theme.go")
}

func TestThemeInterceptor_PutText_ThemeNotFound(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {TextColor: 0xFF0000, BackColor: 0x00FF00},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Attributes with non-existent theme tag
	attrs := CellAttributes{
		ThemeTag: 999,
	}

	interceptor.PutText(TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", attrs)

	// Verify content - should have original attributes
	_, _, content := screen.GetContent()
	assert.Equal(t, 'H', content[0].Rune, "ThemeInterceptor.PutText() rune at theme.go")
	assert.Equal(t, attrs, content[0].Attrs, "ThemeInterceptor.PutText() attrs at theme.go")
}

func TestThemeInterceptor_PutContent_NoThemeTag(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {TextColor: 0xFF0000, BackColor: 0x00FF00},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Content without theme tag
	cells := []Cell{
		{Rune: 'A', Attrs: CellAttributes{TextColor: 0x0000FF}},
		{Rune: 'B', Attrs: CellAttributes{BackColor: 0xFFFF00}},
	}

	interceptor.PutContent(TRect{X: 0, Y: 0, W: 0, H: 0}, cells)

	// Verify content
	_, _, content := screen.GetContent()
	assert.Equal(t, 'A', content[0].Rune, "ThemeInterceptor.PutContent() rune at theme.go")
	assert.Equal(t, cells[0].Attrs, content[0].Attrs, "ThemeInterceptor.PutContent() attrs at theme.go")
	assert.Equal(t, 'B', content[1].Rune, "ThemeInterceptor.PutContent() rune at theme.go")
	assert.Equal(t, cells[1].Attrs, content[1].Attrs, "ThemeInterceptor.PutContent() attrs at theme.go")
}

func TestThemeInterceptor_PutContent_WithThemeTag(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {
			TextColor:   0xFF0000,
			BackColor:   0x00FF00,
			StrikeColor: 0x0000FF,
		},
		"2": {
			TextColor:   0xAAAAAA,
			BackColor:   0xBBBBBB,
			StrikeColor: 0xCCCCCC,
		},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Content with different theme tags
	cells := []Cell{
		{Rune: 'A', Attrs: CellAttributes{ThemeTag: 1}},
		{Rune: 'B', Attrs: CellAttributes{ThemeTag: 2}},
		{Rune: 'C', Attrs: CellAttributes{ThemeTag: 1, TextColor: 0x123456}},
	}

	interceptor.PutContent(TRect{X: 0, Y: 0, W: 0, H: 0}, cells)

	// Verify content
	_, _, content := screen.GetContent()

	// First cell - theme 1
	assert.Equal(t, 'A', content[0].Rune, "ThemeInterceptor.PutContent() rune at theme.go")
	assert.Equal(t, uint32(0xFF0000), content[0].Attrs.TextColor, "ThemeInterceptor.PutContent() text color at theme.go")
	assert.Equal(t, uint32(0x00FF00), content[0].Attrs.BackColor, "ThemeInterceptor.PutContent() back color at theme.go")
	assert.Equal(t, uint32(0x0000FF), content[0].Attrs.StrikeColor, "ThemeInterceptor.PutContent() strike color at theme.go")

	// Second cell - theme 2
	assert.Equal(t, 'B', content[1].Rune, "ThemeInterceptor.PutContent() rune at theme.go")
	assert.Equal(t, uint32(0xAAAAAA), content[1].Attrs.TextColor, "ThemeInterceptor.PutContent() text color at theme.go")
	assert.Equal(t, uint32(0xBBBBBB), content[1].Attrs.BackColor, "ThemeInterceptor.PutContent() back color at theme.go")
	assert.Equal(t, uint32(0xCCCCCC), content[1].Attrs.StrikeColor, "ThemeInterceptor.PutContent() strike color at theme.go")

	// Third cell - theme 1 with explicit text color
	assert.Equal(t, 'C', content[2].Rune, "ThemeInterceptor.PutContent() rune at theme.go")
	assert.Equal(t, uint32(0x123456), content[2].Attrs.TextColor, "ThemeInterceptor.PutContent() text color at theme.go")
	assert.Equal(t, uint32(0x00FF00), content[2].Attrs.BackColor, "ThemeInterceptor.PutContent() back color at theme.go")
	assert.Equal(t, uint32(0x0000FF), content[2].Attrs.StrikeColor, "ThemeInterceptor.PutContent() strike color at theme.go")
}

func TestThemeInterceptor_PutContent_EmptyTheme(t *testing.T) {
	screen := newMockScreen(80, 24)
	interceptor := NewThemeInterceptor(screen, nil)

	// Content with theme tag but no theme
	cells := []Cell{
		{Rune: 'A', Attrs: CellAttributes{ThemeTag: 1}},
	}

	interceptor.PutContent(TRect{X: 0, Y: 0, W: 0, H: 0}, cells)

	// Verify content - should have original attributes
	_, _, content := screen.GetContent()
	assert.Equal(t, 'A', content[0].Rune, "ThemeInterceptor.PutContent() rune at theme.go")
	assert.Equal(t, cells[0].Attrs, content[0].Attrs, "ThemeInterceptor.PutContent() attrs at theme.go")
}

func TestThemeInterceptor_ApplyTheme_MultipleFields(t *testing.T) {
	tests := []struct {
		name     string
		theme    map[string]CellAttributes
		input    CellAttributes
		expected CellAttributes
	}{
		{
			name:  "zero theme tag - no change",
			theme: map[string]CellAttributes{"1": {TextColor: 0xFF0000}},
			input: CellAttributes{
				TextColor: 0x0000FF,
			},
			expected: CellAttributes{
				TextColor: 0x0000FF,
			},
		},
		{
			name: "theme tag with all zero fields",
			theme: map[string]CellAttributes{
				"1": {
					Attributes:  AttrBold | AttrItalic,
					TextColor:   0xFF0000,
					BackColor:   0x00FF00,
					StrikeColor: 0x0000FF,
				},
			},
			input: CellAttributes{
				ThemeTag: 1,
			},
			expected: CellAttributes{
				ThemeTag:    1,
				Attributes:  AttrBold | AttrItalic,
				TextColor:   0xFF0000,
				BackColor:   0x00FF00,
				StrikeColor: 0x0000FF,
			},
		},
		{
			name: "theme tag with some explicit fields",
			theme: map[string]CellAttributes{
				"1": {
					Attributes:  AttrBold,
					TextColor:   0xFF0000,
					BackColor:   0x00FF00,
					StrikeColor: 0x0000FF,
				},
			},
			input: CellAttributes{
				ThemeTag:   1,
				Attributes: AttrItalic,
				BackColor:  0xAAAAAA,
			},
			expected: CellAttributes{
				ThemeTag:    1,
				Attributes:  AttrItalic,
				TextColor:   0xFF0000,
				BackColor:   0xAAAAAA,
				StrikeColor: 0x0000FF,
			},
		},
		{
			name: "theme tag with all explicit fields",
			theme: map[string]CellAttributes{
				"1": {
					Attributes:  AttrBold,
					TextColor:   0xFF0000,
					BackColor:   0x00FF00,
					StrikeColor: 0x0000FF,
				},
			},
			input: CellAttributes{
				ThemeTag:    1,
				Attributes:  AttrItalic,
				TextColor:   0x111111,
				BackColor:   0x222222,
				StrikeColor: 0x333333,
			},
			expected: CellAttributes{
				ThemeTag:    1,
				Attributes:  AttrItalic,
				TextColor:   0x111111,
				BackColor:   0x222222,
				StrikeColor: 0x333333,
			},
		},
		{
			name:  "theme not found",
			theme: map[string]CellAttributes{"1": {TextColor: 0xFF0000}},
			input: CellAttributes{
				ThemeTag:  999,
				TextColor: 0x0000FF,
			},
			expected: CellAttributes{
				ThemeTag:  999,
				TextColor: 0x0000FF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := newMockScreen(80, 24)
			interceptor := NewThemeInterceptor(screen, tt.theme)

			result := interceptor.applyTheme(tt.input)

			assert.Equal(t, tt.expected, result, "ThemeInterceptor.applyTheme() at theme.go")
		})
	}
}

func TestThemeInterceptor_PutText_Clipping(t *testing.T) {
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {TextColor: 0xFF0000, BackColor: 0x00FF00},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Test with clipping rectangle
	attrs := CellAttributes{ThemeTag: 1}
	interceptor.PutText(TRect{X: 0, Y: 0, W: 3, H: 1}, "Hello", attrs)

	// Verify only first 3 characters are written
	_, _, content := screen.GetContent()
	assert.Equal(t, 'H', content[0].Rune, "ThemeInterceptor.PutText() clipping rune at theme.go")
	assert.Equal(t, 'e', content[1].Rune, "ThemeInterceptor.PutText() clipping rune at theme.go")
	assert.Equal(t, 'l', content[2].Rune, "ThemeInterceptor.PutText() clipping rune at theme.go")
	assert.Equal(t, ' ', content[3].Rune, "ThemeInterceptor.PutText() clipping rune at theme.go")
}

func TestThemeInterceptor_Integration(t *testing.T) {
	// Integration test with multiple operations
	screen := newMockScreen(80, 24)
	theme := map[string]CellAttributes{
		"1": {
			Attributes:  AttrBold,
			TextColor:   0xFF0000,
			BackColor:   0x000000,
			StrikeColor: 0xFFFFFF,
		},
		"2": {
			Attributes:  AttrItalic,
			TextColor:   0x00FF00,
			BackColor:   0x111111,
			StrikeColor: 0x222222,
		},
	}
	interceptor := NewThemeInterceptor(screen, theme)

	// Put text with theme 1
	interceptor.PutText(TRect{X: 0, Y: 0, W: 0, H: 0}, "Header", CellAttributes{ThemeTag: 1})

	// Put content with theme 2
	cells := []Cell{
		{Rune: 'B', Attrs: CellAttributes{ThemeTag: 2}},
		{Rune: 'o', Attrs: CellAttributes{ThemeTag: 2}},
		{Rune: 'd', Attrs: CellAttributes{ThemeTag: 2}},
		{Rune: 'y', Attrs: CellAttributes{ThemeTag: 2}},
	}
	interceptor.PutContent(TRect{X: 0, Y: 1, W: 0, H: 0}, cells)

	// Put text without theme
	interceptor.PutText(TRect{X: 0, Y: 2, W: 0, H: 0}, "Plain", CellAttributes{TextColor: 0xAAAAAA})

	// Verify all content
	_, _, content := screen.GetContent()

	// Line 0 - Header with theme 1
	for i := 0; i < 6; i++ {
		assert.Equal(t, AttrBold, content[i].Attrs.Attributes, "ThemeInterceptor integration attrs at theme.go")
		assert.Equal(t, uint32(0xFF0000), content[i].Attrs.TextColor, "ThemeInterceptor integration text color at theme.go")
		assert.Equal(t, uint32(0x000000), content[i].Attrs.BackColor, "ThemeInterceptor integration back color at theme.go")
	}

	// Line 1 - Body with theme 2
	for i := 0; i < 4; i++ {
		idx := 80 + i
		assert.Equal(t, AttrItalic, content[idx].Attrs.Attributes, "ThemeInterceptor integration attrs at theme.go")
		assert.Equal(t, uint32(0x00FF00), content[idx].Attrs.TextColor, "ThemeInterceptor integration text color at theme.go")
		assert.Equal(t, uint32(0x111111), content[idx].Attrs.BackColor, "ThemeInterceptor integration back color at theme.go")
	}

	// Line 2 - Plain without theme
	for i := 0; i < 5; i++ {
		idx := 160 + i
		assert.Equal(t, TextAttributes(0), content[idx].Attrs.Attributes, "ThemeInterceptor integration attrs at theme.go")
		assert.Equal(t, uint32(0xAAAAAA), content[idx].Attrs.TextColor, "ThemeInterceptor integration text color at theme.go")
		assert.Equal(t, uint32(0), content[idx].Attrs.BackColor, "ThemeInterceptor integration back color at theme.go")
	}
}
