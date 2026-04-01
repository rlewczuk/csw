package vfs

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
)

var (
	ErrOldStringNotFound      = errors.New("oldString not found in content")
	ErrOldStringMultipleMatch = errors.New("oldString found multiple times and requires more code context to uniquely identify the intended match. Either provide a larger string with more surrounding context to make it unique or use `replaceAll` to change every instance of `oldString`")
)

// FilePatcher provides functionality to apply edits to file contents.
type FilePatcher interface {
	// ApplyEdits applies a string replacement to a file and returns the unified diff.
	ApplyEdits(path string, oldString string, newString string, replaceAll bool) (string, error)
}

// filePatcher implements the FilePatcher interface.
type filePatcher struct {
	vfs apis.VFS
}

// NewFilePatcher creates a new FilePatcher instance.
func NewFilePatcher(vfs apis.VFS) FilePatcher {
	return &filePatcher{
		vfs: vfs,
	}
}

// ApplyEdits applies a string replacement to the file at the given path.
// It returns a unified diff of the changes or an error if the operation fails.
func (p *filePatcher) ApplyEdits(path string, oldString string, newString string, replaceAll bool) (string, error) {
	// Read the file content
	content, err := p.vfs.ReadFile(path)
	if err != nil {
		// Propagate permission errors and file not found errors without wrapping
		if errors.Is(err, apis.ErrFileNotFound) || errors.Is(err, apis.ErrAskPermission) || errors.Is(err, apis.ErrPermissionDenied) {
			return "", err
		}
		// Check for PermissionError type
		var permErr *PermissionError
		if errors.As(err, &permErr) {
			return "", err
		}
		return "", fmt.Errorf("filePatcher.ApplyEdits: failed to read file: %w", err)
	}

	contentStr := string(content)

	// Count occurrences of oldString
	count := strings.Count(contentStr, oldString)

	// Handle error cases
	if count == 0 {
		return "", fmt.Errorf("filePatcher.ApplyEdits: %w", ErrOldStringNotFound)
	}

	if count > 1 && !replaceAll {
		return "", fmt.Errorf("filePatcher.ApplyEdits: %w", ErrOldStringMultipleMatch)
	}

	// Perform the replacement
	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(contentStr, oldString, newString)
	} else {
		// Replace only the first occurrence
		newContent = strings.Replace(contentStr, oldString, newString, 1)
	}

	// Generate unified diff
	diff := generateUnifiedDiff(path, contentStr, newContent)

	// Write the new content back to the file
	err = p.vfs.WriteFile(path, []byte(newContent))
	if err != nil {
		// Propagate permission errors without wrapping
		if errors.Is(err, apis.ErrAskPermission) || errors.Is(err, apis.ErrPermissionDenied) {
			return "", err
		}
		// Check for PermissionError type
		var permErr *PermissionError
		if errors.As(err, &permErr) {
			return "", err
		}
		return "", fmt.Errorf("filePatcher.ApplyEdits: failed to write file: %w", err)
	}

	return diff, nil
}

// generateUnifiedDiff generates a unified diff between old and new content.
func generateUnifiedDiff(filename string, oldContent string, newContent string) string {
	if oldContent == newContent {
		return ""
	}

	var buf bytes.Buffer

	// Write the diff header
	timestamp := time.Now().Format("2006-01-02 15:04:05.000000000 -0700")
	fmt.Fprintf(&buf, "--- %s\t%s\n", filename, timestamp)
	fmt.Fprintf(&buf, "+++ %s\t%s\n", filename, timestamp)

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Find the differences using a simple line-by-line comparison
	diffs := computeDiff(oldLines, newLines)

	// Generate hunks
	for _, hunk := range diffs {
		// Write hunk header
		fmt.Fprintf(&buf, "@@ -%d,%d +%d,%d @@\n",
			hunk.oldStart+1, hunk.oldCount,
			hunk.newStart+1, hunk.newCount)

		// Write hunk content
		for _, line := range hunk.lines {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// diffHunk represents a hunk in a unified diff.
type diffHunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []string
}

// computeDiff computes the differences between old and new lines.
func computeDiff(oldLines []string, newLines []string) []diffHunk {
	var hunks []diffHunk

	// Use a simple algorithm to find continuous blocks of changes
	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		// Find the next difference
		for i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j] {
			i++
			j++
		}

		if i >= len(oldLines) && j >= len(newLines) {
			break
		}

		// Found a difference, create a hunk
		hunkOldStart := i
		hunkNewStart := j

		var hunkLines []string

		// Add context lines (up to 3 lines before the change)
		contextStart := max(0, i-3)
		for k := contextStart; k < i; k++ {
			hunkLines = append(hunkLines, " "+oldLines[k])
		}
		hunkOldStart = contextStart
		hunkNewStart = j - (i - contextStart)

		// Track changes
		changeStartI := i
		changeStartJ := j

		// Find the end of the change block
		for i < len(oldLines) && j < len(newLines) && oldLines[i] != newLines[j] {
			i++
			j++
		}

		// Add removed lines
		for k := changeStartI; k < i && k < len(oldLines); k++ {
			hunkLines = append(hunkLines, "-"+oldLines[k])
		}

		// Add added lines
		for k := changeStartJ; k < j && k < len(newLines); k++ {
			hunkLines = append(hunkLines, "+"+newLines[k])
		}

		// Handle case where only one side has more lines
		for i < len(oldLines) && (j >= len(newLines) || oldLines[i] != newLines[j]) {
			hunkLines = append(hunkLines, "-"+oldLines[i])
			i++
		}

		for j < len(newLines) && (i >= len(oldLines) || oldLines[i] != newLines[j]) {
			hunkLines = append(hunkLines, "+"+newLines[j])
			j++
		}

		// Add context lines (up to 3 lines after the change)
		contextEnd := min(len(oldLines), i+3)
		for k := i; k < contextEnd; k++ {
			hunkLines = append(hunkLines, " "+oldLines[k])
			i++
			j++
		}

		hunkOldCount := i - hunkOldStart
		hunkNewCount := j - hunkNewStart

		hunks = append(hunks, diffHunk{
			oldStart: hunkOldStart,
			oldCount: hunkOldCount,
			newStart: hunkNewStart,
			newCount: hunkNewCount,
			lines:    hunkLines,
		})
	}

	return hunks
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
