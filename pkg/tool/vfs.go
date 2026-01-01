package tool

import (
	"encoding/json"
	"fmt"

	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// VFSReadTool implements the vfs.read tool.
type VFSReadTool struct {
	vfs vfs.VFS
}

// NewVFSReadTool creates a new VFSReadTool instance.
func NewVFSReadTool(v vfs.VFS) *VFSReadTool {
	return &VFSReadTool{vfs: v}
}

// Name returns the name of the tool.
func (t *VFSReadTool) Name() string {
	return "vfs.read"
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSReadTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments["path"]
	if !ok {
		return ToolResponse{
			ID:     args.ID,
			Error:  fmt.Errorf("missing required argument: path"),
			Result: nil,
			Done:   true,
		}
	}

	content, err := t.vfs.ReadFile(path)
	if err != nil {
		return ToolResponse{
			ID:     args.ID,
			Error:  err,
			Result: nil,
			Done:   true,
		}
	}

	return ToolResponse{
		ID:    args.ID,
		Error: nil,
		Result: map[string]string{
			"content": string(content),
		},
		Done: true,
	}
}

// VFSWriteTool implements the vfs.write tool.
type VFSWriteTool struct {
	vfs vfs.VFS
}

// NewVFSWriteTool creates a new VFSWriteTool instance.
func NewVFSWriteTool(v vfs.VFS) *VFSWriteTool {
	return &VFSWriteTool{vfs: v}
}

// Name returns the name of the tool.
func (t *VFSWriteTool) Name() string {
	return "vfs.write"
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSWriteTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments["path"]
	if !ok {
		return ToolResponse{
			ID:     args.ID,
			Error:  fmt.Errorf("missing required argument: path"),
			Result: nil,
			Done:   true,
		}
	}

	content, ok := args.Arguments["content"]
	if !ok {
		return ToolResponse{
			ID:     args.ID,
			Error:  fmt.Errorf("missing required argument: content"),
			Result: nil,
			Done:   true,
		}
	}

	err := t.vfs.WriteFile(path, []byte(content))
	if err != nil {
		return ToolResponse{
			ID:     args.ID,
			Error:  err,
			Result: nil,
			Done:   true,
		}
	}

	return ToolResponse{
		ID:     args.ID,
		Error:  nil,
		Result: map[string]string{},
		Done:   true,
	}
}

// VFSDeleteTool implements the vfs.delete tool.
type VFSDeleteTool struct {
	vfs vfs.VFS
}

// NewVFSDeleteTool creates a new VFSDeleteTool instance.
func NewVFSDeleteTool(v vfs.VFS) *VFSDeleteTool {
	return &VFSDeleteTool{vfs: v}
}

// Name returns the name of the tool.
func (t *VFSDeleteTool) Name() string {
	return "vfs.delete"
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSDeleteTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments["path"]
	if !ok {
		return ToolResponse{
			ID:     args.ID,
			Error:  fmt.Errorf("missing required argument: path"),
			Result: nil,
			Done:   true,
		}
	}

	err := t.vfs.DeleteFile(path, false, false)
	if err != nil {
		return ToolResponse{
			ID:     args.ID,
			Error:  err,
			Result: nil,
			Done:   true,
		}
	}

	return ToolResponse{
		ID:     args.ID,
		Error:  nil,
		Result: map[string]string{},
		Done:   true,
	}
}

// VFSListTool implements the vfs.list tool.
type VFSListTool struct {
	vfs vfs.VFS
}

// NewVFSListTool creates a new VFSListTool instance.
func NewVFSListTool(v vfs.VFS) *VFSListTool {
	return &VFSListTool{vfs: v}
}

// Name returns the name of the tool.
func (t *VFSListTool) Name() string {
	return "vfs.list"
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSListTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments["path"]
	if !ok {
		return ToolResponse{
			ID:     args.ID,
			Error:  fmt.Errorf("missing required argument: path"),
			Result: nil,
			Done:   true,
		}
	}

	files, err := t.vfs.ListFiles(path, false)
	if err != nil {
		return ToolResponse{
			ID:     args.ID,
			Error:  err,
			Result: nil,
			Done:   true,
		}
	}

	// Convert files array to JSON
	filesJSON, err := json.Marshal(files)
	if err != nil {
		return ToolResponse{
			ID:     args.ID,
			Error:  err,
			Result: nil,
			Done:   true,
		}
	}

	return ToolResponse{
		ID:    args.ID,
		Error: nil,
		Result: map[string]string{
			"files": string(filesJSON),
		},
		Done: true,
	}
}

// VFSMoveTool implements the vfs.move tool.
type VFSMoveTool struct {
	vfs vfs.VFS
}

// NewVFSMoveTool creates a new VFSMoveTool instance.
func NewVFSMoveTool(v vfs.VFS) *VFSMoveTool {
	return &VFSMoveTool{vfs: v}
}

// Name returns the name of the tool.
func (t *VFSMoveTool) Name() string {
	return "vfs.move"
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSMoveTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments["path"]
	if !ok {
		return ToolResponse{
			ID:     args.ID,
			Error:  fmt.Errorf("missing required argument: path"),
			Result: nil,
			Done:   true,
		}
	}

	destination, ok := args.Arguments["destination"]
	if !ok {
		return ToolResponse{
			ID:     args.ID,
			Error:  fmt.Errorf("missing required argument: destination"),
			Result: nil,
			Done:   true,
		}
	}

	err := t.vfs.MoveFile(path, destination)
	if err != nil {
		return ToolResponse{
			ID:     args.ID,
			Error:  err,
			Result: nil,
			Done:   true,
		}
	}

	return ToolResponse{
		ID:     args.ID,
		Error:  nil,
		Result: map[string]string{},
		Done:   true,
	}
}
