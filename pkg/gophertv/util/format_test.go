package util

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gophertv"
	"github.com/stretchr/testify/assert"
)

func TestTextToCells(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []gophertv.Cell
	}{
		{
			name:  "plain text",
			input: "hello",
			expected: []gophertv.Cell{
				{Rune: 'h', Attrs: gophertv.CellAttributes{}},
				{Rune: 'e', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'o', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:  "bold text",
			input: "**bold**",
			expected: []gophertv.Cell{
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold)},
			},
		},
		{
			name:  "italic text",
			input: "*italic*",
			expected: []gophertv.Cell{
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'a', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'c', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
			},
		},
		{
			name:  "strikethrough text",
			input: "~~strike~~",
			expected: []gophertv.Cell{
				{Rune: 's', Attrs: gophertv.Attrs(gophertv.AttrStrikethrough)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrStrikethrough)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrStrikethrough)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrStrikethrough)},
				{Rune: 'k', Attrs: gophertv.Attrs(gophertv.AttrStrikethrough)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrStrikethrough)},
			},
		},
		{
			name:  "underline text",
			input: "__under__",
			expected: []gophertv.Cell{
				{Rune: 'u', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
			},
		},
		{
			name:  "double underline text",
			input: "___double___",
			expected: []gophertv.Cell{
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline)},
				{Rune: 'u', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline)},
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline)},
			},
		},
		{
			name:  "dim text",
			input: "%%dim%%",
			expected: []gophertv.Cell{
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrDim)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrDim)},
				{Rune: 'm', Attrs: gophertv.Attrs(gophertv.AttrDim)},
			},
		},
		{
			name:  "blink text",
			input: "!!blink!!",
			expected: []gophertv.Cell{
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBlink)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBlink)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrBlink)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrBlink)},
				{Rune: 'k', Attrs: gophertv.Attrs(gophertv.AttrBlink)},
			},
		},
		{
			name:  "reverse text",
			input: "<<reverse>>",
			expected: []gophertv.Cell{
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrReverse)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrReverse)},
				{Rune: 'v', Attrs: gophertv.Attrs(gophertv.AttrReverse)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrReverse)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrReverse)},
				{Rune: 's', Attrs: gophertv.Attrs(gophertv.AttrReverse)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrReverse)},
			},
		},
		{
			name:  "bold and italic combined",
			input: "***bold* italic**",
			expected: []gophertv.Cell{
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'a', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'c', Attrs: gophertv.Attrs(gophertv.AttrBold)},
			},
		},
		{
			name:  "bold and blink combined",
			input: "**!!bold blink!!**",
			expected: []gophertv.Cell{
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
				{Rune: 'k', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrBlink)},
			},
		},
		{
			name:  "nested italic inside bold",
			input: "**bold *and italic* bold**",
			expected: []gophertv.Cell{
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'a', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'a', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: 'c', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrItalic)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold)},
			},
		},
		{
			name:  "multiple attributes combined with nesting",
			input: "__!!underline and blink!!__",
			expected: []gophertv.Cell{
				{Rune: 'u', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'a', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'k', Attrs: gophertv.Attrs(gophertv.AttrUnderline | gophertv.AttrBlink)},
			},
		},
		{
			name:  "escaped asterisk",
			input: `\*not italic\*`,
			expected: []gophertv.Cell{
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
				{Rune: 'n', Attrs: gophertv.CellAttributes{}},
				{Rune: 'o', Attrs: gophertv.CellAttributes{}},
				{Rune: 't', Attrs: gophertv.CellAttributes{}},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'i', Attrs: gophertv.CellAttributes{}},
				{Rune: 't', Attrs: gophertv.CellAttributes{}},
				{Rune: 'a', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'i', Attrs: gophertv.CellAttributes{}},
				{Rune: 'c', Attrs: gophertv.CellAttributes{}},
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:  "escaped backslash",
			input: `\\backslash`,
			expected: []gophertv.Cell{
				{Rune: '\\', Attrs: gophertv.CellAttributes{}},
				{Rune: 'b', Attrs: gophertv.CellAttributes{}},
				{Rune: 'a', Attrs: gophertv.CellAttributes{}},
				{Rune: 'c', Attrs: gophertv.CellAttributes{}},
				{Rune: 'k', Attrs: gophertv.CellAttributes{}},
				{Rune: 's', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'a', Attrs: gophertv.CellAttributes{}},
				{Rune: 's', Attrs: gophertv.CellAttributes{}},
				{Rune: 'h', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:  "escaped markers in styled text",
			input: `**bold \*\* still bold**`,
			expected: []gophertv.Cell{
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: '*', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: '*', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 's', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold)},
			},
		},
		{
			name:  "mixed styled and plain text",
			input: "plain **bold** plain *italic* plain",
			expected: []gophertv.Cell{
				{Rune: 'p', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'a', Attrs: gophertv.CellAttributes{}},
				{Rune: 'i', Attrs: gophertv.CellAttributes{}},
				{Rune: 'n', Attrs: gophertv.CellAttributes{}},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'p', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'a', Attrs: gophertv.CellAttributes{}},
				{Rune: 'i', Attrs: gophertv.CellAttributes{}},
				{Rune: 'n', Attrs: gophertv.CellAttributes{}},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'a', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'c', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'p', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'a', Attrs: gophertv.CellAttributes{}},
				{Rune: 'i', Attrs: gophertv.CellAttributes{}},
				{Rune: 'n', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []gophertv.Cell{},
		},
		{
			name:  "unclosed marker treated as literal",
			input: "**bold unclosed",
			expected: []gophertv.Cell{
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
				{Rune: 'b', Attrs: gophertv.CellAttributes{}},
				{Rune: 'o', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'd', Attrs: gophertv.CellAttributes{}},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'u', Attrs: gophertv.CellAttributes{}},
				{Rune: 'n', Attrs: gophertv.CellAttributes{}},
				{Rune: 'c', Attrs: gophertv.CellAttributes{}},
				{Rune: 'l', Attrs: gophertv.CellAttributes{}},
				{Rune: 'o', Attrs: gophertv.CellAttributes{}},
				{Rune: 's', Attrs: gophertv.CellAttributes{}},
				{Rune: 'e', Attrs: gophertv.CellAttributes{}},
				{Rune: 'd', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:     "only markers",
			input:    "****",
			expected: []gophertv.Cell{},
		},
		{
			name:  "triple attributes combination",
			input: "**__!!triple!!__**",
			expected: []gophertv.Cell{
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'p', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrUnderline | gophertv.AttrBlink)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrBold | gophertv.AttrUnderline | gophertv.AttrBlink)},
			},
		},
		{
			name:  "dim and reverse combined",
			input: "%%<<dim reverse>>%%",
			expected: []gophertv.Cell{
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 'm', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 'v', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 's', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrDim | gophertv.AttrReverse)},
			},
		},
		{
			name:  "strikethrough and double underline",
			input: "___~~combo~~___",
			expected: []gophertv.Cell{
				{Rune: 'c', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline | gophertv.AttrStrikethrough)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline | gophertv.AttrStrikethrough)},
				{Rune: 'm', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline | gophertv.AttrStrikethrough)},
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline | gophertv.AttrStrikethrough)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrDoubleUnderline | gophertv.AttrStrikethrough)},
			},
		},
		{
			name:  "all escape sequences",
			input: `\*\~\_\%\!\<\>\\`,
			expected: []gophertv.Cell{
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
				{Rune: '~', Attrs: gophertv.CellAttributes{}},
				{Rune: '_', Attrs: gophertv.CellAttributes{}},
				{Rune: '%', Attrs: gophertv.CellAttributes{}},
				{Rune: '!', Attrs: gophertv.CellAttributes{}},
				{Rune: '<', Attrs: gophertv.CellAttributes{}},
				{Rune: '>', Attrs: gophertv.CellAttributes{}},
				{Rune: '\\', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:  "adjacent different styles",
			input: "**bold**__under__*italic*",
			expected: []gophertv.Cell{
				{Rune: 'b', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'u', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'n', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'd', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'a', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'l', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'i', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: 'c', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
			},
		},
		{
			name:  "unicode characters with formatting",
			input: "**こんにちは**",
			expected: []gophertv.Cell{
				{Rune: 'こ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'ん', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'に', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'ち', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'は', Attrs: gophertv.Attrs(gophertv.AttrBold)},
			},
		},
		{
			name:  "single character styles",
			input: "**x** *y* __z__",
			expected: []gophertv.Cell{
				{Rune: 'x', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'y', Attrs: gophertv.Attrs(gophertv.AttrItalic)},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'z', Attrs: gophertv.Attrs(gophertv.AttrUnderline)},
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
		expected []gophertv.Cell
	}{
		{
			name:  "marker at end of string",
			input: "text**",
			expected: []gophertv.Cell{
				{Rune: 't', Attrs: gophertv.CellAttributes{}},
				{Rune: 'e', Attrs: gophertv.CellAttributes{}},
				{Rune: 'x', Attrs: gophertv.CellAttributes{}},
				{Rune: 't', Attrs: gophertv.CellAttributes{}},
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:  "escape at end of string",
			input: `text\`,
			expected: []gophertv.Cell{
				{Rune: 't', Attrs: gophertv.CellAttributes{}},
				{Rune: 'e', Attrs: gophertv.CellAttributes{}},
				{Rune: 'x', Attrs: gophertv.CellAttributes{}},
				{Rune: 't', Attrs: gophertv.CellAttributes{}},
				{Rune: '\\', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:  "nested same type markers",
			input: "**outer **inner** outer**",
			expected: []gophertv.Cell{
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'u', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'i', Attrs: gophertv.CellAttributes{}},
				{Rune: 'n', Attrs: gophertv.CellAttributes{}},
				{Rune: 'n', Attrs: gophertv.CellAttributes{}},
				{Rune: 'e', Attrs: gophertv.CellAttributes{}},
				{Rune: 'r', Attrs: gophertv.CellAttributes{}},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'o', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'u', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 't', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'e', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: 'r', Attrs: gophertv.Attrs(gophertv.AttrBold)},
			},
		},
		{
			name:  "mismatched markers count",
			input: "***three asterisks",
			expected: []gophertv.Cell{
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
				{Rune: '*', Attrs: gophertv.CellAttributes{}},
				{Rune: 't', Attrs: gophertv.CellAttributes{}},
				{Rune: 'h', Attrs: gophertv.CellAttributes{}},
				{Rune: 'r', Attrs: gophertv.CellAttributes{}},
				{Rune: 'e', Attrs: gophertv.CellAttributes{}},
				{Rune: 'e', Attrs: gophertv.CellAttributes{}},
				{Rune: ' ', Attrs: gophertv.CellAttributes{}},
				{Rune: 'a', Attrs: gophertv.CellAttributes{}},
				{Rune: 's', Attrs: gophertv.CellAttributes{}},
				{Rune: 't', Attrs: gophertv.CellAttributes{}},
				{Rune: 'e', Attrs: gophertv.CellAttributes{}},
				{Rune: 'r', Attrs: gophertv.CellAttributes{}},
				{Rune: 'i', Attrs: gophertv.CellAttributes{}},
				{Rune: 's', Attrs: gophertv.CellAttributes{}},
				{Rune: 'k', Attrs: gophertv.CellAttributes{}},
				{Rune: 's', Attrs: gophertv.CellAttributes{}},
			},
		},
		{
			name:     "empty styled text",
			input:    "****",
			expected: []gophertv.Cell{},
		},
		{
			name:  "whitespace only styled",
			input: "**   **",
			expected: []gophertv.Cell{
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
				{Rune: ' ', Attrs: gophertv.Attrs(gophertv.AttrBold)},
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
				assert.Equal(t, gophertv.CellAttributes{}, cell.Attrs,
					"Cell %d should have no attributes, got %v", i, cell.Attrs)
			}
		})
	}
}
