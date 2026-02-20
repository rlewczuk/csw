package tool

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/lsp"
)

// DiagnosticWithURI wraps a diagnostic with its file URI for proper grouping.
type DiagnosticWithURI struct {
	URI        string
	Diagnostic lsp.Diagnostic
}

// formatDiagnostics formats diagnostics from LSP validation into a human-readable error message.
// The format matches the example: "Error [line:col] message"
func formatDiagnostics(diags []DiagnosticWithURI, editedPath string) string {
	if len(diags) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\nLSP validation found issues:\n")

	// Group diagnostics by file
	diagsByFile := make(map[string][]lsp.Diagnostic)
	for _, d := range diags {
		diagsByFile[d.URI] = append(diagsByFile[d.URI], d.Diagnostic)
	}

	// Convert edited path to URI for comparison
	editedURI := pathToURI(editedPath)

	for uri, fileDiags := range diagsByFile {
		for _, diag := range fileDiags {
			// Only report errors (severity 1)
			if diag.Severity != lsp.SeverityError {
				continue
			}

			// Format: Error [line:col] message
			// LSP uses 0-based line/column numbers, so we add 1 for human-readable output
			line := diag.Range.Start.Line + 1
			col := diag.Range.Start.Character + 1

			// Add file path if it's different from the edited file
			if uri != editedURI {
				// Extract path from URI for display
				displayPath := uriToPath(uri)
				sb.WriteString(fmt.Sprintf("Error in %s [%d:%d] %s\n", displayPath, line, col, diag.Message))
			} else {
				sb.WriteString(fmt.Sprintf("Error [%d:%d] %s\n", line, col, diag.Message))
			}
		}
	}

	return sb.String()
}

// pathToURI converts a file path to a URI.
func pathToURI(path string) string {
	// Ensure path is absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Convert to URI format
	absPath = filepath.ToSlash(absPath)

	// If path doesn't start with /, add it (Windows case)
	if !strings.HasPrefix(absPath, "/") {
		absPath = "/" + absPath
	}

	return "file://" + absPath
}

// uriToPath converts a URI to a file path.
func uriToPath(uri string) string {
	// Remove file:// prefix
	path := strings.TrimPrefix(uri, "file://")
	// Convert to platform-specific path
	return filepath.FromSlash(path)
}

// applyOffsetAndLimit applies offset and limit to content by lines.
// offset is the number of lines to skip, limit is the maximum number of lines to return.
func applyOffsetAndLimit(content string, offset, limit int64) string {
	if content == "" {
		return ""
	}

	// Split content into lines
	lines := splitLines(content)

	// Apply offset
	if offset >= int64(len(lines)) {
		return ""
	}
	if offset > 0 {
		lines = lines[offset:]
	}

	// Apply limit
	if limit > 0 && int64(len(lines)) > limit {
		lines = lines[:limit]
	}

	// Join lines back together
	return joinLines(lines)
}

// splitLines splits content into lines, preserving line endings.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}

	var lines []string
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i+1])
			start = i + 1
		}
	}

	// Add remaining content if any (file doesn't end with newline)
	if start < len(content) {
		lines = append(lines, content[start:])
	}

	return lines
}

// joinLines joins lines back together.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	result := ""
	for _, line := range lines {
		result += line
	}
	return result
}

// formatWithLineNumbers formats content with line numbers in cat -n style.
// Format: 5 columns for line number (right-aligned), two spaces, then content.
// Example: "    1  first line\n    2  second line\n"
func formatWithLineNumbers(content string, startLine int64) string {
	if content == "" {
		return ""
	}

	lines := splitLines(content)
	result := ""

	for i, line := range lines {
		lineNum := startLine + int64(i) + 1
		// Format: %5d (5 columns, right-aligned) + "  " (two spaces) + line content
		result += formatLineNumber(lineNum) + "  " + line
	}

	return result
}

// formatLineNumber formats a line number with 5 columns, right-aligned.
func formatLineNumber(num int64) string {
	str := ""
	// Convert number to string manually
	if num == 0 {
		str = "0"
	} else {
		digits := []byte{}
		n := num
		for n > 0 {
			digits = append([]byte{byte('0' + n%10)}, digits...)
			n /= 10
		}
		str = string(digits)
	}

	// Pad with spaces to 5 columns (right-aligned)
	for len(str) < 5 {
		str = " " + str
	}

	return str
}
