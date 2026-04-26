package vfs

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
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
			errType: apis.ErrPermissionDenied,
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
			errType: apis.ErrPermissionDenied,
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
			errType: apis.ErrAskPermission,
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
			errType: apis.ErrPermissionDenied,
		},
		{
			name: "deny write by default",
			privileges: map[string]conf.FileAccess{
				"other.txt": {Write: conf.AccessAllow},
			},
			path:    "test.txt",
			wantErr: true,
			errType: apis.ErrPermissionDenied,
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
			errType: apis.ErrPermissionDenied,
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
			errType: apis.ErrPermissionDenied,
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
			errType: apis.ErrPermissionDenied,
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
			errType: apis.ErrPermissionDenied,
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
			errType: apis.ErrPermissionDenied,
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
			errType: apis.ErrPermissionDenied,
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
	assert.ErrorIs(t, err, apis.ErrPermissionDenied)

	err = acVFS.DeleteFile("read-only.txt", false, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, apis.ErrPermissionDenied)

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
