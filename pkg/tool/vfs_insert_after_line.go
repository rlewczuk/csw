package tool

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSInsertAfterLineTool implements the vfsInsertAfterLine tool.
type VFSInsertAfterLineTool struct {
	vfs    apis.VFS
	lsp    lsp.LSP
	logger *slog.Logger
}

func (t *VFSInsertAfterLineTool) GetDescription() (string, bool) {
	return "", false
}

// NewVFSInsertAfterLineTool creates a new VFSInsertAfterLineTool instance.
// lsp parameter is optional and can be nil.
func NewVFSInsertAfterLineTool(v apis.VFS, l lsp.LSP) *VFSInsertAfterLineTool {
	return &VFSInsertAfterLineTool{vfs: v, lsp: l}
}

// SetLogger sets the logger for the tool.
func (t *VFSInsertAfterLineTool) SetLogger(logger *slog.Logger) {
	t.logger = logger
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSInsertAfterLineTool) Execute(args *ToolCall) *ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return vfsInsertAfterLineError(args, "missing required argument: path")
	}

	lineNumber, ok := args.Arguments.IntOK("line_number")
	if !ok {
		return vfsInsertAfterLineError(args, "missing required argument: line_number")
	}

	insertContent, ok := args.Arguments.StringOK("content")
	if !ok {
		return vfsInsertAfterLineError(args, "missing required argument: content")
	}

	expectedSHA256, _ := args.Arguments.StringOK("expected_sha256")

	if t.logger != nil {
		t.logger.Info("vfsInsertAfterLine_start", "path", path, "line_number", lineNumber)
	}

	content, err := t.vfs.ReadFile(path)
	if err == apis.ErrAskPermission {
		return NewVFSPermissionDeniedResponse(args, path, "read")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionDeniedResponse(args, perr.Path, perr.Operation)
	}
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	actualSHA256 := sha256Hex(content)
	if expectedSHA256 != "" && !strings.EqualFold(expectedSHA256, actualSHA256) {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSInsertAfterLineTool.Execute() [vfs_insert_after_line.go]: expected_sha256 mismatch: expected %s, got %s", expectedSHA256, actualSHA256),
			Done:  true,
		}
	}

	newContent, err := insertAfterLine(string(content), lineNumber, insertContent)
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	err = t.vfs.WriteFile(path, []byte(newContent))
	if err == apis.ErrAskPermission {
		return NewVFSPermissionDeniedResponse(args, path, "write")
	}
	if perr, ok := err.(*vfs.PermissionError); ok {
		return NewVFSPermissionDeniedResponse(args, perr.Path, perr.Operation)
	}
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	diagnostics := t.validateWithLSP(path)
	t.logComplete(path, diagnostics)

	var result ToolValue
	result.Set("content", "Line inserted successfully")
	result.Set("sha256", sha256Hex([]byte(newContent)))
	if len(diagnostics) > 0 {
		result.Set("content", formatInsertAfterLineDiagnostics(path, diagnostics))
	}

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *VFSInsertAfterLineTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	insertContent, _ := call.Arguments.StringOK("content")
	lineNumber, _ := call.Arguments.IntOK("line_number")
	relativePath := makeRelativePath(path, t.vfs)
	baseJSONL := buildToolRenderJSONL("vfsInsertAfterLine", call, map[string]any{"path": relativePath})

	linesAdded := int64(countLines(insertContent))
	oneLiner := truncateString(fmt.Sprintf("insert after line %s:%d (+%d)", relativePath, lineNumber, linesAdded), 128)
	full := oneLiner + "\n\n"

	if errMsg, ok := call.Arguments.StringOK("error"); ok && errMsg != "" {
		errOneLiner, errFull := formatRenderError(errMsg)
		oneLiner = oneLiner + "\n" + errOneLiner
		full = full + errFull
		return oneLiner, full, baseJSONL, make(map[string]string)
	}

	full += fmt.Sprintf("+++ %s:%d\n", relativePath, lineNumber)
	if insertContent != "" {
		full += insertContent
	}
	return oneLiner, full, baseJSONL, make(map[string]string)
}

func (t *VFSInsertAfterLineTool) validateWithLSP(path string) []lsp.Diagnostic {
	if t.lsp == nil {
		if t.logger != nil {
			t.logger.Info("vfsInsertAfterLine_lsp_not_available", "path", path)
		}
		return nil
	}

	diagnostics, err := t.lsp.TouchAndValidate(path, true)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("vfsInsertAfterLine_lsp_validation_failed", "path", path, "error", err.Error())
		}
		return nil
	}
	return diagnostics
}

func (t *VFSInsertAfterLineTool) logComplete(path string, diagnostics []lsp.Diagnostic) {
	if t.logger == nil {
		return
	}
	if len(diagnostics) == 0 {
		t.logger.Info("vfsInsertAfterLine_complete", "path", path, "result", "success")
		return
	}
	t.logger.Info("vfsInsertAfterLine_complete", "path", path, "result", "success", "diagnostics_count", len(diagnostics))
	for _, diag := range diagnostics {
		if diag.Severity == lsp.SeverityError {
			t.logger.Info("vfsInsertAfterLine_diagnostic", "path", path, "line", diag.Range.Start.Line+1,
				"column", diag.Range.Start.Character+1, "message", diag.Message)
		}
	}
}

// vfsInsertAfterLineError creates an argument validation error response.
func vfsInsertAfterLineError(args *ToolCall, message string) *ToolResponse {
	return &ToolResponse{
		Call:  args,
		Error: fmt.Errorf("VFSInsertAfterLineTool.Execute() [vfs_insert_after_line.go]: %s", message),
		Done:  true,
	}
}

// insertAfterLine inserts content after a 1-based line number, or before line 1 when lineNumber is 0.
func insertAfterLine(content string, lineNumber int64, insertContent string) (string, error) {
	if lineNumber < 0 {
		return "", fmt.Errorf("insertAfterLine() [vfs_insert_after_line.go]: line_number must be greater than or equal to 0")
	}

	lines := splitLines(content)
	if lineNumber > int64(len(lines)) {
		return "", fmt.Errorf("insertAfterLine() [vfs_insert_after_line.go]: line_number %d is outside file with %d lines", lineNumber, len(lines))
	}

	insertIndex := int(lineNumber)
	var builder strings.Builder
	builder.WriteString(joinLines(lines[:insertIndex]))
	builder.WriteString(insertContent)
	builder.WriteString(joinLines(lines[insertIndex:]))
	return builder.String(), nil
}

// formatInsertAfterLineDiagnostics formats LSP diagnostics after inserting a line.
func formatInsertAfterLineDiagnostics(path string, diagnostics []lsp.Diagnostic) string {
	result := "LSP errors detected in this file, please fix:\n"
	result += fmt.Sprintf("<diagnostics file=\"%s\">\n", path)
	for _, diag := range diagnostics {
		if diag.Severity == lsp.SeverityError {
			line := diag.Range.Start.Line + 1
			col := diag.Range.Start.Character + 1
			result += fmt.Sprintf("Error[%d:%d] %s\n", line, col, diag.Message)
		}
	}
	result += "</diagnostics>"
	return result
}
