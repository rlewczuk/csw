package vfs

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilePatcher_ApplyEdits(t *testing.T) {
	tests := []struct {
		name        string
		initialFile string
		oldString   string
		newString   string
		replaceAll  bool
		expectedErr error
		validate    func(t *testing.T, vfs VFS, diff string)
	}{
		{
			name:        "simple replacement",
			initialFile: "Hello World\nThis is a test\n",
			oldString:   "World",
			newString:   "Universe",
			replaceAll:  false,
			expectedErr: nil,
			validate: func(t *testing.T, vfs VFS, diff string) {
				content, err := vfs.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "Hello Universe\nThis is a test\n", string(content))
				assert.Contains(t, diff, "-Hello World")
				assert.Contains(t, diff, "+Hello Universe")
			},
		},
		{
			name:        "file not found error",
			initialFile: "",
			oldString:   "test",
			newString:   "new",
			replaceAll:  false,
			expectedErr: ErrFileNotFound,
			validate: func(t *testing.T, vfs VFS, diff string) {
				assert.Empty(t, diff)
			},
		},
		{
			name:        "old string not found error",
			initialFile: "Hello World\n",
			oldString:   "NotPresent",
			newString:   "Something",
			replaceAll:  false,
			expectedErr: ErrOldStringNotFound,
			validate: func(t *testing.T, vfs VFS, diff string) {
				assert.Empty(t, diff)
				// File should not be modified
				content, err := vfs.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "Hello World\n", string(content))
			},
		},
		{
			name:        "multiple matches without replaceAll error",
			initialFile: "foo bar foo baz\n",
			oldString:   "foo",
			newString:   "qux",
			replaceAll:  false,
			expectedErr: ErrOldStringMultipleMatch,
			validate: func(t *testing.T, vfs VFS, diff string) {
				assert.Empty(t, diff)
				// File should not be modified
				content, err := vfs.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "foo bar foo baz\n", string(content))
			},
		},
		{
			name:        "multiple matches with replaceAll",
			initialFile: "foo bar foo baz foo\n",
			oldString:   "foo",
			newString:   "qux",
			replaceAll:  true,
			expectedErr: nil,
			validate: func(t *testing.T, vfs VFS, diff string) {
				content, err := vfs.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "qux bar qux baz qux\n", string(content))
				assert.Contains(t, diff, "-foo bar foo baz foo")
				assert.Contains(t, diff, "+qux bar qux baz qux")
			},
		},
		{
			name: "multiline replacement",
			initialFile: `package main

func main() {
	println("Hello")
}
`,
			oldString: `func main() {
	println("Hello")
}`,
			newString: `func main() {
	println("Hello World")
	println("Goodbye")
}`,
			replaceAll:  false,
			expectedErr: nil,
			validate: func(t *testing.T, vfs VFS, diff string) {
				content, err := vfs.ReadFile("test.txt")
				require.NoError(t, err)
				expected := `package main

func main() {
	println("Hello World")
	println("Goodbye")
}
`
				assert.Equal(t, expected, string(content))
				assert.Contains(t, diff, "-\tprintln(\"Hello\")")
				assert.Contains(t, diff, "+\tprintln(\"Hello World\")")
				assert.Contains(t, diff, "+\tprintln(\"Goodbye\")")
			},
		},
		{
			name:        "replacement with larger context for uniqueness",
			initialFile: "line 1\nfoo bar\nline 2\nfoo baz\nline 3\n",
			oldString:   "foo bar\nline 2",
			newString:   "qux bar\nline 2",
			replaceAll:  false,
			expectedErr: nil,
			validate: func(t *testing.T, vfs VFS, diff string) {
				content, err := vfs.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "line 1\nqux bar\nline 2\nfoo baz\nline 3\n", string(content))
				assert.Contains(t, diff, "-foo bar")
				assert.Contains(t, diff, "+qux bar")
			},
		},
		{
			name:        "empty old string",
			initialFile: "Hello World\n",
			oldString:   "",
			newString:   "test",
			replaceAll:  false,
			expectedErr: ErrOldStringMultipleMatch,
			validate: func(t *testing.T, vfs VFS, diff string) {
				// Empty string matches at every position, so should fail
				assert.Empty(t, diff)
			},
		},
		{
			name:        "identical old and new strings",
			initialFile: "Hello World\n",
			oldString:   "World",
			newString:   "World",
			replaceAll:  false,
			expectedErr: nil,
			validate: func(t *testing.T, vfs VFS, diff string) {
				content, err := vfs.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "Hello World\n", string(content))
				// Diff should be empty or minimal since nothing changed
				assert.Empty(t, diff)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock VFS
			mockVFS := NewMockVFS()

			// Create test file if initial content is provided
			if tt.initialFile != "" {
				err := mockVFS.WriteFile("test.txt", []byte(tt.initialFile))
				require.NoError(t, err)
			}

			// Create patcher
			patcher := NewFilePatcher(mockVFS)

			// Apply edits
			diff, err := patcher.ApplyEdits("test.txt", tt.oldString, tt.newString, tt.replaceAll)

			// Check error
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr),
					"expected error containing %v, got %v", tt.expectedErr, err)
			} else {
				require.NoError(t, err)
			}

			// Run validation
			if tt.validate != nil {
				tt.validate(t, mockVFS, diff)
			}
		})
	}
}

func TestFilePatcher_DiffFormat(t *testing.T) {
	mockVFS := NewMockVFS()
	err := mockVFS.WriteFile("test.txt", []byte("line 1\nline 2\nline 3\nline 4\nline 5\n"))
	require.NoError(t, err)

	patcher := NewFilePatcher(mockVFS)
	diff, err := patcher.ApplyEdits("test.txt", "line 3", "line three", false)
	require.NoError(t, err)

	// Check diff format
	assert.Contains(t, diff, "--- test.txt")
	assert.Contains(t, diff, "+++ test.txt")
	assert.Contains(t, diff, "@@")
	assert.Contains(t, diff, "-line 3")
	assert.Contains(t, diff, "+line three")

	// Verify diff header format
	lines := strings.Split(diff, "\n")
	assert.True(t, len(lines) > 0)
	assert.True(t, strings.HasPrefix(lines[0], "--- test.txt"))
	assert.True(t, strings.HasPrefix(lines[1], "+++ test.txt"))
}

func TestFilePatcher_UnifiedDiffContext(t *testing.T) {
	mockVFS := NewMockVFS()

	// Create a file with many lines to test context
	content := ""
	for i := 1; i <= 10; i++ {
		content += "line " + string(rune('0'+i)) + "\n"
	}
	err := mockVFS.WriteFile("test.txt", []byte(content))
	require.NoError(t, err)

	patcher := NewFilePatcher(mockVFS)
	diff, err := patcher.ApplyEdits("test.txt", "line 5", "line five", false)
	require.NoError(t, err)

	// The diff should include context lines around the change
	// Typically up to 3 lines before and after
	assert.Contains(t, diff, " line 2")    // context before
	assert.Contains(t, diff, " line 3")    // context before
	assert.Contains(t, diff, " line 4")    // context before
	assert.Contains(t, diff, "-line 5")    // removed line
	assert.Contains(t, diff, "+line five") // added line
	assert.Contains(t, diff, " line 6")    // context after
	assert.Contains(t, diff, " line 7")    // context after
	assert.Contains(t, diff, " line 8")    // context after
}

func TestNewFilePatcher(t *testing.T) {
	mockVFS := NewMockVFS()
	patcher := NewFilePatcher(mockVFS)

	assert.NotNil(t, patcher)
	assert.Implements(t, (*FilePatcher)(nil), patcher)
}
