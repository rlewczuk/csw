package mdv

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"
	"github.com/rlewczuk/csw/pkg/gtv/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMarkdown_ATXHeaders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `# Header 1
## Header 2
### Header 3
#### Header 4
##### Header 5
###### Header 6`

	blocks := ParseMarkdown(content)

	require.Len(t, blocks, 6)

	for i := 0; i < 6; i++ {
		assert.Equal(t, BlockTypeHeader, blocks[i].Type)
		assert.Equal(t, i+1, blocks[i].Level)
		assert.Contains(t, blocks[i].Content, "Header")
	}
}

func TestParseMarkdown_SetextHeaders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `Header 1
========

Header 2
--------`

	blocks := ParseMarkdown(content)

	require.Len(t, blocks, 2)

	assert.Equal(t, BlockTypeHeader, blocks[0].Type)
	assert.Equal(t, 1, blocks[0].Level)
	assert.Equal(t, "Header 1", blocks[0].Content)

	assert.Equal(t, BlockTypeHeader, blocks[1].Type)
	assert.Equal(t, 2, blocks[1].Level)
	assert.Equal(t, "Header 2", blocks[1].Content)
}

func TestParseMarkdown_Paragraphs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `This is a paragraph.

This is another paragraph.`

	blocks := ParseMarkdown(content)

	require.Len(t, blocks, 2)

	assert.Equal(t, BlockTypeParagraph, blocks[0].Type)
	assert.Contains(t, blocks[0].Content, "This is a paragraph")

	assert.Equal(t, BlockTypeParagraph, blocks[1].Type)
	assert.Contains(t, blocks[1].Content, "another paragraph")
}

func TestParseMarkdown_FencedCodeBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := "```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"

	blocks := ParseMarkdown(content)

	require.Len(t, blocks, 1)

	assert.Equal(t, BlockTypeCodeBlock, blocks[0].Type)
	assert.Equal(t, "go", blocks[0].Language)
	assert.Contains(t, blocks[0].Content, "func main()")
}

func TestParseMarkdown_IndentedCodeBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `    code line 1
    code line 2`

	blocks := ParseMarkdown(content)

	require.Len(t, blocks, 1)

	assert.Equal(t, BlockTypeCodeBlock, blocks[0].Type)
	assert.Contains(t, blocks[0].Content, "code line 1")
	assert.Contains(t, blocks[0].Content, "code line 2")
}

func TestParseMarkdown_UnorderedList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `- Item 1
- Item 2
- Item 3`

	blocks := ParseMarkdown(content)

	require.Len(t, blocks, 1)

	assert.Equal(t, BlockTypeList, blocks[0].Type)
	assert.False(t, blocks[0].Ordered)
	assert.Len(t, blocks[0].Children, 3)
}

func TestParseMarkdown_OrderedList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `1. Item 1
2. Item 2
3. Item 3`

	blocks := ParseMarkdown(content)

	require.Len(t, blocks, 1)

	assert.Equal(t, BlockTypeList, blocks[0].Type)
	assert.True(t, blocks[0].Ordered)
	assert.Len(t, blocks[0].Children, 3)
}

func TestParseMarkdown_Blockquote(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `> This is a quote
> spanning multiple lines`

	blocks := ParseMarkdown(content)

	require.Len(t, blocks, 1)

	assert.Equal(t, BlockTypeBlockquote, blocks[0].Type)
	assert.Contains(t, blocks[0].Content, "This is a quote")
	assert.Contains(t, blocks[0].Content, "spanning multiple lines")
}

func TestParseMarkdown_HorizontalRule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	tests := []struct {
		name    string
		content string
	}{
		{"asterisks", "***"},
		{"hyphens", "---"},
		{"underscores", "___"},
		{"with spaces", "* * *"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseMarkdown(tt.content)
			require.Len(t, blocks, 1)
			assert.Equal(t, BlockTypeHorizontalRule, blocks[0].Type)
		})
	}
}

func TestParseMarkdown_MixedContent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `# Title

This is a paragraph with **bold** and *italic* text.

## Code Example

` + "```go\nfunc main() {}\n```" + `

- Item 1
- Item 2

> A quote

---

Another paragraph.`

	blocks := ParseMarkdown(content)

	// Should have: header, paragraph, header, code, list, blockquote, hr, paragraph
	assert.GreaterOrEqual(t, len(blocks), 5)

	// Check first block is header
	assert.Equal(t, BlockTypeHeader, blocks[0].Type)
	assert.Equal(t, 1, blocks[0].Level)
}

func TestMarkdownView_BasicRendering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `# Test Header

This is a test paragraph.`

	// Create application and screen
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create markdown view
	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, content)
	_ = tui.NewApplication(view, screen)

	// Draw the view
	view.Draw(screen)

	// Verify screen has content
	_, _, cells := screen.GetContent()
	assert.NotEmpty(t, cells)

	// Check that some text was rendered
	hasContent := false
	for _, cell := range cells {
		if cell.Rune != 0 && cell.Rune != ' ' {
			hasContent = true
			break
		}
	}
	assert.True(t, hasContent, "Screen should have rendered content")
}

func TestMarkdownView_Scrolling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	// Create content with many lines
	lines := []string{}
	for i := 0; i < 50; i++ {
		lines = append(lines, "Line "+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")

	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 10}, content)

	// Test initial scroll position
	assert.Equal(t, 0, view.GetScrollOffset())

	// Test scroll down
	view.ScrollDown(5)
	assert.Equal(t, 5, view.GetScrollOffset())

	// Test scroll up
	view.ScrollUp(3)
	assert.Equal(t, 2, view.GetScrollOffset())

	// Test page down
	view.PageDown()
	assert.Greater(t, view.GetScrollOffset(), 2)

	// Test page up
	view.PageUp()
	assert.LessOrEqual(t, view.GetScrollOffset(), 10)

	// Test scroll bounds (can't scroll below 0)
	view.SetScrollOffset(-10)
	assert.Equal(t, 0, view.GetScrollOffset())

	// Test scroll bounds (can't scroll beyond content)
	view.SetScrollOffset(1000)
	maxScroll := view.contentHeight - view.viewportHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	assert.Equal(t, maxScroll, view.GetScrollOffset())
}

func TestMarkdownView_SetContent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, "# Initial")

	// Verify initial content
	assert.Len(t, view.blocks, 1)
	assert.Equal(t, BlockTypeHeader, view.blocks[0].Type)

	// Set new content
	view.SetContent("## New Header\n\nNew paragraph.")

	// Verify new content
	assert.GreaterOrEqual(t, len(view.blocks), 1)
	assert.Equal(t, BlockTypeHeader, view.blocks[0].Type)
	assert.Equal(t, 2, view.blocks[0].Level)
}

func TestMarkdownView_KeyboardScrolling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	// Create content with many lines
	content := strings.Repeat("Line\n", 50)
	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 10}, content)

	initialOffset := view.GetScrollOffset()

	// Test down arrow
	downEvent := &tui.TEvent{
		Type: tui.TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type:      gtv.InputEventKey,
			Key:       'B',
			Modifiers: gtv.ModFn,
		},
	}
	view.HandleEvent(downEvent)
	assert.Greater(t, view.GetScrollOffset(), initialOffset)

	// Test up arrow
	upEvent := &tui.TEvent{
		Type: tui.TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type:      gtv.InputEventKey,
			Key:       'A',
			Modifiers: gtv.ModFn,
		},
	}
	view.HandleEvent(upEvent)
	assert.Equal(t, initialOffset, view.GetScrollOffset())

	// Test page down
	pageDownEvent := &tui.TEvent{
		Type: tui.TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type:      gtv.InputEventKey,
			Key:       'N',
			Modifiers: gtv.ModFn,
		},
	}
	view.HandleEvent(pageDownEvent)
	assert.Greater(t, view.GetScrollOffset(), initialOffset)
}

func TestMarkdownView_ResizeEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, "# Test")

	// Send resize event
	newRect := gtv.TRect{X: 0, Y: 0, W: 100, H: 30}
	resizeEvent := &tui.TEvent{
		Type: tui.TEventTypeResize,
		Rect: newRect,
	}
	view.HandleEvent(resizeEvent)

	// Verify new dimensions
	assert.Equal(t, newRect.W, view.Position.W)
	assert.Equal(t, newRect.H, view.Position.H)
	assert.Equal(t, 30, view.viewportHeight)
}

func TestDefaultFactory_CustomCodeBlockFactory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	factory := NewDefaultMarkdownViewFactory()

	// Create a custom factory for "test" language
	customFactory := &testCustomFactory{}
	factory.RegisterCustomFactory("test", customFactory)

	content := "```test\ncustom content\n```"

	view := factory.NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, content)

	// Verify custom factory was used
	assert.True(t, customFactory.called)
	assert.NotNil(t, view)
}

// testCustomFactory is a test implementation of MarkdownViewFactory
type testCustomFactory struct {
	called bool
}

func (f *testCustomFactory) NewMarkdownView(parent tui.IWidget, rect gtv.TRect, content string) IMarkdownView {
	f.called = true
	// Return a simple markdown view for testing
	return NewMarkdownView(parent, rect, content)
}

func TestWordWrap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	tests := []struct {
		name     string
		text     string
		width    int
		expected int // expected number of lines
	}{
		{
			name:     "short text",
			text:     "hello world",
			width:    20,
			expected: 1,
		},
		{
			name:     "exact fit",
			text:     "hello world",
			width:    11,
			expected: 1,
		},
		{
			name:     "needs wrap",
			text:     "hello world this is a test",
			width:    10,
			expected: 3,
		},
		{
			name:     "with newline",
			text:     "line1\nline2",
			width:    20,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wordWrap(tt.text, tt.width)
			assert.Equal(t, tt.expected, len(lines), "Expected %d lines, got %d: %v", tt.expected, len(lines), lines)
		})
	}
}

func TestMarkdownView_AllBlockTypes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `# Header 1
## Header 2

A paragraph with **bold** and *italic*.

` + "```go\ncode\n```" + `

- List item 1
- List item 2

1. Ordered 1
2. Ordered 2

> A blockquote

---

Another paragraph.`

	screen := tio.NewScreenBuffer(80, 50, 0)
	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 50}, content)

	// Draw the view
	view.Draw(screen)

	// Verify blocks were parsed correctly
	assert.GreaterOrEqual(t, len(view.blocks), 8)

	// Check that various block types are present
	hasHeader := false
	hasParagraph := false
	hasCode := false
	hasList := false
	hasQuote := false
	hasHR := false

	for _, block := range view.blocks {
		switch block.Type {
		case BlockTypeHeader:
			hasHeader = true
		case BlockTypeParagraph:
			hasParagraph = true
		case BlockTypeCodeBlock:
			hasCode = true
		case BlockTypeList:
			hasList = true
		case BlockTypeBlockquote:
			hasQuote = true
		case BlockTypeHorizontalRule:
			hasHR = true
		}
	}

	assert.True(t, hasHeader, "Should have header blocks")
	assert.True(t, hasParagraph, "Should have paragraph blocks")
	assert.True(t, hasCode, "Should have code blocks")
	assert.True(t, hasList, "Should have list blocks")
	assert.True(t, hasQuote, "Should have blockquote blocks")
	assert.True(t, hasHR, "Should have horizontal rule blocks")
}

func TestMarkdownView_ScrollingClearsOldContent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	// Create content with lines of different lengths
	// This will expose the bug where old content is not cleared
	content := `Line 1: Short
Line 2: This is a much longer line that will be visible initially
Line 3: Medium length line here
Line 4: X
Line 5: Another very long line that extends quite far to the right
Line 6: Tiny`

	screen := tio.NewScreenBuffer(80, 5, 0)
	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 5}, content)

	// Draw initial view
	view.Draw(screen)

	// Scroll down by 1 line
	view.ScrollDown(1)

	// Redraw WITHOUT clearing the screen - this exposes the bug
	// where old content is not cleared
	view.Draw(screen)

	// Get content after scrolling
	_, _, cells2 := screen.GetContent()
	line1AfterScroll := extractLine(cells2, 80, 1)

	// After scrolling, line 1 should now show "Line 3: Medium length line here"
	// If the bug exists, we might see remnants of the longer "Line 2" text
	assert.Contains(t, line1AfterScroll, "Line 3", "Line 1 should show Line 3 after scrolling")
	assert.NotContains(t, line1AfterScroll, "This is a much longer line", "Should not contain remnants of Line 2")

	// Verify that no old content remains after the actual text ends
	// Find where "Line 3: Medium length line here" ends
	line3Text := "Line 3: Medium length line here"
	line3Len := len(line3Text)

	// Check that positions after the line end are spaces, not old content
	for i := line3Len; i < len(line1AfterScroll); i++ {
		if line1AfterScroll[i] != ' ' && line1AfterScroll[i] != 0 {
			t.Errorf("MarkdownView_ScrollingClearsOldContent: Found non-space character '%c' at position %d after line end (expected space or null)", line1AfterScroll[i], i)
		}
	}
}

func TestMarkdownView_ThemeColorsApplied(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	content := `# Header 1
## Header 2

A paragraph.

` + "```go\ncode\n```"

	// Create a theme with specific colors
	theme := map[string]gtv.CellAttributes{
		"mdv-h1": {
			TextColor: gtv.TextColor(0xFF0000), // Red
			BackColor: gtv.NoColor,
		},
		"mdv-h2": {
			TextColor: gtv.TextColor(0x00FF00), // Green
			BackColor: gtv.NoColor,
		},
		"mdv-paragraph": {
			TextColor: gtv.TextColor(0x0000FF), // Blue
			BackColor: gtv.NoColor,
		},
		"mdv-code-block": {
			TextColor: gtv.TextColor(0xFFFF00), // Yellow
			BackColor: gtv.TextColor(0x333333), // Dark gray
		},
	}

	screen := tio.NewScreenBuffer(80, 20, 0)
	themedScreen := gtv.NewThemeInterceptor(screen, theme)

	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 20}, content)

	// Draw the view with themed screen
	view.Draw(themedScreen)

	// Get screen content
	_, _, cells := screen.GetContent()

	// Find the H1 header text "Header 1" and verify it has red color
	h1Found := false
	for i := 0; i < len(cells); i++ {
		if cells[i].Rune == 'H' && i+6 < len(cells) {
			// Check if this is "Header"
			text := string([]rune{cells[i].Rune, cells[i+1].Rune, cells[i+2].Rune, cells[i+3].Rune, cells[i+4].Rune, cells[i+5].Rune})
			if text == "Header" {
				// This should be H1 (first occurrence)
				if !h1Found {
					h1Found = true
					// Verify the color is red (0xFF0000)
					assert.Equal(t, gtv.TextColor(0xFF0000), cells[i].Attrs.TextColor, "H1 header should have red text color from theme at markdown_view_test.go:TestMarkdownView_ThemeColorsApplied()")
					break
				}
			}
		}
	}

	assert.True(t, h1Found, "Should find H1 header text")

	// Find the paragraph text "A paragraph" and verify it has blue color
	paraFound := false
	for i := 0; i < len(cells); i++ {
		if cells[i].Rune == 'A' && i+11 < len(cells) {
			// Check if this is "A paragraph"
			text := string([]rune{cells[i].Rune, cells[i+1].Rune})
			if text == "A " {
				paraFound = true
				// Verify the color is blue (0x0000FF)
				assert.Equal(t, gtv.TextColor(0x0000FF), cells[i].Attrs.TextColor, "Paragraph should have blue text color from theme at markdown_view_test.go:TestMarkdownView_ThemeColorsApplied()")
				break
			}
		}
	}

	assert.True(t, paraFound, "Should find paragraph text")
}

func TestMarkdownView_ScrollingClearsAttributes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	// Create content where H1 header with underline will be scrolled away
	// and replaced by plain paragraph text
	// The bug occurs when a longer underlined text is replaced by shorter plain text
	content := `# This Is A Very Long Header With Underline And Bold

Short text

More content here

Even more content

Final line of text`

	screen := tio.NewScreenBuffer(80, 3, 0)
	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 3}, content)

	// Draw initial view - H1 header with underline should be visible
	view.Draw(screen)

	// Get initial content
	_, _, cells1 := screen.GetContent()

	// Verify that first line has underline attribute (from H1 header)
	hasUnderlineInitially := false
	headerTextLen := 0
	for x := 0; x < 80; x++ {
		if cells1[x].Attrs.Attributes&gtv.AttrUnderline != 0 {
			hasUnderlineInitially = true
			headerTextLen++
		}
	}
	assert.True(t, hasUnderlineInitially, "Initial view should have underline attribute on first line")
	assert.Greater(t, headerTextLen, 20, "Header should be reasonably long")

	// Scroll down by 2 lines - now "Short text" should be at top
	view.ScrollDown(2)

	// DON'T clear the screen buffer - this simulates the real scenario
	// where the markdown view is responsible for clearing old content
	view.Draw(screen)

	// Get content after scrolling
	_, _, cells2 := screen.GetContent()

	// Verify that first line NO LONGER has underline attribute after "Short text" ends
	// Find where "Short text" ends
	line1Text := extractLine(cells2, 80, 0)
	shortTextEnd := strings.Index(line1Text, "Short text") + len("Short text")

	if shortTextEnd > len("Short text") { // Found the text
		// Check cells after "Short text" ends - they should NOT have underline
		for x := shortTextEnd; x < 80; x++ {
			if cells2[x].Attrs.Attributes&gtv.AttrUnderline != 0 {
				t.Errorf("TestMarkdownView_ScrollingClearsAttributes: Cell at position %d has underline attribute but shouldn't (rune='%c', attrs=%+v). This is beyond the end of 'Short text'.", x, cells2[x].Rune, cells2[x].Attrs)
			}
			if cells2[x].Attrs.Attributes&gtv.AttrBold != 0 {
				t.Errorf("TestMarkdownView_ScrollingClearsAttributes: Cell at position %d has bold attribute but shouldn't (rune='%c', attrs=%+v). This is beyond the end of 'Short text'.", x, cells2[x].Rune, cells2[x].Attrs)
			}
		}
	}

	// Also check that "Short text" itself doesn't have underline
	for x := 0; x < shortTextEnd && x < 80; x++ {
		if cells2[x].Rune != ' ' && cells2[x].Rune != 0 {
			if cells2[x].Attrs.Attributes&gtv.AttrUnderline != 0 {
				t.Errorf("TestMarkdownView_ScrollingClearsAttributes: Cell at position %d in 'Short text' has underline attribute but shouldn't (rune='%c', attrs=%+v)", x, cells2[x].Rune, cells2[x].Attrs)
			}
		}
	}

	// Verify text content is correct
	assert.Contains(t, line1Text, "Short text", "First line should contain 'Short text' after scrolling")
	assert.NotContains(t, line1Text, "Header", "First line should not contain header text after scrolling")
}

// TestMarkdownView_AttributeBleedingBug reproduces the bug where:
// - line with text attributes (eg. underline), then empty line, then line with more characters
// - when scrolling up, empty line receives attributes from previous line to the length of line below it
func TestMarkdownView_AttributeBleedingBug(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	// The bug: empty spacing lines retain background color or attributes from adjacent content
	// This is most visible with code blocks (which have background color) or headers (bold+underline)
	content := `## Lists

### Unordered List

- First item
- Second item`

	screen := tio.NewScreenBuffer(80, 10, 0)
	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 10}, content)

	// Draw initial view
	view.Draw(screen)

	// Get the screen content
	_, _, cells := screen.GetContent()

	// The layout should be:
	// Line 0 (Y=0): "Lists" header with bold+underline
	// Line 1 (Y=1): Empty spacing after "Lists" header
	// Line 2 (Y=2): "Unordered List" header with bold
	// Line 3 (Y=3): Empty spacing after "Unordered List" header
	// Line 4 (Y=4): "• First item"
	// etc.

	// Check Line 1 (empty spacing after "Lists" header)
	// It should NOT have any attributes
	for x := 0; x < 80; x++ {
		cell := cells[1*80+x]
		if cell.Rune != ' ' && cell.Rune != 0 {
			t.Errorf("TestMarkdownView_AttributeBleedingBug: Empty spacing line 1, col %d has non-space character '%c'", x, cell.Rune)
		}
		if cell.Attrs.Attributes != 0 {
			t.Errorf("TestMarkdownView_AttributeBleedingBug: Empty spacing line 1, col %d has attributes %v but should have none", x, cell.Attrs.Attributes)
		}
		if cell.Attrs.BackColor != gtv.NoColor {
			t.Errorf("TestMarkdownView_AttributeBleedingBug: Empty spacing line 1, col %d has background color but shouldn't", x)
		}
	}

	// Check Line 3 (empty spacing after "Unordered List" header)
	for x := 0; x < 80; x++ {
		cell := cells[3*80+x]
		if cell.Rune != ' ' && cell.Rune != 0 {
			t.Errorf("TestMarkdownView_AttributeBleedingBug: Empty spacing line 3, col %d has non-space character '%c'", x, cell.Rune)
		}
		if cell.Attrs.Attributes != 0 {
			t.Errorf("TestMarkdownView_AttributeBleedingBug: Empty spacing line 3, col %d has attributes %v but should have none", x, cell.Attrs.Attributes)
		}
		if cell.Attrs.BackColor != gtv.NoColor {
			t.Errorf("TestMarkdownView_AttributeBleedingBug: Empty spacing line 3, col %d has background color but shouldn't", x)
		}
	}
}

// TestMarkdownView_EmptySpaceUsesDefaultTheme tests that empty space after rendered content
// uses the mdv-paragraph theme tag for background color, not NoColor or some bright background
func TestMarkdownView_EmptySpaceUsesDefaultTheme(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx

	// Create content with short lines to ensure there's empty space
	content := `Short
Line two`

	// Create a theme with a BRIGHT/distinctive background color for mdv-paragraph
	// so we can clearly see if it's being applied
	theme := map[string]gtv.CellAttributes{
		"mdv-paragraph": {
			TextColor: gtv.TextColor(0xFFFFFF), // White
			BackColor: gtv.TextColor(0xFF00FF), // Bright magenta background (very visible)
		},
	}

	screen := tio.NewScreenBuffer(80, 10, 0)
	themedScreen := gtv.NewThemeInterceptor(screen, theme)

	view := NewMarkdownView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 10}, content)

	// Draw the view with themed screen
	view.Draw(themedScreen)

	// Get screen content
	_, _, cells := screen.GetContent()

	// Check the first line - after "Short" text ends, the remaining cells should have
	// the paragraph theme background color (bright magenta, 0xFF00FF)
	// Find where "Short" ends (it's 5 characters)
	shortTextEnd := 5

	// Check cells after "Short" text - they should have paragraph background color
	// If the bug exists, they will have NoColor (0xFFFFFFFF) instead of the theme color
	for x := shortTextEnd; x < 80; x++ {
		cell := cells[x] // First line, cells 0-79
		// The cell should be a space
		if cell.Rune != ' ' && cell.Rune != 0 {
			t.Errorf("TestMarkdownView_EmptySpaceUsesDefaultTheme: Cell at position %d has non-space character '%c'", x, cell.Rune)
		}
		// After theme interceptor, cells with mdv-paragraph theme tag should have bright magenta background
		// If the bug exists, they will have NoColor (0xFFFFFFFF)
		if cell.Attrs.BackColor == gtv.NoColor {
			t.Errorf("TestMarkdownView_EmptySpaceUsesDefaultTheme: Cell at position %d has NoColor background (bug detected - empty space not using theme), attrs=%+v", x, cell.Attrs)
		}
		// Also check that the background is actually the theme color (magenta)
		if cell.Attrs.BackColor != gtv.TextColor(0xFF00FF) {
			t.Errorf("TestMarkdownView_EmptySpaceUsesDefaultTheme: Cell at position %d has unexpected background color %v (expected magenta 0xFF00FF from theme), attrs=%+v", x, cell.Attrs.BackColor, cell.Attrs)
		}
	}
}

// extractLine extracts a line from the screen buffer as a string
func extractLine(cells []gtv.Cell, width, lineNum int) string {
	start := lineNum * width
	end := start + width
	if end > len(cells) {
		end = len(cells)
	}

	runes := make([]rune, 0, width)
	for i := start; i < end; i++ {
		runes = append(runes, cells[i].Rune)
	}

	return string(runes)
}
