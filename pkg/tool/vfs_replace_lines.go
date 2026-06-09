package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSReplaceLinesTool implements the vfsReplaceLines tool.
type VFSReplaceLinesTool struct {
	vfs    apis.VFS
	lsp    lsp.LSP
	logger *slog.Logger
}

func (t *VFSReplaceLinesTool) GetDescription() (string, bool) {
	return "", false
}

// NewVFSReplaceLinesTool creates a new VFSReplaceLinesTool instance.
// lsp parameter is optional and can be nil.
func NewVFSReplaceLinesTool(v apis.VFS, l lsp.LSP) *VFSReplaceLinesTool {
	return &VFSReplaceLinesTool{vfs: v, lsp: l}
}

// SetLogger sets the logger for the tool.
func (t *VFSReplaceLinesTool) SetLogger(logger *slog.Logger) {
	t.logger = logger
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSReplaceLinesTool) Execute(args *ToolCall) *ToolResponse {
	path, ok := args.Arguments.StringOK("path")
	if !ok {
		return vfsReplaceLinesError(args, "missing required argument: path")
	}

	startLine, ok := args.Arguments.IntOK("start_line")
	if !ok {
		return vfsReplaceLinesError(args, "missing required argument: start_line")
	}

	endLine, ok := args.Arguments.IntOK("end_line")
	if !ok {
		return vfsReplaceLinesError(args, "missing required argument: end_line")
	}

	replacement, ok := args.Arguments.StringOK("replacement")
	if !ok {
		return vfsReplaceLinesError(args, "missing required argument: replacement")
	}

	expectedSHA256, _ := args.Arguments.StringOK("expected_sha256")

	if t.logger != nil {
		t.logger.Info("vfsReplaceLines_start", "path", path, "start_line", startLine, "end_line", endLine)
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
			Error: fmt.Errorf("VFSReplaceLinesTool.Execute() [vfs_replace_lines.go]: expected_sha256 mismatch: expected %s, got %s", expectedSHA256, actualSHA256),
			Done:  true,
		}
	}

	newContent, err := replaceLineRange(string(content), startLine, endLine, replacement)
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
	result.Set("content", "Lines replaced successfully")
	result.Set("sha256", sha256Hex([]byte(newContent)))
	if len(diagnostics) > 0 {
		result.Set("content", formatReplaceLinesDiagnostics(path, diagnostics))
	}

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *VFSReplaceLinesTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	path, _ := call.Arguments.StringOK("path")
	replacement, _ := call.Arguments.StringOK("replacement")
	startLine, _ := call.Arguments.IntOK("start_line")
	endLine, _ := call.Arguments.IntOK("end_line")
	relativePath := makeRelativePath(path, t.vfs)
	baseJSONL := buildToolRenderJSONL("vfsReplaceLines", call, map[string]any{"path": relativePath})

	linesRemoved := int64(0)
	if startLine > 0 && endLine >= startLine {
		linesRemoved = endLine - startLine + 1
	}
	linesAdded := int64(countLines(replacement))
	oneLiner := truncateString(fmt.Sprintf("replace lines %s:%d-%d (+%d/-%d)", relativePath, startLine, endLine, linesAdded, linesRemoved), 128)
	full := oneLiner + "\n\n"

	if errMsg, ok := call.Arguments.StringOK("error"); ok && errMsg != "" {
		errOneLiner, errFull := formatRenderError(errMsg)
		oneLiner = oneLiner + "\n" + errOneLiner
		full = full + errFull
		return oneLiner, full, baseJSONL, make(map[string]string)
	}

	full += fmt.Sprintf("--- %s:%d-%d\n", relativePath, startLine, endLine)
	full += fmt.Sprintf("+++ %s:%d-%d\n", relativePath, startLine, endLine)
	if replacement != "" {
		full += replacement
	}
	return oneLiner, full, baseJSONL, make(map[string]string)
}

func (t *VFSReplaceLinesTool) validateWithLSP(path string) []lsp.Diagnostic {
	if t.lsp == nil {
		if t.logger != nil {
			t.logger.Info("vfsReplaceLines_lsp_not_available", "path", path)
		}
		return nil
	}

	diagnostics, err := t.lsp.TouchAndValidate(path, true)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("vfsReplaceLines_lsp_validation_failed", "path", path, "error", err.Error())
		}
		return nil
	}
	return diagnostics
}

func (t *VFSReplaceLinesTool) logComplete(path string, diagnostics []lsp.Diagnostic) {
	if t.logger == nil {
		return
	}
	if len(diagnostics) == 0 {
		t.logger.Info("vfsReplaceLines_complete", "path", path, "result", "success")
		return
	}
	t.logger.Info("vfsReplaceLines_complete", "path", path, "result", "success", "diagnostics_count", len(diagnostics))
	for _, diag := range diagnostics {
		if diag.Severity == lsp.SeverityError {
			t.logger.Info("vfsReplaceLines_diagnostic", "path", path, "line", diag.Range.Start.Line+1,
				"column", diag.Range.Start.Character+1, "message", diag.Message)
		}
	}
}

// vfsReplaceLinesError creates an argument validation error response.
func vfsReplaceLinesError(args *ToolCall, message string) *ToolResponse {
	return &ToolResponse{
		Call:  args,
		Error: fmt.Errorf("VFSReplaceLinesTool.Execute() [vfs_replace_lines.go]: %s", message),
		Done:  true,
	}
}

// replaceLineRange replaces a 1-based inclusive line range in content.
func replaceLineRange(content string, startLine, endLine int64, replacement string) (string, error) {
	if startLine < 1 {
		return "", fmt.Errorf("replaceLineRange() [vfs_replace_lines.go]: start_line must be greater than 0")
	}
	if endLine < startLine {
		return "", fmt.Errorf("replaceLineRange() [vfs_replace_lines.go]: end_line must be greater than or equal to start_line")
	}

	lines := splitLines(content)
	if len(lines) == 0 {
		return "", fmt.Errorf("replaceLineRange() [vfs_replace_lines.go]: line range %d-%d is outside empty file", startLine, endLine)
	}
	if endLine > int64(len(lines)) {
		return "", fmt.Errorf("replaceLineRange() [vfs_replace_lines.go]: line range %d-%d is outside file with %d lines", startLine, endLine, len(lines))
	}

	startIndex := int(startLine - 1)
	endIndex := int(endLine)
	var builder strings.Builder
	builder.WriteString(joinLines(lines[:startIndex]))
	builder.WriteString(replacement)
	builder.WriteString(joinLines(lines[endIndex:]))
	return builder.String(), nil
}

// sha256Hex returns SHA-256 hex digest of content.
func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// formatReplaceLinesDiagnostics formats LSP diagnostics after replacing lines.
func formatReplaceLinesDiagnostics(path string, diagnostics []lsp.Diagnostic) string {
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
