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
	layoutBackground := gtv.CellTag("layout-background")
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&layoutBackground,
	)

	// Create labels for form fields (left column)
	nameLabel := tui.NewLabel(
		mainLayout,
		"Name:",
		gtv.TRect{X: 5, Y: 3, W: 0, H: 0},
		gtv.CellTag("subtitle"),
	)

	emailLabel := tui.NewLabel(
		mainLayout,
		"Email:",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		gtv.CellTag("subtitle"),
	)

	phoneLabel := tui.NewLabel(
		mainLayout,
		"Phone:",
		gtv.TRect{X: 5, Y: 7, W: 0, H: 0},
		gtv.CellTag("subtitle"),
	)

	commentLabel := tui.NewLabel(
		mainLayout,
		"Comment:",
		gtv.TRect{X: 5, Y: 9, W: 0, H: 0},
		gtv.CellTag("subtitle"),
	)

	// Create input boxes for form fields (right column, aligned)
	// The default theme tags are applied automatically
	nameInput := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(15, 3, 30, 1),
	)

	emailInput := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(15, 5, 30, 1),
	)

	phoneInput := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(15, 7, 30, 1),
	)

	// Create text area for comments
	commentTextArea := tui.NewTextArea(
		mainLayout,
		tui.WithTextAreaText(""),
		tui.WithRectangle(15, 9, 30, 3),
	)

	// Create result label (below input fields)
	resultLabel := tui.NewLabel(
		mainLayout,
		"",
		gtv.TRect{X: 5, Y: 14, W: 60, H: 1},
		gtv.CellTag("success"),
	)

	// Create Submit button
	// The default theme tags are applied automatically
	submitButton := tui.NewButton(
		mainLayout,
		"Submit",
		gtv.TRect{X: 15, Y: 12, W: 0, H: 0},
		gtv.CellTag("button"),
		gtv.CellTag("button-focused"),
		gtv.CellTag("button-disabled"),
	)

	// Set Submit button action
	submitButton.SetOnPress(func() {
		name := nameInput.GetText()
		email := emailInput.GetText()
		phone := phoneInput.GetText()
		comment := commentTextArea.GetText()

		if name == "" && email == "" && phone == "" && comment == "" {
			resultLabel.SetText("Please enter at least one field!")
			resultLabel.SetAttrs(gtv.CellTag("error"))
		} else {
			result := fmt.Sprintf("Submitted - Name: %s, Email: %s, Phone: %s, Comment: %s", name, email, phone, comment)
			resultLabel.SetText(result)
			resultLabel.SetAttrs(gtv.CellTag("success"))
		}
	})

	// Create Clear button
	// Use custom colors for the Clear button to differentiate it from Submit
	clearButton := tui.NewButton(
		mainLayout,
		"Clear",
		gtv.TRect{X: 28, Y: 12, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x660000),            // Normal: white on dark red
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0xAA0000), // Focused: bold white on red
		gtv.CellTag("button-disabled"),
	)

	// Set Clear button action
	clearButton.SetOnPress(func() {
		nameInput.SetText("")
		emailInput.SetText("")
		phoneInput.SetText("")
		commentTextArea.SetText("")
		resultLabel.SetText("")
	})

	// Create Quit button
	quitButton := tui.NewButton(
		mainLayout,
		"Quit",
		gtv.TRect{X: 38, Y: 12, W: 0, H: 0},
		gtv.CellTag("button"),
		gtv.CellTag("button-focused"),
		gtv.CellTag("button-disabled"),
	)

	// Add a title label at the top
	titleLabel := tui.NewLabel(
		mainLayout,
		"GTV Demo Application - Data Entry Form",
		gtv.TRect{X: 5, Y: 1, W: 0, H: 0},
		gtv.CellTag("title"),
	)

	// Add instructions at the bottom
	instructionsLabel := tui.NewLabel(
		mainLayout,
		"Press Tab to move between fields. Press Enter/Space on buttons. Press Ctrl+C to quit.",
		gtv.TRect{X: 5, Y: 16, W: 0, H: 0},
		gtv.CellTag("hint"),
	)

	// Avoid unused variable errors
	_ = nameLabel
	_ = emailLabel
	_ = phoneLabel
	_ = commentLabel
	_ = titleLabel
	_ = instructionsLabel

	// Create the application
	app := tui.NewApplication(mainLayout, screen)

	// Set Quit button action (after app is created)
	quitButton.SetOnPress(func() {
		app.Quit()
	})

	// Run the application
	if err := app.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}
}
