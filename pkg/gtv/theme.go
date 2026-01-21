package gtv

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

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

// ThemeDescription represents a theme descriptor with name and description.
type ThemeDescription struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ThemeFile represents the JSON structure of a theme file.
type ThemeFile struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Theme       map[string]CellAttributes `json:"theme"`
}

// ThemeManager manages themes loaded from JSON files.
type ThemeManager struct {
	themes map[string]ThemeFile
	fsList []fs.FS
}

// NewThemeManager creates a new ThemeManager.
// It accepts a list of fs.FS filesystems to search for themes.
// Non-existent paths are ignored.
// Empty list is allowed and treated as no themes available.
func NewThemeManager(fsList ...fs.FS) (*ThemeManager, error) {
	tm := &ThemeManager{
		themes: make(map[string]ThemeFile),
		fsList: fsList,
	}

	if err := tm.Reload(); err != nil {
		return nil, fmt.Errorf("NewThemeManager(): failed to load themes: %w", err)
	}

	return tm, nil
}

// GetTheme returns a theme by name.
func (tm *ThemeManager) GetTheme(name string) (map[string]CellAttributes, error) {
	theme, ok := tm.themes[name]
	if !ok {
		return nil, fmt.Errorf("ThemeManager.GetTheme(): theme not found: %s", name)
	}
	return theme.Theme, nil
}

// ListThemes returns a list of all available theme descriptors.
func (tm *ThemeManager) ListThemes() []ThemeDescription {
	result := make([]ThemeDescription, 0, len(tm.themes))
	for _, theme := range tm.themes {
		result = append(result, ThemeDescription{
			Name:        theme.Name,
			Description: theme.Description,
		})
	}
	return result
}

// Reload reloads all themes from the configured filesystems.
func (tm *ThemeManager) Reload() error {
	tm.themes = make(map[string]ThemeFile)

	for _, filesystem := range tm.fsList {
		if err := tm.loadThemesFromFS(filesystem); err != nil {
			return fmt.Errorf("ThemeManager.Reload(): %w", err)
		}
	}

	return nil
}

// loadThemesFromFS loads all themes from a given filesystem.
func (tm *ThemeManager) loadThemesFromFS(filesystem fs.FS) error {
	err := fs.WalkDir(filesystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Ignore errors for individual files/directories
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Check if the file matches the pattern <name>.theme.json
		if !strings.HasSuffix(path, ".theme.json") {
			return nil
		}

		// Load the theme file
		if err := tm.loadThemeFile(filesystem, path); err != nil {
			// Log error but continue processing other files
			return nil
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("ThemeManager.loadThemesFromFS(): failed to walk directory: %w", err)
	}

	return nil
}

// loadThemeFile loads a single theme file.
func (tm *ThemeManager) loadThemeFile(filesystem fs.FS, path string) error {
	file, err := filesystem.Open(path)
	if err != nil {
		return fmt.Errorf("ThemeManager.loadThemeFile(): failed to open %s: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("ThemeManager.loadThemeFile(): failed to read %s: %w", path, err)
	}

	var themeFile ThemeFile
	if err := json.Unmarshal(data, &themeFile); err != nil {
		return fmt.Errorf("ThemeManager.loadThemeFile(): failed to parse %s: %w", path, err)
	}

	// Validate that the file name matches the theme name
	expectedFileName := themeFile.Name + ".theme.json"
	actualFileName := filepath.Base(path)
	if expectedFileName != actualFileName {
		return fmt.Errorf("ThemeManager.loadThemeFile(): file name %s does not match theme name %s", actualFileName, themeFile.Name)
	}

	// Store the theme
	tm.themes[themeFile.Name] = themeFile

	return nil
}
