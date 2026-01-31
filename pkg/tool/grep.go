package tool

import (
	"fmt"
	"strings"

	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// VFSGrepTool implements the vfs.grep tool.
type VFSGrepTool struct {
	vfs vfs.VFS
}

// NewVFSGrepTool creates a new VFSGrepTool instance.
func NewVFSGrepTool(v vfs.VFS) *VFSGrepTool {
	return &VFSGrepTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSGrepTool) Execute(args ToolCall) ToolResponse {
	pattern, ok := args.Arguments.StringOK("pattern")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSGrepTool.Execute() [grep.go]: missing required argument: pattern"),
			Done:  true,
		}
	}

	// Get optional path parameter, default to "" (root directory)
	path := args.Arguments.String("path")

	// Get optional include parameter, default to ""
	include := args.Arguments.String("include")

	// Get optional limit parameter, default to 100
	limit := int64(100)
	if args.Arguments.Has("limit") {
		if l, ok := args.Arguments.IntOK("limit"); ok {
			limit = l
		}
	}

	// Create glob filter if include patterns are specified
	var globFilter vfs.GlobFilter
	if include != "" {
		// Split include by comma to get multiple patterns
		patterns := strings.Split(include, ",")
		// Trim whitespace from each pattern
		for i := range patterns {
			patterns[i] = strings.TrimSpace(patterns[i])
		}
		// Create glob filter with defaultMatch=false (exclude files by default)
		// Only files matching the patterns will be included
		globFilter = vfs.NewGlobFilter(false, patterns)
	}

	// Create grep filter
	grepFilter, err := vfs.NewGrepFilter(pattern, t.vfs, path, globFilter)
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSGrepTool.Execute() [grep.go]: %w", err),
			Done:  true,
		}
	}

	// Perform the search
	matches, err := grepFilter.Search()
	if err == vfs.ErrAskPermission {
		return createPermissionQuery(args, path, "searching files", "read")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return createPermissionQuery(args, perr.Path, "searching files", "read")
	}
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSGrepTool.Execute() [grep.go]: %w", err),
			Done:  true,
		}
	}

	// Format the results
	content := t.formatResults(matches, limit)

	var result ToolValue
	result.Set("content", content)
	return ToolResponse{
		Call:   &args,
		Result: result,
		Done:   true,
	}
}

// formatResults formats the grep matches into a string with path:line_number format.
// If there are more than limit matches, returns only the first limit matches and adds a truncation message.
func (t *VFSGrepTool) formatResults(matches []vfs.GrepMatch, limit int64) string {
	if len(matches) == 0 {
		return "No files found"
	}

	var builder strings.Builder
	matchCount := int64(0)
	truncated := false

	for _, match := range matches {
		for _, lineNum := range match.Lines {
			if matchCount >= limit {
				truncated = true
				break
			}
			// Format: path:line_number
			builder.WriteString(match.Path)
			builder.WriteString(":")
			builder.WriteString(formatInt64(int64(lineNum)))
			builder.WriteString("\n")
			matchCount++
		}
		if truncated {
			break
		}
	}

	// Add truncation message if needed
	if truncated {
		builder.WriteString("(Results are truncated. Consider using a more specific path or pattern.)")
	} else {
		// Remove the trailing newline if not truncated
		result := builder.String()
		if len(result) > 0 && result[len(result)-1] == '\n' {
			return result[:len(result)-1]
		}
		return result
	}

	return builder.String()
}

// formatInt64 converts an int64 to a string.
func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}

	// Handle negative numbers
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}

	// Convert to string by building up digits
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
