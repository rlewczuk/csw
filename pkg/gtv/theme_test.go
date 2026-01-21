package gtv

import (
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// Test ThemeManager

func TestNewThemeManager_EmptyList(t *testing.T) {
	tm, err := NewThemeManager()

	require.NoError(t, err, "NewThemeManager() should not error with empty list at theme.go")
	assert.NotNil(t, tm, "NewThemeManager() should return non-nil manager at theme.go")
	assert.Empty(t, tm.ListThemes(), "NewThemeManager() should have no themes at theme.go")
}

func TestNewThemeManager_SingleFS(t *testing.T) {
	// Create a test filesystem with one theme
	testFS := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)

	require.NoError(t, err, "NewThemeManager() at theme.go")
	assert.NotNil(t, tm, "NewThemeManager() at theme.go")

	themes := tm.ListThemes()
	assert.Len(t, themes, 1, "NewThemeManager() should load 1 theme at theme.go")
	assert.Equal(t, "dark", themes[0].Name, "NewThemeManager() theme name at theme.go")
	assert.Equal(t, "Dark theme", themes[0].Description, "NewThemeManager() theme description at theme.go")
}

func TestNewThemeManager_MultipleFS(t *testing.T) {
	// Create two test filesystems with different themes
	fs1 := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
	}

	fs2 := fstest.MapFS{
		"light.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "light",
				"description": "Light theme",
				"theme": {
					"1": {"text-color": 0, "back-color": 16777215}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(fs1, fs2)

	require.NoError(t, err, "NewThemeManager() at theme.go")

	themes := tm.ListThemes()
	assert.Len(t, themes, 2, "NewThemeManager() should load 2 themes at theme.go")

	// Check that both themes are present
	themeNames := make(map[string]bool)
	for _, theme := range themes {
		themeNames[theme.Name] = true
	}
	assert.True(t, themeNames["dark"], "NewThemeManager() should have dark theme at theme.go")
	assert.True(t, themeNames["light"], "NewThemeManager() should have light theme at theme.go")
}

func TestNewThemeManager_NestedDirectories(t *testing.T) {
	// Create a test filesystem with nested directories
	testFS := fstest.MapFS{
		"themes/dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
		"themes/subdir/light.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "light",
				"description": "Light theme",
				"theme": {
					"1": {"text-color": 0, "back-color": 16777215}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)

	require.NoError(t, err, "NewThemeManager() at theme.go")

	themes := tm.ListThemes()
	assert.Len(t, themes, 2, "NewThemeManager() should load themes from nested directories at theme.go")
}

func TestNewThemeManager_InvalidJSON(t *testing.T) {
	// Create a test filesystem with invalid JSON
	testFS := fstest.MapFS{
		"invalid.theme.json": &fstest.MapFile{
			Data: []byte(`{"name": "invalid", "description": "`),
		},
	}

	tm, err := NewThemeManager(testFS)

	// Should not error, just skip invalid files
	require.NoError(t, err, "NewThemeManager() should not error on invalid JSON at theme.go")
	assert.Empty(t, tm.ListThemes(), "NewThemeManager() should skip invalid JSON at theme.go")
}

func TestNewThemeManager_MismatchedFileName(t *testing.T) {
	// Create a test filesystem with mismatched file name
	testFS := fstest.MapFS{
		"wrong.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "correct",
				"description": "Theme with wrong file name",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)

	// Should not error, just skip files with mismatched names
	require.NoError(t, err, "NewThemeManager() should not error on mismatched file name at theme.go")
	assert.Empty(t, tm.ListThemes(), "NewThemeManager() should skip mismatched file names at theme.go")
}

func TestNewThemeManager_NonThemeFiles(t *testing.T) {
	// Create a test filesystem with non-theme files
	testFS := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
		"readme.md": &fstest.MapFile{
			Data: []byte(`# Themes`),
		},
		"config.json": &fstest.MapFile{
			Data: []byte(`{"setting": "value"}`),
		},
	}

	tm, err := NewThemeManager(testFS)

	require.NoError(t, err, "NewThemeManager() at theme.go")

	themes := tm.ListThemes()
	assert.Len(t, themes, 1, "NewThemeManager() should only load .theme.json files at theme.go")
	assert.Equal(t, "dark", themes[0].Name, "NewThemeManager() theme name at theme.go")
}

func TestThemeManager_GetTheme(t *testing.T) {
	testFS := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0, "attributes": 1},
					"2": {"text-color": 255, "back-color": 0}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	theme, err := tm.GetTheme("dark")

	require.NoError(t, err, "ThemeManager.GetTheme() at theme.go")
	assert.Len(t, theme, 2, "ThemeManager.GetTheme() should return correct theme at theme.go")

	assert.Equal(t, uint32(16777215), theme["1"].TextColor, "ThemeManager.GetTheme() text color at theme.go")
	assert.Equal(t, uint32(0), theme["1"].BackColor, "ThemeManager.GetTheme() back color at theme.go")
	assert.Equal(t, TextAttributes(1), theme["1"].Attributes, "ThemeManager.GetTheme() attributes at theme.go")

	assert.Equal(t, uint32(255), theme["2"].TextColor, "ThemeManager.GetTheme() text color at theme.go")
	assert.Equal(t, uint32(0), theme["2"].BackColor, "ThemeManager.GetTheme() back color at theme.go")
}

func TestThemeManager_GetTheme_NotFound(t *testing.T) {
	testFS := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	_, err = tm.GetTheme("nonexistent")

	assert.Error(t, err, "ThemeManager.GetTheme() should error on non-existent theme at theme.go")
	assert.Contains(t, err.Error(), "theme not found", "ThemeManager.GetTheme() error message at theme.go")
}

func TestThemeManager_ListThemes(t *testing.T) {
	testFS := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme for night coding",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
		"light.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "light",
				"description": "Light theme for day coding",
				"theme": {
					"1": {"text-color": 0, "back-color": 16777215}
				}
			}`),
		},
		"solarized.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "solarized",
				"description": "Solarized color scheme",
				"theme": {
					"1": {"text-color": 5723991, "back-color": 15"}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	themes := tm.ListThemes()

	// Should skip solarized due to invalid JSON
	assert.Len(t, themes, 2, "ThemeManager.ListThemes() at theme.go")

	// Build a map for easier testing
	themeMap := make(map[string]string)
	for _, theme := range themes {
		themeMap[theme.Name] = theme.Description
	}

	assert.Equal(t, "Dark theme for night coding", themeMap["dark"], "ThemeManager.ListThemes() dark description at theme.go")
	assert.Equal(t, "Light theme for day coding", themeMap["light"], "ThemeManager.ListThemes() light description at theme.go")
}

func TestThemeManager_Reload(t *testing.T) {
	testFS := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	// Verify initial state
	themes := tm.ListThemes()
	assert.Len(t, themes, 1, "ThemeManager initial state at theme.go")

	// Reload
	err = tm.Reload()
	require.NoError(t, err, "ThemeManager.Reload() at theme.go")

	// Verify state after reload
	themes = tm.ListThemes()
	assert.Len(t, themes, 1, "ThemeManager state after reload at theme.go")
	assert.Equal(t, "dark", themes[0].Name, "ThemeManager theme name after reload at theme.go")
}

func TestThemeManager_Reload_ClearsOldThemes(t *testing.T) {
	// Create initial filesystem with one theme
	testFS := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	// Manually add another theme to simulate old state
	tm.themes["old"] = ThemeFile{
		Name:        "old",
		Description: "Old theme",
		Theme:       map[string]CellAttributes{},
	}

	// Verify we have 2 themes
	assert.Len(t, tm.ListThemes(), 2, "ThemeManager before reload at theme.go")

	// Reload should clear old themes
	err = tm.Reload()
	require.NoError(t, err, "ThemeManager.Reload() at theme.go")

	// Should only have themes from filesystem
	themes := tm.ListThemes()
	assert.Len(t, themes, 1, "ThemeManager after reload should clear old themes at theme.go")
	assert.Equal(t, "dark", themes[0].Name, "ThemeManager theme name after reload at theme.go")
}

func TestThemeManager_ComplexTheme(t *testing.T) {
	testFS := fstest.MapFS{
		"complex.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "complex",
				"description": "Complex theme with all attributes",
				"theme": {
					"header": {
						"attributes": 3,
						"text-color": 16777215,
						"back-color": 255,
						"strike-color": 65280
					},
					"body": {
						"attributes": 0,
						"text-color": 13421772,
						"back-color": 0,
						"strike-color": 0
					},
					"footer": {
						"attributes": 8,
						"text-color": 8421504,
						"back-color": 2236962,
						"strike-color": 0
					}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	theme, err := tm.GetTheme("complex")
	require.NoError(t, err, "ThemeManager.GetTheme() at theme.go")

	// Check header
	assert.Equal(t, TextAttributes(3), theme["header"].Attributes, "ThemeManager complex theme header attributes at theme.go")
	assert.Equal(t, uint32(16777215), theme["header"].TextColor, "ThemeManager complex theme header text color at theme.go")
	assert.Equal(t, uint32(255), theme["header"].BackColor, "ThemeManager complex theme header back color at theme.go")
	assert.Equal(t, uint32(65280), theme["header"].StrikeColor, "ThemeManager complex theme header strike color at theme.go")

	// Check body
	assert.Equal(t, TextAttributes(0), theme["body"].Attributes, "ThemeManager complex theme body attributes at theme.go")
	assert.Equal(t, uint32(13421772), theme["body"].TextColor, "ThemeManager complex theme body text color at theme.go")

	// Check footer
	assert.Equal(t, TextAttributes(8), theme["footer"].Attributes, "ThemeManager complex theme footer attributes at theme.go")
	assert.Equal(t, uint32(8421504), theme["footer"].TextColor, "ThemeManager complex theme footer text color at theme.go")
}

func TestThemeManager_EmptyTheme(t *testing.T) {
	testFS := fstest.MapFS{
		"empty.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "empty",
				"description": "Empty theme with no tags",
				"theme": {}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	theme, err := tm.GetTheme("empty")
	require.NoError(t, err, "ThemeManager.GetTheme() at theme.go")
	assert.Empty(t, theme, "ThemeManager empty theme at theme.go")
}

func TestThemeManager_Integration(t *testing.T) {
	// Create a realistic filesystem with multiple themes
	testFS := fstest.MapFS{
		"themes/dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {
					"1": {"text-color": 16777215, "back-color": 0}
				}
			}`),
		},
		"themes/light.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "light",
				"description": "Light theme",
				"theme": {
					"1": {"text-color": 0, "back-color": 16777215}
				}
			}`),
		},
	}

	// Create ThemeManager
	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	// List themes
	themes := tm.ListThemes()
	assert.Len(t, themes, 2, "ThemeManager integration list themes at theme.go")

	// Get dark theme
	darkTheme, err := tm.GetTheme("dark")
	require.NoError(t, err, "ThemeManager.GetTheme(dark) at theme.go")
	assert.Equal(t, uint32(16777215), darkTheme["1"].TextColor, "ThemeManager dark theme text color at theme.go")

	// Get light theme
	lightTheme, err := tm.GetTheme("light")
	require.NoError(t, err, "ThemeManager.GetTheme(light) at theme.go")
	assert.Equal(t, uint32(16777215), lightTheme["1"].BackColor, "ThemeManager light theme back color at theme.go")

	// Create ThemeInterceptor with dark theme
	screen := newMockScreen(80, 24)
	interceptor := NewThemeInterceptor(screen, darkTheme)

	// Write text with theme tag
	interceptor.PutText(TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", CellAttributes{ThemeTag: 1})

	// Verify text is rendered with theme colors
	_, _, content := screen.GetContent()
	assert.Equal(t, uint32(16777215), content[0].Attrs.TextColor, "ThemeManager integration text color at theme.go")
	assert.Equal(t, uint32(0), content[0].Attrs.BackColor, "ThemeManager integration back color at theme.go")
}

func TestThemeManager_MultipleFilesystems(t *testing.T) {
	// Test that themes from multiple filesystems are combined
	fs1 := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Dark theme",
				"theme": {"1": {"text-color": 16777215, "back-color": 0}}
			}`),
		},
	}

	fs2 := fstest.MapFS{
		"light.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "light",
				"description": "Light theme",
				"theme": {"1": {"text-color": 0, "back-color": 16777215}}
			}`),
		},
	}

	fs3 := fstest.MapFS{
		"solarized.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "solarized",
				"description": "Solarized theme",
				"theme": {"1": {"text-color": 5723991, "back-color": 15"}
			}`),
		},
	}

	tm, err := NewThemeManager(fs1, fs2, fs3)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	themes := tm.ListThemes()
	// Should skip solarized due to invalid JSON
	assert.Len(t, themes, 2, "ThemeManager multiple filesystems at theme.go")

	// Verify both themes are accessible
	_, err = tm.GetTheme("dark")
	assert.NoError(t, err, "ThemeManager.GetTheme(dark) at theme.go")

	_, err = tm.GetTheme("light")
	assert.NoError(t, err, "ThemeManager.GetTheme(light) at theme.go")
}

func TestThemeManager_OverridingThemes(t *testing.T) {
	// Test that later filesystems override earlier ones for same theme name
	fs1 := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "First dark theme",
				"theme": {"1": {"text-color": 16777215, "back-color": 0}}
			}`),
		},
	}

	fs2 := fstest.MapFS{
		"dark.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "dark",
				"description": "Second dark theme",
				"theme": {"1": {"text-color": 13421772, "back-color": 2236962}}
			}`),
		},
	}

	tm, err := NewThemeManager(fs1, fs2)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	// Should only have one dark theme
	themes := tm.ListThemes()
	assert.Len(t, themes, 1, "ThemeManager overriding themes count at theme.go")

	// Get the theme and check it's the second one
	theme, err := tm.GetTheme("dark")
	require.NoError(t, err, "ThemeManager.GetTheme(dark) at theme.go")

	// The second filesystem should override the first
	assert.Equal(t, uint32(13421772), theme["1"].TextColor, "ThemeManager overriding theme text color at theme.go")
	assert.Equal(t, uint32(2236962), theme["1"].BackColor, "ThemeManager overriding theme back color at theme.go")

	// Check description is from second theme
	assert.Equal(t, "Second dark theme", themes[0].Description, "ThemeManager overriding theme description at theme.go")
}

func TestThemeManager_NonReadableFS(t *testing.T) {
	// Create a filesystem that will cause errors
	errorFS := &errorFS{}

	tm, err := NewThemeManager(errorFS)

	// Should not error even with a problematic filesystem
	require.NoError(t, err, "NewThemeManager() with error FS at theme.go")
	assert.Empty(t, tm.ListThemes(), "ThemeManager with error FS should have no themes at theme.go")
}

// errorFS is a test filesystem that returns errors
type errorFS struct{}

func (e *errorFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (e *errorFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return nil, fs.ErrInvalid
}

// Implement fs.ReadDirFS interface
type readDirFS interface {
	fs.FS
	ReadDir(name string) ([]fs.DirEntry, error)
}

func TestCellAttributes_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		attrs    CellAttributes
		expected string
	}{
		{
			name: "all fields set",
			attrs: CellAttributes{
				Attributes:  AttrBold | AttrItalic,
				TextColor:   16777215,
				BackColor:   255,
				StrikeColor: 65280,
				ThemeTag:    1,
			},
			expected: `{"attributes":5,"text-color":16777215,"back-color":255,"strike-color":65280,"theme-tag":1}`,
		},
		{
			name: "only text color",
			attrs: CellAttributes{
				TextColor: 16777215,
			},
			expected: `{"text-color":16777215}`,
		},
		{
			name: "only attributes",
			attrs: CellAttributes{
				Attributes: AttrBold,
			},
			expected: `{"attributes":1}`,
		},
		{
			name:     "zero values",
			attrs:    CellAttributes{},
			expected: `{}`,
		},
		{
			name: "partial fields",
			attrs: CellAttributes{
				Attributes: AttrUnderline,
				TextColor:  13421772,
			},
			expected: `{"attributes":8,"text-color":13421772}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.attrs)
			require.NoError(t, err, "json.Marshal() at screen.go")
			assert.JSONEq(t, tt.expected, string(data), "json.Marshal() output at screen.go")

			// Test unmarshaling
			var unmarshaled CellAttributes
			err = json.Unmarshal([]byte(tt.expected), &unmarshaled)
			require.NoError(t, err, "json.Unmarshal() at screen.go")
			assert.Equal(t, tt.attrs, unmarshaled, "json.Unmarshal() result at screen.go")
		})
	}
}

func TestThemeFile_JSONMarshaling(t *testing.T) {
	themeFile := ThemeFile{
		Name:        "test-theme",
		Description: "Test theme for JSON marshaling",
		Theme: map[string]CellAttributes{
			"header": {
				Attributes:  AttrBold,
				TextColor:   16777215,
				BackColor:   255,
				StrikeColor: 0,
			},
			"body": {
				TextColor: 13421772,
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(themeFile)
	require.NoError(t, err, "json.Marshal(ThemeFile) at theme.go")

	// Unmarshal back
	var unmarshaled ThemeFile
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err, "json.Unmarshal(ThemeFile) at theme.go")

	// Verify
	assert.Equal(t, themeFile.Name, unmarshaled.Name, "ThemeFile.Name at theme.go")
	assert.Equal(t, themeFile.Description, unmarshaled.Description, "ThemeFile.Description at theme.go")
	assert.Len(t, unmarshaled.Theme, 2, "ThemeFile.Theme length at theme.go")

	// Check header
	header := unmarshaled.Theme["header"]
	assert.Equal(t, AttrBold, header.Attributes, "ThemeFile header Attributes at theme.go")
	assert.Equal(t, uint32(16777215), header.TextColor, "ThemeFile header TextColor at theme.go")
	assert.Equal(t, uint32(255), header.BackColor, "ThemeFile header BackColor at theme.go")

	// Check body
	body := unmarshaled.Theme["body"]
	assert.Equal(t, uint32(13421772), body.TextColor, "ThemeFile body TextColor at theme.go")
	assert.Equal(t, uint32(0), body.BackColor, "ThemeFile body BackColor at theme.go")
}

func TestCellAttributes_HexColorSupport(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected CellAttributes
		wantErr  bool
	}{
		{
			name: "hex with # prefix",
			json: `{"text-color": "#FFFFFF", "back-color": "#FF0000"}`,
			expected: CellAttributes{
				TextColor: 0xFFFFFF,
				BackColor: 0xFF0000,
			},
			wantErr: false,
		},
		{
			name: "hex with 0x prefix",
			json: `{"text-color": "0xFFFFFF", "back-color": "0xFF0000"}`,
			expected: CellAttributes{
				TextColor: 0xFFFFFF,
				BackColor: 0xFF0000,
			},
			wantErr: false,
		},
		{
			name: "lowercase hex",
			json: `{"text-color": "#ffffff", "back-color": "#ff0000"}`,
			expected: CellAttributes{
				TextColor: 0xFFFFFF,
				BackColor: 0xFF0000,
			},
			wantErr: false,
		},
		{
			name: "mixed hex and decimal",
			json: `{"text-color": "#FFFFFF", "back-color": 255}`,
			expected: CellAttributes{
				TextColor: 0xFFFFFF,
				BackColor: 255,
			},
			wantErr: false,
		},
		{
			name: "all colors as hex",
			json: `{"text-color": "#FFFFFF", "back-color": "#000000", "strike-color": "#FF00FF"}`,
			expected: CellAttributes{
				TextColor:   0xFFFFFF,
				BackColor:   0x000000,
				StrikeColor: 0xFF00FF,
			},
			wantErr: false,
		},
		{
			name: "hex with attributes",
			json: `{"attributes": 1, "text-color": "#FFFFFF"}`,
			expected: CellAttributes{
				Attributes: AttrBold,
				TextColor:  0xFFFFFF,
			},
			wantErr: false,
		},
		{
			name: "short hex colors",
			json: `{"text-color": "#FF", "back-color": "#00FF00"}`,
			expected: CellAttributes{
				TextColor: 0xFF,
				BackColor: 0x00FF00,
			},
			wantErr: false,
		},
		{
			name:    "invalid hex color",
			json:    `{"text-color": "#GGGGGG"}`,
			wantErr: true,
		},
		{
			name:    "invalid hex format",
			json:    `{"text-color": "#"}`,
			wantErr: true,
		},
		{
			name: "hex without prefix",
			json: `{"text-color": "FFFFFF"}`,
			expected: CellAttributes{
				TextColor: 0xFFFFFF,
			},
			wantErr: false,
		},
		{
			name: "uppercase 0X prefix",
			json: `{"text-color": "0XFFFFFF"}`,
			expected: CellAttributes{
				TextColor: 0xFFFFFF,
			},
			wantErr: false,
		},
		{
			name: "common colors",
			json: `{"text-color": "#FF0000", "back-color": "#00FF00", "strike-color": "#0000FF"}`,
			expected: CellAttributes{
				TextColor:   0xFF0000,
				BackColor:   0x00FF00,
				StrikeColor: 0x0000FF,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attrs CellAttributes
			err := json.Unmarshal([]byte(tt.json), &attrs)

			if tt.wantErr {
				assert.Error(t, err, "UnmarshalJSON should error at screen.go")
				return
			}

			require.NoError(t, err, "UnmarshalJSON at screen.go")
			assert.Equal(t, tt.expected, attrs, "UnmarshalJSON result at screen.go")
		})
	}
}

func TestThemeManager_HexColorTheme(t *testing.T) {
	// Test that ThemeManager works with hex colors in theme files
	testFS := fstest.MapFS{
		"hexcolors.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "hexcolors",
				"description": "Theme using hex color notation",
				"theme": {
					"header": {
						"text-color": "#FFFFFF",
						"back-color": "#0000FF",
						"attributes": 1
					},
					"body": {
						"text-color": "#CCCCCC",
						"back-color": "#000000"
					},
					"error": {
						"text-color": "#FF0000",
						"attributes": 1
					},
					"success": {
						"text-color": "#00FF00"
					},
					"warning": {
						"text-color": "0xFFFF00"
					}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	theme, err := tm.GetTheme("hexcolors")
	require.NoError(t, err, "ThemeManager.GetTheme() at theme.go")

	// Verify header
	assert.Equal(t, uint32(0xFFFFFF), theme["header"].TextColor, "header TextColor at theme.go")
	assert.Equal(t, uint32(0x0000FF), theme["header"].BackColor, "header BackColor at theme.go")
	assert.Equal(t, AttrBold, theme["header"].Attributes, "header Attributes at theme.go")

	// Verify body
	assert.Equal(t, uint32(0xCCCCCC), theme["body"].TextColor, "body TextColor at theme.go")
	assert.Equal(t, uint32(0x000000), theme["body"].BackColor, "body BackColor at theme.go")

	// Verify error
	assert.Equal(t, uint32(0xFF0000), theme["error"].TextColor, "error TextColor at theme.go")
	assert.Equal(t, AttrBold, theme["error"].Attributes, "error Attributes at theme.go")

	// Verify success
	assert.Equal(t, uint32(0x00FF00), theme["success"].TextColor, "success TextColor at theme.go")

	// Verify warning (0x prefix)
	assert.Equal(t, uint32(0xFFFF00), theme["warning"].TextColor, "warning TextColor at theme.go")
}

func TestThemeManager_MixedColorFormats(t *testing.T) {
	// Test theme with mixed decimal and hex colors
	testFS := fstest.MapFS{
		"mixed.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "mixed",
				"description": "Theme with mixed color formats",
				"theme": {
					"tag1": {
						"text-color": "#FFFFFF",
						"back-color": 255
					},
					"tag2": {
						"text-color": 16777215,
						"back-color": "#0000FF"
					}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	theme, err := tm.GetTheme("mixed")
	require.NoError(t, err, "ThemeManager.GetTheme() at theme.go")

	// tag1: hex text, decimal back
	assert.Equal(t, uint32(0xFFFFFF), theme["tag1"].TextColor, "tag1 TextColor at theme.go")
	assert.Equal(t, uint32(255), theme["tag1"].BackColor, "tag1 BackColor at theme.go")

	// tag2: decimal text, hex back
	assert.Equal(t, uint32(16777215), theme["tag2"].TextColor, "tag2 TextColor at theme.go")
	assert.Equal(t, uint32(0x0000FF), theme["tag2"].BackColor, "tag2 BackColor at theme.go")
}

func TestThemeManager_JSONCompatibility(t *testing.T) {
	// Test that themes with omitempty work correctly
	testFS := fstest.MapFS{
		"minimal.theme.json": &fstest.MapFile{
			Data: []byte(`{
				"name": "minimal",
				"description": "Minimal theme with sparse attributes",
				"theme": {
					"tag1": {
						"text-color": 16777215
					},
					"tag2": {
						"back-color": 255
					},
					"tag3": {
						"attributes": 1,
						"text-color": 13421772
					}
				}
			}`),
		},
	}

	tm, err := NewThemeManager(testFS)
	require.NoError(t, err, "NewThemeManager() at theme.go")

	theme, err := tm.GetTheme("minimal")
	require.NoError(t, err, "ThemeManager.GetTheme() at theme.go")

	// Verify tag1 - only TextColor set
	assert.Equal(t, uint32(16777215), theme["tag1"].TextColor, "tag1 TextColor at theme.go")
	assert.Equal(t, uint32(0), theme["tag1"].BackColor, "tag1 BackColor should be zero at theme.go")
	assert.Equal(t, TextAttributes(0), theme["tag1"].Attributes, "tag1 Attributes should be zero at theme.go")

	// Verify tag2 - only BackColor set
	assert.Equal(t, uint32(0), theme["tag2"].TextColor, "tag2 TextColor should be zero at theme.go")
	assert.Equal(t, uint32(255), theme["tag2"].BackColor, "tag2 BackColor at theme.go")

	// Verify tag3 - Attributes and TextColor set
	assert.Equal(t, TextAttributes(1), theme["tag3"].Attributes, "tag3 Attributes at theme.go")
	assert.Equal(t, uint32(13421772), theme["tag3"].TextColor, "tag3 TextColor at theme.go")
}
