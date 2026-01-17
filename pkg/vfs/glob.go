package vfs

import (
	"path/filepath"
	"strings"
)

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

// splitGlobPattern splits a pattern by ** while preserving the structure
func splitGlobPattern(pattern string) []string {
	var parts []string
	var current strings.Builder

	i := 0
	for i < len(pattern) {
		if i+1 < len(pattern) && pattern[i:i+2] == "**" {
			// Add current part if not empty
			if current.Len() > 0 {
				parts = append(parts, current.String())
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
		parts = append(parts, current.String())
	}

	return parts
}

// matchGlobParts matches path against pattern parts split by **
func matchGlobParts(parts []string, path string) (bool, error) {
	if len(parts) == 0 {
		return path == "", nil
	}

	// Remove leading/trailing slashes from parts
	for i, part := range parts {
		parts[i] = strings.Trim(part, "/")
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
