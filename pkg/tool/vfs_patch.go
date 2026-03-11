package tool

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode"

	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/vfs"
)

const maxVFSPatchDiagnosticsPerFile = 20

// VFSPatchTool implements the vfsPatch tool.
type VFSPatchTool struct {
	vfs    vfs.VFS
	lsp    lsp.LSP
	logger *slog.Logger
}

func (t *VFSPatchTool) GetDescription() (string, bool) {
	return "", false
}

// NewVFSPatchTool creates a new VFSPatchTool instance.
// lsp parameter is optional and can be nil.
func NewVFSPatchTool(v vfs.VFS, l lsp.LSP) *VFSPatchTool {
	return &VFSPatchTool{vfs: v, lsp: l}
}

// SetLogger sets the logger for the tool.
func (t *VFSPatchTool) SetLogger(logger *slog.Logger) {
	t.logger = logger
}

// Execute executes the tool with the given arguments and returns the response.
func (t *VFSPatchTool) Execute(args *ToolCall) *ToolResponse {
	patchText, ok := args.Arguments.StringOK("patchText")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("VFSPatchTool.Execute() [vfs_patch.go]: missing required argument: patchText"),
			Done:  true,
		}
	}

	parsed, err := shared.ParsePatch(patchText)
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("apply_patch verification failed: %w", err),
			Done:  true,
		}
	}

	if len(parsed.Hunks) == 0 {
		normalized := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(patchText), "\r\n", "\n"), "\r", "\n")
		if normalized == "*** Begin Patch\n*** End Patch" {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("patch rejected: empty patch"),
				Done:  true,
			}
		}
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("apply_patch verification failed: no hunks found"),
			Done:  true,
		}
	}

	type fileChange struct {
		path     string
		movePath string
		oldBody  string
		newBody  string
		typeName string
	}

	changes := make([]fileChange, 0, len(parsed.Hunks))
	for _, h := range parsed.Hunks {
		switch hunk := h.(type) {
		case shared.AddFile:
			newBody := hunk.Contents
			if len(hunk.Contents) != 0 && !strings.HasSuffix(hunk.Contents, "\n") {
				newBody += "\n"
			}
			changes = append(changes, fileChange{
				path:     hunk.Path,
				oldBody:  "",
				newBody:  newBody,
				typeName: "add",
			})
		case shared.UpdateFile:
			body, readErr := t.vfs.ReadFile(hunk.Path)
			if readErr != nil {
				if readErr == vfs.ErrAskPermission {
					return NewVFSPermissionQuery(args, hunk.Path, "reading file", "read")
				}
				if perr, ok := readErr.(*vfs.PermissionError); ok {
					action := vfsActionFromOperation(perr.Operation)
					op := perr.Operation
					if op == "" {
						op = "read"
						action = "reading file"
					}
					return NewVFSPermissionQuery(args, perr.Path, action, op)
				}
				return &ToolResponse{
					Call:  args,
					Error: fmt.Errorf("apply_patch verification failed: Failed to read file to update: %s", hunk.Path),
					Done:  true,
				}
			}

			newBody, deriveErr := deriveUpdatedContent(string(body), hunk.Path, hunk.Chunks)
			if deriveErr != nil {
				return &ToolResponse{
					Call:  args,
					Error: fmt.Errorf("apply_patch verification failed: %w", deriveErr),
					Done:  true,
				}
			}

			changes = append(changes, fileChange{
				path:     hunk.Path,
				movePath: hunk.MovePath,
				oldBody:  string(body),
				newBody:  newBody,
				typeName: map[bool]string{true: "move", false: "update"}[hunk.MovePath != ""],
			})
		case shared.DeleteFile:
			body, readErr := t.vfs.ReadFile(hunk.Path)
			if readErr != nil {
				if readErr == vfs.ErrAskPermission {
					return NewVFSPermissionQuery(args, hunk.Path, "reading file", "read")
				}
				if perr, ok := readErr.(*vfs.PermissionError); ok {
					action := vfsActionFromOperation(perr.Operation)
					op := perr.Operation
					if op == "" {
						op = "read"
						action = "reading file"
					}
					return NewVFSPermissionQuery(args, perr.Path, action, op)
				}
				return &ToolResponse{
					Call:  args,
					Error: fmt.Errorf("apply_patch verification failed: %w", readErr),
					Done:  true,
				}
			}
			changes = append(changes, fileChange{
				path:     hunk.Path,
				oldBody:  string(body),
				newBody:  "",
				typeName: "delete",
			})
		}
	}

	for _, change := range changes {
		switch change.typeName {
		case "add", "update":
			if writeErr := t.vfs.WriteFile(change.path, []byte(change.newBody)); writeErr != nil {
				if writeErr == vfs.ErrAskPermission {
					return NewVFSPermissionQuery(args, change.path, "writing to file", "write")
				}
				if perr, ok := writeErr.(*vfs.PermissionError); ok {
					action := vfsActionFromOperation(perr.Operation)
					op := perr.Operation
					if op == "" {
						op = "write"
						action = "writing to file"
					}
					return NewVFSPermissionQuery(args, perr.Path, action, op)
				}
				return &ToolResponse{Call: args, Error: writeErr, Done: true}
			}
		case "move":
			target := change.movePath
			if writeErr := t.vfs.WriteFile(target, []byte(change.newBody)); writeErr != nil {
				if writeErr == vfs.ErrAskPermission {
					return NewVFSPermissionQuery(args, target, "writing to file", "write")
				}
				if perr, ok := writeErr.(*vfs.PermissionError); ok {
					action := vfsActionFromOperation(perr.Operation)
					op := perr.Operation
					if op == "" {
						op = "write"
						action = "writing to file"
					}
					return NewVFSPermissionQuery(args, perr.Path, action, op)
				}
				return &ToolResponse{Call: args, Error: writeErr, Done: true}
			}
			if delErr := t.vfs.DeleteFile(change.path, false, false); delErr != nil {
				if delErr == vfs.ErrAskPermission {
					return NewVFSPermissionQuery(args, change.path, "deleting file", "delete")
				}
				if perr, ok := delErr.(*vfs.PermissionError); ok {
					action := vfsActionFromOperation(perr.Operation)
					op := perr.Operation
					if op == "" {
						op = "delete"
						action = "deleting file"
					}
					return NewVFSPermissionQuery(args, perr.Path, action, op)
				}
				return &ToolResponse{Call: args, Error: delErr, Done: true}
			}
		case "delete":
			if delErr := t.vfs.DeleteFile(change.path, false, false); delErr != nil {
				if delErr == vfs.ErrAskPermission {
					return NewVFSPermissionQuery(args, change.path, "deleting file", "delete")
				}
				if perr, ok := delErr.(*vfs.PermissionError); ok {
					action := vfsActionFromOperation(perr.Operation)
					op := perr.Operation
					if op == "" {
						op = "delete"
						action = "deleting file"
					}
					return NewVFSPermissionQuery(args, perr.Path, action, op)
				}
				return &ToolResponse{Call: args, Error: delErr, Done: true}
			}
		}
	}

	summaryLines := make([]string, 0, len(changes))
	for _, change := range changes {
		switch change.typeName {
		case "add":
			summaryLines = append(summaryLines, "A "+change.path)
		case "delete":
			summaryLines = append(summaryLines, "D "+change.path)
		default:
			target := change.path
			if change.movePath != "" {
				target = change.movePath
			}
			summaryLines = append(summaryLines, "M "+target)
		}
	}

	output := "Success. Updated the following files:\n" + strings.Join(summaryLines, "\n")
	if t.lsp != nil {
		for _, change := range changes {
			if change.typeName == "delete" {
				continue
			}
			target := change.path
			if change.movePath != "" {
				target = change.movePath
			}
			fileDiags, lspErr := t.lsp.TouchAndValidate(target, true)
			if lspErr != nil {
				if t.logger != nil {
					t.logger.Warn("vfsPatch_lsp_validation_failed", "path", target, "error", lspErr.Error())
				}
				continue
			}
			errorsOnly := make([]lsp.Diagnostic, 0)
			for _, diag := range fileDiags {
				if diag.Severity == lsp.SeverityError {
					errorsOnly = append(errorsOnly, diag)
				}
			}
			if len(errorsOnly) == 0 {
				continue
			}
			limit := len(errorsOnly)
			if limit > maxVFSPatchDiagnosticsPerFile {
				limit = maxVFSPatchDiagnosticsPerFile
			}
			entries := make([]string, 0, limit)
			for _, diag := range errorsOnly[:limit] {
				line := diag.Range.Start.Line + 1
				col := diag.Range.Start.Character + 1
				entries = append(entries, fmt.Sprintf("Error[%d:%d] %s", line, col, diag.Message))
			}
			suffix := ""
			if len(errorsOnly) > maxVFSPatchDiagnosticsPerFile {
				suffix = fmt.Sprintf("\n... and %d more", len(errorsOnly)-maxVFSPatchDiagnosticsPerFile)
			}
			output += fmt.Sprintf("\n\nLSP errors detected in %s, please fix:\n<diagnostics file=\"%s\">\n%s%s\n</diagnostics>", target, target, strings.Join(entries, "\n"), suffix)
		}
	}

	var result ToolValue
	result.Set("content", output)
	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *VFSPatchTool) Render(call *ToolCall) (string, string, map[string]string) {
	patchText, _ := call.Arguments.StringOK("patchText")

	oneLiner := "apply patch"
	if patchText != "" {
		parsed, err := shared.ParsePatch(patchText)
		if err == nil && len(parsed.Hunks) > 0 {
			oneLiner = renderPatchOneLiner(parsed, t.vfs)
		}
	}
	oneLiner = truncateString(oneLiner, 128)

	full := oneLiner

	// Check for error in arguments
	if errMsg, ok := call.Arguments.StringOK("error"); ok && errMsg != "" {
		errOneLiner, errFull := formatRenderError(errMsg)
		// Add error as second line to oneLiner
		oneLiner = oneLiner + "\n" + errOneLiner
		// Add error to full output
		full = full + "\n\n" + errFull
		return oneLiner, full, make(map[string]string)
	}

	if patchText != "" {
		full += "\n\n" + patchText
	}
	return oneLiner, full, make(map[string]string)
}

// renderPatchOneLiner generates a one-line summary of patch operations.
func renderPatchOneLiner(parsed *shared.Patch, v vfs.VFS) string {
	parts := make([]string, 0, len(parsed.Hunks))
	for _, h := range parsed.Hunks {
		switch hunk := h.(type) {
		case shared.AddFile:
			linesAdded := countLines(hunk.Contents)
			path := makeRelativePath(hunk.Path, v)
			parts = append(parts, fmt.Sprintf("A:%s(+%d)", path, linesAdded))
		case shared.DeleteFile:
			path := makeRelativePath(hunk.Path, v)
			parts = append(parts, fmt.Sprintf("D:%s", path))
		case shared.UpdateFile:
			if hunk.MovePath != "" {
				added, removed := countUpdateLines(hunk.Chunks)
				target := makeRelativePath(hunk.MovePath, v)
				parts = append(parts, fmt.Sprintf("M:%s(+%d/-%d)", target, added, removed))
			} else {
				added, removed := countUpdateLines(hunk.Chunks)
				path := makeRelativePath(hunk.Path, v)
				parts = append(parts, fmt.Sprintf("U:%s(+%d/-%d)", path, added, removed))
			}
		}
	}
	return "apply patch: " + strings.Join(parts, " ")
}

// countLines counts the number of lines in a string.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// countUpdateLines counts added and removed lines from update chunks.
func countUpdateLines(chunks []shared.UpdateFileChunk) (added, removed int) {
	for _, chunk := range chunks {
		for _, line := range chunk.NewLines {
			if line != "" {
				added++
			}
		}
		for _, line := range chunk.OldLines {
			if line != "" {
				removed++
			}
		}
	}
	return added, removed
}

type vfsReplacement struct {
	start  int
	length int
	lines  []string
}

func deriveUpdatedContent(originalContent string, filePath string, chunks []shared.UpdateFileChunk) (string, error) {
	originalLines := strings.Split(originalContent, "\n")
	if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
		originalLines = originalLines[:len(originalLines)-1]
	}

	replacements, err := computeReplacements(originalLines, filePath, chunks)
	if err != nil {
		return "", err
	}

	newLines := applyReplacements(originalLines, replacements)
	if len(newLines) == 0 || newLines[len(newLines)-1] != "" {
		newLines = append(newLines, "")
	}
	return strings.Join(newLines, "\n"), nil
}

func computeReplacements(originalLines []string, filePath string, chunks []shared.UpdateFileChunk) ([]vfsReplacement, error) {
	replacements := make([]vfsReplacement, 0, len(chunks))
	lineIndex := 0

	for _, chunk := range chunks {
		if chunk.ChangeContext != "" {
			contextIdx := seekSequence(originalLines, []string{chunk.ChangeContext}, lineIndex, false)
			if contextIdx == -1 {
				return nil, fmt.Errorf("Failed to find context '%s' in %s", chunk.ChangeContext, filePath)
			}
			lineIndex = contextIdx + 1
		}

		if len(chunk.OldLines) == 0 {
			insertionIdx := len(originalLines)
			if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
				insertionIdx = len(originalLines) - 1
			}
			replacements = append(replacements, vfsReplacement{start: insertionIdx, length: 0, lines: chunk.NewLines})
			continue
		}

		pattern := chunk.OldLines
		newSlice := chunk.NewLines
		found := seekSequence(originalLines, pattern, lineIndex, chunk.IsEndOfFile)
		if found == -1 && len(pattern) > 0 && pattern[len(pattern)-1] == "" {
			pattern = pattern[:len(pattern)-1]
			if len(newSlice) > 0 && newSlice[len(newSlice)-1] == "" {
				newSlice = newSlice[:len(newSlice)-1]
			}
			found = seekSequence(originalLines, pattern, lineIndex, chunk.IsEndOfFile)
		}

		if found == -1 {
			return nil, fmt.Errorf("Failed to find expected lines in %s:\n%s", filePath, strings.Join(chunk.OldLines, "\n"))
		}

		replacements = append(replacements, vfsReplacement{start: found, length: len(pattern), lines: newSlice})
		lineIndex = found + len(pattern)
	}

	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start < replacements[j].start
	})

	return replacements, nil
}

func applyReplacements(lines []string, replacements []vfsReplacement) []string {
	result := append([]string(nil), lines...)
	for i := len(replacements) - 1; i >= 0; i-- {
		repl := replacements[i]
		result = append(result[:repl.start], result[repl.start+repl.length:]...)
		segment := append([]string(nil), repl.lines...)
		result = append(result[:repl.start], append(segment, result[repl.start:]...)...)
	}
	return result
}

func seekSequence(lines []string, pattern []string, startIndex int, eof bool) int {
	if len(pattern) == 0 {
		return -1
	}

	if idx := tryMatch(lines, pattern, startIndex, func(a, b string) bool { return a == b }, eof); idx != -1 {
		return idx
	}
	if idx := tryMatch(lines, pattern, startIndex, func(a, b string) bool { return trimRightSpace(a) == trimRightSpace(b) }, eof); idx != -1 {
		return idx
	}
	if idx := tryMatch(lines, pattern, startIndex, func(a, b string) bool { return strings.TrimSpace(a) == strings.TrimSpace(b) }, eof); idx != -1 {
		return idx
	}

	return tryMatch(lines, pattern, startIndex, func(a, b string) bool {
		return normalizeUnicode(strings.TrimSpace(a)) == normalizeUnicode(strings.TrimSpace(b))
	}, eof)
}

func tryMatch(lines []string, pattern []string, startIndex int, compare func(a, b string) bool, eof bool) int {
	if eof {
		fromEnd := len(lines) - len(pattern)
		if fromEnd >= startIndex {
			matches := true
			for i := range pattern {
				if !compare(lines[fromEnd+i], pattern[i]) {
					matches = false
					break
				}
			}
			if matches {
				return fromEnd
			}
		}
	}

	for i := startIndex; i <= len(lines)-len(pattern); i++ {
		matches := true
		for j := range pattern {
			if !compare(lines[i+j], pattern[j]) {
				matches = false
				break
			}
		}
		if matches {
			return i
		}
	}

	return -1
}

func trimRightSpace(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

func normalizeUnicode(s string) string {
	replacer := strings.NewReplacer(
		"\u2018", "'",
		"\u2019", "'",
		"\u201A", "'",
		"\u201B", "'",
		"\u201C", "\"",
		"\u201D", "\"",
		"\u201E", "\"",
		"\u201F", "\"",
		"\u2010", "-",
		"\u2011", "-",
		"\u2012", "-",
		"\u2013", "-",
		"\u2014", "-",
		"\u2015", "-",
		"\u2026", "...",
		"\u00A0", " ",
	)
	return replacer.Replace(s)
}
