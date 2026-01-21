package gtv

import "strconv"

// ThemeInterceptor is an IScreenOutput wrapper that applies theme to the output.
// It intercepts all calls modifying screen content and applies theme based on ThemeTag field.
type ThemeInterceptor struct {
	output IScreenOutput
	theme  map[string]CellAttributes
}

// NewThemeInterceptor creates a new ThemeInterceptor.
// The theme parameter is a map of theme tag strings to CellAttributes.
// Theme tags are matched by converting the ThemeTag field to string.
func NewThemeInterceptor(output IScreenOutput, theme map[string]CellAttributes) *ThemeInterceptor {
	return &ThemeInterceptor{
		output: output,
		theme:  theme,
	}
}

// GetSize returns the size of the screen in characters.
func (t *ThemeInterceptor) GetSize() (width int, height int) {
	return t.output.GetSize()
}

// SetSize changes the size of the screen in characters.
func (t *ThemeInterceptor) SetSize(width int, height int) {
	t.output.SetSize(width, height)
}

// GetContent returns the whole content of the screen.
func (t *ThemeInterceptor) GetContent() (width int, height int, content []Cell) {
	return t.output.GetContent()
}

// PutText puts text at the specified position with the specified attributes,
// applying theme based on ThemeTag field.
func (t *ThemeInterceptor) PutText(rect TRect, text string, attrs CellAttributes) {
	// Apply theme to attributes
	themedAttrs := t.applyTheme(attrs)
	t.output.PutText(rect, text, themedAttrs)
}

// PutContent puts raw cell content at the specified position,
// applying theme to each cell based on ThemeTag field.
func (t *ThemeInterceptor) PutContent(rect TRect, content []Cell) {
	// Apply theme to each cell
	themedContent := make([]Cell, len(content))
	for i, cell := range content {
		themedContent[i] = Cell{
			Rune:  cell.Rune,
			Attrs: t.applyTheme(cell.Attrs),
		}
	}
	t.output.PutContent(rect, themedContent)
}

// MoveCursor moves the cursor to the specified position.
func (t *ThemeInterceptor) MoveCursor(x int, y int) {
	t.output.MoveCursor(x, y)
}

// SetCursorStyle sets the cursor style.
func (t *ThemeInterceptor) SetCursorStyle(style CursorStyle) {
	t.output.SetCursorStyle(style)
}

// applyTheme applies theme to the given cell attributes.
// If ThemeTag is non-zero, it looks up the theme and applies colors
// that are not explicitly set (i.e. zero).
func (t *ThemeInterceptor) applyTheme(attrs CellAttributes) CellAttributes {
	// If ThemeTag is zero, return original attributes
	if attrs.ThemeTag == 0 {
		return attrs
	}

	// Convert ThemeTag to string
	tag := strconv.FormatUint(uint64(attrs.ThemeTag), 10)

	// Look up theme
	themeAttrs, ok := t.theme[tag]
	if !ok {
		// Theme not found, return original attributes
		return attrs
	}

	// Apply theme: override only zero colors and attributes
	result := attrs

	// Override attributes only if original is zero
	if attrs.Attributes == 0 {
		result.Attributes = themeAttrs.Attributes
	}

	// Override TextColor only if original is zero
	if attrs.TextColor == 0 {
		result.TextColor = themeAttrs.TextColor
	}

	// Override BackColor only if original is zero
	if attrs.BackColor == 0 {
		result.BackColor = themeAttrs.BackColor
	}

	// Override StrikeColor only if original is zero
	if attrs.StrikeColor == 0 {
		result.StrikeColor = themeAttrs.StrikeColor
	}

	return result
}
