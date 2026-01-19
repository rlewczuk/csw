package tui

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gophertv"
	"github.com/codesnort/codesnort-swe/pkg/gophertv/tio"
	"github.com/stretchr/testify/assert"
)

func TestNewLabel_BasicCreation(t *testing.T) {
	// Create a label with explicit dimensions
	attrs := gophertv.CellAttributes{
		TextColor: 0xFF0000,
		BackColor: 0x00FF00,
	}
	rect := gophertv.TRect{X: 10, Y: 5, W: 20, H: 1}
	label := NewLabel(nil, "Hello World", rect, attrs)

	assert.NotNil(t, label, "Label should be created")
	assert.Equal(t, "Hello World", label.GetText(), "Text should match")
	assert.Equal(t, attrs, label.GetAttrs(), "Attributes should match")
	assert.Equal(t, uint16(10), label.Position.X, "X position should match")
	assert.Equal(t, uint16(5), label.Position.Y, "Y position should match")
	assert.Equal(t, uint16(20), label.Position.W, "Width should match")
	assert.Equal(t, uint16(1), label.Position.H, "Height should match")
	assert.Nil(t, label.Parent, "Parent should be nil")
}

func TestNewLabel_AutoSize(t *testing.T) {
	// Create a label with zero dimensions (auto-size)
	attrs := gophertv.CellAttributes{}
	rect := gophertv.TRect{X: 10, Y: 5, W: 0, H: 0}
	label := NewLabel(nil, "Hello", rect, attrs)

	assert.NotNil(t, label, "Label should be created")
	assert.Equal(t, uint16(5), label.Position.W, "Width should be auto-sized to text length")
	assert.Equal(t, uint16(1), label.Position.H, "Height should be 1")
}

func TestNewLabel_AutoSizeEmpty(t *testing.T) {
	// Create a label with empty text and zero dimensions
	attrs := gophertv.CellAttributes{}
	rect := gophertv.TRect{X: 0, Y: 0, W: 0, H: 0}
	label := NewLabel(nil, "", rect, attrs)

	assert.NotNil(t, label, "Label should be created")
	assert.Equal(t, uint16(0), label.Position.W, "Width should be 0 for empty text")
	assert.Equal(t, uint16(1), label.Position.H, "Height should be 1")
}

func TestNewLabel_WithParent(t *testing.T) {
	// Create parent widget
	parent := &TWidget{
		Position: gophertv.TRect{X: 10, Y: 20, W: 100, H: 100},
	}

	// Create child label
	attrs := gophertv.CellAttributes{}
	rect := gophertv.TRect{X: 5, Y: 10, W: 15, H: 1}
	label := NewLabel(parent, "Test", rect, attrs)

	assert.NotNil(t, label, "Label should be created")
	assert.Equal(t, parent, label.Parent, "Parent should be set")
	assert.Len(t, parent.Children, 1, "Parent should have one child")
	assert.Equal(t, label, parent.Children[0], "Child should be the label")
}

func TestTLabel_GetText(t *testing.T) {
	label := NewLabel(nil, "Test Text", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, gophertv.CellAttributes{})
	assert.Equal(t, "Test Text", label.GetText(), "GetText should return the current text")
}

func TestTLabel_SetText(t *testing.T) {
	label := NewLabel(nil, "Initial", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, gophertv.CellAttributes{})
	assert.Equal(t, "Initial", label.GetText(), "Initial text should match")

	label.SetText("Updated")
	assert.Equal(t, "Updated", label.GetText(), "Text should be updated")
}

func TestTLabel_SetText_InvalidatesCache(t *testing.T) {
	label := NewLabel(nil, "Initial", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, gophertv.CellAttributes{})

	// Force cache to be populated
	_ = label.getFormattedCells()
	assert.True(t, label.cacheValid, "Cache should be valid after first call")

	// Set new text
	label.SetText("Updated")
	assert.False(t, label.cacheValid, "Cache should be invalid after SetText")
}

func TestTLabel_SetText_SameText_NoInvalidation(t *testing.T) {
	label := NewLabel(nil, "Same", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, gophertv.CellAttributes{})

	// Force cache to be populated
	_ = label.getFormattedCells()
	assert.True(t, label.cacheValid, "Cache should be valid after first call")

	// Set same text
	label.SetText("Same")
	assert.True(t, label.cacheValid, "Cache should remain valid if text is same")
}

func TestTLabel_GetAttrs(t *testing.T) {
	attrs := gophertv.CellAttributes{
		TextColor: 0xFF0000,
		BackColor: 0x00FF00,
	}
	label := NewLabel(nil, "Test", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, attrs)
	assert.Equal(t, attrs, label.GetAttrs(), "GetAttrs should return the current attributes")
}

func TestTLabel_SetAttrs(t *testing.T) {
	initialAttrs := gophertv.CellAttributes{
		TextColor: 0xFF0000,
	}
	label := NewLabel(nil, "Test", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, initialAttrs)

	newAttrs := gophertv.CellAttributes{
		TextColor: 0x00FF00,
	}
	label.SetAttrs(newAttrs)
	assert.Equal(t, newAttrs, label.GetAttrs(), "Attributes should be updated")
}

func TestTLabel_SetAttrs_InvalidatesCache(t *testing.T) {
	attrs := gophertv.CellAttributes{TextColor: 0xFF0000}
	label := NewLabel(nil, "Test", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, attrs)

	// Force cache to be populated
	_ = label.getFormattedCells()
	assert.True(t, label.cacheValid, "Cache should be valid after first call")

	// Set new attributes
	newAttrs := gophertv.CellAttributes{TextColor: 0x00FF00}
	label.SetAttrs(newAttrs)
	assert.False(t, label.cacheValid, "Cache should be invalid after SetAttrs")
}

func TestTLabel_Draw_PlainText(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create label
	attrs := gophertv.CellAttributes{
		TextColor: 0xFF0000,
	}
	label := NewLabel(nil, "Hello", gophertv.TRect{X: 5, Y: 10, W: 10, H: 1}, attrs)

	// Draw label
	label.Draw(screen)

	// Verify output
	_, _, content := screen.GetContent()
	verifier := gophertv.NewScreenVerifier(80, 24, content)

	assert.True(t, verifier.HasText(5, 10, 5, 1, "Hello"), "Text should be drawn at correct position")

	// Verify attributes
	mask := gophertv.AttributeMask{
		CheckTextColor: true,
		TextColor:      0xFF0000,
	}
	assert.True(t, verifier.HasTextWithAttrs(5, 10, 5, 1, "Hello", mask), "Text should have correct color")
}

func TestTLabel_Draw_FormattedText(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create label with formatted text
	attrs := gophertv.CellAttributes{
		TextColor: 0xFF0000,
	}
	label := NewLabel(nil, "**bold** text", gophertv.TRect{X: 0, Y: 0, W: 20, H: 1}, attrs)

	// Draw label
	label.Draw(screen)

	// Verify output
	_, _, content := screen.GetContent()
	verifier := gophertv.NewScreenVerifier(80, 24, content)

	// Check that the text "bold text" is rendered (without markdown markers)
	assert.True(t, verifier.HasText(0, 0, 9, 1, "bold text"), "Formatted text should be rendered without markers")

	// Verify bold attribute on "bold" portion
	boldMask := gophertv.AttributeMask{
		CheckAttributes: true,
		CheckTextColor:  true,
		Attributes:      gophertv.AttrBold,
		TextColor:       0xFF0000,
	}
	assert.True(t, verifier.HasTextWithAttrs(0, 0, 4, 1, "bold", boldMask), "Bold portion should have bold attribute and color")

	// Verify non-bold portion has only color
	plainMask := gophertv.AttributeMask{
		CheckTextColor: true,
		TextColor:      0xFF0000,
	}
	assert.True(t, verifier.HasTextWithAttrs(5, 0, 4, 1, "text", plainMask), "Plain portion should have color only")
}

func TestTLabel_Draw_AttributeCombination(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create label with base attributes
	attrs := gophertv.CellAttributes{
		Attributes: gophertv.AttrDim,
		TextColor:  0xFF0000,
	}
	label := NewLabel(nil, "*italic*", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, attrs)

	// Draw label
	label.Draw(screen)

	// Verify output
	_, _, content := screen.GetContent()
	verifier := gophertv.NewScreenVerifier(80, 24, content)

	// Verify that italic text has both Dim (from base) and Italic (from format) attributes
	combinedMask := gophertv.AttributeMask{
		CheckAttributes: true,
		CheckTextColor:  true,
		Attributes:      gophertv.AttrDim | gophertv.AttrItalic,
		TextColor:       0xFF0000,
	}
	assert.True(t, verifier.HasTextWithAttrs(0, 0, 6, 1, "italic", combinedMask), "Text should have combined attributes")
}

func TestTLabel_Draw_Hidden(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create label
	label := NewLabel(nil, "Hidden", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, gophertv.CellAttributes{})
	label.Flags = WidgetFlagHidden

	// Draw label
	label.Draw(screen)

	// Verify that nothing is drawn
	_, _, content := screen.GetContent()
	verifier := gophertv.NewScreenVerifier(80, 24, content)
	assert.False(t, verifier.HasText(0, 0, 6, 1, "Hidden"), "Hidden label should not be drawn")
}

func TestTLabel_Draw_WithParent(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create parent widget
	parent := &TWidget{
		Position: gophertv.TRect{X: 10, Y: 20, W: 100, H: 100},
	}

	// Create child label
	label := NewLabel(parent, "Child", gophertv.TRect{X: 5, Y: 3, W: 10, H: 1}, gophertv.CellAttributes{})

	// Draw label
	label.Draw(screen)

	// Verify output at absolute position (parent + child)
	_, _, content := screen.GetContent()
	verifier := gophertv.NewScreenVerifier(80, 24, content)
	assert.True(t, verifier.HasText(15, 23, 5, 1, "Child"), "Label should be drawn at absolute position")
}

func TestTLabel_HandleEvent_Position(t *testing.T) {
	label := NewLabel(nil, "Test", gophertv.TRect{X: 10, Y: 20, W: 10, H: 1}, gophertv.CellAttributes{})

	// Create position event
	event := &TEvent{
		Type: TEventTypePosition,
		Rect: gophertv.TRect{X: 30, Y: 40, W: 50, H: 2},
	}

	// Handle event
	label.HandleEvent(event)

	// Verify position was updated
	assert.Equal(t, uint16(30), label.Position.X, "X should be updated")
	assert.Equal(t, uint16(40), label.Position.Y, "Y should be updated")
	assert.Equal(t, uint16(50), label.Position.W, "W should be updated")
	assert.Equal(t, uint16(2), label.Position.H, "H should be updated")
}

func TestTLabel_HandleEvent_Input(t *testing.T) {
	label := NewLabel(nil, "Test", gophertv.TRect{X: 0, Y: 0, W: 10, H: 1}, gophertv.CellAttributes{})

	// Create input event
	event := &TEvent{
		Type: TEventTypeInput,
		InputEvent: &gophertv.InputEvent{
			Type: gophertv.InputEventKey,
		},
	}

	// Handle event (should not panic, labels are passive)
	label.HandleEvent(event)
}

func TestTLabel_GetAbsolutePos_NoParent(t *testing.T) {
	label := NewLabel(nil, "Test", gophertv.TRect{X: 10, Y: 20, W: 30, H: 1}, gophertv.CellAttributes{})

	absPos := label.GetAbsolutePos()
	assert.Equal(t, uint16(10), absPos.X, "X should match label position")
	assert.Equal(t, uint16(20), absPos.Y, "Y should match label position")
	assert.Equal(t, uint16(30), absPos.W, "W should match label position")
	assert.Equal(t, uint16(1), absPos.H, "H should match label position")
}

func TestTLabel_GetAbsolutePos_WithParent(t *testing.T) {
	// Create parent widget
	parent := &TWidget{
		Position: gophertv.TRect{X: 10, Y: 20, W: 100, H: 100},
	}

	// Create child label
	label := NewLabel(parent, "Test", gophertv.TRect{X: 5, Y: 10, W: 30, H: 1}, gophertv.CellAttributes{})

	absPos := label.GetAbsolutePos()
	assert.Equal(t, uint16(15), absPos.X, "X should be parent.X + label.X")
	assert.Equal(t, uint16(30), absPos.Y, "Y should be parent.Y + label.Y")
	assert.Equal(t, uint16(30), absPos.W, "W should match label position")
	assert.Equal(t, uint16(1), absPos.H, "H should match label position")
}

func TestTLabel_ComplexFormatting(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create label with complex formatted text
	attrs := gophertv.CellAttributes{
		Attributes: gophertv.AttrUnderline,
		TextColor:  0x00FF00,
	}
	label := NewLabel(nil, "**bold** and *italic* and ~~strike~~", gophertv.TRect{X: 0, Y: 0, W: 50, H: 1}, attrs)

	// Draw label
	label.Draw(screen)

	// Verify output
	_, _, content := screen.GetContent()
	verifier := gophertv.NewScreenVerifier(80, 24, content)

	// Verify the text is rendered correctly
	expectedText := "bold and italic and strike"
	assert.True(t, verifier.HasText(0, 0, len(expectedText), 1, expectedText), "Complex formatted text should be rendered")

	// Verify bold portion has both underline (base) and bold (format)
	boldMask := gophertv.AttributeMask{
		CheckAttributes: true,
		CheckTextColor:  true,
		Attributes:      gophertv.AttrUnderline | gophertv.AttrBold,
		TextColor:       0x00FF00,
	}
	assert.True(t, verifier.HasTextWithAttrs(0, 0, 4, 1, "bold", boldMask), "Bold text should have combined attributes")

	// Verify italic portion has both underline (base) and italic (format)
	italicMask := gophertv.AttributeMask{
		CheckAttributes: true,
		CheckTextColor:  true,
		Attributes:      gophertv.AttrUnderline | gophertv.AttrItalic,
		TextColor:       0x00FF00,
	}
	assert.True(t, verifier.HasTextWithAttrs(9, 0, 6, 1, "italic", italicMask), "Italic text should have combined attributes")

	// Verify strikethrough portion has both underline (base) and strikethrough (format)
	strikeMask := gophertv.AttributeMask{
		CheckAttributes: true,
		CheckTextColor:  true,
		Attributes:      gophertv.AttrUnderline | gophertv.AttrStrikethrough,
		TextColor:       0x00FF00,
	}
	assert.True(t, verifier.HasTextWithAttrs(20, 0, 6, 1, "strike", strikeMask), "Strikethrough text should have combined attributes")
}

func TestTLabel_EmptyText(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create label with empty text
	label := NewLabel(nil, "", gophertv.TRect{X: 5, Y: 10, W: 10, H: 1}, gophertv.CellAttributes{})

	// Draw label (should not panic)
	label.Draw(screen)

	// Test passes if no panic occurred
}

func TestTLabel_LongText(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create label with text longer than screen width
	longText := "This is a very long text that exceeds the screen width and should be clipped properly"
	label := NewLabel(nil, longText, gophertv.TRect{X: 0, Y: 0, W: uint16(len(longText)), H: 1}, gophertv.CellAttributes{})

	// Draw label (should not panic)
	label.Draw(screen)

	// Verify that text is drawn (will be clipped by screen boundary)
	_, _, content := screen.GetContent()
	verifier := gophertv.NewScreenVerifier(80, 24, content)

	// Should have at least the first 80 characters
	expectedText := longText[:80]
	assert.True(t, verifier.HasText(0, 0, 80, 1, expectedText), "Long text should be drawn and clipped")
}

func TestTLabel_InterfaceCompliance(t *testing.T) {
	// Verify that TLabel implements ILabel interface
	var _ ILabel = (*TLabel)(nil)
}
