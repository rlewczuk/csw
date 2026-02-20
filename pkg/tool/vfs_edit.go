package tool

import (
	"fmt"
	"log/slog"

	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/vfs"
)

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
	_, err := patcher.ApplyEdits(path, oldString, newString, replaceAll)
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
	var diagnostics []lsp.Diagnostic
	if t.lsp != nil {
		fileDiags, lspErr := t.lsp.TouchAndValidate(path, true)
		if lspErr != nil {
			// LSP validation error - log but don't fail the operation
			if t.logger != nil {
				t.logger.Warn("vfsEdit_lsp_validation_failed", "path", path, "error", lspErr.Error())
			}
		} else if len(fileDiags) > 0 {
			diagnostics = fileDiags
		}
	} else {
		if t.logger != nil {
			t.logger.Info("vfsEdit_lsp_not_available", "path", path)
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

	// Return proper response content
	var result ToolValue
	if len(diagnostics) > 0 {
		// Format diagnostics with the new format
		resultContent := "LSP errors detected in this file, please fix:\n"
		resultContent += fmt.Sprintf("<diagnostics file=\"%s\">\n", path)
		for _, diag := range diagnostics {
			if diag.Severity == lsp.SeverityError {
				line := diag.Range.Start.Line + 1
				col := diag.Range.Start.Character + 1
				resultContent += fmt.Sprintf("Error[%d:%d] %s\n", line, col, diag.Message)
			}
		}
		resultContent += "</diagnostics>"
		result.Set("content", resultContent)
	} else {
		// No diagnostics, return success message
		result.Set("content", "Edit applied successfully")
	}
	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *VFSEditTool) Render(call *ToolCall) (string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	oldString, _ := call.Arguments.StringOK("oldString")
	newString, _ := call.Arguments.StringOK("newString")
	oneLiner := truncateString("edit "+path, 128)
	full := oneLiner + "\n\n"
	// Create unified diff without line numbers
	full += "--- " + path + "\n"
	full += "+++ " + path + "\n"
	full += "@@ -1 +1 @@\n"
	full += "-" + oldString + "\n"
	full += "+" + newString + "\n"
	return oneLiner, full, make(map[string]string)
}
