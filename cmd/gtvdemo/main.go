package main

import (
	"fmt"
	"os"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
)

func main() {
	// Create a screen buffer with initial size (will be resized to terminal size in Run())
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout that fills the entire screen
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&gtv.CellAttributes{BackColor: 0x1a1a1a}, // Dark gray background
	)

	// Create labels for form fields (left column)
	nameLabel := tui.NewLabel(
		mainLayout,
		"Name:",
		gtv.TRect{X: 5, Y: 3, W: 0, H: 0},
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0),
	)

	emailLabel := tui.NewLabel(
		mainLayout,
		"Email:",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0),
	)

	phoneLabel := tui.NewLabel(
		mainLayout,
		"Phone:",
		gtv.TRect{X: 5, Y: 7, W: 0, H: 0},
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0),
	)

	// Create input boxes for form fields (right column, aligned)
	nameInput := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(15, 3, 30, 1),
		tui.WithAttrs(gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333)),        // Normal: white on dark gray
		tui.WithFocusedAttrs(gtv.AttrsWithColor(0, 0x000000, 0x00AAFF)), // Focused: black on light blue
	)

	emailInput := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(15, 5, 30, 1),
		tui.WithAttrs(gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333)),
		tui.WithFocusedAttrs(gtv.AttrsWithColor(0, 0x000000, 0x00AAFF)),
	)

	phoneInput := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(15, 7, 30, 1),
		tui.WithAttrs(gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333)),
		tui.WithFocusedAttrs(gtv.AttrsWithColor(0, 0x000000, 0x00AAFF)),
	)

	// Create result label (below input fields)
	resultLabel := tui.NewLabel(
		mainLayout,
		"",
		gtv.TRect{X: 5, Y: 12, W: 60, H: 1},
		gtv.AttrsWithColor(0, 0x00FF00, 0),
	)

	// Create Submit button
	submitButton := tui.NewButton(
		mainLayout,
		"Submit",
		gtv.TRect{X: 15, Y: 9, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x006600),            // Normal: white on dark green
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0x00AA00), // Focused: bold white on green
		gtv.AttrsWithColor(0, 0x888888, 0x333333),            // Disabled: gray on dark gray
	)

	// Set Submit button action
	submitButton.SetOnPress(func() {
		name := nameInput.GetText()
		email := emailInput.GetText()
		phone := phoneInput.GetText()

		if name == "" && email == "" && phone == "" {
			resultLabel.SetText("Please enter at least one field!")
			resultLabel.SetAttrs(gtv.AttrsWithColor(0, 0xFF0000, 0)) // Red text for error
		} else {
			result := fmt.Sprintf("Submitted - Name: %s, Email: %s, Phone: %s", name, email, phone)
			resultLabel.SetText(result)
			resultLabel.SetAttrs(gtv.AttrsWithColor(0, 0x00FF00, 0)) // Green text for success
		}
	})

	// Create Clear button
	clearButton := tui.NewButton(
		mainLayout,
		"Clear",
		gtv.TRect{X: 28, Y: 9, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x660000),            // Normal: white on dark red
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0xAA0000), // Focused: bold white on red
		gtv.AttrsWithColor(0, 0x888888, 0x333333),            // Disabled: gray on dark gray
	)

	// Set Clear button action
	clearButton.SetOnPress(func() {
		nameInput.SetText("")
		emailInput.SetText("")
		phoneInput.SetText("")
		resultLabel.SetText("")
	})

	// Add a title label at the top
	titleLabel := tui.NewLabel(
		mainLayout,
		"GTV Demo Application - Data Entry Form",
		gtv.TRect{X: 5, Y: 1, W: 0, H: 0},
		gtv.AttrsWithColor(gtv.AttrBold|gtv.AttrUnderline, 0x00AAFF, 0),
	)

	// Add instructions at the bottom
	instructionsLabel := tui.NewLabel(
		mainLayout,
		"Press Tab to move between fields. Press Enter/Space on buttons. Press Ctrl+C to quit.",
		gtv.TRect{X: 5, Y: 14, W: 0, H: 0},
		gtv.AttrsWithColor(gtv.AttrItalic, 0x888888, 0),
	)

	// Avoid unused variable errors
	_ = nameLabel
	_ = emailLabel
	_ = phoneLabel
	_ = titleLabel
	_ = instructionsLabel

	// Create the application
	app := tui.NewApplication(mainLayout, screen)

	// Run the application
	if err := app.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}
}
