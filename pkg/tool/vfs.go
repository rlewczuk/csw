package tool

import (
	"fmt"
	"path/filepath"

	"github.com/codesnort/codesnort-swe/pkg/shared"
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

// Info returns information about the tool including its name, description, and argument schema.
func (t *VFSReadTool) Info() ToolInfo {
	schema := NewToolSchema()
	schema.AddProperty("path", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The path to the file to read.",
	}, true)

	return ToolInfo{
		Name:        "vfs.read",
		Description: "Reads the content of a file at the specified path.",
		Schema:      schema,
	}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSReadTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSReadTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	content, err := t.vfs.ReadFile(path)
	if err == vfs.ErrAskPermission {
		return createPermissionQuery(args, path, "reading file", "read")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return createPermissionQuery(args, perr.Path, "reading file", "read")
	}
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: err,
			Done:  true,
		}
	}

	var result ToolValue
	result.Set("content", string(content))
	return ToolResponse{
		Call:   &args,
		Result: result,
		Done:   true,
	}
}

func createPermissionQuery(args ToolCall, path, action, op string) ToolResponse {
	query := &ToolPermissionsQuery{
		Id:      shared.GenerateUUIDv7(),
		Tool:    &args,
		Title:   "Permission Required",
		Details: fmt.Sprintf("Allow %s at path: %s", action, path),
		Options: []string{
			"Allow",
			"Deny",
			fmt.Sprintf("Allow in %s*", filepath.Dir(path)),
			fmt.Sprintf("Allow from %s/*", path),
		},
		AllowCustomResponse: true,
		Meta: map[string]string{
			"type":      "vfs",
			"path":      path,
			"operation": op,
		},
	}
	return ToolResponse{
		Call:  &args,
		Error: query,
		Done:  true,
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

// Info returns information about the tool including its name, description, and argument schema.
func (t *VFSWriteTool) Info() ToolInfo {
	schema := NewToolSchema()
	schema.AddProperty("path", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The path to the file to write.",
	}, true)
	schema.AddProperty("content", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The content to write to the file.",
	}, true)

	return ToolInfo{
		Name:        "vfs.write",
		Description: "Writes content to a file at the specified path. Creates the file if it doesn't exist.",
		Schema:      schema,
	}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSWriteTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSWriteTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	content, ok := args.Arguments.StringOK("content")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSWriteTool.Execute() [vfs.go]: missing required argument: content"),
			Done:  true,
		}
	}

	err := t.vfs.WriteFile(path, []byte(content))
	if err == vfs.ErrAskPermission {
		return createPermissionQuery(args, path, "writing to file", "write")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return createPermissionQuery(args, perr.Path, "writing to file", "write")
	}
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: err,
			Done:  true,
		}
	}

	return ToolResponse{
		Call: &args,
		Done: true,
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

// Info returns information about the tool including its name, description, and argument schema.
func (t *VFSDeleteTool) Info() ToolInfo {
	schema := NewToolSchema()
	schema.AddProperty("path", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The path to the file to delete.",
	}, true)

	return ToolInfo{
		Name:        "vfs.delete",
		Description: "Deletes a file at the specified path.",
		Schema:      schema,
	}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSDeleteTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSDeleteTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	err := t.vfs.DeleteFile(path, false, false)
	if err == vfs.ErrAskPermission {
		return createPermissionQuery(args, path, "deleting file", "delete")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return createPermissionQuery(args, perr.Path, "deleting file", "delete")
	}
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: err,
			Done:  true,
		}
	}

	return ToolResponse{
		Call: &args,
		Done: true,
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

// Info returns information about the tool including its name, description, and argument schema.
func (t *VFSListTool) Info() ToolInfo {
	schema := NewToolSchema()
	schema.AddProperty("path", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The directory path to list files from.",
	}, true)

	return ToolInfo{
		Name:        "vfs.list",
		Description: "Lists files in the specified directory.",
		Schema:      schema,
	}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSListTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSListTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	files, err := t.vfs.ListFiles(path, false)
	if err == vfs.ErrAskPermission {
		return createPermissionQuery(args, path, "listing files", "list")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return createPermissionQuery(args, perr.Path, "listing files", "list")
	}
	if err != nil {

		return ToolResponse{
			Call:  &args,
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
	return ToolResponse{
		Call:   &args,
		Result: result,
		Done:   true,
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

// Info returns information about the tool including its name, description, and argument schema.
func (t *VFSMoveTool) Info() ToolInfo {
	schema := NewToolSchema()
	schema.AddProperty("path", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The source path of the file to move.",
	}, true)
	schema.AddProperty("destination", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The destination path where the file should be moved to.",
	}, true)

	return ToolInfo{
		Name:        "vfs.move",
		Description: "Moves or renames a file from the source path to the destination path.",
		Schema:      schema,
	}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSMoveTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSMoveTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	destination, ok := args.Arguments.StringOK("destination")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSMoveTool.Execute() [vfs.go]: missing required argument: destination"),
			Done:  true,
		}
	}

	err := t.vfs.MoveFile(path, destination)
	if err == vfs.ErrAskPermission {
		return createPermissionQuery(args, path, "moving file", "move")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return createPermissionQuery(args, perr.Path, "moving file", "move")
	}
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: err,
			Done:  true,
		}
	}

	return ToolResponse{
		Call: &args,
		Done: true,
	}
}
