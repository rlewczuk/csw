package vfs

import (
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
)

// PermissionError is returned when an operation requires permission.
type PermissionError struct {
	Path      string
	Operation string
}

func (e *PermissionError) Error() string {
	return "permission required for " + e.Operation + " on " + e.Path
}

// Is implements errors.Is interface to allow checking if PermissionError is ErrAskPermission.
func (e *PermissionError) Is(target error) bool {
	return target == apis.ErrAskPermission
}

type FileOperation string

var denyAll = conf.FileAccess{
	Read:   conf.AccessDeny,
	Write:  conf.AccessDeny,
	Delete: conf.AccessDeny,
	List:   conf.AccessDeny,
	Find:   conf.AccessDeny,
	Move:   conf.AccessDeny,
}

type AccessControlVFS struct {
	// underlying VFS implementation
	vfs apis.VFS

	// map of glob patterns to FileAccess
	privileges map[string]conf.FileAccess
}

func (ac *AccessControlVFS) GetBranch() string {
	return ac.vfs.GetBranch()
}

func (ac *AccessControlVFS) WorktreePath() string {
	return ac.vfs.WorktreePath()
}

func (ac *AccessControlVFS) GetRepo() apis.VCS {
	return ac.vfs.GetRepo()
}

// NewAccessControlVFS creates a new AccessControlVFS with the given underlying VFS and privileges map.
func NewAccessControlVFS(vfs apis.VFS, privileges map[string]conf.FileAccess) *AccessControlVFS {
	return &AccessControlVFS{
		vfs:        vfs,
		privileges: privileges,
	}
}

// checkAccess checks if the given operation is allowed for the given path.
func (ac *AccessControlVFS) checkAccess(path string, op string, flag conf.AccessFlag) error {
	if flag == conf.AccessDeny {
		return apis.ErrPermissionDenied
	}
	if flag == conf.AccessAsk {
		return &PermissionError{Path: path, Operation: op}
	}
	return nil
}

// getAccess returns the FileAccess for the given path by finding the most specific matching glob pattern.
func (ac *AccessControlVFS) getAccess(path string) conf.FileAccess {
	var bestMatch string
	var bestAccess conf.FileAccess
	found := false

	for pattern, access := range ac.privileges {
		if matchPattern(pattern, path) {
			// Find the most specific pattern
			if !found || isMoreSpecific(pattern, bestMatch) {
				bestMatch = pattern
				bestAccess = access
				found = true
			}
		}
	}

	if !found {
		// No match found, return deny all
		return denyAll
	}

	return bestAccess
}

// matchPattern checks if a pattern matches a path.
// It handles both simple wildcards and path patterns.
func matchPattern(pattern, path string) bool {
	// Normalize paths to use forward slashes
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// If pattern has no wildcards and no slashes, use filepath.Match for simple matching
	if !strings.ContainsAny(pattern, "*?[") {
		// Exact match
		return pattern == path
	}

	// If pattern has slashes, match the entire path
	if strings.Contains(pattern, "/") {
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	// Pattern has no slashes, try matching against the full path and just the filename
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Also try matching just the base name
	baseName := filepath.Base(path)
	matched, _ = filepath.Match(pattern, baseName)
	return matched
}

// isMoreSpecific returns true if pattern1 is more specific than pattern2.
// A pattern is more specific if it has:
// 1. Fewer wildcards
// 2. More directory levels
// 3. Longer literal prefix
func isMoreSpecific(pattern1, pattern2 string) bool {
	// Count wildcards
	wildcards1 := strings.Count(pattern1, "*") + strings.Count(pattern1, "?") + strings.Count(pattern1, "[")
	wildcards2 := strings.Count(pattern2, "*") + strings.Count(pattern2, "?") + strings.Count(pattern2, "[")

	if wildcards1 != wildcards2 {
		return wildcards1 < wildcards2
	}

	// Count directory levels (slashes)
	levels1 := strings.Count(pattern1, "/")
	levels2 := strings.Count(pattern2, "/")

	if levels1 != levels2 {
		return levels1 > levels2
	}

	// Compare length (longer is more specific)
	return len(pattern1) > len(pattern2)
}

// ReadFile reads the content of the file located at the given path and returns its data as a byte slice.
func (ac *AccessControlVFS) ReadFile(path string) ([]byte, error) {
	access := ac.getAccess(path)
	if err := ac.checkAccess(path, "read", access.Read); err != nil {
		return nil, err
	}
	return ac.vfs.ReadFile(path)
}

// WriteFile writes the given content to the file located at the given path.
func (ac *AccessControlVFS) WriteFile(path string, content []byte) error {
	access := ac.getAccess(path)
	if err := ac.checkAccess(path, "write", access.Write); err != nil {
		return err
	}
	return ac.vfs.WriteFile(path, content)
}

// DeleteFile deletes the file located at the given path.
func (ac *AccessControlVFS) DeleteFile(path string, recursive bool, force bool) error {
	access := ac.getAccess(path)
	if err := ac.checkAccess(path, "delete", access.Delete); err != nil {
		return err
	}
	return ac.vfs.DeleteFile(path, recursive, force)
}

// ListFiles lists all files and directories located at the given path.
func (ac *AccessControlVFS) ListFiles(path string, recursive bool) ([]string, error) {
	access := ac.getAccess(path)
	if err := ac.checkAccess(path, "list", access.List); err != nil {
		return nil, err
	}
	return ac.vfs.ListFiles(path, recursive)
}

// FindFiles searches for files and directories matching the given query.
func (ac *AccessControlVFS) FindFiles(query string, recursive bool) ([]string, error) {
	access := ac.getAccess(query)
	if err := ac.checkAccess(query, "find", access.Find); err != nil {
		return nil, err
	}
	return ac.vfs.FindFiles(query, recursive)
}

// MoveFile moves or renames a file or directory from src to dst.
func (ac *AccessControlVFS) MoveFile(src, dst string) error {
	// Check both source and destination
	accessSrc := ac.getAccess(src)
	if err := ac.checkAccess(src, "move", accessSrc.Move); err != nil {
		return err
	}

	accessDst := ac.getAccess(dst)
	if err := ac.checkAccess(dst, "write", accessDst.Write); err != nil {
		return err
	}

	return ac.vfs.MoveFile(src, dst)
}

// SetPermission sets the permission for a specific path and operation.
func (ac *AccessControlVFS) SetPermission(path string, op string, flag conf.AccessFlag) {
	// Get current effective access
	access := ac.getAccess(path)

	// Update specific operation
	switch op {
	case "read":
		access.Read = flag
	case "write":
		access.Write = flag
	case "delete":
		access.Delete = flag
	case "list":
		access.List = flag
	case "find":
		access.Find = flag
	case "move":
		access.Move = flag
	}

	// Store back in privileges map
	if ac.privileges == nil {
		ac.privileges = make(map[string]conf.FileAccess)
	}
	ac.privileges[path] = access
}
