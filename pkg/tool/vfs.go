package tool

import (
	"fmt"
	"path/filepath"

	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// VFSReadTool implements the vfs.read tool.
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
func (t *VFSReadTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return ToolResponse{
			Call:  &args,
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

	// Apply offset and limit to content
	contentStr := applyOffsetAndLimit(string(content), offset, limit)

	// Apply line numbers if enabled
	if t.lineNumbers {
		contentStr = formatWithLineNumbers(contentStr, offset)
	}

	var result ToolValue
	result.Set("content", contentStr)
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

// VFSListTool implements the vfs.ls tool.
type VFSListTool struct {
	vfs vfs.VFS
}

// NewVFSListTool creates a new VFSListTool instance.
func NewVFSListTool(v vfs.VFS) *VFSListTool {
	return &VFSListTool{vfs: v}
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

// VFSFindTool implements the vfs.find tool.
type VFSFindTool struct {
	vfs vfs.VFS
}

// NewVFSFindTool creates a new VFSFindTool instance.
func NewVFSFindTool(v vfs.VFS) *VFSFindTool {
	return &VFSFindTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSFindTool) Execute(args ToolCall) ToolResponse {
	query, ok := args.Arguments.StringOK("query")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSFindTool.Execute() [vfs.go]: missing required argument: query"),
			Done:  true,
		}
	}

	// Get recursive flag, default to false if not provided
	recursive := args.Arguments.Bool("recursive")

	files, err := t.vfs.FindFiles(query, recursive)
	if err == vfs.ErrAskPermission {
		return createPermissionQuery(args, query, "finding files", "find")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return createPermissionQuery(args, perr.Path, "finding files", "find")
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

// VFSEditTool implements the vfs.edit tool.
type VFSEditTool struct {
	vfs vfs.VFS
}

// NewVFSEditTool creates a new VFSEditTool instance.
func NewVFSEditTool(v vfs.VFS) *VFSEditTool {
	return &VFSEditTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSEditTool) Execute(args ToolCall) ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSEditTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	oldString, ok := args.Arguments.StringOK("oldString")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSEditTool.Execute() [vfs.go]: missing required argument: oldString"),
			Done:  true,
		}
	}

	newString, ok := args.Arguments.StringOK("newString")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("VFSEditTool.Execute() [vfs.go]: missing required argument: newString"),
			Done:  true,
		}
	}

	// Get replaceAll flag, default to false if not provided
	replaceAll := args.Arguments.Bool("replaceAll")

	// Create patcher and apply edits
	patcher := vfs.NewFilePatcher(t.vfs)
	diff, err := patcher.ApplyEdits(path, oldString, newString, replaceAll)
	if err == vfs.ErrAskPermission {
		return createPermissionQuery(args, path, "editing file", "write")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return createPermissionQuery(args, perr.Path, "editing file", "write")
	}
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: err,
			Done:  true,
		}
	}

	// Return the diff wrapped in a code block
	var result ToolValue
	result.Set("content", "```diff\n"+diff+"```")
	return ToolResponse{
		Call:   &args,
		Result: result,
		Done:   true,
	}
}

// applyOffsetAndLimit applies offset and limit to content by lines.
// offset is the number of lines to skip, limit is the maximum number of lines to return.
func applyOffsetAndLimit(content string, offset, limit int64) string {
	if content == "" {
		return ""
	}

	// Split content into lines
	lines := splitLines(content)

	// Apply offset
	if offset >= int64(len(lines)) {
		return ""
	}
	if offset > 0 {
		lines = lines[offset:]
	}

	// Apply limit
	if limit > 0 && int64(len(lines)) > limit {
		lines = lines[:limit]
	}

	// Join lines back together
	return joinLines(lines)
}

// splitLines splits content into lines, preserving line endings.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}

	var lines []string
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i+1])
			start = i + 1
		}
	}

	// Add remaining content if any (file doesn't end with newline)
	if start < len(content) {
		lines = append(lines, content[start:])
	}

	return lines
}

// joinLines joins lines back together.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	result := ""
	for _, line := range lines {
		result += line
	}
	return result
}

// formatWithLineNumbers formats content with line numbers in cat -n style.
// Format: 5 columns for line number (right-aligned), two spaces, then content.
// Example: "    1  first line\n    2  second line\n"
func formatWithLineNumbers(content string, startLine int64) string {
	if content == "" {
		return ""
	}

	lines := splitLines(content)
	result := ""

	for i, line := range lines {
		lineNum := startLine + int64(i) + 1
		// Format: %5d (5 columns, right-aligned) + "  " (two spaces) + line content
		result += formatLineNumber(lineNum) + "  " + line
	}

	return result
}

// formatLineNumber formats a line number with 5 columns, right-aligned.
func formatLineNumber(num int64) string {
	str := ""
	// Convert number to string manually
	if num == 0 {
		str = "0"
	} else {
		digits := []byte{}
		n := num
		for n > 0 {
			digits = append([]byte{byte('0' + n%10)}, digits...)
			n /= 10
		}
		str = string(digits)
	}

	// Pad with spaces to 5 columns (right-aligned)
	for len(str) < 5 {
		str = " " + str
	}

	return str
}
