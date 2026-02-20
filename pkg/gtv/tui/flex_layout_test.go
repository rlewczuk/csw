package tui_test

import (
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"
	"github.com/rlewczuk/csw/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlexLayout_VerticalLayoutWithFixedAndFlexItems tests a vertical layout
// with a smaller fixed-height item at the bottom and a larger flexible item that fills the space.
// The horizontal size of both widgets should be automatically resized to fill the space.
func TestFlexLayout_VerticalLayoutWithFixedAndFlexItems(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create flex layout with vertical direction
	flexLayout := tui.NewFlexLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		tui.FlexDirectionColumn,
	)

	// Create two label widgets
	// First label should expand to fill the space (flex-grow)
	label1 := tui.NewLabel(
		nil,
		"Expanding Area",
		gtv.TRect{X: 0, Y: 0, W: 10, H: 5},
		gtv.CellTag("label"),
	)

	// Second label has fixed height and should not resize vertically
	label2 := tui.NewLabel(
		nil,
		"Fixed Area",
		gtv.TRect{X: 0, Y: 0, W: 10, H: 3},
		gtv.CellTag("label"),
	)

	// Add children to flex layout
	flexLayout.AddChild(label1)
	flexLayout.AddChild(label2)

	// Set flex properties
	// First label should grow to fill space
	flexLayout.SetItemProperties(label1, tui.FlexItemProperties{
		FlexGrow:   1.0,
		FlexShrink: 1.0,
	})

	// Second label has fixed height (3 lines)
	flexLayout.SetItemProperties(label2, tui.FlexItemProperties{
		FixedHeight: 3,
		FlexGrow:    0,
		FlexShrink:  0,
	})

	// Create application
	app := tui.NewApplication(flexLayout, screen)
	require.NotNil(t, app)

	// Draw initial frame
	flexLayout.Draw(screen)

	// Verify initial layout
	// Label1 should expand vertically to fill available space (24 - 3 = 21 lines)
	// Label2 should be 3 lines at the bottom
	label1Pos := label1.GetPos()
	label2Pos := label2.GetPos()

	assert.Equal(t, uint16(0), label1Pos.X, "Label1 X position")
	assert.Equal(t, uint16(0), label1Pos.Y, "Label1 Y position")
	assert.Equal(t, uint16(80), label1Pos.W, "Label1 width should fill available space")
	assert.Equal(t, uint16(21), label1Pos.H, "Label1 height should expand to fill space")

	assert.Equal(t, uint16(0), label2Pos.X, "Label2 X position")
	assert.Equal(t, uint16(21), label2Pos.Y, "Label2 Y position")
	assert.Equal(t, uint16(80), label2Pos.W, "Label2 width should fill available space")
	assert.Equal(t, uint16(3), label2Pos.H, "Label2 height should be fixed")

	// Test resize - simulate terminal resize to 80x30
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.Resize(80, 30)

	// Give time for resize to process
	time.Sleep(10 * time.Millisecond)

	// Redraw after resize
	flexLayout.Draw(screen)

	// Verify layout after resize
	// Label1 should now be 30 - 3 = 27 lines
	// Label2 should still be 3 lines
	label1Pos = label1.GetPos()
	label2Pos = label2.GetPos()

	assert.Equal(t, uint16(0), label1Pos.X, "Label1 X after resize")
	assert.Equal(t, uint16(0), label1Pos.Y, "Label1 Y after resize")
	assert.Equal(t, uint16(80), label1Pos.W, "Label1 width after resize")
	assert.Equal(t, uint16(27), label1Pos.H, "Label1 height should grow with resize")

	assert.Equal(t, uint16(0), label2Pos.X, "Label2 X after resize")
	assert.Equal(t, uint16(27), label2Pos.Y, "Label2 Y after resize")
	assert.Equal(t, uint16(80), label2Pos.W, "Label2 width after resize")
	assert.Equal(t, uint16(3), label2Pos.H, "Label2 height should remain fixed after resize")
}

// TestFlexLayout_HorizontalLayoutWithFlexGrow tests horizontal layout with flex-grow.
func TestFlexLayout_HorizontalLayoutWithFlexGrow(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create horizontal flex layout
	flexLayout := tui.NewFlexLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		tui.FlexDirectionRow,
	)

	// Create three labels with different flex-grow values
	label1 := tui.NewLabel(nil, "A", gtv.TRect{X: 0, Y: 0, W: 10, H: 24}, gtv.CellTag("label"))
	label2 := tui.NewLabel(nil, "B", gtv.TRect{X: 0, Y: 0, W: 10, H: 24}, gtv.CellTag("label"))
	label3 := tui.NewLabel(nil, "C", gtv.TRect{X: 0, Y: 0, W: 10, H: 24}, gtv.CellTag("label"))

	flexLayout.AddChild(label1)
	flexLayout.AddChild(label2)
	flexLayout.AddChild(label3)

	// Set flex properties: label1 grows 2x, label2 grows 1x, label3 doesn't grow
	flexLayout.SetItemProperties(label1, tui.FlexItemProperties{FlexGrow: 2.0, FlexShrink: 1.0})
	flexLayout.SetItemProperties(label2, tui.FlexItemProperties{FlexGrow: 1.0, FlexShrink: 1.0})
	flexLayout.SetItemProperties(label3, tui.FlexItemProperties{FlexGrow: 0, FlexShrink: 1.0})

	app := tui.NewApplication(flexLayout, screen)
	require.NotNil(t, app)

	flexLayout.Draw(screen)

	// Verify layout
	// Total width = 80
	// Base width used = 10 + 10 + 10 = 30
	// Extra space = 80 - 30 = 50
	// label1 gets 2/3 of 50 = 33.33 (33) + 10 = 43
	// label2 gets 1/3 of 50 = 16.67 (16) + 10 = 26
	// label3 gets 0 + 10 = 10
	label1Pos := label1.GetPos()
	label2Pos := label2.GetPos()
	label3Pos := label3.GetPos()

	assert.Equal(t, uint16(0), label1Pos.X, "Label1 X position")
	assert.Greater(t, label1Pos.W, uint16(10), "Label1 should grow")

	assert.Greater(t, label2Pos.X, uint16(0), "Label2 X should be after label1")
	assert.Greater(t, label2Pos.W, uint16(10), "Label2 should grow")

	assert.Greater(t, label3Pos.X, label2Pos.X+label2Pos.W-1, "Label3 X should be after label2")
	assert.Equal(t, uint16(10), label3Pos.W, "Label3 should not grow")

	// All labels should have same height (stretch on cross axis)
	assert.Equal(t, uint16(24), label1Pos.H, "Label1 height")
	assert.Equal(t, uint16(24), label2Pos.H, "Label2 height")
	assert.Equal(t, uint16(24), label3Pos.H, "Label3 height")
}

// TestFlexLayout_JustifyContent tests different justify content values.
func TestFlexLayout_JustifyContent(t *testing.T) {
	tests := []struct {
		name    string
		justify tui.FlexJustifyContent
	}{
		{"FlexJustifyStart", tui.FlexJustifyStart},
		{"FlexJustifyEnd", tui.FlexJustifyEnd},
		{"FlexJustifyCenter", tui.FlexJustifyCenter},
		{"FlexJustifySpaceBetween", tui.FlexJustifySpaceBetween},
		{"FlexJustifySpaceAround", tui.FlexJustifySpaceAround},
		{"FlexJustifySpaceEvenly", tui.FlexJustifySpaceEvenly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := tio.NewScreenBuffer(80, 24, 0)

			flexLayout := tui.NewFlexLayout(
				nil,
				gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
				tui.FlexDirectionRow,
				tui.WithFlexJustify(tt.justify),
			)

			// Create two fixed-width labels (10 chars each)
			label1 := tui.NewLabel(nil, "A", gtv.TRect{X: 0, Y: 0, W: 10, H: 24}, gtv.CellTag("label"))
			label2 := tui.NewLabel(nil, "B", gtv.TRect{X: 0, Y: 0, W: 10, H: 24}, gtv.CellTag("label"))

			flexLayout.AddChild(label1)
			flexLayout.AddChild(label2)

			// Set items to not grow or shrink
			flexLayout.SetItemProperties(label1, tui.FlexItemProperties{FlexGrow: 0, FlexShrink: 0})
			flexLayout.SetItemProperties(label2, tui.FlexItemProperties{FlexGrow: 0, FlexShrink: 0})

			app := tui.NewApplication(flexLayout, screen)
			require.NotNil(t, app)

			flexLayout.Draw(screen)

			label1Pos := label1.GetPos()
			label2Pos := label2.GetPos()

			switch tt.justify {
			case tui.FlexJustifyStart:
				assert.Equal(t, uint16(0), label1Pos.X, "Label1 should start at 0")
				assert.Equal(t, uint16(10), label2Pos.X, "Label2 should be right after label1")
			case tui.FlexJustifyEnd:
				assert.Equal(t, uint16(70), label2Pos.X, "Label2 should be at end")
			case tui.FlexJustifyCenter:
				// Total used = 20, extra = 60, offset = 30
				assert.Equal(t, uint16(30), label1Pos.X, "Label1 should be centered")
			case tui.FlexJustifySpaceBetween:
				assert.Equal(t, uint16(0), label1Pos.X, "Label1 should be at start")
				assert.Equal(t, uint16(70), label2Pos.X, "Label2 should be at end")
			case tui.FlexJustifySpaceAround:
				// Space around each = 60 / 2 = 30, half = 15
				assert.Equal(t, uint16(15), label1Pos.X, "Label1 should have space around")
			case tui.FlexJustifySpaceEvenly:
				// Space evenly = 60 / 3 = 20
				assert.Equal(t, uint16(20), label1Pos.X, "Label1 should have even space")
			}
		})
	}
}

// TestFlexLayout_AlignItems tests different align items values for horizontal layout.
func TestFlexLayout_AlignItems(t *testing.T) {
	tests := []struct {
		name  string
		align tui.FlexAlignItems
	}{
		{"FlexAlignStart", tui.FlexAlignStart},
		{"FlexAlignEnd", tui.FlexAlignEnd},
		{"FlexAlignCenter", tui.FlexAlignCenter},
		{"FlexAlignStretch", tui.FlexAlignStretch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := tio.NewScreenBuffer(80, 24, 0)

			flexLayout := tui.NewFlexLayout(
				nil,
				gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
				tui.FlexDirectionRow,
				tui.WithFlexAlign(tt.align),
			)

			// Create label
			label := tui.NewLabel(nil, "Test", gtv.TRect{X: 0, Y: 0, W: 10, H: 5}, gtv.CellTag("label"))

			flexLayout.AddChild(label)

			// For stretch test, don't set fixed height; for others, set fixed height
			if tt.align == tui.FlexAlignStretch {
				flexLayout.SetItemProperties(label, tui.FlexItemProperties{
					FlexGrow:   0,
					FlexShrink: 0,
				})
			} else {
				flexLayout.SetItemProperties(label, tui.FlexItemProperties{
					FixedHeight: 5,
					FlexGrow:    0,
					FlexShrink:  0,
				})
			}

			app := tui.NewApplication(flexLayout, screen)
			require.NotNil(t, app)

			flexLayout.Draw(screen)

			labelPos := label.GetPos()

			switch tt.align {
			case tui.FlexAlignStart:
				assert.Equal(t, uint16(0), labelPos.Y, "Label should align to top")
				assert.Equal(t, uint16(5), labelPos.H, "Label height should be fixed")
			case tui.FlexAlignEnd:
				assert.Equal(t, uint16(19), labelPos.Y, "Label should align to bottom")
				assert.Equal(t, uint16(5), labelPos.H, "Label height should be fixed")
			case tui.FlexAlignCenter:
				// (24 - 5) / 2 = 9.5 -> 9
				assert.Equal(t, uint16(9), labelPos.Y, "Label should be centered")
				assert.Equal(t, uint16(5), labelPos.H, "Label height should be fixed")
			case tui.FlexAlignStretch:
				assert.Equal(t, uint16(0), labelPos.Y, "Label should start at top")
				assert.Equal(t, uint16(24), labelPos.H, "Label should stretch to fill height")
			}
		})
	}
}

// TestFlexLayout_MinMaxSize tests minimum and maximum size constraints.
func TestFlexLayout_MinMaxSize(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)

	flexLayout := tui.NewFlexLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		tui.FlexDirectionRow,
	)

	// Create label that should grow but respect max size
	label := tui.NewLabel(nil, "Test", gtv.TRect{X: 0, Y: 0, W: 10, H: 24}, gtv.CellTag("label"))

	flexLayout.AddChild(label)
	flexLayout.SetItemProperties(label, tui.FlexItemProperties{
		FlexGrow:   1.0,
		FlexShrink: 1.0,
		MinSize:    10,
		MaxSize:    30,
	})

	app := tui.NewApplication(flexLayout, screen)
	require.NotNil(t, app)

	flexLayout.Draw(screen)

	labelPos := label.GetPos()

	// Label should grow but not exceed max size of 30
	assert.LessOrEqual(t, labelPos.W, uint16(30), "Label width should not exceed max size")
	assert.GreaterOrEqual(t, labelPos.W, uint16(10), "Label width should not be less than min size")
}

// TestFlexLayout_Padding tests padding for layout and items.
func TestFlexLayout_Padding(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create flex layout with padding
	flexLayout := tui.NewFlexLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		tui.FlexDirectionRow,
		tui.WithFlexPadding(2, 2, 3, 3), // top, bottom, left, right
	)

	label := tui.NewLabel(nil, "Test", gtv.TRect{X: 0, Y: 0, W: 10, H: 10}, gtv.CellTag("label"))

	flexLayout.AddChild(label)
	flexLayout.SetItemProperties(label, tui.FlexItemProperties{
		FlexGrow:      1.0,
		FlexShrink:    1.0,
		PaddingTop:    1,
		PaddingBottom: 1,
		PaddingLeft:   2,
		PaddingRight:  2,
	})

	app := tui.NewApplication(flexLayout, screen)
	require.NotNil(t, app)

	flexLayout.Draw(screen)

	labelPos := label.GetPos()

	// Label should start at layout padding (3 from left) + item padding (2 from left) = 5
	assert.Equal(t, uint16(5), labelPos.X, "Label X should account for padding")

	// Label should start at layout padding (2 from top) + item padding (1 from top) = 3
	assert.Equal(t, uint16(3), labelPos.Y, "Label Y should account for padding")

	// Available width = 80 - 6 (layout padding) = 74
	// Label gets all of it minus its own padding (4) = 70
	assert.Equal(t, uint16(70), labelPos.W, "Label width should account for padding")

	// Available height = 24 - 4 (layout padding) = 20
	// Label gets all of it minus its own padding (2) = 18
	assert.Equal(t, uint16(18), labelPos.H, "Label height should account for padding")
}

// TestFlexLayout_FlexShrink tests flex-shrink when content is larger than container.
func TestFlexLayout_FlexShrink(t *testing.T) {
	screen := tio.NewScreenBuffer(40, 24, 0)

	flexLayout := tui.NewFlexLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 40, H: 24},
		tui.FlexDirectionRow,
	)

	// Create three labels that together are wider than container
	label1 := tui.NewLabel(nil, "A", gtv.TRect{X: 0, Y: 0, W: 20, H: 24}, gtv.CellTag("label"))
	label2 := tui.NewLabel(nil, "B", gtv.TRect{X: 0, Y: 0, W: 20, H: 24}, gtv.CellTag("label"))
	label3 := tui.NewLabel(nil, "C", gtv.TRect{X: 0, Y: 0, W: 20, H: 24}, gtv.CellTag("label"))

	flexLayout.AddChild(label1)
	flexLayout.AddChild(label2)
	flexLayout.AddChild(label3)

	// Set different flex-shrink values
	flexLayout.SetItemProperties(label1, tui.FlexItemProperties{FlexGrow: 0, FlexShrink: 2.0})
	flexLayout.SetItemProperties(label2, tui.FlexItemProperties{FlexGrow: 0, FlexShrink: 1.0})
	flexLayout.SetItemProperties(label3, tui.FlexItemProperties{FlexGrow: 0, FlexShrink: 0})

	app := tui.NewApplication(flexLayout, screen)
	require.NotNil(t, app)

	flexLayout.Draw(screen)

	label1Pos := label1.GetPos()
	label2Pos := label2.GetPos()
	label3Pos := label3.GetPos()

	// Total base width = 60, container = 40, need to shrink by 20
	// label1 shrinks 2x, label2 shrinks 1x, label3 doesn't shrink
	// label1 shrinks by 2/3 of 20 = 13.33 (13), becomes 20 - 13 = 7
	// label2 shrinks by 1/3 of 20 = 6.67 (6), becomes 20 - 6 = 14
	// label3 doesn't shrink, stays 20

	assert.Less(t, label1Pos.W, uint16(20), "Label1 should shrink the most")
	assert.Less(t, label2Pos.W, uint16(20), "Label2 should shrink")
	assert.Equal(t, uint16(20), label3Pos.W, "Label3 should not shrink")
}

// TestFlexLayout_ColumnDirection tests vertical column layout.
func TestFlexLayout_ColumnDirection(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)

	flexLayout := tui.NewFlexLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		tui.FlexDirectionColumn,
	)

	// Create three labels
	label1 := tui.NewLabel(nil, "A", gtv.TRect{X: 0, Y: 0, W: 80, H: 5}, gtv.CellTag("label"))
	label2 := tui.NewLabel(nil, "B", gtv.TRect{X: 0, Y: 0, W: 80, H: 5}, gtv.CellTag("label"))
	label3 := tui.NewLabel(nil, "C", gtv.TRect{X: 0, Y: 0, W: 80, H: 5}, gtv.CellTag("label"))

	flexLayout.AddChild(label1)
	flexLayout.AddChild(label2)
	flexLayout.AddChild(label3)

	// Set flex properties
	flexLayout.SetItemProperties(label1, tui.FlexItemProperties{FlexGrow: 1.0, FlexShrink: 1.0})
	flexLayout.SetItemProperties(label2, tui.FlexItemProperties{FlexGrow: 2.0, FlexShrink: 1.0})
	flexLayout.SetItemProperties(label3, tui.FlexItemProperties{FlexGrow: 0, FlexShrink: 0})

	app := tui.NewApplication(flexLayout, screen)
	require.NotNil(t, app)

	flexLayout.Draw(screen)

	label1Pos := label1.GetPos()
	label2Pos := label2.GetPos()
	label3Pos := label3.GetPos()

	// Verify vertical positioning
	assert.Equal(t, uint16(0), label1Pos.Y, "Label1 should start at top")
	assert.Equal(t, label1Pos.H, label2Pos.Y, "Label2 should follow label1")
	assert.Equal(t, label1Pos.H+label2Pos.H, label3Pos.Y, "Label3 should follow label2")

	// Verify growth
	assert.Greater(t, label1Pos.H, uint16(5), "Label1 should grow")
	assert.Greater(t, label2Pos.H, label1Pos.H, "Label2 should grow more than label1")
	assert.Equal(t, uint16(5), label3Pos.H, "Label3 should not grow")

	// All labels should stretch horizontally
	assert.Equal(t, uint16(80), label1Pos.W, "Label1 width should stretch")
	assert.Equal(t, uint16(80), label2Pos.W, "Label2 width should stretch")
	assert.Equal(t, uint16(80), label3Pos.W, "Label3 width should stretch")
}

// TestFlexLayout_EmptyLayout tests flex layout with no children.
func TestFlexLayout_EmptyLayout(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)

	flexLayout := tui.NewFlexLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		tui.FlexDirectionRow,
	)

	app := tui.NewApplication(flexLayout, screen)
	require.NotNil(t, app)

	// Should not panic
	flexLayout.Draw(screen)
}

// TestFlexLayout_FixedSizes tests items with fixed width and height.
func TestFlexLayout_FixedSizes(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)

	flexLayout := tui.NewFlexLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		tui.FlexDirectionRow,
	)

	label := tui.NewLabel(nil, "Fixed", gtv.TRect{X: 0, Y: 0, W: 50, H: 20}, gtv.CellTag("label"))

	flexLayout.AddChild(label)
	flexLayout.SetItemProperties(label, tui.FlexItemProperties{
		FixedWidth:  30,
		FixedHeight: 10,
		FlexGrow:    0,
		FlexShrink:  0,
	})

	app := tui.NewApplication(flexLayout, screen)
	require.NotNil(t, app)

	flexLayout.Draw(screen)

	labelPos := label.GetPos()

	// Label should have fixed size
	assert.Equal(t, uint16(30), labelPos.W, "Label should have fixed width")
	assert.Equal(t, uint16(10), labelPos.H, "Label should have fixed height")
}
