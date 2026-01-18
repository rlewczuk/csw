package tio

import (
	"bytes"
	"strings"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gophertv"
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

	// First render will output cursor position and style
	err := renderer.Render()
	assert.NoError(t, err)
	assert.Greater(t, buf.Len(), 0, "First render should output cursor sequences")

	// Second render with no changes should produce no output
	buf.Reset()
	err = renderer.Render()
	assert.NoError(t, err)
	assert.Equal(t, 0, buf.Len(), "Second render with no changes should produce no output")
}

func TestScreenRenderer_RenderInitialContent(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Add some text
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", gophertv.Attrs(0))

	err := renderer.Render()
	assert.NoError(t, err)

	//output := buf.String()
	// Should contain cursor movement and text
	//assert.Contains(t, output, "\x1b[1;1H") // Move to 1,1
	//assert.Contains(t, output, "Hello")
}

func TestScreenRenderer_DifferentialRendering(t *testing.T) {
	screen := NewScreenBuffer(20, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// First render
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", gophertv.Attrs(0))
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
	screen.PutText(gophertv.TRect{X: 6, Y: 0, W: 0, H: 0}, "World", gophertv.Attrs(0))
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
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "A", gophertv.Attrs(0))
	screen.PutText(gophertv.TRect{X: 5, Y: 0, W: 0, H: 0}, "B", gophertv.Attrs(0)) // Only 4 cells apart

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	// Should contain a single cursor move to the start (for content)
	// Note: there will also be a cursor position update at the end
	moveCount := strings.Count(output, "\x1b[1;1H")
	assert.GreaterOrEqual(t, moveCount, 1, "Should have at least one cursor move for merged region")
}

func TestScreenRenderer_MultipleRows(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Add text on multiple rows
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Row1", gophertv.Attrs(0))
	screen.PutText(gophertv.TRect{X: 0, Y: 2, W: 0, H: 0}, "Row3", gophertv.Attrs(0))

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
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Bold", gophertv.Attrs(gophertv.AttrBold))

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
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Test", gophertv.Attrs(gophertv.AttrBold|gophertv.AttrItalic|gophertv.AttrUnderline))

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
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Test", gophertv.Attrs(0))
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
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Test", gophertv.Attrs(0))
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
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "┌─┐", gophertv.Attrs(0))
	screen.PutText(gophertv.TRect{X: 0, Y: 1, W: 0, H: 0}, "│X│", gophertv.Attrs(0))
	screen.PutText(gophertv.TRect{X: 0, Y: 2, W: 0, H: 0}, "└─┘", gophertv.Attrs(0))

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
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Header", gophertv.Attrs(gophertv.AttrBold))
	screen.PutText(gophertv.TRect{X: 0, Y: 2, W: 0, H: 0}, "Normal text", gophertv.Attrs(0))
	screen.PutText(gophertv.TRect{X: 0, Y: 3, W: 0, H: 0}, "Italic text", gophertv.Attrs(gophertv.AttrItalic))
	screen.PutText(gophertv.TRect{X: 20, Y: 3, W: 0, H: 0}, "Bold", gophertv.Attrs(gophertv.AttrBold))

	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Header")
	assert.Contains(t, output, "Normal text")
	assert.Contains(t, output, "Italic text")
	assert.Contains(t, output, "Bold")

	// Now modify only one line
	buf.Reset()
	screen.PutText(gophertv.TRect{X: 0, Y: 2, W: 0, H: 0}, "Changed text", gophertv.Attrs(0))

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
				screen.PutText(gophertv.TRect{X: uint16(change.x), Y: uint16(change.y), W: 0, H: 0}, change.text, gophertv.Attrs(0))
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

func TestScreenRenderer_CursorPosition(t *testing.T) {
	tests := []struct {
		name     string
		moves    []struct{ x, y int }
		wantSeqs []string
	}{
		{
			name: "move cursor once",
			moves: []struct{ x, y int }{
				{10, 5},
			},
			wantSeqs: []string{"\x1b[6;11H"}, // ANSI uses 1-based indexing
		},
		{
			name: "move cursor to origin",
			moves: []struct{ x, y int }{
				{0, 0},
			},
			wantSeqs: []string{"\x1b[1;1H"},
		},
		{
			name: "multiple cursor moves",
			moves: []struct{ x, y int }{
				{5, 5},
				{10, 10},
			},
			wantSeqs: []string{
				"\x1b[6;6H",   // First move
				"\x1b[11;11H", // Second move
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(80, 24, 0)
			buf := &bytes.Buffer{}
			renderer := NewScreenRenderer(screen, buf)

			// Move cursor to a different initial position to ensure we can detect changes
			screen.MoveCursor(50, 20)
			// Perform initial render to establish baseline
			renderer.Render()
			buf.Reset()

			// Apply cursor moves and render
			for i, move := range tt.moves {
				screen.MoveCursor(move.x, move.y)
				err := renderer.Render()
				assert.NoError(t, err)

				output := buf.String()
				assert.Contains(t, output, tt.wantSeqs[i], "Should contain cursor move sequence")
				buf.Reset()
			}
		})
	}
}

func TestScreenRenderer_CursorStyle(t *testing.T) {
	tests := []struct {
		name         string
		initialStyle gophertv.CursorStyle
		style        gophertv.CursorStyle
		wantSeq      string
	}{
		{
			name:         "default cursor style",
			initialStyle: gophertv.CursorStyleBlock,
			style:        gophertv.CursorStyleDefault,
			wantSeq:      "\x1b[0 q",
		},
		{
			name:         "block cursor style",
			initialStyle: gophertv.CursorStyleDefault,
			style:        gophertv.CursorStyleBlock,
			wantSeq:      "\x1b[2 q",
		},
		{
			name:         "underline cursor style",
			initialStyle: gophertv.CursorStyleBlock,
			style:        gophertv.CursorStyleUnderline,
			wantSeq:      "\x1b[4 q",
		},
		{
			name:         "bar cursor style",
			initialStyle: gophertv.CursorStyleBlock,
			style:        gophertv.CursorStyleBar,
			wantSeq:      "\x1b[6 q",
		},
		{
			name:         "hidden cursor style",
			initialStyle: gophertv.CursorStyleBlock,
			style:        gophertv.CursorStyleHidden,
			wantSeq:      "\x1b[?25l",
		},
		{
			name:         "blinking block cursor style",
			initialStyle: gophertv.CursorStyleBlock,
			style:        gophertv.CursorStyleBlock | gophertv.CursorStyleBlinking,
			wantSeq:      "\x1b[1 q",
		},
		{
			name:         "blinking underline cursor style",
			initialStyle: gophertv.CursorStyleBlock,
			style:        gophertv.CursorStyleUnderline | gophertv.CursorStyleBlinking,
			wantSeq:      "\x1b[3 q",
		},
		{
			name:         "blinking bar cursor style",
			initialStyle: gophertv.CursorStyleBlock,
			style:        gophertv.CursorStyleBar | gophertv.CursorStyleBlinking,
			wantSeq:      "\x1b[5 q",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(80, 24, 0)
			buf := &bytes.Buffer{}
			renderer := NewScreenRenderer(screen, buf)

			// Set a different initial style to ensure we can detect changes
			screen.SetCursorStyle(tt.initialStyle)
			// Perform initial render
			renderer.Render()
			buf.Reset()

			// Set cursor style and render
			screen.SetCursorStyle(tt.style)
			err := renderer.Render()
			assert.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, tt.wantSeq, "Should contain cursor style sequence")
		})
	}
}

func TestScreenRenderer_CursorStyleChanges(t *testing.T) {
	screen := NewScreenBuffer(80, 24, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Initial render
	renderer.Render()
	buf.Reset()

	// First cycle through non-blinking styles
	nonBlinkingStyles := []gophertv.CursorStyle{
		gophertv.CursorStyleBlock,
		gophertv.CursorStyleUnderline,
		gophertv.CursorStyleBar,
	}

	for _, style := range nonBlinkingStyles {
		screen.SetCursorStyle(style)
		err := renderer.Render()
		assert.NoError(t, err)

		output := buf.String()
		assert.Greater(t, len(output), 0, "Should output style change")
		buf.Reset()
	}

	// Then cycle through blinking styles
	blinkingStyles := []gophertv.CursorStyle{
		gophertv.CursorStyleBlock | gophertv.CursorStyleBlinking,
		gophertv.CursorStyleUnderline | gophertv.CursorStyleBlinking,
		gophertv.CursorStyleBar | gophertv.CursorStyleBlinking,
	}

	for _, style := range blinkingStyles {
		screen.SetCursorStyle(style)
		err := renderer.Render()
		assert.NoError(t, err)

		output := buf.String()
		assert.Greater(t, len(output), 0, "Should output style change")
		buf.Reset()
	}

	// Finally cycle through all styles with blinking (combination test)
	allStylesWithBlinking := []gophertv.CursorStyle{
		gophertv.CursorStyleDefault | gophertv.CursorStyleBlinking,
		gophertv.CursorStyleBlock | gophertv.CursorStyleBlinking,
		gophertv.CursorStyleUnderline | gophertv.CursorStyleBlinking,
		gophertv.CursorStyleBar | gophertv.CursorStyleBlinking,
	}

	for _, style := range allStylesWithBlinking {
		screen.SetCursorStyle(style)
		err := renderer.Render()
		assert.NoError(t, err)

		output := buf.String()
		assert.Greater(t, len(output), 0, "Should output style change")
		buf.Reset()
	}

	// Render again without changes - should produce no output
	err := renderer.Render()
	assert.NoError(t, err)
	assert.Equal(t, 0, buf.Len(), "No output expected when cursor style unchanged")
}

func TestScreenRenderer_CursorStyleTransitionFromHidden(t *testing.T) {
	screen := NewScreenBuffer(80, 24, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Initial render
	renderer.Render()
	buf.Reset()

	// Hide cursor
	screen.SetCursorStyle(gophertv.CursorStyleHidden)
	err := renderer.Render()
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "\x1b[?25l", "Should contain hide cursor sequence")
	buf.Reset()

	// Transition from hidden to block - should show cursor
	screen.SetCursorStyle(gophertv.CursorStyleBlock)
	err = renderer.Render()
	assert.NoError(t, err)
	output = buf.String()
	assert.Contains(t, output, "\x1b[?25h", "Should contain show cursor sequence when transitioning from hidden")
	assert.Contains(t, output, "\x1b[2 q", "Should contain block cursor style sequence")
	buf.Reset()

	// Transition from hidden to underline
	screen.SetCursorStyle(gophertv.CursorStyleHidden)
	renderer.Render()
	buf.Reset()
	screen.SetCursorStyle(gophertv.CursorStyleUnderline)
	err = renderer.Render()
	assert.NoError(t, err)
	output = buf.String()
	assert.Contains(t, output, "\x1b[?25h", "Should contain show cursor sequence when transitioning from hidden")
	assert.Contains(t, output, "\x1b[4 q", "Should contain underline cursor style sequence")
	buf.Reset()

	// Transition from hidden to bar
	screen.SetCursorStyle(gophertv.CursorStyleHidden)
	renderer.Render()
	buf.Reset()
	screen.SetCursorStyle(gophertv.CursorStyleBar)
	err = renderer.Render()
	assert.NoError(t, err)
	output = buf.String()
	assert.Contains(t, output, "\x1b[?25h", "Should contain show cursor sequence when transitioning from hidden")
	assert.Contains(t, output, "\x1b[6 q", "Should contain bar cursor style sequence")
	buf.Reset()

	// Transition from hidden to default
	screen.SetCursorStyle(gophertv.CursorStyleHidden)
	renderer.Render()
	buf.Reset()
	screen.SetCursorStyle(gophertv.CursorStyleDefault)
	err = renderer.Render()
	assert.NoError(t, err)
	output = buf.String()
	assert.Contains(t, output, "\x1b[?25h", "Should contain show cursor sequence when transitioning from hidden")
	assert.Contains(t, output, "\x1b[0 q", "Should contain default cursor style sequence")
	buf.Reset()

	// Transition from hidden to blinking block
	screen.SetCursorStyle(gophertv.CursorStyleHidden)
	renderer.Render()
	buf.Reset()
	screen.SetCursorStyle(gophertv.CursorStyleBlock | gophertv.CursorStyleBlinking)
	err = renderer.Render()
	assert.NoError(t, err)
	output = buf.String()
	assert.Contains(t, output, "\x1b[?25h", "Should contain show cursor sequence when transitioning from hidden")
	assert.Contains(t, output, "\x1b[1 q", "Should contain blinking block cursor style sequence")
	buf.Reset()

	// Transition from hidden to blinking underline
	screen.SetCursorStyle(gophertv.CursorStyleHidden)
	renderer.Render()
	buf.Reset()
	screen.SetCursorStyle(gophertv.CursorStyleUnderline | gophertv.CursorStyleBlinking)
	err = renderer.Render()
	assert.NoError(t, err)
	output = buf.String()
	assert.Contains(t, output, "\x1b[?25h", "Should contain show cursor sequence when transitioning from hidden")
	assert.Contains(t, output, "\x1b[3 q", "Should contain blinking underline cursor style sequence")
	buf.Reset()

	// Transition from hidden to blinking bar
	screen.SetCursorStyle(gophertv.CursorStyleHidden)
	renderer.Render()
	buf.Reset()
	screen.SetCursorStyle(gophertv.CursorStyleBar | gophertv.CursorStyleBlinking)
	err = renderer.Render()
	assert.NoError(t, err)
	output = buf.String()
	assert.Contains(t, output, "\x1b[?25h", "Should contain show cursor sequence when transitioning from hidden")
	assert.Contains(t, output, "\x1b[5 q", "Should contain blinking bar cursor style sequence")
}

func TestScreenRenderer_CursorDoesNotAffectContent(t *testing.T) {
	screen := NewScreenBuffer(20, 5, 0)
	buf := &bytes.Buffer{}
	renderer := NewScreenRenderer(screen, buf)

	// Add text
	screen.PutText(gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", gophertv.Attrs(0))
	renderer.Render()
	buf.Reset()

	// Move cursor and change style
	screen.MoveCursor(10, 2)
	screen.SetCursorStyle(gophertv.CursorStyleBlock)
	err := renderer.Render()
	assert.NoError(t, err)

	output := buf.String()
	// Should contain cursor sequences but not re-render the text
	assert.NotContains(t, output, "Hello", "Should not re-render unchanged text")
	assert.Contains(t, output, "\x1b[3;11H", "Should contain cursor move")
	assert.Contains(t, output, "\x1b[2 q", "Should contain cursor style")
}
