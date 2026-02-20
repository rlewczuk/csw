package tool

import (
	"fmt"

	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSDeleteTool implements the vfsDelete tool.
type VFSDeleteTool struct {
	vfs vfs.VFS
}

// NewVFSDeleteTool creates a new VFSDeleteTool instance.
func NewVFSDeleteTool(v vfs.VFS) *VFSDeleteTool {
	return &VFSDeleteTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSDeleteTool) Execute(args *ToolCall) *ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSDeleteTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	err := t.vfs.DeleteFile(path, false, false)
	if err == vfs.ErrAskPermission {
		return NewVFSPermissionQuery(args, path, "deleting file", "delete")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionQuery(args, perr.Path, "deleting file", "delete")
	}
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: err,
			Done:  true,
		}
	}

	return &ToolResponse{
		Call: args,
		Done: true,
	}
}

// Render returns a string representation of the tool call.
func (t *VFSDeleteTool) Render(call *ToolCall) (string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	oneLiner := truncateString("delete "+path, 128)
	full := oneLiner
	return oneLiner, full, make(map[string]string)
}
