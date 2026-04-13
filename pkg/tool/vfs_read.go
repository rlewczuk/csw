package tool

import (
	"fmt"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSReadTool implements the vfsRead tool.
type VFSReadTool struct {
	vfs         apis.VFS
	lineNumbers bool
}

func (t *VFSReadTool) GetDescription() (string, bool) {
	return "", false
}

// NewVFSReadTool creates a new VFSReadTool instance.
// If lineNumbers is true, the tool will format content with line numbers (cat -n style).
func NewVFSReadTool(v apis.VFS, lineNumbers bool) *VFSReadTool {
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
	if err == apis.ErrAskPermission {
		return NewVFSPermissionDeniedResponse(args, path, "read")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionDeniedResponse(args, perr.Path, perr.Operation)
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
func (t *VFSReadTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	relativePath := makeRelativePath(path, t.vfs)
	baseJSONL := buildToolRenderJSONL("vfsRead", call, map[string]any{"path": relativePath})

	// Check for error in arguments
	if errMsg, ok := call.Arguments.StringOK("error"); ok && errMsg != "" {
		oneLiner := truncateString("read "+relativePath, 128)
		full := oneLiner + "\n\n"
		errOneLiner, errFull := formatRenderError(errMsg)
		// Add error as second line to oneLiner
		oneLiner = oneLiner + "\n" + errOneLiner
		// Add error to full output
		full = full + errFull
		return oneLiner, full, baseJSONL, make(map[string]string)
	}

	// Try to get content from result if available
	content, _ := call.Arguments.Get("content").AsStringOK()
	lineCount := countLines(content)
	oneLiner := truncateString(fmt.Sprintf("read %s (%d lines)", relativePath, lineCount), 128)
	full := oneLiner + "\n\n"
	if content != "" {
		full += content
	}
	return oneLiner, full, baseJSONL, make(map[string]string)
}
