package tool

import (
	"path/filepath"

	"github.com/rlewczuk/csw/pkg/vfs"
)

// VFSFindTool implements the vfsFind tool.
type VFSFindTool struct {
	vfs vfs.VFS
}

func (t *VFSFindTool) GetDescription() (string, bool) {
	return "", false
}

// NewVFSFindTool creates a new VFSFindTool instance.
func NewVFSFindTool(v vfs.VFS) *VFSFindTool {
	return &VFSFindTool{vfs: v}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSFindTool) Execute(args *ToolCall) *ToolResponse {
	path := args.Arguments.String("path")

	// Get query parameter, empty string means match all files (use "**")
	query := args.Arguments.String("query")
	if query == "" {
		query = "**"
	}

	// Get recursive flag, default to true if not provided
	recursive := true
	if args.Arguments.Has("recursive") {
		recursive = args.Arguments.Bool("recursive")
	}

	var (
		files []string
		err   error
	)

	if filepath.IsAbs(path) {
		files, err = t.findFilesInPath(path, query, recursive)
		if err == vfs.ErrAskPermission {
			return NewVFSPermissionQuery(args, path, "finding files", "find")
		}
	} else {
		files, err = t.vfs.FindFiles(query, recursive)
		if err == vfs.ErrAskPermission {
			return NewVFSPermissionQuery(args, query, "finding files", "find")
		}
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

// findFilesInPath finds files under an absolute path and filters them by query.
func (t *VFSFindTool) findFilesInPath(path, query string, recursive bool) ([]string, error) {
	listedFiles, err := t.vfs.ListFiles(path, recursive)
	if err != nil {
		return nil, err
	}

	filter := vfs.NewGlobFilter(false, []string{query})
	filtered := make([]string, 0, len(listedFiles))
	for _, listedFile := range listedFiles {
		absoluteFile := normalizeAbsoluteFindResult(t.vfs.WorktreePath(), listedFile)
		relativeToSearchPath, relErr := filepath.Rel(path, absoluteFile)
		if relErr != nil {
			continue
		}
		relativeToSearchPath = filepath.Clean(relativeToSearchPath)

		if filter.Matches(relativeToSearchPath) || filter.Matches(filepath.Base(relativeToSearchPath)) {
			filtered = append(filtered, absoluteFile)
		}
	}

	return filtered, nil
}

// normalizeAbsoluteFindResult converts list results to absolute host paths.
func normalizeAbsoluteFindResult(worktreeRoot, listedPath string) string {
	if filepath.IsAbs(listedPath) {
		return filepath.Clean(listedPath)
	}
	if worktreeRoot == "" {
		return filepath.Clean(listedPath)
	}
	return filepath.Clean(filepath.Join(worktreeRoot, listedPath))
}

// Render returns a string representation of the tool call.
func (t *VFSFindTool) Render(call *ToolCall) (string, string, map[string]string) {
	query, _ := call.Arguments.StringOK("query")

	// Count results from files if available
	resultCount := 0
	var files []ToolValue
	if filesArr, ok := call.Arguments.Get("files").ArrayOK(); ok {
		files = filesArr
		resultCount = len(filesArr)
	}

	// Build result count suffix
	resultSuffix := ""
	if resultCount > 0 {
		resultSuffix = formatResultCount(resultCount)
	}

	baseText := truncateString("find "+query, 128)

	oneLiner := baseText + resultSuffix
	full := baseText + resultSuffix + "\n\n"

	// Add files to full output
	for _, f := range files {
		full += f.AsString() + "\n"
	}

	// Handle error if present
	if errMsg, ok := call.Arguments.Get("error").AsStringOK(); ok && errMsg != "" {
		errOneLiner, errFull := formatRenderError(errMsg)
		oneLiner += "\n" + errOneLiner
		full += "\n\n" + errFull
	}

	return oneLiner, full, make(map[string]string)
}
