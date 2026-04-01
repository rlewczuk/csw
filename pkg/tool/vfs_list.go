package tool

import (
	"fmt"
	"path/filepath"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSListTool implements the vfsList tool.
type VFSListTool struct {
	vfs apis.VFS
}

func (t *VFSListTool) GetDescription() (string, bool) {
	return "", false
}

// NewVFSListTool creates a new VFSListTool instance.
func NewVFSListTool(v apis.VFS) *VFSListTool {
	return &VFSListTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSListTool) Execute(args *ToolCall) *ToolResponse {
	path := args.Arguments.String("path")
	if path == "" {
		path = "."
	}

	recursive := false
	if args.Arguments.Has("recursive") {
		recursive = args.Arguments.Bool("recursive")
	}

	pattern := args.Arguments.String("pattern")
	if pattern == "" {
		pattern = "*"
	}

	limit := int64(0)
	if args.Arguments.Has("limit") {
		if l, ok := args.Arguments.IntOK("limit"); ok {
			limit = l
		}
	}
	if limit < 0 {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSListTool.Execute() [vfs_list.go]: limit must be >= 0"),
			Done:  true,
		}
	}

	files, err := t.vfs.ListFiles(path, recursive)
	if err == apis.ErrAskPermission {
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

	filter := vfs.NewGlobFilter(false, []string{pattern})
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if filter.Matches(file) || filter.Matches(filepath.Base(file)) {
			if filepath.IsAbs(path) {
				file = normalizeAbsoluteListResult(t.vfs.WorktreePath(), file)
			}
			filtered = append(filtered, file)
		}
	}

	resultSuffix := ""
	if len(filtered) > tooManyResultsCap {
		filtered = filtered[:tooManyResultsLimit]
		resultSuffix = tooManyResultsSuffix
	}

	truncated := false
	if limit > 0 && int64(len(filtered)) > limit {
		filtered = filtered[:limit]
		truncated = true
	}

	filesArray := make([]any, len(filtered))
	for i, file := range filtered {
		filesArray[i] = file
	}

	var result ToolValue
	result.Set("files", filesArray)
	result.Set("truncated", truncated)
	if resultSuffix != "" {
		result.Set("suffix", resultSuffix)
	}

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// normalizeAbsoluteListResult converts list results to absolute host paths.
func normalizeAbsoluteListResult(worktreeRoot, listedPath string) string {
	if filepath.IsAbs(listedPath) {
		return filepath.Clean(listedPath)
	}
	if worktreeRoot == "" {
		return filepath.Clean(listedPath)
	}
	return filepath.Clean(filepath.Join(worktreeRoot, listedPath))
}

// Render returns a string representation of the tool call.
func (t *VFSListTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	path := call.Arguments.String("path")
	if path == "" {
		path = "."
	}
	pattern := call.Arguments.String("pattern")

	resultCount := 0
	var files []ToolValue
	if filesArr, ok := call.Arguments.Get("files").ArrayOK(); ok {
		files = filesArr
		resultCount = len(filesArr)
	}

	baseText := "list " + path
	if pattern != "" {
		baseText += " matching " + pattern
	}
	baseText = truncateString(baseText, 128)

	oneLiner := baseText + formatResultCount(resultCount)
	full := oneLiner + "\n\n"
	jsonl := buildToolRenderJSONL("vfsList", call, map[string]any{"path": path, "pattern": pattern, "count": resultCount})

	for _, file := range files {
		full += file.AsString() + "\n"
	}

	if suffix, ok := call.Arguments.Get("suffix").AsStringOK(); ok && suffix != "" {
		if len(files) > 0 {
			full += "\n"
		}
		full += suffix
	}

	if call.Arguments.Bool("truncated") {
		full += "\n(Results are truncated. Consider using a more specific path or pattern.)"
	}

	if errMsg, ok := call.Arguments.Get("error").AsStringOK(); ok && errMsg != "" {
		errOneLiner, errFull := formatRenderError(errMsg)
		oneLiner += "\n" + errOneLiner
		full += "\n\n" + errFull
	}

	return oneLiner, full, jsonl, make(map[string]string)
}
