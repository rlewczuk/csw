package vfs

import (
	"path/filepath"
	"strings"

	"github.com/codesnort/codesnort-swe/pkg/shared"
)

type FileOperation string

type FileAccess struct {
	Read   shared.AccessFlag `json:"read"`
	Write  shared.AccessFlag `json:"write"`
	Delete shared.AccessFlag `json:"delete"`
	List   shared.AccessFlag `json:"list"`
	Find   shared.AccessFlag `json:"find"`
	Move   shared.AccessFlag `json:"move"`
}

var denyAll = FileAccess{
	Read:   shared.AccessDeny,
	Write:  shared.AccessDeny,
	Delete: shared.AccessDeny,
	List:   shared.AccessDeny,
	Find:   shared.AccessDeny,
	Move:   shared.AccessDeny,
}

type AccessControlVFS struct {
	// underlying VFS implementation
	vfs VFS

	// map of glob patterns to FileAccess
	privileges map[string]FileAccess
}

// NewAccessControlVFS creates a new AccessControlVFS with the given underlying VFS and privileges map.
func NewAccessControlVFS(vfs VFS, privileges map[string]FileAccess) *AccessControlVFS {
	return &AccessControlVFS{
		vfs:        vfs,
		privileges: privileges,
	}
}

// checkAccess checks if the given operation is allowed for the given path.
func (ac *AccessControlVFS) checkAccess(path string, flag shared.AccessFlag) error {
	if flag == shared.AccessDeny {
		return ErrPermissionDenied
	}
	// AccessAllow and AccessAsk are both allowed for now
	// AccessAsk would require UI interaction which is not implemented yet
	return nil
}

// getAccess returns the FileAccess for the given path by finding the most specific matching glob pattern.
func (ac *AccessControlVFS) getAccess(path string) FileAccess {
	var bestMatch string
	var bestAccess FileAccess
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
	if err := ac.checkAccess(path, access.Read); err != nil {
		return nil, err
	}
	return ac.vfs.ReadFile(path)
}

// WriteFile writes the given content to the file located at the given path.
func (ac *AccessControlVFS) WriteFile(path string, content []byte) error {
	access := ac.getAccess(path)
	if err := ac.checkAccess(path, access.Write); err != nil {
		return err
	}
	return ac.vfs.WriteFile(path, content)
}

// DeleteFile deletes the file located at the given path.
func (ac *AccessControlVFS) DeleteFile(path string, recursive bool, force bool) error {
	access := ac.getAccess(path)
	if err := ac.checkAccess(path, access.Delete); err != nil {
		return err
	}
	return ac.vfs.DeleteFile(path, recursive, force)
}

// ListFiles lists all files and directories located at the given path.
func (ac *AccessControlVFS) ListFiles(path string, recursive bool) ([]string, error) {
	access := ac.getAccess(path)
	if err := ac.checkAccess(path, access.List); err != nil {
		return nil, err
	}
	return ac.vfs.ListFiles(path, recursive)
}

// FindFiles searches for files and directories matching the given query.
func (ac *AccessControlVFS) FindFiles(query string, recursive bool) ([]string, error) {
	access := ac.getAccess(query)
	if err := ac.checkAccess(query, access.Find); err != nil {
		return nil, err
	}
	return ac.vfs.FindFiles(query, recursive)
}

// MoveFile moves or renames a file or directory from src to dst.
func (ac *AccessControlVFS) MoveFile(src, dst string) error {
	// Check both source and destination
	accessSrc := ac.getAccess(src)
	if err := ac.checkAccess(src, accessSrc.Move); err != nil {
		return err
	}

	accessDst := ac.getAccess(dst)
	if err := ac.checkAccess(dst, accessDst.Write); err != nil {
		return err
	}

	return ac.vfs.MoveFile(src, dst)
}
