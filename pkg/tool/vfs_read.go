package tool

import (
	"fmt"

	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSReadTool implements the vfsRead tool.
type VFSReadTool struct {
	vfs         vfs.VFS
	lineNumbers bool
}

// NewVFSReadTool creates a new VFSReadTool instance.
// If lineNumbers is true, the tool will format content with line numbers (cat -n style).
func NewVFSReadTool(v vfs.VFS, lineNumbers bool) *VFSReadTool {
	return &VFSReadTool{vfs: v, lineNumbers: lineNumbers}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSReadTool) Execute(args *ToolCall) *ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSReadTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	// Get limit parameter, default to 2000 if not provided
	limit := int64(2000)
	if args.Arguments.Has("limit") {
		if l, ok := args.Arguments.IntOK("limit"); ok {
			limit = l
		}
	}

	// Get offset parameter, default to 0 if not provided
	offset := int64(0)
	if args.Arguments.Has("offset") {
		if o, ok := args.Arguments.IntOK("offset"); ok {
			offset = o
		}
	}

	content, err := t.vfs.ReadFile(path)
	if err == vfs.ErrAskPermission {
		return NewVFSPermissionQuery(args, path, "reading file", "read")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionQuery(args, perr.Path, "reading file", "read")
	}
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: err,
			Done:  true,
		}
	}

	// Apply offset and limit to content
	contentStr := applyOffsetAndLimit(string(content), offset, limit)

	// Apply line numbers if enabled
	if t.lineNumbers {
		contentStr = formatWithLineNumbers(contentStr, offset)
	}

	var result ToolValue
	result.Set("content", contentStr)
	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *VFSReadTool) Render(call *ToolCall) (string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	relativePath := makeRelativePath(path, t.vfs)
	oneLiner := truncateString("read "+relativePath, 128)
	full := oneLiner + "\n\n"
	// Try to get content from result if available
	if content, ok := call.Arguments.Get("content").AsStringOK(); ok && content != "" {
		full += content
	}
	return oneLiner, full, make(map[string]string)
}
