package mdv

import (
	"fmt"
	"strings"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
	"github.com/codesnort/codesnort-swe/pkg/gtv/util"
)

// DefaultMarkdownViewFactory is the default implementation of MarkdownViewFactory
type DefaultMarkdownViewFactory struct {
	// customFactories allows registering custom factories for specific block types or languages
	customFactories map[string]MarkdownViewFactory
}

// NewDefaultMarkdownViewFactory creates a new default markdown view factory
func NewDefaultMarkdownViewFactory() *DefaultMarkdownViewFactory {
	return &DefaultMarkdownViewFactory{
		customFactories: make(map[string]MarkdownViewFactory),
	}
}

// RegisterCustomFactory registers a custom factory for a specific block type or code language
func (f *DefaultMarkdownViewFactory) RegisterCustomFactory(key string, factory MarkdownViewFactory) {
	f.customFactories[key] = factory
}

// NewMarkdownView creates a new widget rendering the given chunk of markdown content
func (f *DefaultMarkdownViewFactory) NewMarkdownView(parent tui.IWidget, rect gtv.TRect, content string) IMarkdownView {
	return newMarkdownViewWithFactory(parent, rect, content, f)
}

// renderBlock renders a single block based on its type
func (f *DefaultMarkdownViewFactory) renderBlock(parent tui.IWidget, block Block, yOffset *int, width int) []tui.IWidget {
	widgets := []tui.IWidget{}

	switch block.Type {
	case BlockTypeHeader:
		widget := f.renderHeader(parent, block, yOffset, width)
		widgets = append(widgets, widget)

	case BlockTypeParagraph:
		widget := f.renderParagraph(parent, block, yOffset, width)
		widgets = append(widgets, widget)

	case BlockTypeCodeBlock:
		// Check for custom factory for this language
		if block.Language != "" {
			if customFactory, ok := f.customFactories[block.Language]; ok {
				widget := customFactory.NewMarkdownView(parent, gtv.TRect{
					X: 0,
					Y: uint16(*yOffset),
					W: uint16(width),
					H: 0, // Auto-size
				}, block.Content)
				widgets = append(widgets, widget)
				*yOffset += int(widget.GetPos().H)
				return widgets
			}
		}

		widget := f.renderCodeBlock(parent, block, yOffset, width)
		widgets = append(widgets, widget)

	case BlockTypeList:
		listWidgets := f.renderList(parent, block, yOffset, width)
		widgets = append(widgets, listWidgets...)

	case BlockTypeBlockquote:
		widget := f.renderBlockquote(parent, block, yOffset, width)
		widgets = append(widgets, widget)

	case BlockTypeHorizontalRule:
		widget := f.renderHorizontalRule(parent, yOffset, width)
		widgets = append(widgets, widget)
	}

	return widgets
}

// renderHeader renders a header block
func (f *DefaultMarkdownViewFactory) renderHeader(parent tui.IWidget, block Block, yOffset *int, width int) tui.IWidget {
	themeTag := fmt.Sprintf("mdv-h%d", block.Level)
	attrs := gtv.CellTag(themeTag)

	// Add bold and underline for H1 and H2
	if block.Level == 1 || block.Level == 2 {
		attrs.Attributes |= gtv.AttrBold | gtv.AttrUnderline
	} else if block.Level == 3 || block.Level == 4 {
		attrs.Attributes |= gtv.AttrBold
	}

	label := tui.NewLabel(parent, block.Content, gtv.TRect{
		X: 0,
		Y: uint16(*yOffset),
		W: uint16(width),
		H: 1,
	}, attrs)

	*yOffset += 1
	// Add spacing after header
	*yOffset += 1

	return label
}

// renderParagraph renders a paragraph block with inline formatting
func (f *DefaultMarkdownViewFactory) renderParagraph(parent tui.IWidget, block Block, yOffset *int, width int) tui.IWidget {
	// Word wrap the content to fit width
	lines := wordWrap(block.Content, width)

	attrs := gtv.CellTag("mdv-paragraph")

	// For simplicity, create multiple labels for multi-line paragraphs
	// In a more sophisticated implementation, we'd create a custom multi-line widget
	startY := *yOffset
	for i, line := range lines {
		tui.NewLabel(parent, line, gtv.TRect{
			X: 0,
			Y: uint16(*yOffset + i),
			W: uint16(width),
			H: 1,
		}, attrs)
	}

	*yOffset += len(lines)
	// Add spacing after paragraph
	*yOffset += 1

	// Return a placeholder widget (in reality, we created multiple labels)
	return tui.NewWidget(parent, tui.WithRectangle(0, startY, width, len(lines)))
}

// renderCodeBlock renders a code block
func (f *DefaultMarkdownViewFactory) renderCodeBlock(parent tui.IWidget, block Block, yOffset *int, width int) tui.IWidget {
	attrs := gtv.CellTag("mdv-code-block")

	lines := strings.Split(block.Content, "\n")
	startY := *yOffset

	for i, line := range lines {
		// Escape the line to prevent formatting interpretation
		escapedLine := util.EscapeText(line)

		tui.NewLabel(parent, escapedLine, gtv.TRect{
			X: 0,
			Y: uint16(*yOffset + i),
			W: uint16(width),
			H: 1,
		}, attrs)
	}

	*yOffset += len(lines)
	// Add spacing after code block
	*yOffset += 1

	return tui.NewWidget(parent, tui.WithRectangle(0, startY, width, len(lines)))
}

// renderList renders a list block
func (f *DefaultMarkdownViewFactory) renderList(parent tui.IWidget, block Block, yOffset *int, width int) []tui.IWidget {
	widgets := []tui.IWidget{}

	for i, item := range block.Children {
		var bullet string
		if block.Ordered {
			bullet = fmt.Sprintf("%d. ", i+1)
		} else {
			bullet = "• "
		}

		// Render bullet
		bulletAttrs := gtv.CellTag("mdv-list-bullet")
		if block.Ordered {
			bulletAttrs.Attributes |= gtv.AttrBold
		}

		bulletLabel := tui.NewLabel(parent, bullet, gtv.TRect{
			X: 0,
			Y: uint16(*yOffset),
			W: uint16(len(bullet)),
			H: 1,
		}, bulletAttrs)
		widgets = append(widgets, bulletLabel)

		// Render item content (word-wrapped)
		itemWidth := width - len(bullet)
		lines := wordWrap(item.Content, itemWidth)

		itemAttrs := gtv.CellTag("mdv-paragraph")
		for j, line := range lines {
			itemLabel := tui.NewLabel(parent, line, gtv.TRect{
				X: uint16(len(bullet)),
				Y: uint16(*yOffset + j),
				W: uint16(itemWidth),
				H: 1,
			}, itemAttrs)
			widgets = append(widgets, itemLabel)
		}

		*yOffset += len(lines)
	}

	// Add spacing after list
	*yOffset += 1

	return widgets
}

// renderBlockquote renders a blockquote block
func (f *DefaultMarkdownViewFactory) renderBlockquote(parent tui.IWidget, block Block, yOffset *int, width int) tui.IWidget {
	attrs := gtv.CellTag("mdv-quote")
	attrs.Attributes |= gtv.AttrItalic

	lines := strings.Split(block.Content, "\n")
	startY := *yOffset

	for i, line := range lines {
		// Add border character
		quoteLine := "│ " + line

		tui.NewLabel(parent, quoteLine, gtv.TRect{
			X: 0,
			Y: uint16(*yOffset + i),
			W: uint16(width),
			H: 1,
		}, attrs)
	}

	*yOffset += len(lines)
	// Add spacing after blockquote
	*yOffset += 1

	return tui.NewWidget(parent, tui.WithRectangle(0, startY, width, len(lines)))
}

// renderHorizontalRule renders a horizontal rule
func (f *DefaultMarkdownViewFactory) renderHorizontalRule(parent tui.IWidget, yOffset *int, width int) tui.IWidget {
	attrs := gtv.CellTag("mdv-hr")

	// Create a line of horizontal box-drawing characters
	line := strings.Repeat("─", width)

	label := tui.NewLabel(parent, line, gtv.TRect{
		X: 0,
		Y: uint16(*yOffset),
		W: uint16(width),
		H: 1,
	}, attrs)

	*yOffset += 1
	// Add spacing after HR
	*yOffset += 1

	return label
}

// wordWrap wraps text to fit within the specified width
func wordWrap(text string, width int) []string {
	if width <= 0 {
		width = 80 // Default width
	}

	// Split by existing newlines first
	paragraphs := strings.Split(text, "\n")
	lines := []string{}

	for _, para := range paragraphs {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		currentLine := ""
		for _, word := range words {
			// Account for markdown formatting in length calculation
			// For simplicity, we'll use rune count
			wordLen := len([]rune(word))
			currentLen := len([]rune(currentLine))

			if currentLen == 0 {
				currentLine = word
			} else if currentLen+1+wordLen <= width {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}

		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	return lines
}
