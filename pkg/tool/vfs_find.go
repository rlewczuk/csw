package tool

import (
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// VFSFindTool implements the vfsFind tool.
type VFSFindTool struct {
	vfs vfs.VFS
}

// NewVFSFindTool creates a new VFSFindTool instance.
func NewVFSFindTool(v vfs.VFS) *VFSFindTool {
	return &VFSFindTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSFindTool) Execute(args *ToolCall) *ToolResponse {
	// Get query parameter, empty string means match all files (use "**")
	query := args.Arguments.String("query")
	if query == "" {
		query = "**"
	}

	// Get recursive flag, default to true if not provided
	recursive := true
	if args.Arguments.Has("recursive") {
		recursive = args.Arguments.Bool("recursive")
	}

	files, err := t.vfs.FindFiles(query, recursive)
	if err == vfs.ErrAskPermission {
		return NewVFSPermissionQuery(args, query, "finding files", "find")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionQuery(args, perr.Path, "finding files", "find")
	}
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: err,
			Done:  true,
		}
	}

	// Convert files to array of any for ToolValue
	filesArray := make([]any, len(files))
	for i, f := range files {
		filesArray[i] = f
	}

	var result ToolValue
	result.Set("files", filesArray)
	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *VFSFindTool) Render(call *ToolCall) (string, string, map[string]string) {
	query, _ := call.Arguments.StringOK("query")
	oneLiner := truncateString("find "+query, 128)
	full := oneLiner + "\n\n"
	// Try to get files from result if available
	if files, ok := call.Arguments.Get("files").ArrayOK(); ok && len(files) > 0 {
		for _, f := range files {
			full += f.AsString() + "\n"
		}
	}
	return oneLiner, full, make(map[string]string)
}
