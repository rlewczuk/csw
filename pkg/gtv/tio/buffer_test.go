package tio

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a ScreenVerifier from a ScreenBuffer
func getVerifier(screen *ScreenBuffer) *gtv.ScreenVerifier {
	width, height, content := screen.GetContent()
	return gtv.NewScreenVerifier(width, height, content)
}

// Helper function to create a ScreenVerifier with cursor info from a ScreenBuffer
func getVerifierWithCursor(screen *ScreenBuffer) *gtv.ScreenVerifier {
	width, height, content := screen.GetContent()
	cursorX, cursorY := screen.GetCursorPosition()
	cursorStyle := screen.GetCursorStyle()
	return gtv.NewScreenVerifierWithCursor(width, height, content, cursorX, cursorY, cursorStyle)
}

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
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			require.NotNil(t, screen)

			w, h := screen.GetSize()
			assert.Equal(t, tt.width, w)
			assert.Equal(t, tt.height, h)

			// Verify all cells are initialized with spaces
			verifier := getVerifier(screen)
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					cell := verifier.GetCell(x, y)
					assert.Equal(t, ' ', cell.Rune)
					assert.Equal(t, gtv.CellAttributes{}, cell.Attrs)
				}
			}
		})
	}
}

func TestMockScreen_Size(t *testing.T) {
	screen := NewScreenBuffer(100, 50, 0)
	w, h := screen.GetSize()
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
		attrs     gtv.TextAttributes
		wantText  string
		wantAttrs gtv.TextAttributes
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
			attrs:     gtv.AttrBold,
			wantText:  "Hello",
			wantAttrs: gtv.AttrBold,
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
			attrs:     gtv.AttrItalic | gtv.AttrUnderline,
			wantText:  "World",
			wantAttrs: gtv.AttrItalic | gtv.AttrUnderline,
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
			attrs:     gtv.AttrBold,
			wantText:  "Hello 世界",
			wantAttrs: gtv.AttrBold,
			checkX:    0,
			checkY:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutText(gtv.TRect{X: uint16(tt.x), Y: uint16(tt.y), W: 0, H: 0}, tt.text, gtv.Attrs(tt.attrs))

			// Verify each character
			verifier := getVerifier(screen)
			runes := []rune(tt.wantText)
			for i, r := range runes {
				cell := verifier.GetCell(tt.checkX+i, tt.checkY)
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
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutText(gtv.TRect{X: uint16(tt.x), Y: uint16(tt.y), W: 0, H: 0}, tt.text, gtv.Attrs(gtv.AttrBold))

			// Count non-space characters
			verifier := getVerifier(screen)
			count := 0
			for x := tt.x; x < tt.width; x++ {
				cell := verifier.GetCell(x, tt.y)
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
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			// Should not panic
			screen.PutText(gtv.TRect{X: uint16(tt.x), Y: uint16(tt.y), W: 0, H: 0}, tt.text, gtv.Attrs(gtv.AttrBold))

			// Verify screen is still all spaces
			verifier := getVerifier(screen)
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					cell := verifier.GetCell(x, y)
					assert.Equal(t, ' ', cell.Rune)
				}
			}
		})
	}
}

func TestMockScreen_GetCell(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	screen.PutText(gtv.TRect{X: 2, Y: 1, W: 0, H: 0}, "Test", gtv.Attrs(gtv.AttrBold))
	verifier := getVerifier(screen)

	tests := []struct {
		name      string
		x         int
		y         int
		wantRune  rune
		wantAttrs gtv.TextAttributes
	}{
		{
			name:      "first character",
			x:         2,
			y:         1,
			wantRune:  'T',
			wantAttrs: gtv.AttrBold,
		},
		{
			name:      "middle character",
			x:         3,
			y:         1,
			wantRune:  'e',
			wantAttrs: gtv.AttrBold,
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
			cell := verifier.GetCell(tt.x, tt.y)
			assert.Equal(t, tt.wantRune, cell.Rune)
			assert.Equal(t, tt.wantAttrs, cell.Attrs.Attributes)
		})
	}
}

func TestMockScreen_GetText(t *testing.T) {
	screen := NewScreenBuffer(20, 10, 0)
	screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", gtv.Attrs(0))
	screen.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "World", gtv.Attrs(0))
	screen.PutText(gtv.TRect{X: 5, Y: 2, W: 0, H: 0}, "Test", gtv.Attrs(0))
	verifier := getVerifier(screen)

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
			got := verifier.GetText(tt.x, tt.y, tt.width, tt.height)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockScreen_HasText(t *testing.T) {
	screen := NewScreenBuffer(20, 10, 0)
	screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", gtv.Attrs(0))
	screen.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "World", gtv.Attrs(0))
	screen.PutText(gtv.TRect{X: 5, Y: 2, W: 0, H: 0}, "Test", gtv.Attrs(0))
	verifier := getVerifier(screen)

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
			got := verifier.HasText(tt.x, tt.y, tt.width, tt.height, tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockScreen_HasTextWithAttrs(t *testing.T) {
	screen := NewScreenBuffer(20, 10, 0)
	screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Bold", gtv.Attrs(gtv.AttrBold))
	screen.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "Italic", gtv.Attrs(gtv.AttrItalic))
	screen.PutText(gtv.TRect{X: 0, Y: 2, W: 0, H: 0}, "Both", gtv.Attrs(gtv.AttrBold|gtv.AttrItalic))
	verifier := getVerifier(screen)

	tests := []struct {
		name   string
		x      int
		y      int
		width  int
		height int
		text   string
		mask   gtv.AttributeMask
		want   bool
	}{
		{
			name:   "check bold attribute",
			x:      0,
			y:      0,
			width:  4,
			height: 1,
			text:   "Bold",
			mask: gtv.AttributeMask{
				CheckAttributes: true,
				Attributes:      gtv.AttrBold,
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
			mask: gtv.AttributeMask{
				CheckAttributes: true,
				Attributes:      gtv.AttrItalic,
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
			mask: gtv.AttributeMask{
				CheckAttributes: true,
				Attributes:      gtv.AttrBold | gtv.AttrItalic,
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
			mask: gtv.AttributeMask{
				CheckAttributes: true,
				Attributes:      gtv.AttrItalic,
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
			mask: gtv.AttributeMask{
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
			mask: gtv.AttributeMask{
				CheckAttributes: true,
				Attributes:      gtv.AttrBold,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifier.HasTextWithAttrs(tt.x, tt.y, tt.width, tt.height, tt.text, tt.mask)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockScreen_HasTextWithAttrs_Colors(t *testing.T) {
	screen := NewScreenBuffer(20, 10, 0)

	// Create cells with specific colors manually
	redColor := gtv.TextColor(0xFF0000)
	blueColor := gtv.TextColor(0x0000FF)
	greenBack := gtv.TextColor(0x00FF00)

	// Put text and then modify colors
	screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Red", gtv.Attrs(0))
	width, _, _ := screen.GetContent()
	for i := 0; i < 3; i++ {
		idx := 0*width + i
		screen.buffer[idx].Attrs.TextColor = redColor
	}

	screen.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "Blue", gtv.Attrs(0))
	for i := 0; i < 4; i++ {
		idx := 1*width + i
		screen.buffer[idx].Attrs.TextColor = blueColor
		screen.buffer[idx].Attrs.BackColor = greenBack
	}

	tests := []struct {
		name   string
		x      int
		y      int
		width  int
		height int
		text   string
		mask   gtv.AttributeMask
		want   bool
	}{
		{
			name:   "check text color",
			x:      0,
			y:      0,
			width:  3,
			height: 1,
			text:   "Red",
			mask: gtv.AttributeMask{
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
			mask: gtv.AttributeMask{
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
			mask: gtv.AttributeMask{
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
			mask: gtv.AttributeMask{
				CheckBackColor: true,
				BackColor:      greenBack,
			},
			want: true,
		},
	}

	verifier := getVerifier(screen)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifier.HasTextWithAttrs(tt.x, tt.y, tt.width, tt.height, tt.text, tt.mask)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMockScreen_Clear(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", gtv.Attrs(gtv.AttrBold))
	screen.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "World", gtv.Attrs(gtv.AttrItalic))

	// Verify text is present
	verifier := getVerifier(screen)
	assert.True(t, verifier.HasText(0, 0, 5, 1, "Hello"))
	assert.True(t, verifier.HasText(0, 1, 5, 1, "World"))

	screen.Clear()

	// Verify all cells are spaces with no attributes
	verifier = getVerifier(screen)
	for y := 0; y < 5; y++ {
		for x := 0; x < 10; x++ {
			cell := verifier.GetCell(x, y)
			assert.Equal(t, ' ', cell.Rune)
			assert.Equal(t, gtv.CellAttributes{}, cell.Attrs)
		}
	}
}

func TestMockScreen_InterfaceCompliance(t *testing.T) {
	var _ gtv.IScreenOutput = (*ScreenBuffer)(nil)
}

func TestAttributeMask_Partial(t *testing.T) {
	screen := NewScreenBuffer(20, 10, 0)

	// Create text with bold and italic
	screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Text", gtv.Attrs(gtv.AttrBold|gtv.AttrItalic|gtv.AttrUnderline))
	width, _, _ := screen.GetContent()
	for i := 0; i < 4; i++ {
		idx := 0*width + i
		screen.buffer[idx].Attrs.TextColor = 0xFF0000
	}

	verifier := getVerifier(screen)
	tests := []struct {
		name string
		mask gtv.AttributeMask
		want bool
	}{
		{
			name: "check only bold (ignore italic and underline)",
			mask: gtv.AttributeMask{
				CheckAttributes: true,
				Attributes:      gtv.AttrBold | gtv.AttrItalic | gtv.AttrUnderline,
			},
			want: true,
		},
		{
			name: "check only text color (ignore attributes)",
			mask: gtv.AttributeMask{
				CheckTextColor: true,
				TextColor:      0xFF0000,
			},
			want: true,
		},
		{
			name: "check both color and attributes",
			mask: gtv.AttributeMask{
				CheckAttributes: true,
				CheckTextColor:  true,
				Attributes:      gtv.AttrBold | gtv.AttrItalic | gtv.AttrUnderline,
				TextColor:       0xFF0000,
			},
			want: true,
		},
		{
			name: "check nothing - always matches",
			mask: gtv.AttributeMask{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifier.HasTextWithAttrs(0, 0, 4, 1, "Text", tt.mask)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScreenBuffer_Listen_Notify(t *testing.T) {
	tests := []struct {
		name        string
		queueSize   int
		eventCount  int
		channelSize int
		wantEvents  int
	}{
		{
			name:        "single event to single listener",
			queueSize:   10,
			eventCount:  1,
			channelSize: 1,
			wantEvents:  1,
		},
		{
			name:        "multiple events to single listener",
			queueSize:   10,
			eventCount:  5,
			channelSize: 5,
			wantEvents:  5,
		},
		{
			name:        "events with buffered channel",
			queueSize:   10,
			eventCount:  3,
			channelSize: 10,
			wantEvents:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(10, 5, tt.queueSize)
			ch := make(chan gtv.InputEvent, tt.channelSize)
			screen.Listen(ch)

			// Send events
			for i := 0; i < tt.eventCount; i++ {
				event := gtv.InputEvent{
					Type: gtv.InputEventKey,
					Key:  rune('a' + i),
				}
				screen.Notify(event)
			}

			// Receive events
			receivedCount := 0
			for i := 0; i < tt.wantEvents; i++ {
				select {
				case ev := <-ch:
					assert.Equal(t, gtv.InputEventKey, ev.Type)
					assert.Equal(t, rune('a'+i), ev.Key)
					receivedCount++
				default:
					t.Fatalf("ScreenBuffer.Notify() at buffer_test.go: expected event %d but channel is empty", i)
				}
			}

			assert.Equal(t, tt.wantEvents, receivedCount)
		})
	}
}

func TestScreenBuffer_Notify_MultipleListeners(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 10)

	// Create multiple listeners
	ch1 := make(chan gtv.InputEvent, 10)
	ch2 := make(chan gtv.InputEvent, 10)
	ch3 := make(chan gtv.InputEvent, 10)

	screen.Listen(ch1)
	screen.Listen(ch2)
	screen.Listen(ch3)

	// Send an event
	event := gtv.InputEvent{
		Type: gtv.InputEventKey,
		Key:  'x',
	}
	screen.Notify(event)

	// All listeners should receive the event
	ev1 := <-ch1
	ev2 := <-ch2
	ev3 := <-ch3

	assert.Equal(t, gtv.InputEventKey, ev1.Type)
	assert.Equal(t, 'x', ev1.Key)
	assert.Equal(t, gtv.InputEventKey, ev2.Type)
	assert.Equal(t, 'x', ev2.Key)
	assert.Equal(t, gtv.InputEventKey, ev3.Type)
	assert.Equal(t, 'x', ev3.Key)
}

func TestScreenBuffer_Notify_FullChannel_Queue(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 10)

	// Create a channel with size 1
	ch := make(chan gtv.InputEvent, 1)
	screen.Listen(ch)

	// Send first event - should go directly to channel
	event1 := gtv.InputEvent{Type: gtv.InputEventKey, Key: '1'}
	screen.Notify(event1)

	// Verify channel has the first event
	select {
	case ev := <-ch:
		assert.Equal(t, '1', ev.Key)
	default:
		t.Fatal("ScreenBuffer.Notify() at buffer_test.go: expected event in channel")
	}

	// Now channel is empty, send multiple events to fill channel and queue
	screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: '2'})
	screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: '3'})
	screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: '4'})

	// Receive events - queued events should be delivered when we make space
	for i := 2; i <= 4; i++ {
		// Read from channel
		var ev gtv.InputEvent
		select {
		case ev = <-ch:
			// Got an event
		default:
			// Channel empty, notify to trigger queue delivery
			screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: 'X'})
			ev = <-ch
		}
		expectedKey := rune('0' + i)
		assert.Equal(t, expectedKey, ev.Key, "ScreenBuffer.Notify() at buffer_test.go: event mismatch at position %d", i)
	}
}

func TestScreenBuffer_Notify_QueueOverflow(t *testing.T) {
	queueSize := 3
	screen := NewScreenBuffer(10, 5, queueSize)

	// Create a buffered channel
	ch := make(chan gtv.InputEvent, 1)
	screen.Listen(ch)

	// Fill the channel first
	screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: '0'})

	// Now send more events than queue size - they will be queued
	// Send queueSize + 2 more events (total queueSize + 3)
	for i := 1; i < queueSize+3; i++ {
		event := gtv.InputEvent{Type: gtv.InputEventKey, Key: rune('0' + i)}
		screen.Notify(event)
	}

	// Read the first event from channel
	ev := <-ch
	assert.Equal(t, '0', ev.Key)

	// Notify to trigger delivery of queued events
	screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: 'X'})

	// Count how many events we can receive
	// We should get at most queueSize events (oldest ones were dropped)
	receivedCount := 0
	for {
		select {
		case ev := <-ch:
			receivedCount++
			t.Logf("ScreenBuffer.Notify() at buffer_test.go: received event with key %c", ev.Key)
		default:
			// No more events available
			goto done
		}
	}
done:
	// We should have received at most queueSize + 1 events
	// (queue size + the trigger event 'X')
	assert.LessOrEqual(t, receivedCount, queueSize+1, "ScreenBuffer.Notify() at buffer_test.go: received too many events")
}

func TestScreenBuffer_Notify_ClosedChannel(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 10)

	// Create and register a channel
	ch := make(chan gtv.InputEvent, 10)
	screen.Listen(ch)

	// Send an event
	event1 := gtv.InputEvent{Type: gtv.InputEventKey, Key: 'a'}
	screen.Notify(event1)

	// Receive it
	ev := <-ch
	assert.Equal(t, 'a', ev.Key)

	// Close the channel
	close(ch)

	// Send another event - should not panic
	event2 := gtv.InputEvent{Type: gtv.InputEventKey, Key: 'b'}
	require.NotPanics(t, func() {
		screen.Notify(event2)
	})
}

func TestScreenBuffer_Notify_EventOrder(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 100)

	ch := make(chan gtv.InputEvent, 1)
	screen.Listen(ch)

	// Fill the channel
	screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: '0'})

	// Queue multiple events
	for i := 1; i <= 10; i++ {
		screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: rune('0' + i)})
	}

	// Receive all events and verify order
	for i := 0; i <= 10; i++ {
		// Read from channel to make space
		ev := <-ch

		// Trigger delivery of queued events
		if i < 10 {
			screen.Notify(gtv.InputEvent{Type: gtv.InputEventKey, Key: 'X'})
		}

		expectedKey := rune('0' + i)
		assert.Equal(t, expectedKey, ev.Key, "ScreenBuffer.Notify() at buffer_test.go: event order mismatch at position %d", i)
	}
}

func TestScreenBuffer_Notify_NoListeners(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 10)

	// Send event with no listeners - should not panic
	event := gtv.InputEvent{Type: gtv.InputEventKey, Key: 'a'}
	require.NotPanics(t, func() {
		screen.Notify(event)
	})
}

func TestScreenBuffer_Listen_MultipleRegistrations(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 10)

	ch := make(chan gtv.InputEvent, 10)

	// Register the same channel multiple times
	screen.Listen(ch)
	screen.Listen(ch)

	// Send an event
	event := gtv.InputEvent{Type: gtv.InputEventKey, Key: 'a'}
	screen.Notify(event)

	// Should only receive one event (channel registered once)
	ev := <-ch
	assert.Equal(t, 'a', ev.Key)

	// Channel should be empty now
	select {
	case <-ch:
		t.Fatal("ScreenBuffer.Listen() at buffer_test.go: unexpected duplicate event received")
	default:
		// Expected - channel is empty
	}
}

func TestScreenBuffer_SetSize_HorizontalExpansion(t *testing.T) {
	tests := []struct {
		name      string
		oldWidth  int
		oldHeight int
		newWidth  int
		newHeight int
		text      string
		textX     int
		textY     int
	}{
		{
			name:      "expand width by 5",
			oldWidth:  10,
			oldHeight: 5,
			newWidth:  15,
			newHeight: 5,
			text:      "Hello",
			textX:     0,
			textY:     0,
		},
		{
			name:      "expand width significantly",
			oldWidth:  5,
			oldHeight: 3,
			newWidth:  20,
			newHeight: 3,
			text:      "Test",
			textX:     0,
			textY:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.oldWidth, tt.oldHeight, 0)
			screen.PutText(gtv.TRect{X: uint16(tt.textX), Y: uint16(tt.textY), W: 0, H: 0}, tt.text, gtv.Attrs(gtv.AttrBold))

			// Resize
			screen.SetSize(tt.newWidth, tt.newHeight)

			// Verify size changed
			w, h := screen.GetSize()
			assert.Equal(t, tt.newWidth, w)
			assert.Equal(t, tt.newHeight, h)

			// Verify original content is preserved
			verifier := getVerifier(screen)
			assert.True(t, verifier.HasText(tt.textX, tt.textY, len(tt.text), 1, tt.text))

			// Verify new cells on the right are spaces
			for y := 0; y < tt.newHeight; y++ {
				for x := tt.oldWidth; x < tt.newWidth; x++ {
					cell := verifier.GetCell(x, y)
					assert.Equal(t, ' ', cell.Rune, "ScreenBuffer.SetSize() at buffer.go: cell at (%d,%d) should be space", x, y)
					assert.Equal(t, gtv.CellAttributes{}, cell.Attrs, "ScreenBuffer.SetSize() at buffer.go: cell at (%d,%d) should have default attrs", x, y)
				}
			}
		})
	}
}

func TestScreenBuffer_SetSize_VerticalExpansion(t *testing.T) {
	tests := []struct {
		name      string
		oldWidth  int
		oldHeight int
		newWidth  int
		newHeight int
		lines     []string
	}{
		{
			name:      "expand height by 3",
			oldWidth:  10,
			oldHeight: 5,
			newWidth:  10,
			newHeight: 8,
			lines:     []string{"Line 1", "Line 2", "Line 3"},
		},
		{
			name:      "expand height significantly",
			oldWidth:  20,
			oldHeight: 3,
			newWidth:  20,
			newHeight: 10,
			lines:     []string{"First", "Second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.oldWidth, tt.oldHeight, 0)
			for i, line := range tt.lines {
				screen.PutText(gtv.TRect{X: 0, Y: uint16(i), W: 0, H: 0}, line, gtv.Attrs(gtv.AttrBold))
			}

			// Resize
			screen.SetSize(tt.newWidth, tt.newHeight)

			// Verify size changed
			w, h := screen.GetSize()
			assert.Equal(t, tt.newWidth, w)
			assert.Equal(t, tt.newHeight, h)

			// Verify original content is preserved
			verifier := getVerifier(screen)
			for i, line := range tt.lines {
				assert.True(t, verifier.HasText(0, i, len(line), 1, line))
			}

			// Verify new rows at the bottom are spaces
			for y := tt.oldHeight; y < tt.newHeight; y++ {
				for x := 0; x < tt.newWidth; x++ {
					cell := verifier.GetCell(x, y)
					assert.Equal(t, ' ', cell.Rune, "ScreenBuffer.SetSize() at buffer.go: cell at (%d,%d) should be space", x, y)
					assert.Equal(t, gtv.CellAttributes{}, cell.Attrs, "ScreenBuffer.SetSize() at buffer.go: cell at (%d,%d) should have default attrs", x, y)
				}
			}
		})
	}
}

func TestScreenBuffer_SetSize_HorizontalShrinking(t *testing.T) {
	tests := []struct {
		name      string
		oldWidth  int
		oldHeight int
		newWidth  int
		newHeight int
		text      string
		textX     int
		textY     int
		wantText  string
	}{
		{
			name:      "shrink width - keep leftmost",
			oldWidth:  20,
			oldHeight: 5,
			newWidth:  10,
			newHeight: 5,
			text:      "HelloWorld1234567890",
			textX:     0,
			textY:     0,
			wantText:  "HelloWorld",
		},
		{
			name:      "shrink width significantly",
			oldWidth:  15,
			oldHeight: 3,
			newWidth:  5,
			newHeight: 3,
			text:      "VeryLongText",
			textX:     0,
			textY:     1,
			wantText:  "VeryL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.oldWidth, tt.oldHeight, 0)
			screen.PutText(gtv.TRect{X: uint16(tt.textX), Y: uint16(tt.textY), W: 0, H: 0}, tt.text, gtv.Attrs(gtv.AttrBold))

			// Resize
			screen.SetSize(tt.newWidth, tt.newHeight)

			// Verify size changed
			w, h := screen.GetSize()
			assert.Equal(t, tt.newWidth, w)
			assert.Equal(t, tt.newHeight, h)

			// Verify leftmost content is preserved
			verifier := getVerifier(screen)
			assert.True(t, verifier.HasText(tt.textX, tt.textY, len(tt.wantText), 1, tt.wantText))
		})
	}
}

func TestScreenBuffer_SetSize_VerticalShrinking(t *testing.T) {
	tests := []struct {
		name      string
		oldWidth  int
		oldHeight int
		newWidth  int
		newHeight int
		lines     []string
		wantLines []string
	}{
		{
			name:      "shrink height - keep topmost",
			oldWidth:  10,
			oldHeight: 5,
			newWidth:  10,
			newHeight: 3,
			lines:     []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5"},
			wantLines: []string{"Line 1", "Line 2", "Line 3"},
		},
		{
			name:      "shrink height significantly",
			oldWidth:  20,
			oldHeight: 10,
			newWidth:  20,
			newHeight: 2,
			lines:     []string{"First", "Second", "Third", "Fourth", "Fifth"},
			wantLines: []string{"First", "Second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.oldWidth, tt.oldHeight, 0)
			for i, line := range tt.lines {
				if i < tt.oldHeight {
					screen.PutText(gtv.TRect{X: 0, Y: uint16(i), W: 0, H: 0}, line, gtv.Attrs(gtv.AttrBold))
				}
			}

			// Resize
			screen.SetSize(tt.newWidth, tt.newHeight)

			// Verify size changed
			w, h := screen.GetSize()
			assert.Equal(t, tt.newWidth, w)
			assert.Equal(t, tt.newHeight, h)

			// Verify topmost content is preserved
			verifier := getVerifier(screen)
			for i, line := range tt.wantLines {
				assert.True(t, verifier.HasText(0, i, len(line), 1, line))
			}
		})
	}
}

func TestScreenBuffer_SetSize_Combined(t *testing.T) {
	tests := []struct {
		name       string
		oldWidth   int
		oldHeight  int
		newWidth   int
		newHeight  int
		setupFunc  func(*ScreenBuffer)
		verifyFunc func(*testing.T, *gtv.ScreenVerifier)
	}{
		{
			name:      "expand both dimensions",
			oldWidth:  5,
			oldHeight: 3,
			newWidth:  10,
			newHeight: 6,
			setupFunc: func(s *ScreenBuffer) {
				s.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", gtv.Attrs(gtv.AttrBold))
				s.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "World", gtv.Attrs(gtv.AttrItalic))
			},
			verifyFunc: func(t *testing.T, v *gtv.ScreenVerifier) {
				assert.True(t, v.HasText(0, 0, 5, 1, "Hello"))
				assert.True(t, v.HasText(0, 1, 5, 1, "World"))
				// Check new columns are spaces
				for y := 0; y < 3; y++ {
					for x := 5; x < 10; x++ {
						cell := v.GetCell(x, y)
						assert.Equal(t, ' ', cell.Rune)
					}
				}
				// Check new rows are spaces
				for y := 3; y < 6; y++ {
					for x := 0; x < 10; x++ {
						cell := v.GetCell(x, y)
						assert.Equal(t, ' ', cell.Rune)
					}
				}
			},
		},
		{
			name:      "shrink both dimensions",
			oldWidth:  20,
			oldHeight: 10,
			newWidth:  10,
			newHeight: 5,
			setupFunc: func(s *ScreenBuffer) {
				s.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "This is a long line", gtv.Attrs(0))
				s.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "Another long line!!", gtv.Attrs(0))
				s.PutText(gtv.TRect{X: 0, Y: 2, W: 0, H: 0}, "Third line here!!!!", gtv.Attrs(0))
			},
			verifyFunc: func(t *testing.T, v *gtv.ScreenVerifier) {
				assert.True(t, v.HasText(0, 0, 10, 1, "This is a "))
				assert.True(t, v.HasText(0, 1, 10, 1, "Another lo"))
				assert.True(t, v.HasText(0, 2, 10, 1, "Third line"))
			},
		},
		{
			name:      "expand width, shrink height",
			oldWidth:  5,
			oldHeight: 10,
			newWidth:  15,
			newHeight: 3,
			setupFunc: func(s *ScreenBuffer) {
				s.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Line1", gtv.Attrs(0))
				s.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "Line2", gtv.Attrs(0))
				s.PutText(gtv.TRect{X: 0, Y: 2, W: 0, H: 0}, "Line3", gtv.Attrs(0))
				s.PutText(gtv.TRect{X: 0, Y: 5, W: 0, H: 0}, "Line6", gtv.Attrs(0))
			},
			verifyFunc: func(t *testing.T, v *gtv.ScreenVerifier) {
				assert.True(t, v.HasText(0, 0, 5, 1, "Line1"))
				assert.True(t, v.HasText(0, 1, 5, 1, "Line2"))
				assert.True(t, v.HasText(0, 2, 5, 1, "Line3"))
				// Line6 should not be present (was at y=5, now height is 3)
				// Check new columns are spaces
				for y := 0; y < 3; y++ {
					for x := 5; x < 15; x++ {
						cell := v.GetCell(x, y)
						assert.Equal(t, ' ', cell.Rune)
					}
				}
			},
		},
		{
			name:      "shrink width, expand height",
			oldWidth:  20,
			oldHeight: 3,
			newWidth:  8,
			newHeight: 8,
			setupFunc: func(s *ScreenBuffer) {
				s.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "VeryLongLineOfText!!", gtv.Attrs(0))
				s.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "AnotherLongLine!!!!!", gtv.Attrs(0))
			},
			verifyFunc: func(t *testing.T, v *gtv.ScreenVerifier) {
				assert.True(t, v.HasText(0, 0, 8, 1, "VeryLong"))
				assert.True(t, v.HasText(0, 1, 8, 1, "AnotherL"))
				// Check new rows are spaces
				for y := 3; y < 8; y++ {
					for x := 0; x < 8; x++ {
						cell := v.GetCell(x, y)
						assert.Equal(t, ' ', cell.Rune)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.oldWidth, tt.oldHeight, 0)
			tt.setupFunc(screen)

			// Resize
			screen.SetSize(tt.newWidth, tt.newHeight)

			// Verify size changed
			w, h := screen.GetSize()
			assert.Equal(t, tt.newWidth, w)
			assert.Equal(t, tt.newHeight, h)

			// Verify content
			verifier := getVerifier(screen)
			tt.verifyFunc(t, verifier)
		})
	}
}

func TestScreenBuffer_SetSize_SameDimensions(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)
	screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Hello", gtv.Attrs(gtv.AttrBold))
	screen.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "World", gtv.Attrs(0))

	// Get buffer reference before resize
	_, _, originalBuffer := screen.GetContent()

	// Resize to same dimensions
	screen.SetSize(10, 5)

	// Verify size unchanged
	w, h := screen.GetSize()
	assert.Equal(t, 10, w)
	assert.Equal(t, 5, h)

	// Verify content is unchanged
	verifier := getVerifier(screen)
	assert.True(t, verifier.HasText(0, 0, 5, 1, "Hello"))
	assert.True(t, verifier.HasText(0, 1, 5, 1, "World"))

	// Verify buffer reference is the same (no reallocation)
	_, _, currentBuffer := screen.GetContent()
	assert.Equal(t, len(originalBuffer), len(currentBuffer))
}

func TestScreenBuffer_SetSize_PreservesAttributes(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)

	// Put text with different attributes
	screen.PutText(gtv.TRect{X: 0, Y: 0, W: 0, H: 0}, "Bold", gtv.Attrs(gtv.AttrBold))
	screen.PutText(gtv.TRect{X: 0, Y: 1, W: 0, H: 0}, "Italic", gtv.Attrs(gtv.AttrItalic))
	screen.PutText(gtv.TRect{X: 0, Y: 2, W: 0, H: 0}, "Under", gtv.Attrs(gtv.AttrUnderline))

	// Resize to larger dimensions
	screen.SetSize(15, 8)

	// Verify attributes are preserved
	verifier := getVerifier(screen)
	assert.True(t, verifier.HasTextWithAttrs(0, 0, 4, 1, "Bold", gtv.AttributeMask{
		CheckAttributes: true,
		Attributes:      gtv.AttrBold,
	}))
	assert.True(t, verifier.HasTextWithAttrs(0, 1, 6, 1, "Italic", gtv.AttributeMask{
		CheckAttributes: true,
		Attributes:      gtv.AttrItalic,
	}))
	assert.True(t, verifier.HasTextWithAttrs(0, 2, 5, 1, "Under", gtv.AttributeMask{
		CheckAttributes: true,
		Attributes:      gtv.AttrUnderline,
	}))
}

func TestScreenBuffer_SetSize_EmptyBuffer(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 0)

	// Don't put any text, resize empty buffer
	screen.SetSize(15, 8)

	// Verify size changed
	w, h := screen.GetSize()
	assert.Equal(t, 15, w)
	assert.Equal(t, 8, h)

	// Verify all cells are spaces
	verifier := getVerifier(screen)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := verifier.GetCell(x, y)
			assert.Equal(t, ' ', cell.Rune)
			assert.Equal(t, gtv.CellAttributes{}, cell.Attrs)
		}
	}
}

func TestScreenBuffer_MoveCursor(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		moves      []struct{ x, y int }
		wantFinalX int
		wantFinalY int
	}{
		{
			name:   "move to origin",
			width:  80,
			height: 24,
			moves: []struct{ x, y int }{
				{0, 0},
			},
			wantFinalX: 0,
			wantFinalY: 0,
		},
		{
			name:   "move to middle",
			width:  80,
			height: 24,
			moves: []struct{ x, y int }{
				{40, 12},
			},
			wantFinalX: 40,
			wantFinalY: 12,
		},
		{
			name:   "multiple moves",
			width:  80,
			height: 24,
			moves: []struct{ x, y int }{
				{10, 5},
				{20, 10},
				{5, 15},
			},
			wantFinalX: 5,
			wantFinalY: 15,
		},
		{
			name:   "move to bottom right",
			width:  80,
			height: 24,
			moves: []struct{ x, y int }{
				{79, 23},
			},
			wantFinalX: 79,
			wantFinalY: 23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)

			for _, move := range tt.moves {
				screen.MoveCursor(move.x, move.y)
			}

			verifier := getVerifierWithCursor(screen)
			x, y := verifier.GetCursorPosition()
			assert.Equal(t, tt.wantFinalX, x)
			assert.Equal(t, tt.wantFinalY, y)
		})
	}
}

func TestScreenBuffer_SetCursorStyle(t *testing.T) {
	tests := []struct {
		name      string
		styles    []gtv.CursorStyle
		wantFinal gtv.CursorStyle
	}{
		{
			name:      "default style",
			styles:    []gtv.CursorStyle{gtv.CursorStyleDefault},
			wantFinal: gtv.CursorStyleDefault,
		},
		{
			name:      "block style",
			styles:    []gtv.CursorStyle{gtv.CursorStyleBlock},
			wantFinal: gtv.CursorStyleBlock,
		},
		{
			name:      "underline style",
			styles:    []gtv.CursorStyle{gtv.CursorStyleUnderline},
			wantFinal: gtv.CursorStyleUnderline,
		},
		{
			name:      "bar style",
			styles:    []gtv.CursorStyle{gtv.CursorStyleBar},
			wantFinal: gtv.CursorStyleBar,
		},
		{
			name:      "hidden style",
			styles:    []gtv.CursorStyle{gtv.CursorStyleHidden},
			wantFinal: gtv.CursorStyleHidden,
		},
		{
			name: "multiple style changes",
			styles: []gtv.CursorStyle{
				gtv.CursorStyleBlock,
				gtv.CursorStyleUnderline,
				gtv.CursorStyleBar,
			},
			wantFinal: gtv.CursorStyleBar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(80, 24, 0)

			for _, style := range tt.styles {
				screen.SetCursorStyle(style)
			}

			verifier := getVerifierWithCursor(screen)
			style := verifier.GetCursorStyle()
			assert.Equal(t, tt.wantFinal, style)
		})
	}
}

func TestScreenBuffer_CursorDoesNotAffectContent(t *testing.T) {
	screen := NewScreenBuffer(80, 24, 0)

	// Put some text
	screen.PutText(gtv.TRect{X: 10, Y: 5, W: 0, H: 0}, "Hello, World!", gtv.Attrs(gtv.AttrBold))

	// Move cursor to different positions
	screen.MoveCursor(0, 0)
	screen.MoveCursor(20, 10)
	screen.MoveCursor(5, 5)

	// Change cursor style
	screen.SetCursorStyle(gtv.CursorStyleBlock)
	screen.SetCursorStyle(gtv.CursorStyleUnderline)

	// Verify text is still there
	verifier := getVerifier(screen)
	assert.True(t, verifier.HasText(10, 5, 13, 1, "Hello, World!"))

	// Verify attributes are preserved
	for i, r := range "Hello, World!" {
		cell := verifier.GetCell(10+i, 5)
		assert.Equal(t, r, cell.Rune)
		assert.Equal(t, gtv.AttrBold, cell.Attrs.Attributes)
	}
}

func TestScreenBuffer_PutText_ClippingToScreen(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		rect       gtv.TRect
		text       string
		wantText   string
		wantLength int
	}{
		{
			name:       "text clipped at screen right boundary",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 7, Y: 0, W: 0, H: 0},
			text:       "HelloWorld",
			wantText:   "Hel",
			wantLength: 3,
		},
		{
			name:       "text starting at right edge",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 9, Y: 0, W: 0, H: 0},
			text:       "Hello",
			wantText:   "H",
			wantLength: 1,
		},
		{
			name:       "text starting beyond right edge - nothing rendered",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 10, Y: 0, W: 0, H: 0},
			text:       "Hello",
			wantText:   "",
			wantLength: 0,
		},
		{
			name:       "text starting at negative X - nothing rendered",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 65535, Y: 0, W: 0, H: 0}, // uint16 wraparound for -1
			text:       "Hello",
			wantText:   "",
			wantLength: 0,
		},
		{
			name:       "text on row beyond bottom - nothing rendered",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 0, Y: 5, W: 0, H: 0},
			text:       "Hello",
			wantText:   "",
			wantLength: 0,
		},
		{
			name:       "text on negative Y - nothing rendered",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 0, Y: 65535, W: 0, H: 0}, // uint16 wraparound for -1
			text:       "Hello",
			wantText:   "",
			wantLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutText(tt.rect, tt.text, gtv.Attrs(gtv.AttrBold))

			verifier := getVerifier(screen)

			// Count how many non-space characters were rendered
			count := 0
			if tt.rect.X < uint16(tt.width) && tt.rect.Y < uint16(tt.height) {
				for x := int(tt.rect.X); x < tt.width; x++ {
					cell := verifier.GetCell(x, int(tt.rect.Y))
					if cell.Rune != ' ' {
						count++
					}
				}
			}

			assert.Equal(t, tt.wantLength, count, "ScreenBuffer.PutText() at buffer.go: wrong number of characters rendered")
		})
	}
}

func TestScreenBuffer_PutText_ClippingToRectangle(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		rect       gtv.TRect
		text       string
		wantText   string
		wantLength int
	}{
		{
			name:       "text clipped to rectangle width",
			width:      80,
			height:     24,
			rect:       gtv.TRect{X: 10, Y: 5, W: 5, H: 1},
			text:       "HelloWorld",
			wantText:   "Hello",
			wantLength: 5,
		},
		{
			name:       "text shorter than rectangle width",
			width:      80,
			height:     24,
			rect:       gtv.TRect{X: 10, Y: 5, W: 20, H: 1},
			text:       "Hello",
			wantText:   "Hello",
			wantLength: 5,
		},
		{
			name:       "rectangle extends beyond screen - clipped to screen",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 7, Y: 0, W: 10, H: 1},
			text:       "HelloWorld",
			wantText:   "Hel",
			wantLength: 3,
		},
		{
			name:       "small rectangle in middle of screen",
			width:      80,
			height:     24,
			rect:       gtv.TRect{X: 20, Y: 10, W: 3, H: 1},
			text:       "Testing",
			wantText:   "Tes",
			wantLength: 3,
		},
		{
			name:       "rectangle width 1 - only one character",
			width:      80,
			height:     24,
			rect:       gtv.TRect{X: 10, Y: 5, W: 1, H: 1},
			text:       "Hello",
			wantText:   "H",
			wantLength: 1,
		},
		{
			name:       "rectangle at screen right edge",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 5, Y: 0, W: 5, H: 1},
			text:       "Hello",
			wantText:   "Hello",
			wantLength: 5,
		},
		{
			name:       "rectangle partially beyond screen right",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 5, Y: 0, W: 10, H: 1},
			text:       "HelloWorld",
			wantText:   "Hello",
			wantLength: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutText(tt.rect, tt.text, gtv.Attrs(gtv.AttrBold))

			verifier := getVerifier(screen)

			// Verify the rendered text
			actual := verifier.GetText(int(tt.rect.X), int(tt.rect.Y), tt.wantLength, 1)
			assert.Equal(t, tt.wantText, actual, "ScreenBuffer.PutText() at buffer.go: wrong text rendered")
		})
	}
}

func TestScreenBuffer_PutText_WithZeroWidthHeight(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		rect       gtv.TRect
		text       string
		wantText   string
		wantLength int
	}{
		{
			name:       "W=0 H=0 uses full screen width",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			text:       "HelloWorld",
			wantText:   "HelloWorld",
			wantLength: 10,
		},
		{
			name:       "W=0 H=0 from middle of line",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 5, Y: 0, W: 0, H: 0},
			text:       "Hello",
			wantText:   "Hello",
			wantLength: 5,
		},
		{
			name:       "W=0 H=0 clips at screen boundary",
			width:      10,
			height:     5,
			rect:       gtv.TRect{X: 7, Y: 0, W: 0, H: 0},
			text:       "HelloWorld",
			wantText:   "Hel",
			wantLength: 3,
		},
		{
			name:       "W=0 H=0 with long text on wide screen",
			width:      80,
			height:     24,
			rect:       gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			text:       "This is a long line of text that should be clipped at 80 characters if needed",
			wantText:   "This is a long line of text that should be clipped at 80 characters if needed",
			wantLength: 77,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutText(tt.rect, tt.text, gtv.Attrs(gtv.AttrBold))

			verifier := getVerifier(screen)

			// Verify the rendered text
			actual := verifier.GetText(int(tt.rect.X), int(tt.rect.Y), tt.wantLength, 1)
			assert.Equal(t, tt.wantText, actual, "ScreenBuffer.PutText() at buffer.go: wrong text rendered")
		})
	}
}

func TestScreenBuffer_PutText_BorderCases(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		rect        gtv.TRect
		text        string
		wantText    string
		wantLength  int
		checkX      int
		checkY      int
		checkWidth  int
		checkHeight int
	}{
		{
			name:        "empty text",
			width:       10,
			height:      5,
			rect:        gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			text:        "",
			wantText:    "",
			wantLength:  0,
			checkX:      0,
			checkY:      0,
			checkWidth:  1,
			checkHeight: 1,
		},
		{
			name:        "single character",
			width:       10,
			height:      5,
			rect:        gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			text:        "X",
			wantText:    "X",
			wantLength:  1,
			checkX:      0,
			checkY:      0,
			checkWidth:  1,
			checkHeight: 1,
		},
		{
			name:        "unicode text",
			width:       20,
			height:      5,
			rect:        gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			text:        "Hello 世界",
			wantText:    "Hello 世界",
			wantLength:  8,
			checkX:      0,
			checkY:      0,
			checkWidth:  8,
			checkHeight: 1,
		},
		{
			name:        "unicode text clipped to rectangle",
			width:       20,
			height:      5,
			rect:        gtv.TRect{X: 0, Y: 0, W: 7, H: 1},
			text:        "Hello 世界",
			wantText:    "Hello 世",
			wantLength:  7,
			checkX:      0,
			checkY:      0,
			checkWidth:  7,
			checkHeight: 1,
		},
		{
			name:        "text at last row of screen",
			width:       10,
			height:      5,
			rect:        gtv.TRect{X: 0, Y: 4, W: 0, H: 0},
			text:        "LastRow",
			wantText:    "LastRow",
			wantLength:  7,
			checkX:      0,
			checkY:      4,
			checkWidth:  7,
			checkHeight: 1,
		},
		{
			name:        "text at last column of screen",
			width:       10,
			height:      5,
			rect:        gtv.TRect{X: 9, Y: 0, W: 0, H: 0},
			text:        "ABC",
			wantText:    "A",
			wantLength:  1,
			checkX:      9,
			checkY:      0,
			checkWidth:  1,
			checkHeight: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutText(tt.rect, tt.text, gtv.Attrs(gtv.AttrBold))

			verifier := getVerifier(screen)

			if tt.wantLength > 0 {
				// Verify the rendered text
				actual := verifier.GetText(tt.checkX, tt.checkY, tt.checkWidth, tt.checkHeight)
				assert.Equal(t, tt.wantText, actual, "ScreenBuffer.PutText() at buffer.go: wrong text rendered")
			}
		})
	}
}

func TestScreenBuffer_PutContent_ClippingToScreen(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		rect       gtv.TRect
		content    []gtv.Cell
		wantLength int
	}{
		{
			name:   "content clipped at screen right boundary",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 7, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'H', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'e', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantLength: 3,
		},
		{
			name:   "content starting at right edge",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 9, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'H', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'i', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantLength: 1,
		},
		{
			name:   "content starting beyond right edge - nothing rendered",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 10, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'H', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantLength: 0,
		},
		{
			name:   "content starting at negative X - nothing rendered",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 65535, Y: 0, W: 0, H: 0}, // uint16 wraparound for -1
			content: []gtv.Cell{
				{Rune: 'H', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantLength: 0,
		},
		{
			name:   "content on row beyond bottom - nothing rendered",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 0, Y: 5, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'H', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantLength: 0,
		},
		{
			name:   "content on negative Y - nothing rendered",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 0, Y: 65535, W: 0, H: 0}, // uint16 wraparound for -1
			content: []gtv.Cell{
				{Rune: 'H', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutContent(tt.rect, tt.content)

			verifier := getVerifier(screen)

			// Count how many non-space characters were rendered
			count := 0
			if tt.rect.X < uint16(tt.width) && tt.rect.Y < uint16(tt.height) {
				for x := int(tt.rect.X); x < tt.width; x++ {
					cell := verifier.GetCell(x, int(tt.rect.Y))
					if cell.Rune != ' ' {
						count++
					}
				}
			}

			assert.Equal(t, tt.wantLength, count, "ScreenBuffer.PutContent() at buffer.go: wrong number of characters rendered")
		})
	}
}

func TestScreenBuffer_PutContent_ClippingToRectangle(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		rect       gtv.TRect
		content    []gtv.Cell
		wantText   string
		wantLength int
	}{
		{
			name:   "content clipped to rectangle width",
			width:  80,
			height: 24,
			rect:   gtv.TRect{X: 10, Y: 5, W: 5, H: 1},
			content: []gtv.Cell{
				{Rune: 'H', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'e', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'W', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'o', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'r', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'l', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'd', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:   "Hello",
			wantLength: 5,
		},
		{
			name:   "content shorter than rectangle width",
			width:  80,
			height: 24,
			rect:   gtv.TRect{X: 10, Y: 5, W: 20, H: 1},
			content: []gtv.Cell{
				{Rune: 'H', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'i', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:   "Hi",
			wantLength: 2,
		},
		{
			name:   "rectangle extends beyond screen - clipped to screen",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 7, Y: 0, W: 10, H: 1},
			content: []gtv.Cell{
				{Rune: 'A', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'B', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'C', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'D', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'E', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:   "ABC",
			wantLength: 3,
		},
		{
			name:   "small rectangle in middle of screen",
			width:  80,
			height: 24,
			rect:   gtv.TRect{X: 20, Y: 10, W: 3, H: 1},
			content: []gtv.Cell{
				{Rune: 'T', Attrs: gtv.Attrs(gtv.AttrItalic)},
				{Rune: 'e', Attrs: gtv.Attrs(gtv.AttrItalic)},
				{Rune: 's', Attrs: gtv.Attrs(gtv.AttrItalic)},
				{Rune: 't', Attrs: gtv.Attrs(gtv.AttrItalic)},
			},
			wantText:   "Tes",
			wantLength: 3,
		},
		{
			name:   "rectangle width 1 - only one cell",
			width:  80,
			height: 24,
			rect:   gtv.TRect{X: 10, Y: 5, W: 1, H: 1},
			content: []gtv.Cell{
				{Rune: 'X', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'Y', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:   "X",
			wantLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutContent(tt.rect, tt.content)

			verifier := getVerifier(screen)

			// Verify the rendered text
			actual := verifier.GetText(int(tt.rect.X), int(tt.rect.Y), tt.wantLength, 1)
			assert.Equal(t, tt.wantText, actual, "ScreenBuffer.PutContent() at buffer.go: wrong text rendered")
		})
	}
}

func TestScreenBuffer_PutContent_WithZeroWidthHeight(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		rect       gtv.TRect
		content    []gtv.Cell
		wantText   string
		wantLength int
	}{
		{
			name:   "W=0 H=0 uses full screen width",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'A', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'B', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'C', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'D', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'E', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:   "ABCDE",
			wantLength: 5,
		},
		{
			name:   "W=0 H=0 from middle of line",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 5, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'X', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'Y', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'Z', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:   "XYZ",
			wantLength: 3,
		},
		{
			name:   "W=0 H=0 clips at screen boundary",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 7, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'L', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'O', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'N', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'G', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:   "LON",
			wantLength: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutContent(tt.rect, tt.content)

			verifier := getVerifier(screen)

			// Verify the rendered text
			actual := verifier.GetText(int(tt.rect.X), int(tt.rect.Y), tt.wantLength, 1)
			assert.Equal(t, tt.wantText, actual, "ScreenBuffer.PutContent() at buffer.go: wrong text rendered")
		})
	}
}

func TestScreenBuffer_PutContent_BorderCases(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		rect        gtv.TRect
		content     []gtv.Cell
		wantText    string
		wantLength  int
		checkX      int
		checkY      int
		checkWidth  int
		checkHeight int
	}{
		{
			name:        "empty content",
			width:       10,
			height:      5,
			rect:        gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			content:     []gtv.Cell{},
			wantText:    "",
			wantLength:  0,
			checkX:      0,
			checkY:      0,
			checkWidth:  1,
			checkHeight: 1,
		},
		{
			name:   "single cell",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'X', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:    "X",
			wantLength:  1,
			checkX:      0,
			checkY:      0,
			checkWidth:  1,
			checkHeight: 1,
		},
		{
			name:   "content with different attributes per cell",
			width:  20,
			height: 5,
			rect:   gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'B', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'I', Attrs: gtv.Attrs(gtv.AttrItalic)},
				{Rune: 'U', Attrs: gtv.Attrs(gtv.AttrUnderline)},
			},
			wantText:    "BIU",
			wantLength:  3,
			checkX:      0,
			checkY:      0,
			checkWidth:  3,
			checkHeight: 1,
		},
		{
			name:   "content at last row of screen",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 0, Y: 4, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'L', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'a', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 's', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 't', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:    "Last",
			wantLength:  4,
			checkX:      0,
			checkY:      4,
			checkWidth:  4,
			checkHeight: 1,
		},
		{
			name:   "content at last column of screen",
			width:  10,
			height: 5,
			rect:   gtv.TRect{X: 9, Y: 0, W: 0, H: 0},
			content: []gtv.Cell{
				{Rune: 'A', Attrs: gtv.Attrs(gtv.AttrBold)},
				{Rune: 'B', Attrs: gtv.Attrs(gtv.AttrBold)},
			},
			wantText:    "A",
			wantLength:  1,
			checkX:      9,
			checkY:      0,
			checkWidth:  1,
			checkHeight: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutContent(tt.rect, tt.content)

			verifier := getVerifier(screen)

			if tt.wantLength > 0 {
				// Verify the rendered text
				actual := verifier.GetText(tt.checkX, tt.checkY, tt.checkWidth, tt.checkHeight)
				assert.Equal(t, tt.wantText, actual, "ScreenBuffer.PutContent() at buffer.go: wrong text rendered")

				// Verify attributes are preserved for content with different attributes
				if tt.name == "content with different attributes per cell" {
					cell0 := verifier.GetCell(0, 0)
					assert.Equal(t, 'B', cell0.Rune)
					assert.Equal(t, gtv.AttrBold, cell0.Attrs.Attributes)

					cell1 := verifier.GetCell(1, 0)
					assert.Equal(t, 'I', cell1.Rune)
					assert.Equal(t, gtv.AttrItalic, cell1.Attrs.Attributes)

					cell2 := verifier.GetCell(2, 0)
					assert.Equal(t, 'U', cell2.Rune)
					assert.Equal(t, gtv.AttrUnderline, cell2.Attrs.Attributes)
				}
			}
		})
	}
}

func TestScreenBuffer_PutContent_PreservesAttributes(t *testing.T) {
	screen := NewScreenBuffer(80, 24, 0)

	// Create content with different colors and attributes
	content := []gtv.Cell{
		{Rune: 'R', Attrs: gtv.AttrsWithColor(gtv.AttrBold, 0xFF0000, 0x000000)},
		{Rune: 'G', Attrs: gtv.AttrsWithColor(gtv.AttrItalic, 0x00FF00, 0x000000)},
		{Rune: 'B', Attrs: gtv.AttrsWithColor(gtv.AttrUnderline, 0x0000FF, 0xFFFFFF)},
	}

	screen.PutContent(gtv.TRect{X: 10, Y: 5, W: 0, H: 0}, content)

	verifier := getVerifier(screen)

	// Verify first cell (Red, Bold)
	cell0 := verifier.GetCell(10, 5)
	assert.Equal(t, 'R', cell0.Rune)
	assert.Equal(t, gtv.AttrBold, cell0.Attrs.Attributes)
	assert.Equal(t, gtv.TextColor(0xFF0000), cell0.Attrs.TextColor)
	assert.Equal(t, gtv.TextColor(0x000000), cell0.Attrs.BackColor)

	// Verify second cell (Green, Italic)
	cell1 := verifier.GetCell(11, 5)
	assert.Equal(t, 'G', cell1.Rune)
	assert.Equal(t, gtv.AttrItalic, cell1.Attrs.Attributes)
	assert.Equal(t, gtv.TextColor(0x00FF00), cell1.Attrs.TextColor)
	assert.Equal(t, gtv.TextColor(0x000000), cell1.Attrs.BackColor)

	// Verify third cell (Blue, Underline)
	cell2 := verifier.GetCell(12, 5)
	assert.Equal(t, 'B', cell2.Rune)
	assert.Equal(t, gtv.AttrUnderline, cell2.Attrs.Attributes)
	assert.Equal(t, gtv.TextColor(0x0000FF), cell2.Attrs.TextColor)
	assert.Equal(t, gtv.TextColor(0xFFFFFF), cell2.Attrs.BackColor)
}
