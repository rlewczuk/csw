package mdv

import (
	"regexp"
	"strings"
)

// BlockType represents the type of markdown block element
type BlockType int

const (
	BlockTypeParagraph BlockType = iota
	BlockTypeHeader
	BlockTypeCodeBlock
	BlockTypeList
	BlockTypeBlockquote
	BlockTypeHorizontalRule
)

// Block represents a parsed markdown block element
type Block struct {
	Type     BlockType
	Level    int     // For headers (1-6) and list nesting
	Language string  // For code blocks
	Content  string  // Raw content of the block
	Children []Block // For nested structures like lists
	Ordered  bool    // For lists: true if ordered, false if unordered
}

// ParseMarkdown parses markdown content into a list of blocks
func ParseMarkdown(content string) []Block {
	lines := strings.Split(content, "\n")
	blocks := []Block{}
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Skip empty lines between blocks
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}

		// Try to parse as fenced code block
		if block, consumed := parseFencedCodeBlock(lines, i); consumed > 0 {
			blocks = append(blocks, block)
			i += consumed
			continue
		}

		// Try to parse as ATX header
		if block, ok := parseATXHeader(line); ok {
			blocks = append(blocks, block)
			i++
			continue
		}

		// Try to parse as horizontal rule
		if isHorizontalRule(line) {
			blocks = append(blocks, Block{Type: BlockTypeHorizontalRule})
			i++
			continue
		}

		// Try to parse as blockquote
		if block, consumed := parseBlockquote(lines, i); consumed > 0 {
			blocks = append(blocks, block)
			i += consumed
			continue
		}

		// Try to parse as list
		if block, consumed := parseList(lines, i); consumed > 0 {
			blocks = append(blocks, block)
			i += consumed
			continue
		}

		// Try to parse as indented code block
		if block, consumed := parseIndentedCodeBlock(lines, i); consumed > 0 {
			blocks = append(blocks, block)
			i += consumed
			continue
		}

		// Try to parse as setext header (must be checked after other blocks)
		if i+1 < len(lines) {
			if block, ok := parseSetextHeader(lines[i], lines[i+1]); ok {
				blocks = append(blocks, block)
				i += 2
				continue
			}
		}

		// Default: parse as paragraph
		block, consumed := parseParagraph(lines, i)
		blocks = append(blocks, block)
		i += consumed
	}

	return blocks
}

// parseATXHeader parses ATX-style headers (# Header)
func parseATXHeader(line string) (Block, bool) {
	// ATX headers: 1-6 # characters, followed by space, then content
	re := regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(line))

	if matches == nil {
		return Block{}, false
	}

	level := len(matches[1])
	content := matches[2]

	// Remove trailing # characters if present
	content = strings.TrimRight(content, "# ")
	content = strings.TrimSpace(content)

	return Block{
		Type:    BlockTypeHeader,
		Level:   level,
		Content: content,
	}, true
}

// parseSetextHeader parses setext-style headers (underlined with = or -)
func parseSetextHeader(line1, line2 string) (Block, bool) {
	line1 = strings.TrimSpace(line1)
	line2 = strings.TrimSpace(line2)

	if line1 == "" {
		return Block{}, false
	}

	// Check for = underline (level 1)
	if matched, _ := regexp.MatchString(`^=+$`, line2); matched {
		return Block{
			Type:    BlockTypeHeader,
			Level:   1,
			Content: line1,
		}, true
	}

	// Check for - underline (level 2)
	if matched, _ := regexp.MatchString(`^-+$`, line2); matched {
		return Block{
			Type:    BlockTypeHeader,
			Level:   2,
			Content: line1,
		}, true
	}

	return Block{}, false
}

// isHorizontalRule checks if a line is a horizontal rule
func isHorizontalRule(line string) bool {
	line = strings.TrimSpace(line)

	// Must be at least 3 of the same character (*, -, or _)
	// with optional spaces between them
	patterns := []string{
		`^\*\s*\*\s*\*[\s*]*$`,
		`^-\s*-\s*-[\s-]*$`,
		`^_\s*_\s*_[\s_]*$`,
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			return true
		}
	}

	return false
}

// parseFencedCodeBlock parses fenced code blocks (``` or ~~~)
func parseFencedCodeBlock(lines []string, start int) (Block, int) {
	line := strings.TrimSpace(lines[start])

	// Check for opening fence
	var fence string
	var language string

	if strings.HasPrefix(line, "```") {
		fence = "```"
		language = strings.TrimSpace(strings.TrimPrefix(line, "```"))
	} else if strings.HasPrefix(line, "~~~") {
		fence = "~~~"
		language = strings.TrimSpace(strings.TrimPrefix(line, "~~~"))
	} else {
		return Block{}, 0
	}

	// Find closing fence
	content := []string{}
	i := start + 1
	for i < len(lines) {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), fence) {
			// Found closing fence
			return Block{
				Type:     BlockTypeCodeBlock,
				Language: language,
				Content:  strings.Join(content, "\n"),
			}, i - start + 1
		}
		content = append(content, lines[i])
		i++
	}

	// No closing fence found, treat as code block until end
	return Block{
		Type:     BlockTypeCodeBlock,
		Language: language,
		Content:  strings.Join(content, "\n"),
	}, i - start
}

// parseIndentedCodeBlock parses indented code blocks (4+ spaces)
func parseIndentedCodeBlock(lines []string, start int) (Block, int) {
	// Must start with 4 spaces or a tab
	if !strings.HasPrefix(lines[start], "    ") && !strings.HasPrefix(lines[start], "\t") {
		return Block{}, 0
	}

	content := []string{}
	i := start

	for i < len(lines) {
		line := lines[i]

		// Empty lines are included in code block
		if strings.TrimSpace(line) == "" {
			content = append(content, "")
			i++
			continue
		}

		// Line must be indented to be part of code block
		if strings.HasPrefix(line, "    ") {
			content = append(content, line[4:])
			i++
		} else if strings.HasPrefix(line, "\t") {
			content = append(content, line[1:])
			i++
		} else {
			break
		}
	}

	if len(content) == 0 {
		return Block{}, 0
	}

	return Block{
		Type:    BlockTypeCodeBlock,
		Content: strings.Join(content, "\n"),
	}, i - start
}

// parseBlockquote parses blockquote blocks (> prefix)
func parseBlockquote(lines []string, start int) (Block, int) {
	line := strings.TrimSpace(lines[start])

	if !strings.HasPrefix(line, ">") {
		return Block{}, 0
	}

	content := []string{}
	i := start

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if line == "" {
			// Empty line ends blockquote
			break
		}

		if strings.HasPrefix(line, ">") {
			// Remove > prefix and optional space after it
			line = strings.TrimPrefix(line, ">")
			if strings.HasPrefix(line, " ") {
				line = line[1:]
			}
			content = append(content, line)
			i++
		} else {
			break
		}
	}

	return Block{
		Type:    BlockTypeBlockquote,
		Content: strings.Join(content, "\n"),
	}, i - start
}

// parseList parses ordered or unordered lists
func parseList(lines []string, start int) (Block, int) {
	line := strings.TrimSpace(lines[start])

	// Check for unordered list markers (-, *, +)
	unorderedRe := regexp.MustCompile(`^[-*+]\s+(.*)$`)
	// Check for ordered list markers (1., 2., etc.)
	orderedRe := regexp.MustCompile(`^\d+\.\s+(.*)$`)

	var ordered bool
	var matches []string

	if matches = unorderedRe.FindStringSubmatch(line); matches != nil {
		ordered = false
	} else if matches = orderedRe.FindStringSubmatch(line); matches != nil {
		ordered = true
	} else {
		return Block{}, 0
	}

	// Parse all consecutive list items
	items := []Block{}
	i := start

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if line == "" {
			// Empty line might end list or might be between items
			// For simplicity, we'll end the list here
			break
		}

		var itemMatches []string
		if ordered {
			itemMatches = orderedRe.FindStringSubmatch(line)
		} else {
			itemMatches = unorderedRe.FindStringSubmatch(line)
		}

		if itemMatches == nil {
			// Not a list item anymore
			break
		}

		// Create a paragraph block for the list item content
		items = append(items, Block{
			Type:    BlockTypeParagraph,
			Content: itemMatches[1],
		})
		i++
	}

	return Block{
		Type:     BlockTypeList,
		Ordered:  ordered,
		Children: items,
	}, i - start
}

// parseParagraph parses a paragraph (default case)
func parseParagraph(lines []string, start int) (Block, int) {
	content := []string{}
	i := start

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Empty line ends paragraph
		if trimmed == "" {
			break
		}

		// Check if next line starts a new block structure
		if strings.HasPrefix(trimmed, "#") {
			break
		}
		if strings.HasPrefix(trimmed, ">") {
			break
		}
		if matched, _ := regexp.MatchString(`^[-*+]\s`, trimmed); matched {
			break
		}
		if matched, _ := regexp.MatchString(`^\d+\.\s`, trimmed); matched {
			break
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			break
		}
		if isHorizontalRule(trimmed) {
			break
		}

		content = append(content, line)
		i++
	}

	return Block{
		Type:    BlockTypeParagraph,
		Content: strings.Join(content, "\n"),
	}, i - start
}
