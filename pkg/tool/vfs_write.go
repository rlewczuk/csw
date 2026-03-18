package tool

import (
	"fmt"
	"log/slog"

	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSWriteTool implements the vfsWrite tool.
type VFSWriteTool struct {
	vfs    vfs.VFS
	lsp    lsp.LSP
	logger *slog.Logger
}

func (t *VFSWriteTool) GetDescription() (string, bool) {
	return "", false
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
	} else {
		if t.logger != nil {
			t.logger.Info("vfsWrite_lsp_not_available", "path", path)
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

// Render returns a string representation of the tool call.
func (t *VFSWriteTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	relativePath := makeRelativePath(path, t.vfs)
	baseJSONL := buildToolRenderJSONL("vfsWrite", call, map[string]any{"path": relativePath})
	oneLiner := truncateString("write "+relativePath, 128)
	full := oneLiner + "\n\n"

	// Check for error in arguments
	if errMsg, ok := call.Arguments.StringOK("error"); ok && errMsg != "" {
		errOneLiner, errFull := formatRenderError(errMsg)
		// Add error as second line to oneLiner
		oneLiner = oneLiner + "\n" + errOneLiner
		// Add error to full output
		full = full + errFull
		return oneLiner, full, baseJSONL, make(map[string]string)
	}

	// Try to get content from arguments
	if content, ok := call.Arguments.StringOK("content"); ok && content != "" {
		full += content
	}
	return oneLiner, full, baseJSONL, make(map[string]string)
}
