package tool

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/codesnort/codesnort-swe/pkg/lsp"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
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

// VFSWriteTool implements the vfsWrite tool.
type VFSWriteTool struct {
	vfs    vfs.VFS
	lsp    lsp.LSP
	logger *slog.Logger
}

// NewVFSWriteTool creates a new VFSWriteTool instance.
// lsp parameter is optional and can be nil.
func NewVFSWriteTool(v vfs.VFS, l lsp.LSP) *VFSWriteTool {
	return &VFSWriteTool{vfs: v, lsp: l}
}

// SetLogger sets the logger for the tool.
func (t *VFSWriteTool) SetLogger(logger *slog.Logger) {
	t.logger = logger
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSWriteTool) Execute(args *ToolCall) *ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSWriteTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	content, ok := args.Arguments.StringOK("content")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSWriteTool.Execute() [vfs.go]: missing required argument: content"),
			Done:  true,
		}
	}

	// Log before calling tool
	if t.logger != nil {
		t.logger.Info("vfsWrite_start", "path", path)
	}

	err := t.vfs.WriteFile(path, []byte(content))
	if err == vfs.ErrAskPermission {
		if t.logger != nil {
			t.logger.Info("vfsWrite_permission_required", "path", path)
		}
		return NewVFSPermissionQuery(args, path, "writing to file", "write")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		if t.logger != nil {
			t.logger.Info("vfsWrite_permission_required", "path", perr.Path)
		}
		return NewVFSPermissionQuery(args, perr.Path, "writing to file", "write")
	}
	if err != nil {
		if t.logger != nil {
			t.logger.Error("vfsWrite_error", "path", path, "error", err.Error())
		}
		return &ToolResponse{
			Call:  args,
			Error: err,
			Done:  true,
		}
	}

	// Validate with LSP if available
	var validationMsg string
	var diagnostics []lsp.Diagnostic
	if t.lsp != nil {
		fileDiags, lspErr := t.lsp.TouchAndValidate(path, true)
		if lspErr != nil {
			// LSP validation error - log but don't fail the operation
			validationMsg = fmt.Sprintf("\n\nWarning: LSP validation failed: %v", lspErr)
			if t.logger != nil {
				t.logger.Warn("vfsWrite_lsp_validation_failed", "path", path, "error", lspErr.Error())
			}
		} else if len(fileDiags) > 0 {
			diagnostics = fileDiags
			// Format diagnostics for the edited file
			diagsWithURI := make([]DiagnosticWithURI, len(fileDiags))
			for i, d := range fileDiags {
				diagsWithURI[i] = DiagnosticWithURI{
					URI:        pathToURI(path),
					Diagnostic: d,
				}
			}
			validationMsg = formatDiagnostics(diagsWithURI, path)
		}
	}

	// Log after calling tool with result
	if t.logger != nil {
		if len(diagnostics) > 0 {
			t.logger.Info("vfsWrite_complete", "path", path, "result", "success", "diagnostics_count", len(diagnostics))
			for _, diag := range diagnostics {
				if diag.Severity == lsp.SeverityError {
					t.logger.Info("vfsWrite_diagnostic", "path", path, "line", diag.Range.Start.Line+1,
						"column", diag.Range.Start.Character+1, "message", diag.Message)
				}
			}
		} else {
			t.logger.Info("vfsWrite_complete", "path", path, "result", "success")
		}
	}

	var result ToolValue
	if validationMsg != "" {
		result.Set("validation", validationMsg)
	}

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

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

// VFSMoveTool implements the vfsMode tool.
type VFSMoveTool struct {
	vfs vfs.VFS
}

// NewVFSMoveTool creates a new VFSMoveTool instance.
func NewVFSMoveTool(v vfs.VFS) *VFSMoveTool {
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

	err := t.vfs.MoveFile(path, destination)
	if err == vfs.ErrAskPermission {
		return NewVFSPermissionQuery(args, path, "moving file", "move")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionQuery(args, perr.Path, "moving file", "move")
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
	query, ok := args.Arguments.StringOK("query")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSFindTool.Execute() [vfs.go]: missing required argument: query"),
			Done:  true,
		}
	}

	// Get recursive flag, default to false if not provided
	recursive := args.Arguments.Bool("recursive")

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

// VFSEditTool implements the vfsEdit tool.
type VFSEditTool struct {
	vfs    vfs.VFS
	lsp    lsp.LSP
	logger *slog.Logger
}

// NewVFSEditTool creates a new VFSEditTool instance.
// lsp parameter is optional and can be nil.
func NewVFSEditTool(v vfs.VFS, l lsp.LSP) *VFSEditTool {
	return &VFSEditTool{vfs: v, lsp: l}
}

// SetLogger sets the logger for the tool.
func (t *VFSEditTool) SetLogger(logger *slog.Logger) {
	t.logger = logger
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSEditTool) Execute(args *ToolCall) *ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSEditTool.Execute() [vfs.go]: missing required argument: path"),
			Done:  true,
		}
	}

	oldString, ok := args.Arguments.StringOK("oldString")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSEditTool.Execute() [vfs.go]: missing required argument: oldString"),
			Done:  true,
		}
	}

	newString, ok := args.Arguments.StringOK("newString")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSEditTool.Execute() [vfs.go]: missing required argument: newString"),
			Done:  true,
		}
	}

	// Get replaceAll flag, default to false if not provided
	replaceAll := args.Arguments.Bool("replaceAll")

	// Log before calling tool
	if t.logger != nil {
		t.logger.Info("vfsEdit_start", "path", path)
	}

	// Create patcher and apply edits
	patcher := vfs.NewFilePatcher(t.vfs)
	diff, err := patcher.ApplyEdits(path, oldString, newString, replaceAll)
	if err == vfs.ErrAskPermission {
		if t.logger != nil {
			t.logger.Info("vfsEdit_permission_required", "path", path)
		}
		return NewVFSPermissionQuery(args, path, "editing file", "write")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		if t.logger != nil {
			t.logger.Info("vfsEdit_permission_required", "path", perr.Path)
		}
		return NewVFSPermissionQuery(args, perr.Path, "editing file", "write")
	}
	if err != nil {
		if t.logger != nil {
			t.logger.Error("vfsEdit_error", "path", path, "error", err.Error())
		}
		return &ToolResponse{
			Call:  args,
			Error: err,
			Done:  true,
		}
	}

	// Validate with LSP if available
	var validationMsg string
	var diagnostics []lsp.Diagnostic
	if t.lsp != nil {
		fileDiags, lspErr := t.lsp.TouchAndValidate(path, true)
		if lspErr != nil {
			// LSP validation error - log but don't fail the operation
			validationMsg = fmt.Sprintf("\n\nWarning: LSP validation failed: %v", lspErr)
			if t.logger != nil {
				t.logger.Warn("vfsEdit_lsp_validation_failed", "path", path, "error", lspErr.Error())
			}
		} else if len(fileDiags) > 0 {
			diagnostics = fileDiags
			// Format diagnostics for the edited file
			diagsWithURI := make([]DiagnosticWithURI, len(fileDiags))
			for i, d := range fileDiags {
				diagsWithURI[i] = DiagnosticWithURI{
					URI:        pathToURI(path),
					Diagnostic: d,
				}
			}
			validationMsg = formatDiagnostics(diagsWithURI, path)
		}
	}

	// Log after calling tool with result
	if t.logger != nil {
		if len(diagnostics) > 0 {
			t.logger.Info("vfsEdit_complete", "path", path, "result", "success", "diagnostics_count", len(diagnostics))
			for _, diag := range diagnostics {
				if diag.Severity == lsp.SeverityError {
					t.logger.Info("vfsEdit_diagnostic", "path", path, "line", diag.Range.Start.Line+1,
						"column", diag.Range.Start.Character+1, "message", diag.Message)
				}
			}
		} else {
			t.logger.Info("vfsEdit_complete", "path", path, "result", "success")
		}
	}

	// Return the diff wrapped in a code block, plus validation results
	var result ToolValue
	resultContent := "```diff\n" + diff + "```"
	if validationMsg != "" {
		resultContent += validationMsg
	}
	result.Set("content", resultContent)
	return &ToolResponse{
		Call:   args,
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

// DiagnosticWithURI wraps a diagnostic with its file URI for proper grouping.
type DiagnosticWithURI struct {
	URI        string
	Diagnostic lsp.Diagnostic
}

// formatDiagnostics formats diagnostics from LSP validation into a human-readable error message.
// The format matches the example: "Error [line:col] message"
func formatDiagnostics(diags []DiagnosticWithURI, editedPath string) string {
	if len(diags) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\nLSP validation found issues:\n")

	// Group diagnostics by file
	diagsByFile := make(map[string][]lsp.Diagnostic)
	for _, d := range diags {
		diagsByFile[d.URI] = append(diagsByFile[d.URI], d.Diagnostic)
	}

	// Convert edited path to URI for comparison
	editedURI := pathToURI(editedPath)

	for uri, fileDiags := range diagsByFile {
		for _, diag := range fileDiags {
			// Only report errors (severity 1)
			if diag.Severity != lsp.SeverityError {
				continue
			}

			// Format: Error [line:col] message
			// LSP uses 0-based line/column numbers, so we add 1 for human-readable output
			line := diag.Range.Start.Line + 1
			col := diag.Range.Start.Character + 1

			// Add file path if it's different from the edited file
			if uri != editedURI {
				// Extract path from URI for display
				displayPath := uriToPath(uri)
				sb.WriteString(fmt.Sprintf("Error in %s [%d:%d] %s\n", displayPath, line, col, diag.Message))
			} else {
				sb.WriteString(fmt.Sprintf("Error [%d:%d] %s\n", line, col, diag.Message))
			}
		}
	}

	return sb.String()
}

// pathToURI converts a file path to a URI.
func pathToURI(path string) string {
	// Ensure path is absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Convert to URI format
	absPath = filepath.ToSlash(absPath)

	// If path doesn't start with /, add it (Windows case)
	if !strings.HasPrefix(absPath, "/") {
		absPath = "/" + absPath
	}

	return "file://" + absPath
}

// uriToPath converts a URI to a file path.
func uriToPath(uri string) string {
	// Remove file:// prefix
	path := strings.TrimPrefix(uri, "file://")
	// Convert to platform-specific path
	return filepath.FromSlash(path)
}

// Display returns a string representation of the tool call.
func (t *VFSReadTool) Display(mode DisplayMode, color bool) (string, map[string]string) {
	return "vfsRead", make(map[string]string)
}

// Display returns a string representation of the tool call.
func (t *VFSWriteTool) Display(mode DisplayMode, color bool) (string, map[string]string) {
	return "vfsWrite", make(map[string]string)
}

// Display returns a string representation of the tool call.
func (t *VFSDeleteTool) Display(mode DisplayMode, color bool) (string, map[string]string) {
	return "vfsDelete", make(map[string]string)
}

// Display returns a string representation of the tool call.
func (t *VFSListTool) Display(mode DisplayMode, color bool) (string, map[string]string) {
	return "vfsList", make(map[string]string)
}

// Display returns a string representation of the tool call.
func (t *VFSMoveTool) Display(mode DisplayMode, color bool) (string, map[string]string) {
	return "vfsMove", make(map[string]string)
}

// Display returns a string representation of the tool call.
func (t *VFSFindTool) Display(mode DisplayMode, color bool) (string, map[string]string) {
	return "vfsFind", make(map[string]string)
}

// Display returns a string representation of the tool call.
func (t *VFSEditTool) Display(mode DisplayMode, color bool) (string, map[string]string) {
	return "vfsEdit", make(map[string]string)
}
