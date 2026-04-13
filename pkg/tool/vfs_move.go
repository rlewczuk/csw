package tool

import (
	"fmt"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSMoveTool implements the vfsMove tool.
type VFSMoveTool struct {
	vfs apis.VFS
}

func (t *VFSMoveTool) GetDescription() (string, bool) {
	return "", false
}

// NewVFSMoveTool creates a new VFSMoveTool instance.
func NewVFSMoveTool(v apis.VFS) *VFSMoveTool {
	return &VFSMoveTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSMoveTool) Execute(args *ToolCall) *ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSMoveTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	destination, ok := args.Arguments.StringOK("destination")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSMoveTool.Execute() [vfs.go]: missing required argument: destination"),
			Done:  true,
		}
	}

	result := ToolValue{}
	result.Set("path", path)
	result.Set("destination", destination)

	err := t.vfs.MoveFile(path, destination)
	if err == apis.ErrAskPermission {
		return NewVFSPermissionDeniedResponse(args, path, "move")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionDeniedResponse(args, perr.Path, perr.Operation)
	}
	if err != nil {
		return &ToolResponse{
			Call:   args,
			Error:  err,
			Result: result,
			Done:   true,
		}
	}

	return &ToolResponse{
		Call:   args,
		Result: *result.Set("message", "File successfully moved"),
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *VFSMoveTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	destination, _ := call.Arguments.StringOK("destination")
	relativePath := makeRelativePath(path, t.vfs)
	relativeDest := makeRelativePath(destination, t.vfs)
	oneLiner := truncateString("move "+relativePath+" -> "+relativeDest, 128)
	jsonl := buildToolRenderJSONL("vfsMove", call, map[string]any{"path": relativePath, "destination": relativeDest})
	full := oneLiner

	// Check for error in arguments
	if errMsg, ok := call.Arguments.StringOK("error"); ok && errMsg != "" {
		errOneLiner, errFull := formatRenderError(errMsg)
		// Add error as second line to oneLiner
		oneLiner = oneLiner + "\n" + errOneLiner
		// Add error to full output
		full = full + "\n\n" + errFull
		return oneLiner, full, jsonl, make(map[string]string)
	}

	return oneLiner, full, jsonl, make(map[string]string)
}
