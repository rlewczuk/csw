package vfs

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
)

// GrepMatch represents a single file match with line numbers where the pattern was found.
type GrepMatch struct {
	// Path is the path to the file from VFS root
	Path string
	// Lines contains all line numbers (1-based) where matches were found
	Lines []int
}

// GrepFilter searches for files containing a given pattern in a VFS.
type GrepFilter interface {
	// Search performs the grep operation and returns matches.
	// Returns an array of GrepMatch structs, one per matching file.
	Search() ([]GrepMatch, error)
}

// grepFilter implements GrepFilter interface.
type grepFilter struct {
	pattern    *regexp.Regexp
	vfs        apis.VFS
	rootDir    string
	globFilter GlobFilter
}

// NewGrepFilter creates a new GrepFilter that searches for files containing the given regex pattern.
// Parameters:
//   - pattern: regex pattern to search for (using Go regexp syntax)
//   - vfs: VFS instance to search in
//   - rootDir: root directory to search from (or "" to search in the whole VFS)
//   - globFilter: optional GlobFilter to filter files (can be nil to include all files)
//
// Returns an error if the pattern is invalid.
func NewGrepFilter(pattern string, vfs apis.VFS, rootDir string, globFilter GlobFilter) (GrepFilter, error) {
	if vfs == nil {
		return nil, fmt.Errorf("NewGrepFilter() [grep.go]: VFS cannot be nil")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("NewGrepFilter() [grep.go]: invalid regex pattern: %w", err)
	}

	// Normalize rootDir
	if rootDir == "" {
		rootDir = "."
	}

	return &grepFilter{
		pattern:    re,
		vfs:        vfs,
		rootDir:    rootDir,
		globFilter: globFilter,
	}, nil
}

// Search performs the grep operation and returns matches.
// It recursively searches all files in the root directory (or whole VFS if root is ""),
// applying the glob filter if provided.
// Returns an array of GrepMatch structs, one per matching file.
func (g *grepFilter) Search() ([]GrepMatch, error) {
	// Get all files recursively
	files, err := g.vfs.ListFiles(g.rootDir, true)
	if err != nil {
		return nil, fmt.Errorf("grepFilter.Search() [grep.go]: %w", err)
	}

	var matches []GrepMatch

	for _, filePath := range files {
		// Calculate the path relative to VFS root
		relPath := filePath
		if g.rootDir != "." && g.rootDir != "" {
			// If we're searching from a subdirectory, the paths returned by ListFiles
			// already include the rootDir prefix, so we use them as-is
			relPath = filePath
		}

		// Apply glob filter if provided
		if g.globFilter != nil {
			if !g.globFilter.Matches(relPath) {
				continue
			}
		}

		// Try to read the file (skip directories)
		content, err := g.vfs.ReadFile(relPath)
		if err != nil {
			// Skip if it's a directory or unreadable
			continue
		}

		// Search for matches in the file
		lineNumbers := g.searchInContent(content)
		if len(lineNumbers) > 0 {
			matches = append(matches, GrepMatch{
				Path:  relPath,
				Lines: lineNumbers,
			})
		}
	}

	return matches, nil
}

// searchInContent searches for the pattern in the given content and returns line numbers (1-based).
func (g *grepFilter) searchInContent(content []byte) []int {
	var lineNumbers []int
	seenLines := make(map[int]bool)

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check if the line matches the pattern
		if g.pattern.MatchString(line) {
			// Only add if we haven't seen this line number yet
			// (though with our logic, each line is only scanned once)
			if !seenLines[lineNum] {
				lineNumbers = append(lineNumbers, lineNum)
				seenLines[lineNum] = true
			}
		}
	}

	return lineNumbers
}

// normalizePath normalizes a path by converting it to use forward slashes
// and removing leading slashes.
func normalizePath(path string) string {
	path = filepath.ToSlash(path)
	path = strings.TrimPrefix(path, "/")
	return path
}
