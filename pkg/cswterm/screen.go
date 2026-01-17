package cswterm

type TextAttributes uint32

const (
	AttrBold TextAttributes = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrDoubleUnderline
	AttrCurlyUnderline
	AttrDottedUnderline
	AttrDashedUnderline
	AttrBlink
	AttrReverse
	AttrHidden
	AttrStrikethrough
	Overline
)

type TextColor uint32

type CellAttributes struct {
	// Attrs is a bitfield of attributes. It is a combination of the following:
	Attributes TextAttributes
	// Foreground sets color of the text. It is 24-bit color in RGB format.
	TextColor uint32
	// Background sets color of the background. It is 24-bit color in RGB format.
	BackColor uint32
	// StrikeColor sets color of the strike through line. It is 24-bit color in RGB format.
	StrikeColor uint32
}

// Screen represents a terminal screen. It consists of a grid of cells.
// Each cell contains a rune and a set of attributes.
type Screen interface {

	// Size returns the size of the screen in characters.
	Size() (width int, height int)

	// PutText puts text at the specified position with the specified attributes.
	// if the text is longer than the width of the screen, it is truncated.
	PutText(x int, y int, text string, attrs TextAttributes)
}
