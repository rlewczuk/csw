package tool

import (
	"fmt"

	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// VFSListTool implements the vfsList tool.
type VFSListTool struct {
	vfs vfs.VFS
}

// NewVFSListTool creates a new VFSListTool instance.
func NewVFSListTool(v vfs.VFS) *VFSListTool {
	return &VFSListTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSListTool) Execute(args *ToolCall) *ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSListTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	files, err := t.vfs.ListFiles(path, false)
	if err == vfs.ErrAskPermission {
		return NewVFSPermissionQuery(args, path, "listing files", "list")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionQuery(args, perr.Path, "listing files", "list")
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
func (t *VFSListTool) Render(call *ToolCall) (string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	oneLiner := truncateString("list "+path, 128)
	full := oneLiner + "\n\n"
	// Try to get files from result if available
	if files, ok := call.Arguments.Get("files").ArrayOK(); ok && len(files) > 0 {
		for _, f := range files {
			full += f.AsString() + "\n"
		}
	}
	return oneLiner, full, make(map[string]string)
}
