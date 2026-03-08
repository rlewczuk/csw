package vfs

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ShadowVFS routes configured paths to a shadow filesystem while keeping all other paths on base VFS.
type ShadowVFS struct {
	base      VFS
	shadow    VFS
	baseRoot  string
	shadowRoot string
	filter    GlobFilter
}

// NewShadowVFS creates a VFS wrapper that redirects matching paths to shadow VFS.
func NewShadowVFS(base VFS, shadow VFS, shadowPatterns []string) (*ShadowVFS, error) {
	if base == nil {
		return nil, fmt.Errorf("NewShadowVFS() [shadow.go]: base VFS cannot be nil")
	}
	if shadow == nil {
		return nil, fmt.Errorf("NewShadowVFS() [shadow.go]: shadow VFS cannot be nil")
	}

	baseRoot := filepath.Clean(base.WorktreePath())
	shadowRoot := filepath.Clean(shadow.WorktreePath())

	return &ShadowVFS{
		base:       base,
		shadow:     shadow,
		baseRoot:   baseRoot,
		shadowRoot: shadowRoot,
		filter:     NewGlobFilter(false, shadowPatterns),
	}, nil
}

// ReadFile reads a file from shadow VFS when path matches shadow patterns.
func (s *ShadowVFS) ReadFile(path string) ([]byte, error) {
	if s.isShadowed(path) {
		return s.shadow.ReadFile(s.pathForShadow(path))
	}
	return s.base.ReadFile(s.pathForBase(path))
}

// WriteFile writes a file to shadow VFS when path matches shadow patterns.
func (s *ShadowVFS) WriteFile(path string, content []byte) error {
	if s.isShadowed(path) {
		return s.shadow.WriteFile(s.pathForShadow(path), content)
	}
	return s.base.WriteFile(s.pathForBase(path), content)
}

// DeleteFile deletes from shadow VFS when path matches shadow patterns.
func (s *ShadowVFS) DeleteFile(path string, recursive bool, force bool) error {
	if s.isShadowed(path) {
		return s.shadow.DeleteFile(s.pathForShadow(path), recursive, force)
	}
	return s.base.DeleteFile(s.pathForBase(path), recursive, force)
}

// ListFiles returns merged listing with shadow paths overriding base paths for matched globs.
func (s *ShadowVFS) ListFiles(path string, recursive bool) ([]string, error) {
	if s.isShadowed(path) {
		return s.shadow.ListFiles(s.pathForShadow(path), recursive)
	}

	baseFiles, baseErr := s.base.ListFiles(s.pathForBase(path), recursive)
	shadowFiles, shadowErr := s.shadow.ListFiles(s.pathForShadow(path), recursive)

	if baseErr != nil && shadowErr != nil {
		return nil, baseErr
	}

	result := make([]string, 0, len(baseFiles)+len(shadowFiles))
	seen := make(map[string]struct{}, len(baseFiles)+len(shadowFiles))

	if baseErr == nil {
		for _, file := range baseFiles {
			if s.isShadowed(file) {
				continue
			}
			result = append(result, file)
			seen[file] = struct{}{}
		}
	}

	if shadowErr == nil {
		for _, file := range shadowFiles {
			if !s.isShadowed(file) {
				continue
			}
			if _, ok := seen[file]; ok {
				continue
			}
			result = append(result, file)
			seen[file] = struct{}{}
		}
	}

	return result, nil
}

// FindFiles returns merged find results with shadow paths overriding base paths.
func (s *ShadowVFS) FindFiles(query string, recursive bool) ([]string, error) {
	baseFiles, baseErr := s.base.FindFiles(query, recursive)
	shadowFiles, shadowErr := s.shadow.FindFiles(query, recursive)

	if baseErr != nil && shadowErr != nil {
		return nil, baseErr
	}

	result := make([]string, 0, len(baseFiles)+len(shadowFiles))
	seen := make(map[string]struct{}, len(baseFiles)+len(shadowFiles))

	if baseErr == nil {
		for _, file := range baseFiles {
			if s.isShadowed(file) {
				continue
			}
			result = append(result, file)
			seen[file] = struct{}{}
		}
	}

	if shadowErr == nil {
		for _, file := range shadowFiles {
			if !s.isShadowed(file) {
				continue
			}
			if _, ok := seen[file]; ok {
				continue
			}
			result = append(result, file)
			seen[file] = struct{}{}
		}
	}

	return result, nil
}

// MoveFile moves file within base or shadow VFS depending on source/destination patterns.
func (s *ShadowVFS) MoveFile(src, dst string) error {
	srcShadowed := s.isShadowed(src)
	dstShadowed := s.isShadowed(dst)

	if srcShadowed != dstShadowed {
		return fmt.Errorf("ShadowVFS.MoveFile() [shadow.go]: %w", ErrPermissionDenied)
	}

	if srcShadowed {
		return s.shadow.MoveFile(s.pathForShadow(src), s.pathForShadow(dst))
	}

	return s.base.MoveFile(s.pathForBase(src), s.pathForBase(dst))
}

// GetRepo returns repository from base VFS.
func (s *ShadowVFS) GetRepo() VCS {
	return s.base.GetRepo()
}

// GetBranch returns branch/worktree name from base VFS.
func (s *ShadowVFS) GetBranch() string {
	return s.base.GetBranch()
}

// WorktreePath returns base worktree path.
func (s *ShadowVFS) WorktreePath() string {
	return s.base.WorktreePath()
}

func (s *ShadowVFS) isShadowed(path string) bool {
	rel, ok := s.relativePath(path)
	if !ok {
		return false
	}
	return s.filter.Matches(rel)
}

func (s *ShadowVFS) relativePath(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}

	if filepath.IsAbs(path) {
		clean := filepath.Clean(path)
		if rel, ok := relativeIfWithin(s.baseRoot, clean); ok {
			return rel, true
		}
		if rel, ok := relativeIfWithin(s.shadowRoot, clean); ok {
			return rel, true
		}
		return "", false
	}

	clean := filepath.Clean(path)
	if clean == "." || clean == "" {
		return ".", true
	}
	if strings.HasPrefix(clean, "..") {
		return "", false
	}
	return clean, true
}

func (s *ShadowVFS) pathForShadow(path string) string {
	rel, ok := s.relativePath(path)
	if !ok {
		return path
	}
	if filepath.IsAbs(path) {
		return filepath.Join(s.shadowRoot, rel)
	}
	return rel
}

func (s *ShadowVFS) pathForBase(path string) string {
	rel, ok := s.relativePath(path)
	if !ok {
		return path
	}
	if filepath.IsAbs(path) {
		return filepath.Join(s.baseRoot, rel)
	}
	return rel
}

func relativeIfWithin(root string, target string) (string, bool) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", false
	}
	if rel == "." {
		return ".", true
	}
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}
