package cswterm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a ScreenVerifier from a ScreenBuffer
func getVerifier(screen *ScreenBuffer) *ScreenVerifier {
	width, height, content := screen.GetContent()
	return NewScreenVerifier(width, height, content)
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
			screen := NewMockScreen(tt.width, tt.height, 0)
			require.NotNil(t, screen)

			w, h := screen.Size()
			assert.Equal(t, tt.width, w)
			assert.Equal(t, tt.height, h)

			// Verify all cells are initialized with spaces
			verifier := getVerifier(screen)
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					cell := verifier.GetCell(x, y)
					assert.Equal(t, ' ', cell.Rune)
					assert.Equal(t, CellAttributes{}, cell.Attrs)
				}
			}
		})
	}
}

func TestMockScreen_Size(t *testing.T) {
	screen := NewMockScreen(100, 50, 0)
	w, h := screen.Size()
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
		attrs     TextAttributes
		wantText  string
		wantAttrs TextAttributes
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
			attrs:     AttrBold,
			wantText:  "Hello",
			wantAttrs: AttrBold,
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
			attrs:     AttrItalic | AttrUnderline,
			wantText:  "World",
			wantAttrs: AttrItalic | AttrUnderline,
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
			attrs:     AttrBold,
			wantText:  "Hello 世界",
			wantAttrs: AttrBold,
			checkX:    0,
			checkY:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := NewMockScreen(tt.width, tt.height, 0)
			screen.PutText(tt.x, tt.y, tt.text, Attrs(tt.attrs))

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
			screen := NewMockScreen(tt.width, tt.height, 0)
			screen.PutText(tt.x, tt.y, tt.text, Attrs(AttrBold))

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
			screen := NewMockScreen(tt.width, tt.height, 0)
			// Should not panic
			screen.PutText(tt.x, tt.y, tt.text, Attrs(AttrBold))

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
	screen := NewMockScreen(10, 5, 0)
	screen.PutText(2, 1, "Test", Attrs(AttrBold))
	verifier := getVerifier(screen)

	tests := []struct {
		name      string
		x         int
		y         int
		wantRune  rune
		wantAttrs TextAttributes
	}{
		{
			name:      "first character",
			x:         2,
			y:         1,
			wantRune:  'T',
			wantAttrs: AttrBold,
		},
		{
			name:      "middle character",
			x:         3,
			y:         1,
			wantRune:  'e',
			wantAttrs: AttrBold,
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
	screen := NewMockScreen(20, 10, 0)
	screen.PutText(0, 0, "Hello", Attrs(0))
	screen.PutText(0, 1, "World", Attrs(0))
	screen.PutText(5, 2, "Test", Attrs(0))
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
	screen := NewMockScreen(20, 10, 0)
	screen.PutText(0, 0, "Hello", Attrs(0))
	screen.PutText(0, 1, "World", Attrs(0))
	screen.PutText(5, 2, "Test", Attrs(0))
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
	screen := NewMockScreen(20, 10, 0)
	screen.PutText(0, 0, "Bold", Attrs(AttrBold))
	screen.PutText(0, 1, "Italic", Attrs(AttrItalic))
	screen.PutText(0, 2, "Both", Attrs(AttrBold|AttrItalic))
	verifier := getVerifier(screen)

	tests := []struct {
		name   string
		x      int
		y      int
		width  int
		height int
		text   string
		mask   AttributeMask
		want   bool
	}{
		{
			name:   "check bold attribute",
			x:      0,
			y:      0,
			width:  4,
			height: 1,
			text:   "Bold",
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrBold,
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
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrItalic,
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
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrBold | AttrItalic,
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
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrItalic,
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
			mask: AttributeMask{
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
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrBold,
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
	screen := NewMockScreen(20, 10, 0)

	// Create cells with specific colors manually
	redColor := uint32(0xFF0000)
	blueColor := uint32(0x0000FF)
	greenBack := uint32(0x00FF00)

	// Put text and then modify colors
	screen.PutText(0, 0, "Red", Attrs(0))
	width, _, _ := screen.GetContent()
	for i := 0; i < 3; i++ {
		idx := 0*width + i
		screen.buffer[idx].Attrs.TextColor = redColor
	}

	screen.PutText(0, 1, "Blue", Attrs(0))
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
		mask   AttributeMask
		want   bool
	}{
		{
			name:   "check text color",
			x:      0,
			y:      0,
			width:  3,
			height: 1,
			text:   "Red",
			mask: AttributeMask{
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
			mask: AttributeMask{
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
			mask: AttributeMask{
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
			mask: AttributeMask{
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
	screen := NewMockScreen(10, 5, 0)
	screen.PutText(0, 0, "Hello", Attrs(AttrBold))
	screen.PutText(0, 1, "World", Attrs(AttrItalic))

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
			assert.Equal(t, CellAttributes{}, cell.Attrs)
		}
	}
}

func TestMockScreen_InterfaceCompliance(t *testing.T) {
	var _ Screen = (*ScreenBuffer)(nil)
}

func TestAttributeMask_Partial(t *testing.T) {
	screen := NewMockScreen(20, 10, 0)

	// Create text with bold and italic
	screen.PutText(0, 0, "Text", Attrs(AttrBold|AttrItalic|AttrUnderline))
	width, _, _ := screen.GetContent()
	for i := 0; i < 4; i++ {
		idx := 0*width + i
		screen.buffer[idx].Attrs.TextColor = 0xFF0000
	}

	verifier := getVerifier(screen)
	tests := []struct {
		name string
		mask AttributeMask
		want bool
	}{
		{
			name: "check only bold (ignore italic and underline)",
			mask: AttributeMask{
				CheckAttributes: true,
				Attributes:      AttrBold | AttrItalic | AttrUnderline,
			},
			want: true,
		},
		{
			name: "check only text color (ignore attributes)",
			mask: AttributeMask{
				CheckTextColor: true,
				TextColor:      0xFF0000,
			},
			want: true,
		},
		{
			name: "check both color and attributes",
			mask: AttributeMask{
				CheckAttributes: true,
				CheckTextColor:  true,
				Attributes:      AttrBold | AttrItalic | AttrUnderline,
				TextColor:       0xFF0000,
			},
			want: true,
		},
		{
			name: "check nothing - always matches",
			mask: AttributeMask{},
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
			screen := NewMockScreen(10, 5, tt.queueSize)
			ch := make(chan InputEvent, tt.channelSize)
			screen.Listen(ch)

			// Send events
			for i := 0; i < tt.eventCount; i++ {
				event := InputEvent{
					Type: InputEventKey,
					Key:  rune('a' + i),
				}
				screen.Notify(event)
			}

			// Receive events
			receivedCount := 0
			for i := 0; i < tt.wantEvents; i++ {
				select {
				case ev := <-ch:
					assert.Equal(t, InputEventKey, ev.Type)
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
	screen := NewMockScreen(10, 5, 10)

	// Create multiple listeners
	ch1 := make(chan InputEvent, 10)
	ch2 := make(chan InputEvent, 10)
	ch3 := make(chan InputEvent, 10)

	screen.Listen(ch1)
	screen.Listen(ch2)
	screen.Listen(ch3)

	// Send an event
	event := InputEvent{
		Type: InputEventKey,
		Key:  'x',
	}
	screen.Notify(event)

	// All listeners should receive the event
	ev1 := <-ch1
	ev2 := <-ch2
	ev3 := <-ch3

	assert.Equal(t, InputEventKey, ev1.Type)
	assert.Equal(t, 'x', ev1.Key)
	assert.Equal(t, InputEventKey, ev2.Type)
	assert.Equal(t, 'x', ev2.Key)
	assert.Equal(t, InputEventKey, ev3.Type)
	assert.Equal(t, 'x', ev3.Key)
}

func TestScreenBuffer_Notify_FullChannel_Queue(t *testing.T) {
	screen := NewMockScreen(10, 5, 10)

	// Create a channel with size 1
	ch := make(chan InputEvent, 1)
	screen.Listen(ch)

	// Send first event - should go directly to channel
	event1 := InputEvent{Type: InputEventKey, Key: '1'}
	screen.Notify(event1)

	// Verify channel has the first event
	select {
	case ev := <-ch:
		assert.Equal(t, '1', ev.Key)
	default:
		t.Fatal("ScreenBuffer.Notify() at buffer_test.go: expected event in channel")
	}

	// Now channel is empty, send multiple events to fill channel and queue
	screen.Notify(InputEvent{Type: InputEventKey, Key: '2'})
	screen.Notify(InputEvent{Type: InputEventKey, Key: '3'})
	screen.Notify(InputEvent{Type: InputEventKey, Key: '4'})

	// Receive events - queued events should be delivered when we make space
	for i := 2; i <= 4; i++ {
		// Read from channel
		var ev InputEvent
		select {
		case ev = <-ch:
			// Got an event
		default:
			// Channel empty, notify to trigger queue delivery
			screen.Notify(InputEvent{Type: InputEventKey, Key: 'X'})
			ev = <-ch
		}
		expectedKey := rune('0' + i)
		assert.Equal(t, expectedKey, ev.Key, "ScreenBuffer.Notify() at buffer_test.go: event mismatch at position %d", i)
	}
}

func TestScreenBuffer_Notify_QueueOverflow(t *testing.T) {
	queueSize := 3
	screen := NewMockScreen(10, 5, queueSize)

	// Create a buffered channel
	ch := make(chan InputEvent, 1)
	screen.Listen(ch)

	// Fill the channel first
	screen.Notify(InputEvent{Type: InputEventKey, Key: '0'})

	// Now send more events than queue size - they will be queued
	// Send queueSize + 2 more events (total queueSize + 3)
	for i := 1; i < queueSize+3; i++ {
		event := InputEvent{Type: InputEventKey, Key: rune('0' + i)}
		screen.Notify(event)
	}

	// Read the first event from channel
	ev := <-ch
	assert.Equal(t, '0', ev.Key)

	// Notify to trigger delivery of queued events
	screen.Notify(InputEvent{Type: InputEventKey, Key: 'X'})

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
	screen := NewMockScreen(10, 5, 10)

	// Create and register a channel
	ch := make(chan InputEvent, 10)
	screen.Listen(ch)

	// Send an event
	event1 := InputEvent{Type: InputEventKey, Key: 'a'}
	screen.Notify(event1)

	// Receive it
	ev := <-ch
	assert.Equal(t, 'a', ev.Key)

	// Close the channel
	close(ch)

	// Send another event - should not panic
	event2 := InputEvent{Type: InputEventKey, Key: 'b'}
	require.NotPanics(t, func() {
		screen.Notify(event2)
	})
}

func TestScreenBuffer_Notify_EventOrder(t *testing.T) {
	screen := NewMockScreen(10, 5, 100)

	ch := make(chan InputEvent, 1)
	screen.Listen(ch)

	// Fill the channel
	screen.Notify(InputEvent{Type: InputEventKey, Key: '0'})

	// Queue multiple events
	for i := 1; i <= 10; i++ {
		screen.Notify(InputEvent{Type: InputEventKey, Key: rune('0' + i)})
	}

	// Receive all events and verify order
	for i := 0; i <= 10; i++ {
		// Read from channel to make space
		ev := <-ch

		// Trigger delivery of queued events
		if i < 10 {
			screen.Notify(InputEvent{Type: InputEventKey, Key: 'X'})
		}

		expectedKey := rune('0' + i)
		assert.Equal(t, expectedKey, ev.Key, "ScreenBuffer.Notify() at buffer_test.go: event order mismatch at position %d", i)
	}
}

func TestScreenBuffer_Notify_NoListeners(t *testing.T) {
	screen := NewMockScreen(10, 5, 10)

	// Send event with no listeners - should not panic
	event := InputEvent{Type: InputEventKey, Key: 'a'}
	require.NotPanics(t, func() {
		screen.Notify(event)
	})
}

func TestScreenBuffer_Listen_MultipleRegistrations(t *testing.T) {
	screen := NewMockScreen(10, 5, 10)

	ch := make(chan InputEvent, 10)

	// Register the same channel multiple times
	screen.Listen(ch)
	screen.Listen(ch)

	// Send an event
	event := InputEvent{Type: InputEventKey, Key: 'a'}
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
