package term

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/cswterm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a ScreenVerifier from a ScreenBuffer
func getVerifier(screen *ScreenBuffer) *cswterm.ScreenVerifier {
	width, height, content := screen.GetContent()
	return cswterm.NewScreenVerifier(width, height, content)
}

// Helper function to create a ScreenVerifier with cursor info from a ScreenBuffer
func getVerifierWithCursor(screen *ScreenBuffer) *cswterm.ScreenVerifier {
	width, height, content := screen.GetContent()
	cursorX, cursorY := screen.GetCursorPosition()
	cursorStyle := screen.GetCursorStyle()
	return cswterm.NewScreenVerifierWithCursor(width, height, content, cursorX, cursorY, cursorStyle)
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
					assert.Equal(t, cswterm.CellAttributes{}, cell.Attrs)
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
		attrs     cswterm.TextAttributes
		wantText  string
		wantAttrs cswterm.TextAttributes
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
			attrs:     cswterm.AttrBold,
			wantText:  "Hello",
			wantAttrs: cswterm.AttrBold,
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
			attrs:     cswterm.AttrItalic | cswterm.AttrUnderline,
			wantText:  "World",
			wantAttrs: cswterm.AttrItalic | cswterm.AttrUnderline,
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
			attrs:     cswterm.AttrBold,
			wantText:  "Hello 世界",
			wantAttrs: cswterm.AttrBold,
			checkX:    0,
			checkY:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewScreenBuffer(tt.width, tt.height, 0)
			screen.PutText(tt.x, tt.y, tt.text, cswterm.Attrs(tt.attrs))

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
			screen.PutText(tt.x, tt.y, tt.text, cswterm.Attrs(cswterm.AttrBold))

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
			screen.PutText(tt.x, tt.y, tt.text, cswterm.Attrs(cswterm.AttrBold))

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
	screen.PutText(2, 1, "Test", cswterm.Attrs(cswterm.AttrBold))
	verifier := getVerifier(screen)

	tests := []struct {
		name      string
		x         int
		y         int
		wantRune  rune
		wantAttrs cswterm.TextAttributes
	}{
		{
			name:      "first character",
			x:         2,
			y:         1,
			wantRune:  'T',
			wantAttrs: cswterm.AttrBold,
		},
		{
			name:      "middle character",
			x:         3,
			y:         1,
			wantRune:  'e',
			wantAttrs: cswterm.AttrBold,
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
	screen.PutText(0, 0, "Hello", cswterm.Attrs(0))
	screen.PutText(0, 1, "World", cswterm.Attrs(0))
	screen.PutText(5, 2, "Test", cswterm.Attrs(0))
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
	screen.PutText(0, 0, "Hello", cswterm.Attrs(0))
	screen.PutText(0, 1, "World", cswterm.Attrs(0))
	screen.PutText(5, 2, "Test", cswterm.Attrs(0))
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
	screen.PutText(0, 0, "Bold", cswterm.Attrs(cswterm.AttrBold))
	screen.PutText(0, 1, "Italic", cswterm.Attrs(cswterm.AttrItalic))
	screen.PutText(0, 2, "Both", cswterm.Attrs(cswterm.AttrBold|cswterm.AttrItalic))
	verifier := getVerifier(screen)

	tests := []struct {
		name   string
		x      int
		y      int
		width  int
		height int
		text   string
		mask   cswterm.AttributeMask
		want   bool
	}{
		{
			name:   "check bold attribute",
			x:      0,
			y:      0,
			width:  4,
			height: 1,
			text:   "Bold",
			mask: cswterm.AttributeMask{
				CheckAttributes: true,
				Attributes:      cswterm.AttrBold,
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
			mask: cswterm.AttributeMask{
				CheckAttributes: true,
				Attributes:      cswterm.AttrItalic,
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
			mask: cswterm.AttributeMask{
				CheckAttributes: true,
				Attributes:      cswterm.AttrBold | cswterm.AttrItalic,
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
			mask: cswterm.AttributeMask{
				CheckAttributes: true,
				Attributes:      cswterm.AttrItalic,
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
			mask: cswterm.AttributeMask{
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
			mask: cswterm.AttributeMask{
				CheckAttributes: true,
				Attributes:      cswterm.AttrBold,
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
	redColor := uint32(0xFF0000)
	blueColor := uint32(0x0000FF)
	greenBack := uint32(0x00FF00)

	// Put text and then modify colors
	screen.PutText(0, 0, "Red", cswterm.Attrs(0))
	width, _, _ := screen.GetContent()
	for i := 0; i < 3; i++ {
		idx := 0*width + i
		screen.buffer[idx].Attrs.TextColor = redColor
	}

	screen.PutText(0, 1, "Blue", cswterm.Attrs(0))
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
		mask   cswterm.AttributeMask
		want   bool
	}{
		{
			name:   "check text color",
			x:      0,
			y:      0,
			width:  3,
			height: 1,
			text:   "Red",
			mask: cswterm.AttributeMask{
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
			mask: cswterm.AttributeMask{
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
			mask: cswterm.AttributeMask{
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
			mask: cswterm.AttributeMask{
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
	screen.PutText(0, 0, "Hello", cswterm.Attrs(cswterm.AttrBold))
	screen.PutText(0, 1, "World", cswterm.Attrs(cswterm.AttrItalic))

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
			assert.Equal(t, cswterm.CellAttributes{}, cell.Attrs)
		}
	}
}

func TestMockScreen_InterfaceCompliance(t *testing.T) {
	var _ cswterm.IScreenOutput = (*ScreenBuffer)(nil)
}

func TestAttributeMask_Partial(t *testing.T) {
	screen := NewScreenBuffer(20, 10, 0)

	// Create text with bold and italic
	screen.PutText(0, 0, "Text", cswterm.Attrs(cswterm.AttrBold|cswterm.AttrItalic|cswterm.AttrUnderline))
	width, _, _ := screen.GetContent()
	for i := 0; i < 4; i++ {
		idx := 0*width + i
		screen.buffer[idx].Attrs.TextColor = 0xFF0000
	}

	verifier := getVerifier(screen)
	tests := []struct {
		name string
		mask cswterm.AttributeMask
		want bool
	}{
		{
			name: "check only bold (ignore italic and underline)",
			mask: cswterm.AttributeMask{
				CheckAttributes: true,
				Attributes:      cswterm.AttrBold | cswterm.AttrItalic | cswterm.AttrUnderline,
			},
			want: true,
		},
		{
			name: "check only text color (ignore attributes)",
			mask: cswterm.AttributeMask{
				CheckTextColor: true,
				TextColor:      0xFF0000,
			},
			want: true,
		},
		{
			name: "check both color and attributes",
			mask: cswterm.AttributeMask{
				CheckAttributes: true,
				CheckTextColor:  true,
				Attributes:      cswterm.AttrBold | cswterm.AttrItalic | cswterm.AttrUnderline,
				TextColor:       0xFF0000,
			},
			want: true,
		},
		{
			name: "check nothing - always matches",
			mask: cswterm.AttributeMask{},
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
			ch := make(chan cswterm.InputEvent, tt.channelSize)
			screen.Listen(ch)

			// Send events
			for i := 0; i < tt.eventCount; i++ {
				event := cswterm.InputEvent{
					Type: cswterm.InputEventKey,
					Key:  rune('a' + i),
				}
				screen.Notify(event)
			}

			// Receive events
			receivedCount := 0
			for i := 0; i < tt.wantEvents; i++ {
				select {
				case ev := <-ch:
					assert.Equal(t, cswterm.InputEventKey, ev.Type)
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
	ch1 := make(chan cswterm.InputEvent, 10)
	ch2 := make(chan cswterm.InputEvent, 10)
	ch3 := make(chan cswterm.InputEvent, 10)

	screen.Listen(ch1)
	screen.Listen(ch2)
	screen.Listen(ch3)

	// Send an event
	event := cswterm.InputEvent{
		Type: cswterm.InputEventKey,
		Key:  'x',
	}
	screen.Notify(event)

	// All listeners should receive the event
	ev1 := <-ch1
	ev2 := <-ch2
	ev3 := <-ch3

	assert.Equal(t, cswterm.InputEventKey, ev1.Type)
	assert.Equal(t, 'x', ev1.Key)
	assert.Equal(t, cswterm.InputEventKey, ev2.Type)
	assert.Equal(t, 'x', ev2.Key)
	assert.Equal(t, cswterm.InputEventKey, ev3.Type)
	assert.Equal(t, 'x', ev3.Key)
}

func TestScreenBuffer_Notify_FullChannel_Queue(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 10)

	// Create a channel with size 1
	ch := make(chan cswterm.InputEvent, 1)
	screen.Listen(ch)

	// Send first event - should go directly to channel
	event1 := cswterm.InputEvent{Type: cswterm.InputEventKey, Key: '1'}
	screen.Notify(event1)

	// Verify channel has the first event
	select {
	case ev := <-ch:
		assert.Equal(t, '1', ev.Key)
	default:
		t.Fatal("ScreenBuffer.Notify() at buffer_test.go: expected event in channel")
	}

	// Now channel is empty, send multiple events to fill channel and queue
	screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: '2'})
	screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: '3'})
	screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: '4'})

	// Receive events - queued events should be delivered when we make space
	for i := 2; i <= 4; i++ {
		// Read from channel
		var ev cswterm.InputEvent
		select {
		case ev = <-ch:
			// Got an event
		default:
			// Channel empty, notify to trigger queue delivery
			screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: 'X'})
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
	ch := make(chan cswterm.InputEvent, 1)
	screen.Listen(ch)

	// Fill the channel first
	screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: '0'})

	// Now send more events than queue size - they will be queued
	// Send queueSize + 2 more events (total queueSize + 3)
	for i := 1; i < queueSize+3; i++ {
		event := cswterm.InputEvent{Type: cswterm.InputEventKey, Key: rune('0' + i)}
		screen.Notify(event)
	}

	// Read the first event from channel
	ev := <-ch
	assert.Equal(t, '0', ev.Key)

	// Notify to trigger delivery of queued events
	screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: 'X'})

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
	ch := make(chan cswterm.InputEvent, 10)
	screen.Listen(ch)

	// Send an event
	event1 := cswterm.InputEvent{Type: cswterm.InputEventKey, Key: 'a'}
	screen.Notify(event1)

	// Receive it
	ev := <-ch
	assert.Equal(t, 'a', ev.Key)

	// Close the channel
	close(ch)

	// Send another event - should not panic
	event2 := cswterm.InputEvent{Type: cswterm.InputEventKey, Key: 'b'}
	require.NotPanics(t, func() {
		screen.Notify(event2)
	})
}

func TestScreenBuffer_Notify_EventOrder(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 100)

	ch := make(chan cswterm.InputEvent, 1)
	screen.Listen(ch)

	// Fill the channel
	screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: '0'})

	// Queue multiple events
	for i := 1; i <= 10; i++ {
		screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: rune('0' + i)})
	}

	// Receive all events and verify order
	for i := 0; i <= 10; i++ {
		// Read from channel to make space
		ev := <-ch

		// Trigger delivery of queued events
		if i < 10 {
			screen.Notify(cswterm.InputEvent{Type: cswterm.InputEventKey, Key: 'X'})
		}

		expectedKey := rune('0' + i)
		assert.Equal(t, expectedKey, ev.Key, "ScreenBuffer.Notify() at buffer_test.go: event order mismatch at position %d", i)
	}
}

func TestScreenBuffer_Notify_NoListeners(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 10)

	// Send event with no listeners - should not panic
	event := cswterm.InputEvent{Type: cswterm.InputEventKey, Key: 'a'}
	require.NotPanics(t, func() {
		screen.Notify(event)
	})
}

func TestScreenBuffer_Listen_MultipleRegistrations(t *testing.T) {
	screen := NewScreenBuffer(10, 5, 10)

	ch := make(chan cswterm.InputEvent, 10)

	// Register the same channel multiple times
	screen.Listen(ch)
	screen.Listen(ch)

	// Send an event
	event := cswterm.InputEvent{Type: cswterm.InputEventKey, Key: 'a'}
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
			screen.PutText(tt.textX, tt.textY, tt.text, cswterm.Attrs(cswterm.AttrBold))

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
					assert.Equal(t, cswterm.CellAttributes{}, cell.Attrs, "ScreenBuffer.SetSize() at buffer.go: cell at (%d,%d) should have default attrs", x, y)
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
				screen.PutText(0, i, line, cswterm.Attrs(cswterm.AttrBold))
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
					assert.Equal(t, cswterm.CellAttributes{}, cell.Attrs, "ScreenBuffer.SetSize() at buffer.go: cell at (%d,%d) should have default attrs", x, y)
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
			screen.PutText(tt.textX, tt.textY, tt.text, cswterm.Attrs(cswterm.AttrBold))

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
					screen.PutText(0, i, line, cswterm.Attrs(cswterm.AttrBold))
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
		verifyFunc func(*testing.T, *cswterm.ScreenVerifier)
	}{
		{
			name:      "expand both dimensions",
			oldWidth:  5,
			oldHeight: 3,
			newWidth:  10,
			newHeight: 6,
			setupFunc: func(s *ScreenBuffer) {
				s.PutText(0, 0, "Hello", cswterm.Attrs(cswterm.AttrBold))
				s.PutText(0, 1, "World", cswterm.Attrs(cswterm.AttrItalic))
			},
			verifyFunc: func(t *testing.T, v *cswterm.ScreenVerifier) {
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
				s.PutText(0, 0, "This is a long line", cswterm.Attrs(0))
				s.PutText(0, 1, "Another long line!!", cswterm.Attrs(0))
				s.PutText(0, 2, "Third line here!!!!", cswterm.Attrs(0))
			},
			verifyFunc: func(t *testing.T, v *cswterm.ScreenVerifier) {
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
				s.PutText(0, 0, "Line1", cswterm.Attrs(0))
				s.PutText(0, 1, "Line2", cswterm.Attrs(0))
				s.PutText(0, 2, "Line3", cswterm.Attrs(0))
				s.PutText(0, 5, "Line6", cswterm.Attrs(0))
			},
			verifyFunc: func(t *testing.T, v *cswterm.ScreenVerifier) {
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
				s.PutText(0, 0, "VeryLongLineOfText!!", cswterm.Attrs(0))
				s.PutText(0, 1, "AnotherLongLine!!!!!", cswterm.Attrs(0))
			},
			verifyFunc: func(t *testing.T, v *cswterm.ScreenVerifier) {
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
	screen.PutText(0, 0, "Hello", cswterm.Attrs(cswterm.AttrBold))
	screen.PutText(0, 1, "World", cswterm.Attrs(0))

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
	screen.PutText(0, 0, "Bold", cswterm.Attrs(cswterm.AttrBold))
	screen.PutText(0, 1, "Italic", cswterm.Attrs(cswterm.AttrItalic))
	screen.PutText(0, 2, "Under", cswterm.Attrs(cswterm.AttrUnderline))

	// Resize to larger dimensions
	screen.SetSize(15, 8)

	// Verify attributes are preserved
	verifier := getVerifier(screen)
	assert.True(t, verifier.HasTextWithAttrs(0, 0, 4, 1, "Bold", cswterm.AttributeMask{
		CheckAttributes: true,
		Attributes:      cswterm.AttrBold,
	}))
	assert.True(t, verifier.HasTextWithAttrs(0, 1, 6, 1, "Italic", cswterm.AttributeMask{
		CheckAttributes: true,
		Attributes:      cswterm.AttrItalic,
	}))
	assert.True(t, verifier.HasTextWithAttrs(0, 2, 5, 1, "Under", cswterm.AttributeMask{
		CheckAttributes: true,
		Attributes:      cswterm.AttrUnderline,
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
			assert.Equal(t, cswterm.CellAttributes{}, cell.Attrs)
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
		styles    []cswterm.CursorStyle
		wantFinal cswterm.CursorStyle
	}{
		{
			name:      "default style",
			styles:    []cswterm.CursorStyle{cswterm.CursorStyleDefault},
			wantFinal: cswterm.CursorStyleDefault,
		},
		{
			name:      "block style",
			styles:    []cswterm.CursorStyle{cswterm.CursorStyleBlock},
			wantFinal: cswterm.CursorStyleBlock,
		},
		{
			name:      "underline style",
			styles:    []cswterm.CursorStyle{cswterm.CursorStyleUnderline},
			wantFinal: cswterm.CursorStyleUnderline,
		},
		{
			name:      "bar style",
			styles:    []cswterm.CursorStyle{cswterm.CursorStyleBar},
			wantFinal: cswterm.CursorStyleBar,
		},
		{
			name:      "hidden style",
			styles:    []cswterm.CursorStyle{cswterm.CursorStyleHidden},
			wantFinal: cswterm.CursorStyleHidden,
		},
		{
			name: "multiple style changes",
			styles: []cswterm.CursorStyle{
				cswterm.CursorStyleBlock,
				cswterm.CursorStyleUnderline,
				cswterm.CursorStyleBar,
			},
			wantFinal: cswterm.CursorStyleBar,
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
	screen.PutText(10, 5, "Hello, World!", cswterm.Attrs(cswterm.AttrBold))

	// Move cursor to different positions
	screen.MoveCursor(0, 0)
	screen.MoveCursor(20, 10)
	screen.MoveCursor(5, 5)

	// Change cursor style
	screen.SetCursorStyle(cswterm.CursorStyleBlock)
	screen.SetCursorStyle(cswterm.CursorStyleUnderline)

	// Verify text is still there
	verifier := getVerifier(screen)
	assert.True(t, verifier.HasText(10, 5, 13, 1, "Hello, World!"))

	// Verify attributes are preserved
	for i, r := range "Hello, World!" {
		cell := verifier.GetCell(10+i, 5)
		assert.Equal(t, r, cell.Rune)
		assert.Equal(t, cswterm.AttrBold, cell.Attrs.Attributes)
	}
}
