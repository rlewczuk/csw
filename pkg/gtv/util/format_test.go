package util

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/stretchr/testify/assert"
)

// cellAttrs creates CellAttributes with NoColor for all color fields
// This matches the behavior of TextToCells after the theme color fix
func cellAttrs(attrs gtv.TextAttributes) gtv.CellAttributes {
	return gtv.CellAttributes{
		Attributes:  attrs,
		TextColor:   gtv.NoColor,
		BackColor:   gtv.NoColor,
		StrikeColor: gtv.NoColor,
	}
}

func TestTextToCells(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []gtv.Cell
	}{
		{
			name:  "plain text",
			input: "hello",
			expected: []gtv.Cell{
				{Rune: 'h', Attrs: cellAttrs(0)},
				{Rune: 'e', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'o', Attrs: cellAttrs(0)},
			},
		},
		{
			name:  "bold text",
			input: "**bold**",
			expected: []gtv.Cell{
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold)},
			},
		},
		{
			name:  "italic text",
			input: "*italic*",
			expected: []gtv.Cell{
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'a', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'c', Attrs: cellAttrs(gtv.AttrItalic)},
			},
		},
		{
			name:  "strikethrough text",
			input: "~~strike~~",
			expected: []gtv.Cell{
				{Rune: 's', Attrs: cellAttrs(gtv.AttrStrikethrough)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrStrikethrough)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrStrikethrough)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrStrikethrough)},
				{Rune: 'k', Attrs: cellAttrs(gtv.AttrStrikethrough)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrStrikethrough)},
			},
		},
		{
			name:  "underline text",
			input: "__under__",
			expected: []gtv.Cell{
				{Rune: 'u', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrUnderline)},
			},
		},
		{
			name:  "double underline text",
			input: "___double___",
			expected: []gtv.Cell{
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrDoubleUnderline)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrDoubleUnderline)},
				{Rune: 'u', Attrs: cellAttrs(gtv.AttrDoubleUnderline)},
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrDoubleUnderline)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrDoubleUnderline)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrDoubleUnderline)},
			},
		},
		{
			name:  "dim text",
			input: "%%dim%%",
			expected: []gtv.Cell{
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrDim)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrDim)},
				{Rune: 'm', Attrs: cellAttrs(gtv.AttrDim)},
			},
		},
		{
			name:  "blink text",
			input: "!!blink!!",
			expected: []gtv.Cell{
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBlink)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBlink)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrBlink)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrBlink)},
				{Rune: 'k', Attrs: cellAttrs(gtv.AttrBlink)},
			},
		},
		{
			name:  "reverse text",
			input: "<<reverse>>",
			expected: []gtv.Cell{
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrReverse)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrReverse)},
				{Rune: 'v', Attrs: cellAttrs(gtv.AttrReverse)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrReverse)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrReverse)},
				{Rune: 's', Attrs: cellAttrs(gtv.AttrReverse)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrReverse)},
			},
		},
		{
			name:  "bold and italic combined",
			input: "***bold* italic**",
			expected: []gtv.Cell{
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'a', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'c', Attrs: cellAttrs(gtv.AttrBold)},
			},
		},
		{
			name:  "bold and blink combined",
			input: "**!!bold blink!!**",
			expected: []gtv.Cell{
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
				{Rune: 'k', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrBlink)},
			},
		},
		{
			name:  "nested italic inside bold",
			input: "**bold *and italic* bold**",
			expected: []gtv.Cell{
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'a', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'a', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: 'c', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrItalic)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold)},
			},
		},
		{
			name:  "multiple attributes combined with nesting",
			input: "__!!underline and blink!!__",
			expected: []gtv.Cell{
				{Rune: 'u', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'a', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'k', Attrs: cellAttrs(gtv.AttrUnderline | gtv.AttrBlink)},
			},
		},
		{
			name:  "escaped asterisk",
			input: `\*not italic\*`,
			expected: []gtv.Cell{
				{Rune: '*', Attrs: cellAttrs(0)},
				{Rune: 'n', Attrs: cellAttrs(0)},
				{Rune: 'o', Attrs: cellAttrs(0)},
				{Rune: 't', Attrs: cellAttrs(0)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'i', Attrs: cellAttrs(0)},
				{Rune: 't', Attrs: cellAttrs(0)},
				{Rune: 'a', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'i', Attrs: cellAttrs(0)},
				{Rune: 'c', Attrs: cellAttrs(0)},
				{Rune: '*', Attrs: cellAttrs(0)},
			},
		},
		{
			name:  "escaped backslash",
			input: `\\backslash`,
			expected: []gtv.Cell{
				{Rune: '\\', Attrs: cellAttrs(0)},
				{Rune: 'b', Attrs: cellAttrs(0)},
				{Rune: 'a', Attrs: cellAttrs(0)},
				{Rune: 'c', Attrs: cellAttrs(0)},
				{Rune: 'k', Attrs: cellAttrs(0)},
				{Rune: 's', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'a', Attrs: cellAttrs(0)},
				{Rune: 's', Attrs: cellAttrs(0)},
				{Rune: 'h', Attrs: cellAttrs(0)},
			},
		},
		{
			name:  "escaped markers in styled text",
			input: `**bold \*\* still bold**`,
			expected: []gtv.Cell{
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: '*', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: '*', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 's', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold)},
			},
		},
		{
			name:  "mixed styled and plain text",
			input: "plain **bold** plain *italic* plain",
			expected: []gtv.Cell{
				{Rune: 'p', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'a', Attrs: cellAttrs(0)},
				{Rune: 'i', Attrs: cellAttrs(0)},
				{Rune: 'n', Attrs: cellAttrs(0)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'p', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'a', Attrs: cellAttrs(0)},
				{Rune: 'i', Attrs: cellAttrs(0)},
				{Rune: 'n', Attrs: cellAttrs(0)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'a', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'c', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'p', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'a', Attrs: cellAttrs(0)},
				{Rune: 'i', Attrs: cellAttrs(0)},
				{Rune: 'n', Attrs: cellAttrs(0)},
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []gtv.Cell{},
		},
		{
			name:  "unclosed marker treated as literal",
			input: "**bold unclosed",
			expected: []gtv.Cell{
				{Rune: '*', Attrs: cellAttrs(0)},
				{Rune: '*', Attrs: cellAttrs(0)},
				{Rune: 'b', Attrs: cellAttrs(0)},
				{Rune: 'o', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'd', Attrs: cellAttrs(0)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'u', Attrs: cellAttrs(0)},
				{Rune: 'n', Attrs: cellAttrs(0)},
				{Rune: 'c', Attrs: cellAttrs(0)},
				{Rune: 'l', Attrs: cellAttrs(0)},
				{Rune: 'o', Attrs: cellAttrs(0)},
				{Rune: 's', Attrs: cellAttrs(0)},
				{Rune: 'e', Attrs: cellAttrs(0)},
				{Rune: 'd', Attrs: cellAttrs(0)},
			},
		},
		{
			name:     "only markers",
			input:    "****",
			expected: []gtv.Cell{},
		},
		{
			name:  "triple attributes combination",
			input: "**__!!triple!!__**",
			expected: []gtv.Cell{
				{Rune: 't', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'p', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrUnderline | gtv.AttrBlink)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrBold | gtv.AttrUnderline | gtv.AttrBlink)},
			},
		},
		{
			name:  "dim and reverse combined",
			input: "%%<<dim reverse>>%%",
			expected: []gtv.Cell{
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 'm', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 'v', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 's', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrDim | gtv.AttrReverse)},
			},
		},
		{
			name:  "strikethrough and double underline",
			input: "___~~combo~~___",
			expected: []gtv.Cell{
				{Rune: 'c', Attrs: cellAttrs(gtv.AttrDoubleUnderline | gtv.AttrStrikethrough)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrDoubleUnderline | gtv.AttrStrikethrough)},
				{Rune: 'm', Attrs: cellAttrs(gtv.AttrDoubleUnderline | gtv.AttrStrikethrough)},
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrDoubleUnderline | gtv.AttrStrikethrough)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrDoubleUnderline | gtv.AttrStrikethrough)},
			},
		},
		{
			name:  "all escape sequences",
			input: `\*\~\_\%\!\<\>\\`,
			expected: []gtv.Cell{
				{Rune: '*', Attrs: cellAttrs(0)},
				{Rune: '~', Attrs: cellAttrs(0)},
				{Rune: '_', Attrs: cellAttrs(0)},
				{Rune: '%', Attrs: cellAttrs(0)},
				{Rune: '!', Attrs: cellAttrs(0)},
				{Rune: '<', Attrs: cellAttrs(0)},
				{Rune: '>', Attrs: cellAttrs(0)},
				{Rune: '\\', Attrs: cellAttrs(0)},
			},
		},
		{
			name:  "adjacent different styles",
			input: "**bold**__under__*italic*",
			expected: []gtv.Cell{
				{Rune: 'b', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'u', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'n', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'd', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrUnderline)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'a', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'l', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'i', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: 'c', Attrs: cellAttrs(gtv.AttrItalic)},
			},
		},
		{
			name:  "unicode characters with formatting",
			input: "**こんにちは**",
			expected: []gtv.Cell{
				{Rune: 'こ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'ん', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'に', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'ち', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'は', Attrs: cellAttrs(gtv.AttrBold)},
			},
		},
		{
			name:  "single character styles",
			input: "**x** *y* __z__",
			expected: []gtv.Cell{
				{Rune: 'x', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'y', Attrs: cellAttrs(gtv.AttrItalic)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'z', Attrs: cellAttrs(gtv.AttrUnderline)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TextToCells(tt.input)
			assert.Equal(t, tt.expected, result, "TextToCells(%q) mismatch", tt.input)
		})
	}
}

func TestTextToCells_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []gtv.Cell
	}{
		{
			name:  "marker at end of string",
			input: "text**",
			expected: []gtv.Cell{
				{Rune: 't', Attrs: cellAttrs(0)},
				{Rune: 'e', Attrs: cellAttrs(0)},
				{Rune: 'x', Attrs: cellAttrs(0)},
				{Rune: 't', Attrs: cellAttrs(0)},
				{Rune: '*', Attrs: cellAttrs(0)},
				{Rune: '*', Attrs: cellAttrs(0)},
			},
		},
		{
			name:  "escape at end of string",
			input: `text\`,
			expected: []gtv.Cell{
				{Rune: 't', Attrs: cellAttrs(0)},
				{Rune: 'e', Attrs: cellAttrs(0)},
				{Rune: 'x', Attrs: cellAttrs(0)},
				{Rune: 't', Attrs: cellAttrs(0)},
				{Rune: '\\', Attrs: cellAttrs(0)},
			},
		},
		{
			name:  "nested same type markers",
			input: "**outer **inner** outer**",
			expected: []gtv.Cell{
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'u', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'i', Attrs: cellAttrs(0)},
				{Rune: 'n', Attrs: cellAttrs(0)},
				{Rune: 'n', Attrs: cellAttrs(0)},
				{Rune: 'e', Attrs: cellAttrs(0)},
				{Rune: 'r', Attrs: cellAttrs(0)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'u', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 't', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'e', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: 'r', Attrs: cellAttrs(gtv.AttrBold)},
			},
		},
		{
			name:  "mismatched markers count",
			input: "***three asterisks",
			expected: []gtv.Cell{
				{Rune: '*', Attrs: cellAttrs(0)},
				{Rune: '*', Attrs: cellAttrs(0)},
				{Rune: '*', Attrs: cellAttrs(0)},
				{Rune: 't', Attrs: cellAttrs(0)},
				{Rune: 'h', Attrs: cellAttrs(0)},
				{Rune: 'r', Attrs: cellAttrs(0)},
				{Rune: 'e', Attrs: cellAttrs(0)},
				{Rune: 'e', Attrs: cellAttrs(0)},
				{Rune: ' ', Attrs: cellAttrs(0)},
				{Rune: 'a', Attrs: cellAttrs(0)},
				{Rune: 's', Attrs: cellAttrs(0)},
				{Rune: 't', Attrs: cellAttrs(0)},
				{Rune: 'e', Attrs: cellAttrs(0)},
				{Rune: 'r', Attrs: cellAttrs(0)},
				{Rune: 'i', Attrs: cellAttrs(0)},
				{Rune: 's', Attrs: cellAttrs(0)},
				{Rune: 'k', Attrs: cellAttrs(0)},
				{Rune: 's', Attrs: cellAttrs(0)},
			},
		},
		{
			name:     "empty styled text",
			input:    "****",
			expected: []gtv.Cell{},
		},
		{
			name:  "whitespace only styled",
			input: "**   **",
			expected: []gtv.Cell{
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
				{Rune: ' ', Attrs: cellAttrs(gtv.AttrBold)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TextToCells(tt.input)
			assert.Equal(t, tt.expected, result, "TextToCells(%q) mismatch", tt.input)
		})
	}
}

func TestEscapeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "plain text without special characters",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "text with asterisks",
			input:    "**bold**",
			expected: `\*\*bold\*\*`,
		},
		{
			name:     "text with single asterisk",
			input:    "*italic*",
			expected: `\*italic\*`,
		},
		{
			name:     "text with tildes",
			input:    "~~strikethrough~~",
			expected: `\~\~strikethrough\~\~`,
		},
		{
			name:     "text with underscores",
			input:    "__underline__",
			expected: `\_\_underline\_\_`,
		},
		{
			name:     "text with triple underscores",
			input:    "___double underline___",
			expected: `\_\_\_double underline\_\_\_`,
		},
		{
			name:     "text with percent signs",
			input:    "%%dim%%",
			expected: `\%\%dim\%\%`,
		},
		{
			name:     "text with exclamation marks",
			input:    "!!blink!!",
			expected: `\!\!blink\!\!`,
		},
		{
			name:     "text with angle brackets",
			input:    "<<reverse>>",
			expected: `\<\<reverse\>\>`,
		},
		{
			name:     "text with backslashes",
			input:    `\backslash\`,
			expected: `\\backslash\\`,
		},
		{
			name:     "all special characters",
			input:    `*~_%!<>\`,
			expected: `\*\~\_\%\!\<\>\\`,
		},
		{
			name:     "mixed text with formatting",
			input:    "plain **bold** plain",
			expected: `plain \*\*bold\*\* plain`,
		},
		{
			name:     "nested formatting syntax",
			input:    "**bold *and italic* bold**",
			expected: `\*\*bold \*and italic\* bold\*\*`,
		},
		{
			name:     "unicode text with special chars",
			input:    "**こんにちは**",
			expected: `\*\*こんにちは\*\*`,
		},
		{
			name:     "single special character",
			input:    "*",
			expected: `\*`,
		},
		{
			name:     "multiple consecutive special chars",
			input:    "****",
			expected: `\*\*\*\*`,
		},
		{
			name:     "text with already escaped chars",
			input:    `\*escaped\*`,
			expected: `\\\*escaped\\\*`,
		},
		{
			name:     "complex nested formatting",
			input:    "**__!!triple!!__**",
			expected: `\*\*\_\_\!\!triple\!\!\_\_\*\*`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeText(tt.input)
			assert.Equal(t, tt.expected, result, "EscapeText(%q) mismatch", tt.input)
		})
	}
}

func TestEscapeText_RoundTrip(t *testing.T) {
	// Test that escaped text is rendered verbatim by TextToCells
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "bold markers",
			input: "**bold**",
		},
		{
			name:  "italic markers",
			input: "*italic*",
		},
		{
			name:  "strikethrough markers",
			input: "~~strike~~",
		},
		{
			name:  "underline markers",
			input: "__underline__",
		},
		{
			name:  "double underline markers",
			input: "___double___",
		},
		{
			name:  "dim markers",
			input: "%%dim%%",
		},
		{
			name:  "blink markers",
			input: "!!blink!!",
		},
		{
			name:  "reverse markers",
			input: "<<reverse>>",
		},
		{
			name:  "backslash",
			input: `\backslash\`,
		},
		{
			name:  "all special characters",
			input: `*~_%!<>\`,
		},
		{
			name:  "complex nested formatting",
			input: "**bold *and italic* bold**",
		},
		{
			name:  "mixed special chars",
			input: "Hello **world** with ~~formatting~~!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Escape the input
			escaped := EscapeText(tt.input)

			// Convert escaped text to cells
			cells := TextToCells(escaped)

			// Extract runes from cells (should be verbatim input)
			var result []rune
			for _, cell := range cells {
				result = append(result, cell.Rune)
			}
			resultStr := string(result)

			// Verify that the result matches the original input
			assert.Equal(t, tt.input, resultStr,
				"Round trip failed: EscapeText(%q) -> TextToCells() should produce verbatim text", tt.input)

			// Verify that all cells have no attributes (no formatting applied)
			for i, cell := range cells {
				assert.Equal(t, cellAttrs(0), cell.Attrs,
					"Cell %d should have no attributes, got %v", i, cell.Attrs)
			}
		})
	}
}
