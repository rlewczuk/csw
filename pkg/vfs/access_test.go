package vfs

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessControlVFS_ReadFile(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		path       string
		wantErr    bool
		errType    error
	}{
		{
			name: "allow read with specific pattern",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Read: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: false,
		},
		{
			name: "deny read with specific pattern",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Read: conf.AccessDeny},
			},
			path:    "test.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
		{
			name: "allow read with wildcard pattern",
			privileges: map[string]conf.FileAccess{
				"*.txt": {Read: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: false,
		},
		{
			name: "deny read by default",
			privileges: map[string]conf.FileAccess{
				"other.txt": {Read: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
		{
			name: "allow read with default wildcard",
			privileges: map[string]conf.FileAccess{
				"*": {Read: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: false,
		},
		{
			name: "ask access returns ErrAskPermission",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Read: conf.AccessAsk},
			},
			path:    "test.txt",
			wantErr: true,
			errType: ErrAskPermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			mockVFS.WriteFile(tt.path, []byte("content"))

			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			_, err := acVFS.ReadFile(tt.path)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAccessControlVFS_WriteFile(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		path       string
		wantErr    bool
		errType    error
	}{
		{
			name: "allow write",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Write: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: false,
		},
		{
			name: "deny write",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Write: conf.AccessDeny},
			},
			path:    "test.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
		{
			name: "deny write by default",
			privileges: map[string]conf.FileAccess{
				"other.txt": {Write: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			err := acVFS.WriteFile(tt.path, []byte("content"))

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAccessControlVFS_DeleteFile(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		path       string
		wantErr    bool
		errType    error
	}{
		{
			name: "allow delete",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Delete: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: false,
		},
		{
			name: "deny delete",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Delete: conf.AccessDeny},
			},
			path:    "test.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			mockVFS.WriteFile(tt.path, []byte("content"))

			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			err := acVFS.DeleteFile(tt.path, false, false)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAccessControlVFS_ListFiles(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		path       string
		wantErr    bool
		errType    error
	}{
		{
			name: "allow list",
			privileges: map[string]conf.FileAccess{
				"dir": {List: conf.AccessAllow},
			},
			path:    "dir",
			wantErr: false,
		},
		{
			name: "deny list",
			privileges: map[string]conf.FileAccess{
				"dir": {List: conf.AccessDeny},
			},
			path:    "dir",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			// Create the directory first
			mockVFS.WriteFile(tt.path+"/dummy.txt", []byte("test"))

			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			_, err := acVFS.ListFiles(tt.path, false)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAccessControlVFS_FindFiles(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		query      string
		wantErr    bool
		errType    error
	}{
		{
			name: "allow find",
			privileges: map[string]conf.FileAccess{
				"*.txt": {Find: conf.AccessAllow},
			},
			query:   "*.txt",
			wantErr: false,
		},
		{
			name: "deny find",
			privileges: map[string]conf.FileAccess{
				"*.txt": {Find: conf.AccessDeny},
			},
			query:   "*.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			_, err := acVFS.FindFiles(tt.query, false)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAccessControlVFS_MoveFile(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		src        string
		dst        string
		wantErr    bool
		errType    error
	}{
		{
			name: "allow move with both permissions",
			privileges: map[string]conf.FileAccess{
				"src.txt": {Move: conf.AccessAllow},
				"dst.txt": {Write: conf.AccessAllow},
			},
			src:     "src.txt",
			dst:     "dst.txt",
			wantErr: false,
		},
		{
			name: "deny move on source",
			privileges: map[string]conf.FileAccess{
				"src.txt": {Move: conf.AccessDeny},
				"dst.txt": {Write: conf.AccessAllow},
			},
			src:     "src.txt",
			dst:     "dst.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
		{
			name: "deny write on destination",
			privileges: map[string]conf.FileAccess{
				"src.txt": {Move: conf.AccessAllow},
				"dst.txt": {Write: conf.AccessDeny},
			},
			src:     "src.txt",
			dst:     "dst.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
		{
			name: "deny both",
			privileges: map[string]conf.FileAccess{
				"src.txt": {Move: conf.AccessDeny},
				"dst.txt": {Write: conf.AccessDeny},
			},
			src:     "src.txt",
			dst:     "dst.txt",
			wantErr: true,
			errType: ErrPermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			mockVFS.WriteFile(tt.src, []byte("content"))

			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			err := acVFS.MoveFile(tt.src, tt.dst)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAccessControlVFS_GlobMatching(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		path       string
		wantErr    bool
	}{
		{
			name: "exact match takes precedence over wildcard",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Read: conf.AccessAllow},
				"*.txt":    {Read: conf.AccessDeny},
			},
			path:    "test.txt",
			wantErr: false,
		},
		{
			name: "more specific wildcard takes precedence",
			privileges: map[string]conf.FileAccess{
				"test*.txt": {Read: conf.AccessAllow},
				"*.txt":     {Read: conf.AccessDeny},
			},
			path:    "test123.txt",
			wantErr: false,
		},
		{
			name: "path with slashes - specific takes precedence",
			privileges: map[string]conf.FileAccess{
				"dir/test.txt": {Read: conf.AccessAllow},
				"dir/*.txt":    {Read: conf.AccessDeny},
				"*":            {Read: conf.AccessDeny},
			},
			path:    "dir/test.txt",
			wantErr: false,
		},
		{
			name: "default wildcard allows all",
			privileges: map[string]conf.FileAccess{
				"*": {Read: conf.AccessAllow},
			},
			path:    "any/path/file.txt",
			wantErr: false,
		},
		{
			name: "no match defaults to deny",
			privileges: map[string]conf.FileAccess{
				"other.txt": {Read: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			mockVFS.WriteFile(tt.path, []byte("content"))

			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			_, err := acVFS.ReadFile(tt.path)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAccessControlVFS_SpecificityRules(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		path       string
		wantErr    bool
		desc       string
	}{
		{
			name: "fewer wildcards is more specific",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Read: conf.AccessAllow}, // 0 wildcards
				"*.txt":    {Read: conf.AccessDeny},  // 1 wildcard
			},
			path:    "test.txt",
			wantErr: false,
			desc:    "exact match should win over wildcard",
		},
		{
			name: "more directory levels is more specific",
			privileges: map[string]conf.FileAccess{
				"a/b/c/test.txt": {Read: conf.AccessAllow}, // 3 levels
				"a/b/*.txt":      {Read: conf.AccessDeny},  // 2 levels
				"a/*.txt":        {Read: conf.AccessDeny},  // 1 level
			},
			path:    "a/b/c/test.txt",
			wantErr: false,
			desc:    "deeper path should win",
		},
		{
			name: "longer pattern is more specific",
			privileges: map[string]conf.FileAccess{
				"testfile.txt": {Read: conf.AccessAllow}, // 12 chars
				"test*.txt":    {Read: conf.AccessDeny},  // 10 chars (but has wildcard)
			},
			path:    "testfile.txt",
			wantErr: false,
			desc:    "exact match should win over wildcard pattern",
		},
		{
			name: "wildcard count takes precedence over length",
			privileges: map[string]conf.FileAccess{
				"ab.txt": {Read: conf.AccessAllow}, // shorter but exact
				"*.txt":  {Read: conf.AccessDeny},  // longer but has wildcard
			},
			path:    "ab.txt",
			wantErr: false,
			desc:    "exact match should always win",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			mockVFS.WriteFile(tt.path, []byte("content"))

			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			_, err := acVFS.ReadFile(tt.path)

			if tt.wantErr {
				require.Error(t, err, tt.desc)
			} else {
				require.NoError(t, err, tt.desc)
			}
		})
	}
}

func TestAccessControlVFS_MultipleOperations(t *testing.T) {
	mockVFS := NewMockVFS()

	privileges := map[string]conf.FileAccess{
		"read-only.txt": {
			Read:   conf.AccessAllow,
			Write:  conf.AccessDeny,
			Delete: conf.AccessDeny,
		},
		"read-write.txt": {
			Read:  conf.AccessAllow,
			Write: conf.AccessAllow,
		},
		"dir": {
			List: conf.AccessAllow,
		},
		"dir/*": {
			Find: conf.AccessAllow,
		},
	}

	acVFS := NewAccessControlVFS(mockVFS, privileges)

	// Test read-only file
	mockVFS.WriteFile("read-only.txt", []byte("content"))
	_, err := acVFS.ReadFile("read-only.txt")
	require.NoError(t, err)

	err = acVFS.WriteFile("read-only.txt", []byte("new content"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPermissionDenied)

	err = acVFS.DeleteFile("read-only.txt", false, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPermissionDenied)

	// Test read-write file
	err = acVFS.WriteFile("read-write.txt", []byte("content"))
	require.NoError(t, err)

	_, err = acVFS.ReadFile("read-write.txt")
	require.NoError(t, err)

	// Test directory operations
	// Create a directory first by writing a file into it
	mockVFS.WriteFile("dir/test.txt", []byte("test"))

	_, err = acVFS.ListFiles("dir", false)
	require.NoError(t, err)

	_, err = acVFS.FindFiles("dir/*", false)
	require.NoError(t, err)
}

func TestIsMoreSpecific(t *testing.T) {
	tests := []struct {
		name     string
		pattern1 string
		pattern2 string
		want     bool
	}{
		{
			name:     "exact match vs wildcard",
			pattern1: "test.txt",
			pattern2: "*.txt",
			want:     true,
		},
		{
			name:     "wildcard vs exact match",
			pattern1: "*.txt",
			pattern2: "test.txt",
			want:     false,
		},
		{
			name:     "more levels vs fewer levels",
			pattern1: "a/b/c.txt",
			pattern2: "a/c.txt",
			want:     true,
		},
		{
			name:     "same wildcards, more levels",
			pattern1: "a/b/*.txt",
			pattern2: "a/*.txt",
			want:     true,
		},
		{
			name:     "same wildcards and levels, longer",
			pattern1: "testfile.txt",
			pattern2: "test.txt",
			want:     true,
		},
		{
			name:     "same everything",
			pattern1: "test.txt",
			pattern2: "test.txt",
			want:     false,
		},
		{
			name:     "multiple wildcards vs single",
			pattern1: "*.txt",
			pattern2: "*.*.txt",
			want:     true,
		},
		{
			name:     "question mark wildcard",
			pattern1: "test.txt",
			pattern2: "test.tx?",
			want:     true,
		},
		{
			name:     "bracket wildcard",
			pattern1: "test.txt",
			pattern2: "test.[a-z]xt",
			want:     true,
		},
		{
			name:     "foo/bar* vs foo/* - longer literal prefix",
			pattern1: "foo/bar*",
			pattern2: "foo/*",
			want:     true,
		},
		{
			name:     "foo/bar/baz* vs foo/b*/baz* - fewer wildcards",
			pattern1: "foo/bar/baz*",
			pattern2: "foo/b*/baz*",
			want:     true,
		},
		{
			name:     "a/b/c/d.txt vs a/b/*/d.txt - no wildcards vs one",
			pattern1: "a/b/c/d.txt",
			pattern2: "a/b/*/d.txt",
			want:     true,
		},
		{
			name:     "a/b/*/d.txt vs a/*/*/d.txt - one vs two wildcards",
			pattern1: "a/b/*/d.txt",
			pattern2: "a/*/*/d.txt",
			want:     true,
		},
		{
			name:     "test*.txt vs t*.txt - longer prefix",
			pattern1: "test*.txt",
			pattern2: "t*.txt",
			want:     true,
		},
		{
			name:     "*.test.txt vs *.txt - longer pattern",
			pattern1: "*.test.txt",
			pattern2: "*.txt",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMoreSpecific(tt.pattern1, tt.pattern2)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNuancedGlobPatterns(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		path       string
		wantErr    bool
		desc       string
	}{
		{
			name: "/foo/bar* is more specific than /foo/*",
			privileges: map[string]conf.FileAccess{
				"foo/bar*": {Read: conf.AccessAllow},
				"foo/*":    {Read: conf.AccessDeny},
			},
			path:    "foo/bar123",
			wantErr: false,
			desc:    "pattern with longer literal prefix should win",
		},
		{
			name: "/foo/* denies when /foo/bar* doesn't match",
			privileges: map[string]conf.FileAccess{
				"foo/bar*": {Read: conf.AccessAllow},
				"foo/*":    {Read: conf.AccessDeny},
			},
			path:    "foo/baz123",
			wantErr: true,
			desc:    "should use less specific pattern when more specific doesn't match",
		},
		{
			name: "/foo/bar/baz* is more specific than /foo/b*/baz*",
			privileges: map[string]conf.FileAccess{
				"foo/bar/baz*": {Read: conf.AccessAllow},
				"foo/b*/baz*":  {Read: conf.AccessDeny},
			},
			path:    "foo/bar/baz123",
			wantErr: false,
			desc:    "pattern with fewer wildcards should win",
		},
		{
			name: "/foo/b*/baz* matches when more specific doesn't",
			privileges: map[string]conf.FileAccess{
				"foo/bar/baz*": {Read: conf.AccessAllow},
				"foo/b*/baz*":  {Read: conf.AccessDeny},
			},
			path:    "foo/bbb/baz123",
			wantErr: true,
			desc:    "should use less specific pattern when more specific doesn't match",
		},
		{
			name: "a/b/c/d.txt vs a/b/*/d.txt vs a/*/c/d.txt vs a/*/*/d.txt",
			privileges: map[string]conf.FileAccess{
				"a/b/c/d.txt": {Read: conf.AccessAllow}, // 0 wildcards, most specific
				"a/b/*/d.txt": {Read: conf.AccessDeny},  // 1 wildcard
				"a/*/c/d.txt": {Read: conf.AccessDeny},  // 1 wildcard
				"a/*/*/d.txt": {Read: conf.AccessDeny},  // 2 wildcards
			},
			path:    "a/b/c/d.txt",
			wantErr: false,
			desc:    "exact match should win over all wildcard patterns",
		},
		{
			name: "a/b/x/d.txt matches a/b/*/d.txt not a/*/c/d.txt",
			privileges: map[string]conf.FileAccess{
				"a/b/*/d.txt": {Read: conf.AccessAllow}, // matches
				"a/*/c/d.txt": {Read: conf.AccessDeny},  // doesn't match
			},
			path:    "a/b/x/d.txt",
			wantErr: false,
			desc:    "only matching pattern should apply",
		},
		{
			name: "deeper path with wildcard vs shallow exact",
			privileges: map[string]conf.FileAccess{
				"a/b/c/*.txt": {Read: conf.AccessAllow}, // 1 wildcard, 3 levels
				"a/b/*.txt":   {Read: conf.AccessDeny},  // 1 wildcard, 2 levels
			},
			path:    "a/b/c/file.txt",
			wantErr: false,
			desc:    "deeper path should win when wildcard count is same",
		},
		{
			name: "wildcard count takes precedence over depth",
			privileges: map[string]conf.FileAccess{
				"a/b/c/d/e/*.txt": {Read: conf.AccessDeny},  // 1 wildcard, 5 levels
				"a/b/*.txt":       {Read: conf.AccessAllow}, // 1 wildcard, 2 levels
			},
			path:    "a/b/file.txt",
			wantErr: false,
			desc:    "pattern with same wildcards and more depth should only match its own path",
		},
		{
			name: "prefix matching - longer literal prefix wins",
			privileges: map[string]conf.FileAccess{
				"test*.txt": {Read: conf.AccessAllow}, // 4 char prefix
				"t*.txt":    {Read: conf.AccessDeny},  // 1 char prefix
				"*.txt":     {Read: conf.AccessDeny},  // 0 char prefix
			},
			path:    "test123.txt",
			wantErr: false,
			desc:    "longer literal prefix should be more specific",
		},
		{
			name: "suffix matching - file extensions",
			privileges: map[string]conf.FileAccess{
				"*.test.txt": {Read: conf.AccessAllow}, // more specific suffix
				"*.txt":      {Read: conf.AccessDeny},  // less specific suffix
			},
			path:    "file.test.txt",
			wantErr: false,
			desc:    "longer suffix pattern should be more specific",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVFS := NewMockVFS()
			mockVFS.WriteFile(tt.path, []byte("content"))

			acVFS := NewAccessControlVFS(mockVFS, tt.privileges)
			_, err := acVFS.ReadFile(tt.path)

			if tt.wantErr {
				require.Error(t, err, tt.desc)
				assert.ErrorIs(t, err, ErrPermissionDenied, tt.desc)
			} else {
				require.NoError(t, err, tt.desc)
			}
		})
	}
}

func TestGetAccess(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.FileAccess
		path       string
		wantAccess conf.FileAccess
	}{
		{
			name: "exact match",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Read: conf.AccessAllow},
			},
			path:       "test.txt",
			wantAccess: conf.FileAccess{Read: conf.AccessAllow},
		},
		{
			name: "wildcard match",
			privileges: map[string]conf.FileAccess{
				"*.txt": {Read: conf.AccessAllow, Write: conf.AccessDeny},
			},
			path:       "test.txt",
			wantAccess: conf.FileAccess{Read: conf.AccessAllow, Write: conf.AccessDeny},
		},
		{
			name: "no match returns deny all",
			privileges: map[string]conf.FileAccess{
				"other.txt": {Read: conf.AccessAllow},
			},
			path:       "test.txt",
			wantAccess: denyAll,
		},
		{
			name: "most specific wins",
			privileges: map[string]conf.FileAccess{
				"test.txt": {Read: conf.AccessAllow},
				"*.txt":    {Read: conf.AccessDeny},
				"*":        {Read: conf.AccessDeny},
			},
			path:       "test.txt",
			wantAccess: conf.FileAccess{Read: conf.AccessAllow},
		},
		{
			name: "invalid pattern is ignored",
			privileges: map[string]conf.FileAccess{
				"[invalid": {Read: conf.AccessAllow},
				"*.txt":    {Read: conf.AccessDeny},
			},
			path:       "test.txt",
			wantAccess: conf.FileAccess{Read: conf.AccessDeny},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acVFS := &AccessControlVFS{
				privileges: tt.privileges,
			}
			got := acVFS.getAccess(tt.path)
			assert.Equal(t, tt.wantAccess, got)
		})
	}
}
