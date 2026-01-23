package mdv

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
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
