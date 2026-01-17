package cswterm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockScreen(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{
			name:   "small screen",
			width:  10,
			height: 5,
		},
		{
			name:   "standard terminal size",
			width:  80,
			height: 24,
		},
		{
			name:   "single cell",
			width:  1,
			height: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewMockScreen(tt.width, tt.height)
			require.NotNil(t, screen)

			w, h := screen.Size()
			assert.Equal(t, tt.width, w)
			assert.Equal(t, tt.height, h)

			// Verify all cells are initialized with spaces
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					cell := screen.GetCell(x, y)
					assert.Equal(t, ' ', cell.Rune)
					assert.Equal(t, CellAttributes{}, cell.Attrs)
				}
			}
		})
	}
}

func TestMockScreen_Size(t *testing.T) {
	screen := NewMockScreen(100, 50)
	w, h := screen.Size()
	assert.Equal(t, 100, w)
	assert.Equal(t, 50, h)
}

func TestMockScreen_PutText(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		x         int
		y         int
		text      string
		attrs     TextAttributes
		wantText  string
		wantAttrs TextAttributes
		checkX    int
		checkY    int
	}{
		{
			name:      "simple text at origin",
			width:     20,
			height:    10,
			x:         0,
			y:         0,
			text:      "Hello",
			attrs:     AttrBold,
			wantText:  "Hello",
			wantAttrs: AttrBold,
			checkX:    0,
			checkY:    0,
		},
		{
			name:      "text with offset",
			width:     20,
			height:    10,
			x:         5,
			y:         3,
			text:      "World",
			attrs:     AttrItalic | AttrUnderline,
			wantText:  "World",
			wantAttrs: AttrItalic | AttrUnderline,
			checkX:    5,
			checkY:    3,
		},
		{
			name:      "text without attributes",
			width:     20,
			height:    10,
			x:         0,
			y:         0,
			text:      "Plain",
			attrs:     0,
			wantText:  "Plain",
			wantAttrs: 0,
			checkX:    0,
			checkY:    0,
		},
		{
			name:      "unicode text",
			width:     20,
			height:    10,
			x:         0,
			y:         0,
			text:      "Hello 世界",
			attrs:     AttrBold,
			wantText:  "Hello 世界",
			wantAttrs: AttrBold,
			checkX:    0,
			checkY:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewMockScreen(tt.width, tt.height)
			screen.PutText(tt.x, tt.y, tt.text, tt.attrs)

			// Verify each character
			runes := []rune(tt.wantText)
			for i, r := range runes {
				cell := screen.GetCell(tt.checkX+i, tt.checkY)
				assert.Equal(t, r, cell.Rune, "rune at position %d", i)
				assert.Equal(t, tt.wantAttrs, cell.Attrs.Attributes, "attributes at position %d", i)
			}
		})
	}
}

func TestMockScreen_PutText_Truncation(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		x           int
		y           int
		text        string
		expectedLen int
	}{
		{
			name:        "text truncated at right edge",
			width:       10,
			height:      5,
			x:           0,
			y:           0,
			text:        "VeryLongTextThatShouldBeTruncated",
			expectedLen: 10,
		},
		{
			name:        "text truncated when starting mid-line",
			width:       10,
			height:      5,
			x:           5,
			y:           0,
			text:        "LongText",
			expectedLen: 5, // Only 5 characters fit from x=5 to x=9
		},
		{
			name:        "exact fit",
			width:       10,
			height:      5,
			x:           0,
			y:           0,
			text:        "ExactlyTen",
			expectedLen: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewMockScreen(tt.width, tt.height)
			screen.PutText(tt.x, tt.y, tt.text, AttrBold)

			// Count non-space characters
			count := 0
			for x := tt.x; x < tt.width; x++ {
				cell := screen.GetCell(x, tt.y)
				if cell.Rune != ' ' {
					count++
				}
			}
			assert.Equal(t, tt.expectedLen, count)
		})
	}
}

func TestMockScreen_PutText_OutOfBounds(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		x      int
		y      int
		text   string
	}{
		{
			name:   "negative x",
			width:  10,
			height: 5,
			x:      -1,
			y:      0,
			text:   "Hello",
		},
		{
			name:   "negative y",
			width:  10,
			height: 5,
			x:      0,
			y:      -1,
			text:   "Hello",
		},
		{
			name:   "x beyond width",
			width:  10,
			height: 5,
			x:      10,
			y:      0,
			text:   "Hello",
		},
		{
			name:   "y beyond height",
			width:  10,
			height: 5,
			x:      0,
			y:      5,
			text:   "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewMockScreen(tt.width, tt.height)
			// Should not panic
			screen.PutText(tt.x, tt.y, tt.text, AttrBold)

			// Verify screen is still all spaces
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					cell := screen.GetCell(x, y)
					assert.Equal(t, ' ', cell.Rune)
				}
			}
		})
	}
}

func TestMockScreen_GetCell(t *testing.T) {
	screen := NewMockScreen(10, 5)
	screen.PutText(2, 1, "Test", AttrBold)

	tests := []struct {
		name      string
		x         int
		y         int
		wantRune  rune
		wantAttrs TextAttributes
	}{
		{
			name:      "first character",
			x:         2,
			y:         1,
			wantRune:  'T',
			wantAttrs: AttrBold,
		},
		{
			name:      "middle character",
			x:         3,
			y:         1,
			wantRune:  'e',
			wantAttrs: AttrBold,
		},
		{
			name:      "empty cell",
			x:         0,
			y:         0,
			wantRune:  ' ',
			wantAttrs: 0,
		},
		{
			name:      "out of bounds negative x",
			x:         -1,
			y:         0,
			wantRune:  ' ',
			wantAttrs: 0,
		},
		{
			name:      "out of bounds beyond width",
			x:         10,
			y:         0,
			wantRune:  ' ',
			wantAttrs: 0,
		},
		{
			name:      "out of bounds beyond height",
			x:         0,
			y:         5,
			wantRune:  ' ',
			wantAttrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := screen.GetCell(tt.x, tt.y)
			assert.Equal(t, tt.wantRune, cell.Rune)
			assert.Equal(t, tt.wantAttrs, cell.Attrs.Attributes)
		})
	}
}

func TestMockScreen_GetText(t *testing.T) {
	screen := NewMockScreen(20, 10)
	screen.PutText(0, 0, "Hello", 0)
	screen.PutText(0, 1, "World", 0)
	screen.PutText(5, 2, "Test", 0)

	tests := []struct {
		name   string
		x      int
		y      int
		width  int
		height int
		want   string
	}{
		{
			name:   "single line",
			x:      0,
			y:      0,
			width:  5,
			height: 1,
			want:   "Hello",
		},
		{
			name:   "multiple lines",
			x:      0,
			y:      0,
			width:  5,
			height: 2,
			want:   "Hello\nWorld",
		},
		{
			name:   "partial line",
			x:      1,
			y:      0,
			width:  3,
			height: 1,
			want:   "ell",
		},
		{
			name:   "text with spaces before",
			x:      3,
			y:      2,
			width:  6,
			height: 1,
			want:   "  Test",
		},
		{
			name:   "empty area",
			x:      0,
			y:      5,
			width:  5,
			height: 1,
			want:   "     ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := screen.GetText(tt.x, tt.y, tt.width, tt.height)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockScreen_HasText(t *testing.T) {
	screen := NewMockScreen(20, 10)
	screen.PutText(0, 0, "Hello", 0)
	screen.PutText(0, 1, "World", 0)
	screen.PutText(5, 2, "Test", 0)

	tests := []struct {
		name   string
		x      int
		y      int
		width  int
		height int
		text   string
		want   bool
	}{
		{
			name:   "exact match single line",
			x:      0,
			y:      0,
			width:  5,
			height: 1,
			text:   "Hello",
			want:   true,
		},
		{
			name:   "exact match multiple lines",
			x:      0,
			y:      0,
			width:  5,
			height: 2,
			text:   "Hello\nWorld",
			want:   true,
		},
		{
			name:   "mismatch",
			x:      0,
			y:      0,
			width:  5,
			height: 1,
			text:   "Goodbye",
			want:   false,
		},
		{
			name:   "partial match",
			x:      1,
			y:      0,
			width:  3,
			height: 1,
			text:   "ell",
			want:   true,
		},
		{
			name:   "text with spaces",
			x:      3,
			y:      2,
			width:  6,
			height: 1,
			text:   "  Test",
			want:   true,
		},
		{
			name:   "empty text check",
			x:      10,
			y:      5,
			width:  5,
			height: 1,
			text:   "     ",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := screen.HasText(tt.x, tt.y, tt.width, tt.height, tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockScreen_HasTextWithAttrs(t *testing.T) {
	screen := NewMockScreen(20, 10)
	screen.PutText(0, 0, "Bold", AttrBold)
	screen.PutText(0, 1, "Italic", AttrItalic)
	screen.PutText(0, 2, "Both", AttrBold|AttrItalic)

	tests := []struct {
		name   string
		x      int
		y      int
		width  int
		height int
		text   string
		mask   AttributeMask
		want   bool
	}{
		{
			name:   "check bold attribute",
			x:      0,
			y:      0,
			width:  4,
			height: 1,
			text:   "Bold",
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrBold,
			},
			want: true,
		},
		{
			name:   "check italic attribute",
			x:      0,
			y:      1,
			width:  6,
			height: 1,
			text:   "Italic",
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrItalic,
			},
			want: true,
		},
		{
			name:   "check combined attributes",
			x:      0,
			y:      2,
			width:  4,
			height: 1,
			text:   "Both",
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrBold | AttrItalic,
			},
			want: true,
		},
		{
			name:   "wrong attributes",
			x:      0,
			y:      0,
			width:  4,
			height: 1,
			text:   "Bold",
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrItalic,
			},
			want: false,
		},
		{
			name:   "no attribute check - only text",
			x:      0,
			y:      0,
			width:  4,
			height: 1,
			text:   "Bold",
			mask: AttributeMask{
				CheckAttributes: false,
			},
			want: true,
		},
		{
			name:   "wrong text but matching attributes",
			x:      0,
			y:      0,
			width:  4,
			height: 1,
			text:   "Test",
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrBold,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := screen.HasTextWithAttrs(tt.x, tt.y, tt.width, tt.height, tt.text, tt.mask)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockScreen_HasTextWithAttrs_Colors(t *testing.T) {
	screen := NewMockScreen(20, 10)

	// Create cells with specific colors manually
	redColor := uint32(0xFF0000)
	blueColor := uint32(0x0000FF)
	greenBack := uint32(0x00FF00)

	// Put text and then modify colors
	screen.PutText(0, 0, "Red", 0)
	for i := 0; i < 3; i++ {
		screen.buffer[0][i].Attrs.TextColor = redColor
	}

	screen.PutText(0, 1, "Blue", 0)
	for i := 0; i < 4; i++ {
		screen.buffer[1][i].Attrs.TextColor = blueColor
		screen.buffer[1][i].Attrs.BackColor = greenBack
	}

	tests := []struct {
		name   string
		x      int
		y      int
		width  int
		height int
		text   string
		mask   AttributeMask
		want   bool
	}{
		{
			name:   "check text color",
			x:      0,
			y:      0,
			width:  3,
			height: 1,
			text:   "Red",
			mask: AttributeMask{
				CheckTextColor: true,
				TextColor:      redColor,
			},
			want: true,
		},
		{
			name:   "check text and back color",
			x:      0,
			y:      1,
			width:  4,
			height: 1,
			text:   "Blue",
			mask: AttributeMask{
				CheckTextColor: true,
				CheckBackColor: true,
				TextColor:      blueColor,
				BackColor:      greenBack,
			},
			want: true,
		},
		{
			name:   "wrong text color",
			x:      0,
			y:      0,
			width:  3,
			height: 1,
			text:   "Red",
			mask: AttributeMask{
				CheckTextColor: true,
				TextColor:      blueColor,
			},
			want: false,
		},
		{
			name:   "check only back color",
			x:      0,
			y:      1,
			width:  4,
			height: 1,
			text:   "Blue",
			mask: AttributeMask{
				CheckBackColor: true,
				BackColor:      greenBack,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := screen.HasTextWithAttrs(tt.x, tt.y, tt.width, tt.height, tt.text, tt.mask)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockScreen_Clear(t *testing.T) {
	screen := NewMockScreen(10, 5)
	screen.PutText(0, 0, "Hello", AttrBold)
	screen.PutText(0, 1, "World", AttrItalic)

	// Verify text is present
	assert.True(t, screen.HasText(0, 0, 5, 1, "Hello"))
	assert.True(t, screen.HasText(0, 1, 5, 1, "World"))

	screen.Clear()

	// Verify all cells are spaces with no attributes
	for y := 0; y < 5; y++ {
		for x := 0; x < 10; x++ {
			cell := screen.GetCell(x, y)
			assert.Equal(t, ' ', cell.Rune)
			assert.Equal(t, CellAttributes{}, cell.Attrs)
		}
	}
}

func TestMockScreen_InterfaceCompliance(t *testing.T) {
	var _ Screen = (*MockScreen)(nil)
}

func TestAttributeMask_Partial(t *testing.T) {
	screen := NewMockScreen(20, 10)

	// Create text with bold and italic
	screen.PutText(0, 0, "Text", AttrBold|AttrItalic|AttrUnderline)
	for i := 0; i < 4; i++ {
		screen.buffer[0][i].Attrs.TextColor = 0xFF0000
	}

	tests := []struct {
		name string
		mask AttributeMask
		want bool
	}{
		{
			name: "check only bold (ignore italic and underline)",
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrBold | AttrItalic | AttrUnderline,
			},
			want: true,
		},
		{
			name: "check only text color (ignore attributes)",
			mask: AttributeMask{
				CheckTextColor: true,
				TextColor:      0xFF0000,
			},
			want: true,
		},
		{
			name: "check both color and attributes",
			mask: AttributeMask{
				CheckAttributes: true,
				CheckTextColor:  true,
				Attributes:      AttrBold | AttrItalic | AttrUnderline,
				TextColor:       0xFF0000,
			},
			want: true,
		},
		{
			name: "check nothing - always matches",
			mask: AttributeMask{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := screen.HasTextWithAttrs(0, 0, 4, 1, "Text", tt.mask)
			assert.Equal(t, tt.want, got)
		})
	}
}
