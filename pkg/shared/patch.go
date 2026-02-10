package shared

import (
	"fmt"
	"regexp"
	"strings"
)

// Patch represents a parsed patch containing multiple file operations.
type Patch struct {
	Hunks []Hunk
}

// Hunk represents a single file operation in a patch.
// It can be one of: AddFile, DeleteFile, or UpdateFile.
type Hunk interface {
	isHunk()
}

// AddFile represents a file creation operation.
type AddFile struct {
	Path     string
	Contents string
}

func (AddFile) isHunk() {}

// DeleteFile represents a file deletion operation.
type DeleteFile struct {
	Path string
}

func (DeleteFile) isHunk() {}

// UpdateFile represents a file modification operation.
type UpdateFile struct {
	Path     string
	MovePath string // Optional: if set, file should be moved to this path
	Chunks   []UpdateFileChunk
}

func (UpdateFile) isHunk() {}

// UpdateFileChunk represents a single change block within an update operation.
type UpdateFileChunk struct {
	OldLines       []string
	NewLines       []string
	ChangeContext  string // Optional context line (content after @@)
	IsEndOfFile    bool   // True if this chunk is anchored at end of file
}

// ParsePatch parses a patch string and returns the structured Patch representation.
// The patch format is a stripped-down, file-oriented diff format.
//
// Format:
//
//	*** Begin Patch
//	*** Add File: <path>
//	+<line content>
//	*** Delete File: <path>
//	*** Update File: <path>
//	*** Move to: <new_path>  (optional)
//	@@ <context>
//	-<old line>
//	+<new line>
//	 <context line (space prefix)>
//	*** End of File  (optional, marks EOF anchor)
//	*** End Patch
func ParsePatch(patchText string) (*Patch, error) {
	cleaned := stripHeredoc(strings.TrimSpace(patchText))
	lines := strings.Split(cleaned, "\n")

	// Find Begin/End patch markers
	const beginMarker = "*** Begin Patch"
	const endMarker = "*** End Patch"

	beginIdx := -1
	endIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == beginMarker {
			beginIdx = i
		} else if trimmed == endMarker {
			endIdx = i
		}
	}

	if beginIdx == -1 || endIdx == -1 || beginIdx >= endIdx {
		return nil, fmt.Errorf("ParsePatch: invalid patch format: missing or invalid Begin/End markers")
	}

	patch := &Patch{
		Hunks: make([]Hunk, 0),
	}

	// Parse content between markers
	i := beginIdx + 1
	for i < endIdx {
		header, nextIdx, err := parsePatchHeader(lines, i)
		if err != nil {
			return nil, err
		}
		if header == nil {
			i++
			continue
		}

		switch header.opType {
		case "add":
			content, newIdx := parseAddFileContent(lines, nextIdx, endIdx)
			patch.Hunks = append(patch.Hunks, AddFile{
				Path:     header.filePath,
				Contents: content,
			})
			i = newIdx

		case "delete":
			patch.Hunks = append(patch.Hunks, DeleteFile{
				Path: header.filePath,
			})
			i = nextIdx

		case "update":
			chunks, newIdx := parseUpdateFileChunks(lines, nextIdx, endIdx)
			patch.Hunks = append(patch.Hunks, UpdateFile{
				Path:     header.filePath,
				MovePath: header.movePath,
				Chunks:   chunks,
			})
			i = newIdx
		}
	}

	return patch, nil
}

// patchHeader represents the parsed header of a file operation.
type patchHeader struct {
	opType   string // "add", "delete", or "update"
	filePath string
	movePath string // Only for update operations with move
}

// parsePatchHeader parses a file operation header and returns the header info and next line index.
func parsePatchHeader(lines []string, startIdx int) (*patchHeader, int, error) {
	if startIdx >= len(lines) {
		return nil, startIdx, nil
	}

	line := lines[startIdx]

	if strings.HasPrefix(line, "*** Add File:") {
		filePath := strings.TrimSpace(strings.TrimPrefix(line, "*** Add File:"))
		if filePath == "" {
			return nil, 0, fmt.Errorf("parsePatchHeader: empty file path in Add File header at line %d", startIdx)
		}
		return &patchHeader{opType: "add", filePath: filePath}, startIdx + 1, nil
	}

	if strings.HasPrefix(line, "*** Delete File:") {
		filePath := strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File:"))
		if filePath == "" {
			return nil, 0, fmt.Errorf("parsePatchHeader: empty file path in Delete File header at line %d", startIdx)
		}
		return &patchHeader{opType: "delete", filePath: filePath}, startIdx + 1, nil
	}

	if strings.HasPrefix(line, "*** Update File:") {
		filePath := strings.TrimSpace(strings.TrimPrefix(line, "*** Update File:"))
		if filePath == "" {
			return nil, 0, fmt.Errorf("parsePatchHeader: empty file path in Update File header at line %d", startIdx)
		}

		nextIdx := startIdx + 1
		movePath := ""

		// Check for optional Move to directive
		if nextIdx < len(lines) && strings.HasPrefix(lines[nextIdx], "*** Move to:") {
			movePath = strings.TrimSpace(strings.TrimPrefix(lines[nextIdx], "*** Move to:"))
			nextIdx++
		}

		return &patchHeader{
			opType:   "update",
			filePath: filePath,
			movePath: movePath,
		}, nextIdx, nil
	}

	return nil, startIdx, nil
}

// parseAddFileContent parses the content lines for an Add File operation.
// Lines starting with '+' are included in the content.
func parseAddFileContent(lines []string, startIdx, endIdx int) (string, int) {
	contentLines := []string{}
	i := startIdx

	for i < len(lines) && i < endIdx && !isFileOperationHeader(lines[i]) {
		if strings.HasPrefix(lines[i], "+") {
			contentLines = append(contentLines, lines[i][1:])
		}
		i++
	}

	return strings.Join(contentLines, "\n"), i
}

// parseUpdateFileChunks parses the chunks for an Update File operation.
func parseUpdateFileChunks(lines []string, startIdx, endIdx int) ([]UpdateFileChunk, int) {
	var chunks []UpdateFileChunk
	i := startIdx

	for i < len(lines) && i < endIdx && !isFileOperationHeader(lines[i]) {
		if strings.HasPrefix(lines[i], "@@") {
			// Parse context line (content after @@)
			contextLine := strings.TrimSpace(lines[i][2:])
			i++

			oldLines := []string{}
			newLines := []string{}
			isEndOfFile := false

			// Parse change lines until next @@, file header, or end
			for i < len(lines) && i < endIdx && !strings.HasPrefix(lines[i], "@@") && !isFileOperationHeader(lines[i]) {
				changeLine := lines[i]

				if changeLine == "*** End of File" {
					isEndOfFile = true
					i++
					break
				}

				if len(changeLine) == 0 {
					// Empty line - skip
					i++
					continue
				}

				prefix := changeLine[0]
				content := changeLine[1:]

				switch prefix {
				case ' ':
					// Context line - appears in both old and new
					oldLines = append(oldLines, content)
					newLines = append(newLines, content)
				case '-':
					// Remove line - only in old
					oldLines = append(oldLines, content)
				case '+':
					// Add line - only in new
					newLines = append(newLines, content)
				}

				i++
			}

			chunk := UpdateFileChunk{
				OldLines:      oldLines,
				NewLines:      newLines,
				IsEndOfFile:   isEndOfFile,
			}
			if contextLine != "" {
				chunk.ChangeContext = contextLine
			}

			chunks = append(chunks, chunk)
		} else {
			i++
		}
	}

	return chunks, i
}

// isFileOperationHeader checks if a line is a file operation header.
func isFileOperationHeader(line string) bool {
	return strings.HasPrefix(line, "*** Add File:") ||
		strings.HasPrefix(line, "*** Delete File:") ||
		strings.HasPrefix(line, "*** Update File:") ||
		strings.HasPrefix(line, "*** End Patch")
}

// stripHeredoc removes heredoc wrapper from patch text if present.
// Matches patterns like: cat <<'EOF'\n...\nEOF or <<EOF\n...\nEOF
func stripHeredoc(input string) string {
	// Simple heredoc detection without backreferences
	// Look for pattern: [cat ]<<['"]DELIMITER['"]\n...\nDELIMITER
	lines := strings.Split(input, "\n")
	if len(lines) < 2 {
		return input
	}

	firstLine := lines[0]

	// Check if first line looks like a heredoc start
	// Pattern: optional "cat " followed by <<, optional quote, delimiter, optional quote
	heredocStartPattern := regexp.MustCompile(`^(?:cat\s+)?<<['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?\s*$`)
	matches := heredocStartPattern.FindStringSubmatch(firstLine)
	if matches == nil {
		return input
	}

	delimiter := matches[1]

	// Find the closing delimiter (must be on its own line)
	for i := len(lines) - 1; i >= 1; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == delimiter {
			// Return content between first and last delimiter
			if i > 1 {
				return strings.Join(lines[1:i], "\n")
			}
			return ""
		}
	}

	return input
}
