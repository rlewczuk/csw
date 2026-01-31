package vfs

import (
	"path/filepath"
	"strings"
)

// GlobFilter filters paths based on glob patterns, compatible with .gitignore format.
type GlobFilter interface {
	// Matches returns true if the path matches the filter rules.
	Matches(path string) bool
}

// globFilter implements GlobFilter with .gitignore compatible syntax.
type globFilter struct {
	patterns     []globPattern
	defaultMatch bool // value returned when no patterns exist or path doesn't match any pattern
}

// globPattern represents a single glob pattern with inclusion/exclusion flag.
type globPattern struct {
	pattern string
	negate  bool // true for negation patterns (starting with !)
}

// NewGlobFilter creates a new GlobFilter from glob patterns and optional gitignore-like file contents.
// Patterns support .gitignore syntax:
//   - Lines starting with ! are negation patterns (re-include previously excluded files)
//   - Lines starting with # are comments (ignored)
//   - Empty lines are ignored
//   - Patterns can use *, ?, [abc], [a-z], and ** wildcards
//
// Parameters:
//   - defaultMatch: value returned when no patterns exist or path doesn't match any pattern
//   - patterns: slice of glob patterns
//   - gitignoreContents: optional contents of .gitignore-like files (variadic)
func NewGlobFilter(defaultMatch bool, patterns []string, gitignoreContents ...string) GlobFilter {
	f := &globFilter{
		patterns:     make([]globPattern, 0),
		defaultMatch: defaultMatch,
	}

	// Parse user-provided patterns
	for _, p := range patterns {
		if parsed := parseGlobPattern(p); parsed != nil {
			f.patterns = append(f.patterns, *parsed)
		}
	}

	// Parse gitignore contents
	for _, content := range gitignoreContents {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if parsed := parseGlobPattern(line); parsed != nil {
				f.patterns = append(f.patterns, *parsed)
			}
		}
	}

	// Optimization: if !** appears in the list, all patterns before it (and including it) are irrelevant
	// because !** excludes everything, making all previous patterns ineffective
	f.patterns = optimizeExcludeAll(f.patterns)

	return f
}

// optimizeExcludeAll removes all patterns up to and including the last occurrence of !**
// since !** excludes everything and makes all previous patterns irrelevant.
func optimizeExcludeAll(patterns []globPattern) []globPattern {
	// Find the last occurrence of !** (negate=true, pattern="**")
	lastExcludeAllIdx := -1
	for i, p := range patterns {
		if p.negate && p.pattern == "**" {
			lastExcludeAllIdx = i
		}
	}

	// If !** was found, discard all patterns up to and including it
	if lastExcludeAllIdx >= 0 {
		// Keep only patterns after the last !**
		return patterns[lastExcludeAllIdx+1:]
	}

	return patterns
}

// parseGlobPattern parses a single glob pattern line, handling comments and negation.
// Returns nil for empty lines or comments.
func parseGlobPattern(line string) *globPattern {
	// Trim whitespace
	line = strings.TrimSpace(line)

	// Skip empty lines and comments
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	// Check for negation pattern
	negate := false
	if strings.HasPrefix(line, "!") {
		negate = true
		line = strings.TrimPrefix(line, "!")
		line = strings.TrimSpace(line)
	}

	// Skip if pattern is empty after removing negation
	if line == "" {
		return nil
	}

	return &globPattern{
		pattern: line,
		negate:  negate,
	}
}

// Matches returns true if the path matches the filter rules.
// If no patterns are defined, returns the defaultMatch value.
// If path doesn't match any pattern, returns the defaultMatch value.
// Patterns are evaluated in order following .gitignore semantics:
//   - Normal patterns are inclusion rules (paths matching are included)
//   - Negation patterns (!) are exclusion rules (paths matching are excluded)
//   - Later patterns override earlier ones
func (f *globFilter) Matches(path string) bool {
	// If no patterns, return default
	if len(f.patterns) == 0 {
		return f.defaultMatch
	}

	// Normalize path to use forward slashes
	path = filepath.ToSlash(path)
	// Remove leading slash for consistent matching
	path = strings.TrimPrefix(path, "/")

	// Track if any pattern matched
	hasAnyMatch := false
	matched := false

	// Process patterns in order
	for _, p := range f.patterns {
		isMatch := matchGitignorePattern(p.pattern, path)

		if isMatch {
			hasAnyMatch = true
			// Negation patterns exclude, normal patterns include
			matched = !p.negate
		}
	}

	// If no patterns matched, return default
	if !hasAnyMatch {
		return f.defaultMatch
	}

	// Return the final matched state
	return matched
}

// matchGitignorePattern matches a path against a gitignore-style pattern.
// It handles directory patterns (ending with /) specially.
func matchGitignorePattern(pattern, path string) bool {
	// Normalize
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		// Directory pattern matches the directory and everything under it
		dirPattern := strings.TrimSuffix(pattern, "/")

		// Check if path starts with the directory
		if path == dirPattern || strings.HasPrefix(path, dirPattern+"/") {
			return true
		}

		// Also check with ** for patterns like **/tmp/
		if strings.Contains(dirPattern, "**") {
			// Try matching the directory itself
			match, err := matchGlob(dirPattern, path)
			if err == nil && match {
				return true
			}

			// Try matching as a prefix for any parent directory
			pathParts := strings.Split(path, "/")
			for i := 0; i < len(pathParts); i++ {
				parentPath := strings.Join(pathParts[:i+1], "/")
				match, err := matchGlob(dirPattern, parentPath)
				if err == nil && match {
					return true
				}
			}
		}

		return false
	}

	// Regular pattern matching
	match, err := matchGlob(pattern, path)
	if err != nil {
		return false
	}
	return match
}

// matchGlob matches a path against a glob pattern.
// Supports:
//   - * matches any number of characters except /
//   - ? matches any single character except /
//   - [abc] matches any character in the set
//   - [a-z] matches any character in the range
//   - ** matches any number of characters including /
//
// The path parameter should use forward slashes as separators.
// The pattern parameter should use forward slashes as separators.
func matchGlob(pattern, path string) (bool, error) {
	// Convert to forward slashes for consistent matching
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		return matchGlobWithDoubleStar(pattern, path)
	}

	// For patterns without **, use filepath.Match
	// But we need to handle path separators properly
	return matchGlobSimple(pattern, path)
}

// matchGlobSimple handles patterns without ** by matching the full path
func matchGlobSimple(pattern, path string) (bool, error) {
	// If pattern contains path separator, match against full path
	if strings.Contains(pattern, "/") {
		return filepath.Match(pattern, path)
	}

	// Otherwise, match against just the filename
	return filepath.Match(pattern, filepath.Base(path))
}

// matchGlobWithDoubleStar handles patterns containing **
func matchGlobWithDoubleStar(pattern, path string) (bool, error) {
	// Split pattern by /** or **/ or just **
	parts := splitGlobPattern(pattern)

	// Handle special case: pattern is just "**"
	if len(parts) == 1 && parts[0] == "**" {
		return true, nil
	}

	return matchGlobParts(parts, path)
}

// splitGlobPattern splits a pattern by ** and / while preserving the structure
func splitGlobPattern(pattern string) []string {
	var parts []string
	var current strings.Builder

	i := 0
	for i < len(pattern) {
		if i+1 < len(pattern) && pattern[i:i+2] == "**" {
			// Add current part if not empty
			if current.Len() > 0 {
				// Before adding, split by / if it contains any
				currentStr := current.String()
				currentStr = strings.Trim(currentStr, "/")
				if currentStr != "" {
					// Split by / and add each part
					subParts := strings.Split(currentStr, "/")
					for _, sp := range subParts {
						if sp != "" {
							parts = append(parts, sp)
						}
					}
				}
				current.Reset()
			}
			// Add ** as a separate part
			parts = append(parts, "**")
			i += 2
			// Skip following slashes
			for i < len(pattern) && pattern[i] == '/' {
				i++
			}
		} else {
			current.WriteByte(pattern[i])
			i++
		}
	}

	// Add remaining part if not empty
	if current.Len() > 0 {
		currentStr := current.String()
		currentStr = strings.Trim(currentStr, "/")
		if currentStr != "" {
			// Split by / and add each part
			subParts := strings.Split(currentStr, "/")
			for _, sp := range subParts {
				if sp != "" {
					parts = append(parts, sp)
				}
			}
		}
	}

	return parts
}

// matchGlobParts matches path against pattern parts split by **
func matchGlobParts(parts []string, path string) (bool, error) {
	if len(parts) == 0 {
		return path == "", nil
	}

	pathParts := strings.Split(path, "/")
	return matchGlobPartsRecursive(parts, 0, pathParts, 0)
}

// matchGlobPartsRecursive recursively matches pattern parts against path parts
func matchGlobPartsRecursive(patternParts []string, pi int, pathParts []string, pathIdx int) (bool, error) {
	// If we've matched all pattern parts, check if we've consumed all path parts
	if pi >= len(patternParts) {
		return pathIdx >= len(pathParts), nil
	}

	currentPattern := patternParts[pi]

	// Handle ** wildcard
	if currentPattern == "**" {
		// ** can match zero or more path segments
		// Try matching the rest of the pattern starting from each position
		for i := pathIdx; i <= len(pathParts); i++ {
			matched, err := matchGlobPartsRecursive(patternParts, pi+1, pathParts, i)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}

	// Handle regular pattern (with *, ?, [...])
	// This pattern must match one path segment
	if pathIdx >= len(pathParts) {
		return false, nil
	}

	// Match current pattern against current path part
	matched, err := filepath.Match(currentPattern, pathParts[pathIdx])
	if err != nil {
		return false, err
	}

	if !matched {
		return false, nil
	}

	// Continue matching next parts
	return matchGlobPartsRecursive(patternParts, pi+1, pathParts, pathIdx+1)
}
