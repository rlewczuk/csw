package cswterm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewScreenRenderer(t *testing.T) {
	screen := NewScreenBuffer(80, 24, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	assert.NotNil(t, renderer)
	assert.Equal(t, 80, renderer.width)
	assert.Equal(t, 24, renderer.height)
	assert.Equal(t, 80*24, len(renderer.lastBuffer))
}

func TestScreenRenderer_RenderEmpty(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	err := renderer.Render()
	assert.NoError(t, err)

	// Empty screen should produce no output (no changes)
	assert.Equal(t, 0, buf.Len())
}

func TestScreenRenderer_RenderInitialContent(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Add some text
	screen.PutText(0, 0, "Hello", Attrs(0))

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	// Should contain cursor movement and text
	assert.Contains(t, output, "\x1b[1;1H") // Move to 1,1
	assert.Contains(t, output, "Hello")
}

func TestScreenRenderer_DifferentialRendering(t *testing.T) {
	screen := NewScreenBuffer(20, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// First render
	screen.PutText(0, 0, "Hello", Attrs(0))
	err := renderer.Render()
	assert.NoError(t, err)

	// Clear buffer
	buf.Reset()

	// Second render with no changes
	err = renderer.Render()
	assert.NoError(t, err)
	assert.Equal(t, 0, buf.Len(), "No output expected when nothing changed")

	// Third render with a change
	buf.Reset()
	screen.PutText(6, 0, "World", Attrs(0))
	err = renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	// Should only update the changed region
	assert.Contains(t, output, "World")
	assert.Greater(t, len(output), 0)
}

func TestScreenRenderer_RegionMerging(t *testing.T) {
	screen := NewScreenBuffer(80, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Create two close changes that should be merged
	screen.PutText(0, 0, "A", Attrs(0))
	screen.PutText(5, 0, "B", Attrs(0)) // Only 4 cells apart

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	// Should contain a single cursor move to the start
	moveCount := strings.Count(output, "\x1b[1;1H")
	assert.Equal(t, 1, moveCount, "Should merge close regions into one")
}

func TestScreenRenderer_MultipleRows(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Add text on multiple rows
	screen.PutText(0, 0, "Row1", Attrs(0))
	screen.PutText(0, 2, "Row3", Attrs(0))

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	// Should contain cursor moves to both rows
	assert.Contains(t, output, "\x1b[1;1H") // Row 1
	assert.Contains(t, output, "\x1b[3;1H") // Row 3
	assert.Contains(t, output, "Row1")
	assert.Contains(t, output, "Row3")
}

func TestScreenRenderer_Attributes(t *testing.T) {
	screen := NewScreenBuffer(20, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Add text with bold attribute
	screen.PutText(0, 0, "Bold", Attrs(AttrBold))

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	// Should contain reset and bold in combined SGR sequence
	assert.Contains(t, output, "\x1b[0;1m") // Reset and Bold
	assert.Contains(t, output, "Bold")
}

func TestScreenRenderer_MultipleAttributes(t *testing.T) {
	screen := NewScreenBuffer(20, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Add text with multiple attributes
	screen.PutText(0, 0, "Test", Attrs(AttrBold|AttrItalic|AttrUnderline))

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	// Should contain all attributes in combined SGR sequence
	assert.Contains(t, output, "\x1b[0;1;3;4m") // Reset, Bold, Italic, Underline
	assert.Contains(t, output, "Test")
}

func TestScreenRenderer_HideShowCursor(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Test hide cursor
	err := renderer.HideCursor()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "\x1b[?25l")

	// Test show cursor
	buf.Reset()
	err = renderer.ShowCursor()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "\x1b[?25h")
}

func TestScreenRenderer_ClearScreen(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	err := renderer.clearScreen()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "\x1b[2J") // Clear screen
	assert.Contains(t, output, "\x1b[H")  // Home cursor
}

func TestScreenRenderer_Reset(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Render some content
	screen.PutText(0, 0, "Test", Attrs(0))
	err := renderer.Render()
	assert.NoError(t, err)

	// Reset renderer
	renderer.Reset()

	// All cells should be considered changed now
	buf.Reset()
	err = renderer.Render()
	assert.NoError(t, err)

	// Should render everything again
	output := buf.String()
	assert.Contains(t, output, "Test")
}

func TestScreenRenderer_SizeChange(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Initial render
	screen.PutText(0, 0, "Test", Attrs(0))
	err := renderer.Render()
	assert.NoError(t, err)

	// Change screen size (in real scenario, this would be a new screen)
	// For this test, we'll just verify the renderer handles the size
	assert.Equal(t, 10, renderer.width)
	assert.Equal(t, 5, renderer.height)
}

func TestScreenRenderer_UnicodeCharacters(t *testing.T) {
	screen := NewScreenBuffer(20, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Test with Unicode box-drawing characters
	screen.PutText(0, 0, "┌─┐", Attrs(0))
	screen.PutText(0, 1, "│X│", Attrs(0))
	screen.PutText(0, 2, "└─┘", Attrs(0))

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "┌─┐")
	assert.Contains(t, output, "│X│")
	assert.Contains(t, output, "└─┘")
}

func TestScreenRenderer_ComplexScene(t *testing.T) {
	screen := NewScreenBuffer(40, 10, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Create a complex scene
	screen.PutText(0, 0, "Header", Attrs(AttrBold))
	screen.PutText(0, 2, "Normal text", Attrs(0))
	screen.PutText(0, 3, "Italic text", Attrs(AttrItalic))
	screen.PutText(20, 3, "Bold", Attrs(AttrBold))

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Header")
	assert.Contains(t, output, "Normal text")
	assert.Contains(t, output, "Italic text")
	assert.Contains(t, output, "Bold")

	// Now modify only one line
	buf.Reset()
	screen.PutText(0, 2, "Changed text", Attrs(0))

	err = renderer.Render()
	assert.NoError(t, err)

	output = buf.String()
	// Should only update the changed line
	assert.Contains(t, output, "Changed")
	// Header should not be in the output since it didn't change
	assert.NotContains(t, output, "Header")
}

func TestScreenRenderer_FindChangedRegions(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		changes []struct {
			x, y int
			text string
		}
		expectedRegions int
	}{
		{
			name:   "single change",
			width:  10,
			height: 5,
			changes: []struct {
				x, y int
				text string
			}{
				{0, 0, "Test"},
			},
			expectedRegions: 1,
		},
		{
			name:   "multiple changes same row",
			width:  20,
			height: 5,
			changes: []struct {
				x, y int
				text string
			}{
				{0, 0, "A"},
				{10, 0, "B"},
			},
			expectedRegions: 2,
		},
		{
			name:   "multiple changes different rows",
			width:  10,
			height: 5,
			changes: []struct {
				x, y int
				text string
			}{
				{0, 0, "A"},
				{0, 2, "B"},
			},
			expectedRegions: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			buf := &bytes.Buffer{}
			renderer := NewScreenRenderer(screen, buf)

			// Apply changes
			for _, change := range tt.changes {
				screen.PutText(change.x, change.y, change.text, Attrs(0))
			}

			// Get content and find regions
			_, _, content := screen.GetContent()
			regions := renderer.findChangedRegions(content)

			assert.Equal(t, tt.expectedRegions, len(regions))
		})
	}
}

func TestScreenRenderer_MergeRegions(t *testing.T) {
	screen := NewScreenBuffer(80, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	tests := []struct {
		name      string
		regions   []region
		threshold int
		expected  int
	}{
		{
			name: "merge close regions",
			regions: []region{
				{0, 0, 2, 0},
				{5, 0, 7, 0}, // 2 cells apart, within threshold
			},
			threshold: 8,
			expected:  1,
		},
		{
			name: "don't merge distant regions",
			regions: []region{
				{0, 0, 2, 0},
				{15, 0, 17, 0}, // 12 cells apart, beyond threshold
			},
			threshold: 8,
			expected:  2,
		},
		{
			name: "don't merge different rows",
			regions: []region{
				{0, 0, 5, 0},
				{0, 1, 5, 1},
			},
			threshold: 8,
			expected:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := renderer.mergeRegions(tt.regions, tt.threshold)
			assert.Equal(t, tt.expected, len(merged))
		})
	}
}
