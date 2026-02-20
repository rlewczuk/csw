package util

import "github.com/rlewczuk/csw/pkg/gtv"

// marker represents an active formatting marker on the stack
type marker struct {
	text     string
	attr     gtv.TextAttributes
	startPos int
}

// TextToCells converts markdown-like formatted text to an array of Cell structs.
// It supports various text formatting attributes using markdown-like syntax with some extensions.
//
// Supported formatting syntax:
//
//	**text**     - Bold text
//	*text*       - Italic text
//	~~text~~     - Strikethrough text
//	__text__     - Underline text (two underscores)
//	___text___   - Double underline text (three underscores)
//	%%text%%     - Dim text (reduced intensity)
//	!!text!!     - Blinking text
//	<<text>>     - Reverse colors (swap foreground and background)
//
// Formatting can be nested and combined:
//
//	**bold *and italic* bold**           - Nested italic inside bold
//	**!!bold and blink!!**               - Bold with blink combined
//	**__!!triple combination!!__**       - Three attributes combined
//
// Escape sequences:
//
// Use backslash (\) to escape special formatting characters and render them literally:
//
//	\*        - Literal asterisk
//	\~        - Literal tilde
//	\_        - Literal underscore
//	\%        - Literal percent
//	\!        - Literal exclamation
//	\<        - Literal less-than
//	\>        - Literal greater-than
//	\\        - Literal backslash
//
// Examples:
//
//	TextToCells("hello")                    // Plain text: "hello"
//	TextToCells("**bold**")                 // Bold: "bold"
//	TextToCells("*italic*")                 // Italic: "italic"
//	TextToCells("**bold *nested* bold**")   // Bold with nested italic
//	TextToCells(`\*not italic\*`)           // Escaped: "*not italic*"
//	TextToCells("plain **bold** plain")     // Mixed: "plain bold plain"
//	TextToCells("!!<<blink reverse>>!!")    // Blink + Reverse: "blink reverse"
//
// Unclosed markers:
//
// If a formatting marker is not properly closed (i.e., no matching closing marker is found),
// the opening marker is treated as literal text:
//
//	TextToCells("**unclosed")               // Literal: "**unclosed"
//
// Empty styled text:
//
// If markers contain no text or only whitespace, they are processed normally:
//
//	TextToCells("****")                     // Empty result
//	TextToCells("**   **")                  // Bold spaces: "   "
func TextToCells(s string) []gtv.Cell {
	if len(s) == 0 {
		return []gtv.Cell{}
	}

	runes := []rune(s)
	cells := []gtv.Cell{}
	// Initialize attrs with NoColor for all color fields
	// This ensures that theme colors can be applied later
	attrs := gtv.CellAttributes{
		TextColor:   gtv.NoColor,
		BackColor:   gtv.NoColor,
		StrikeColor: gtv.NoColor,
	}

	// Stack to track active formatting markers and their attributes
	stack := []marker{}

	i := 0
	for i < len(runes) {
		// Handle escape sequences
		if runes[i] == '\\' {
			if i+1 < len(runes) {
				// Escape next character
				cells = append(cells, gtv.Cell{Rune: runes[i+1], Attrs: attrs})
				i += 2
				continue
			} else {
				// Backslash at end of string
				cells = append(cells, gtv.Cell{Rune: '\\', Attrs: attrs})
				i++
				continue
			}
		}

		// Try to match formatting markers
		matched := false

		// Check for triple underscore (must check before double and single)
		if i+2 < len(runes) && runes[i] == '_' && runes[i+1] == '_' && runes[i+2] == '_' {
			if inStack, idx := findInStack(stack, "___"); inStack {
				// Closing marker - pop from stack and update attrs
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 3
				matched = true
			} else {
				// Opening marker - push to stack
				stack = append(stack, marker{text: "___", attr: gtv.AttrDoubleUnderline, startPos: i})
				attrs = calculateAttrs(stack)
				i += 3
				matched = true
			}
		}

		// Check for double underscore (if not already matched)
		if !matched && i+1 < len(runes) && runes[i] == '_' && runes[i+1] == '_' {
			if inStack, idx := findInStack(stack, "__"); inStack {
				// Closing marker
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				// Opening marker
				stack = append(stack, marker{text: "__", attr: gtv.AttrUnderline, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		// Check for double tilde
		if !matched && i+1 < len(runes) && runes[i] == '~' && runes[i+1] == '~' {
			if inStack, idx := findInStack(stack, "~~"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "~~", attr: gtv.AttrStrikethrough, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		// Check for double percent
		if !matched && i+1 < len(runes) && runes[i] == '%' && runes[i+1] == '%' {
			if inStack, idx := findInStack(stack, "%%"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "%%", attr: gtv.AttrDim, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		// Check for double exclamation
		if !matched && i+1 < len(runes) && runes[i] == '!' && runes[i+1] == '!' {
			if inStack, idx := findInStack(stack, "!!"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "!!", attr: gtv.AttrBlink, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		// Check for double angle brackets
		if !matched && i+1 < len(runes) && runes[i] == '<' && runes[i+1] == '<' {
			if inStack, idx := findInStack(stack, "<<"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "<<", attr: gtv.AttrReverse, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		if !matched && i+1 < len(runes) && runes[i] == '>' && runes[i+1] == '>' {
			if inStack, idx := findInStack(stack, "<<"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				// Opening >> without << - treat as literal
				cells = append(cells, gtv.Cell{Rune: runes[i], Attrs: attrs})
				i++
				matched = true
			}
		}

		// Handle asterisks (*, **, ***) with smart matching
		// We need to be careful with greedy matching to handle cases like "****" correctly
		if !matched && runes[i] == '*' {
			// Count consecutive asterisks
			asteriskCount := 0
			for i+asteriskCount < len(runes) && runes[i+asteriskCount] == '*' {
				asteriskCount++
			}

			// Try to match the longest sequence first, but only if it exists in stack
			// This prevents "****" from being parsed as "***" + "*"
			if asteriskCount >= 3 {
				if inStack, idx := findInStack(stack, "***"); inStack {
					// Close ***
					stack = append(stack[:idx], stack[idx+1:]...)
					attrs = calculateAttrs(stack)
					i += 3
					matched = true
				}
			}

			if !matched && asteriskCount >= 2 {
				if inStack, idx := findInStack(stack, "**"); inStack {
					// Close **
					stack = append(stack[:idx], stack[idx+1:]...)
					attrs = calculateAttrs(stack)
					i += 2
					matched = true
				}
			}

			if !matched && asteriskCount >= 1 {
				if inStack, idx := findInStack(stack, "*"); inStack {
					// Close *
					stack = append(stack[:idx], stack[idx+1:]...)
					attrs = calculateAttrs(stack)
					i++
					matched = true
				}
			}

			// If no closing match found, open a new marker
			// Prefer ** over *** to allow "****" to be parsed as "**" + "**"
			if !matched {
				if asteriskCount >= 2 {
					stack = append(stack, marker{text: "**", attr: gtv.AttrBold, startPos: i})
					attrs = calculateAttrs(stack)
					i += 2
					matched = true
				} else if asteriskCount >= 1 {
					stack = append(stack, marker{text: "*", attr: gtv.AttrItalic, startPos: i})
					attrs = calculateAttrs(stack)
					i++
					matched = true
				}
			}
		}

		// If no marker matched, add character to output
		if !matched {
			cells = append(cells, gtv.Cell{Rune: runes[i], Attrs: attrs})
			i++
		}
	}

	// If there are unclosed markers, we need to backtrack and treat them as literals
	if len(stack) > 0 {
		// Reparse the entire string treating unclosed markers as literals
		return reparseWithLiterals(s, stack)
	}

	return cells
}

// findInStack searches for a marker in the stack from top to bottom
// Returns true and index if found, false and -1 if not found
func findInStack(stack []marker, text string) (bool, int) {
	// Search from top of stack (end of slice) backwards
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].text == text {
			return true, i
		}
	}
	return false, -1
}

// calculateAttrs combines all attributes from the current stack
func calculateAttrs(stack []marker) gtv.CellAttributes {
	var combined gtv.TextAttributes
	for _, m := range stack {
		combined |= m.attr
	}
	// Create CellAttributes with NoColor to allow theme colors to be applied
	return gtv.CellAttributes{
		Attributes:  combined,
		TextColor:   gtv.NoColor,
		BackColor:   gtv.NoColor,
		StrikeColor: gtv.NoColor,
	}
}

// reparseWithLiterals reparses the string treating unclosed markers as literal text
func reparseWithLiterals(s string, unclosed []marker) []gtv.Cell {
	runes := []rune(s)
	cells := []gtv.Cell{}
	// Initialize attrs with NoColor for all color fields
	// This ensures that theme colors can be applied later
	attrs := gtv.CellAttributes{
		TextColor:   gtv.NoColor,
		BackColor:   gtv.NoColor,
		StrikeColor: gtv.NoColor,
	}

	stack := []marker{}

	// Build set of positions that should be treated as literals
	literalPositions := make(map[int]bool)
	for _, m := range unclosed {
		for j := 0; j < len(m.text); j++ {
			literalPositions[m.startPos+j] = true
		}
	}

	i := 0
	for i < len(runes) {
		// Handle escape sequences
		if runes[i] == '\\' {
			if i+1 < len(runes) {
				cells = append(cells, gtv.Cell{Rune: runes[i+1], Attrs: attrs})
				i += 2
				continue
			} else {
				cells = append(cells, gtv.Cell{Rune: '\\', Attrs: attrs})
				i++
				continue
			}
		}

		// If this position is marked as literal, add as-is
		if literalPositions[i] {
			cells = append(cells, gtv.Cell{Rune: runes[i], Attrs: attrs})
			i++
			continue
		}

		matched := false

		// Try all marker patterns (same as before, but skip if position is literal)
		// Triple underscore
		if i+2 < len(runes) && runes[i] == '_' && runes[i+1] == '_' && runes[i+2] == '_' &&
			!literalPositions[i] && !literalPositions[i+1] && !literalPositions[i+2] {
			if inStack, idx := findInStack(stack, "___"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 3
				matched = true
			} else {
				stack = append(stack, marker{text: "___", attr: gtv.AttrDoubleUnderline, startPos: i})
				attrs = calculateAttrs(stack)
				i += 3
				matched = true
			}
		}

		// Double underscore
		if !matched && i+1 < len(runes) && runes[i] == '_' && runes[i+1] == '_' &&
			!literalPositions[i] && !literalPositions[i+1] {
			if inStack, idx := findInStack(stack, "__"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "__", attr: gtv.AttrUnderline, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		// Double tilde
		if !matched && i+1 < len(runes) && runes[i] == '~' && runes[i+1] == '~' &&
			!literalPositions[i] && !literalPositions[i+1] {
			if inStack, idx := findInStack(stack, "~~"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "~~", attr: gtv.AttrStrikethrough, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		// Double percent
		if !matched && i+1 < len(runes) && runes[i] == '%' && runes[i+1] == '%' &&
			!literalPositions[i] && !literalPositions[i+1] {
			if inStack, idx := findInStack(stack, "%%"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "%%", attr: gtv.AttrDim, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		// Double exclamation
		if !matched && i+1 < len(runes) && runes[i] == '!' && runes[i+1] == '!' &&
			!literalPositions[i] && !literalPositions[i+1] {
			if inStack, idx := findInStack(stack, "!!"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "!!", attr: gtv.AttrBlink, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		// Angle brackets
		if !matched && i+1 < len(runes) && runes[i] == '<' && runes[i+1] == '<' &&
			!literalPositions[i] && !literalPositions[i+1] {
			if inStack, idx := findInStack(stack, "<<"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				stack = append(stack, marker{text: "<<", attr: gtv.AttrReverse, startPos: i})
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			}
		}

		if !matched && i+1 < len(runes) && runes[i] == '>' && runes[i+1] == '>' &&
			!literalPositions[i] && !literalPositions[i+1] {
			if inStack, idx := findInStack(stack, "<<"); inStack {
				stack = append(stack[:idx], stack[idx+1:]...)
				attrs = calculateAttrs(stack)
				i += 2
				matched = true
			} else {
				cells = append(cells, gtv.Cell{Rune: runes[i], Attrs: attrs})
				i++
				matched = true
			}
		}

		// Handle asterisks with smart matching (same as main parsing)
		if !matched && runes[i] == '*' && !literalPositions[i] {
			// Count consecutive asterisks
			asteriskCount := 0
			for i+asteriskCount < len(runes) && runes[i+asteriskCount] == '*' && !literalPositions[i+asteriskCount] {
				asteriskCount++
			}

			// Try closing markers first
			if asteriskCount >= 2 {
				if inStack, idx := findInStack(stack, "**"); inStack {
					stack = append(stack[:idx], stack[idx+1:]...)
					attrs = calculateAttrs(stack)
					i += 2
					matched = true
				}
			}

			if !matched && asteriskCount >= 1 {
				if inStack, idx := findInStack(stack, "*"); inStack {
					stack = append(stack[:idx], stack[idx+1:]...)
					attrs = calculateAttrs(stack)
					i++
					matched = true
				}
			}

			// If no closing match, open new marker
			if !matched {
				if asteriskCount >= 2 {
					stack = append(stack, marker{text: "**", attr: gtv.AttrBold, startPos: i})
					attrs = calculateAttrs(stack)
					i += 2
					matched = true
				} else if asteriskCount >= 1 {
					stack = append(stack, marker{text: "*", attr: gtv.AttrItalic, startPos: i})
					attrs = calculateAttrs(stack)
					i++
					matched = true
				}
			}
		}

		if !matched {
			cells = append(cells, gtv.Cell{Rune: runes[i], Attrs: attrs})
			i++
		}
	}

	return cells
}

// EscapeText escapes special formatting characters in the input string
// so that it will be rendered verbatim by TextToCells function.
//
// The following characters are escaped with backslash (\):
//
//   - - Asterisk (used for bold and italic)
//     ~  - Tilde (used for strikethrough)
//     _  - Underscore (used for underline and double underline)
//     %  - Percent (used for dim)
//     !  - Exclamation (used for blink)
//     <  - Less-than (used for reverse)
//     >  - Greater-than (used for reverse)
//     \  - Backslash (escape character itself)
//
// Example:
//
//	EscapeText("**bold**")  // Returns: "\\*\\*bold\\*\\*"
//	EscapeText("Hello!")    // Returns: "Hello\\!"
func EscapeText(s string) string {
	if len(s) == 0 {
		return s
	}

	runes := []rune(s)
	var result []rune

	for _, r := range runes {
		switch r {
		case '*', '~', '_', '%', '!', '<', '>', '\\':
			// Escape special characters with backslash
			result = append(result, '\\', r)
		default:
			result = append(result, r)
		}
	}

	return string(result)
}
