package vfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGrepFilter_InvalidPattern(t *testing.T) {
	vfs := NewMockVFS()

	// Invalid regex pattern
	_, err := NewGrepFilter("[invalid", vfs, "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")
}

func TestNewGrepFilter_NilVFS(t *testing.T) {
	_, err := NewGrepFilter("test", nil, "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VFS cannot be nil")
}

func TestGrepFilter_SimpleSearch(t *testing.T) {
	vfs := NewMockVFS()

	// Create test files
	err := vfs.WriteFile("file1.txt", []byte("hello world\nfoo bar\nhello again"))
	require.NoError(t, err)
	err = vfs.WriteFile("file2.txt", []byte("no match here\njust text"))
	require.NoError(t, err)
	err = vfs.WriteFile("file3.txt", []byte("hello there\nworld"))
	require.NoError(t, err)

	// Search for "hello"
	grep, err := NewGrepFilter("hello", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	// Should match file1.txt and file3.txt
	assert.Len(t, matches, 2)

	// Check file1.txt
	found := false
	for _, match := range matches {
		if match.Path == "file1.txt" {
			found = true
			assert.Equal(t, []int{1, 3}, match.Lines)
		}
	}
	assert.True(t, found, "file1.txt should be in matches")

	// Check file3.txt
	found = false
	for _, match := range matches {
		if match.Path == "file3.txt" {
			found = true
			assert.Equal(t, []int{1}, match.Lines)
		}
	}
	assert.True(t, found, "file3.txt should be in matches")
}

func TestGrepFilter_RegexPattern(t *testing.T) {
	vfs := NewMockVFS()

	err := vfs.WriteFile("test.txt", []byte("error: something\nERROR: another\nwarning: test\nError in line"))
	require.NoError(t, err)

	testCases := []struct {
		name            string
		pattern         string
		expectedLines   []int
		expectedMatches int
	}{
		{
			name:            "case insensitive error",
			pattern:         "(?i)error",
			expectedLines:   []int{1, 2, 4},
			expectedMatches: 1,
		},
		{
			name:            "exact match ERROR",
			pattern:         "ERROR",
			expectedLines:   []int{2},
			expectedMatches: 1,
		},
		{
			name:            "word boundary error",
			pattern:         `\berror\b`,
			expectedLines:   []int{1},
			expectedMatches: 1,
		},
		{
			name:            "error or warning",
			pattern:         "(error|warning)",
			expectedLines:   []int{1, 3},
			expectedMatches: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			grep, err := NewGrepFilter(tc.pattern, vfs, "", nil)
			require.NoError(t, err)

			matches, err := grep.Search()
			require.NoError(t, err)
			assert.Len(t, matches, tc.expectedMatches)

			if len(matches) > 0 {
				assert.Equal(t, tc.expectedLines, matches[0].Lines)
			}
		})
	}
}

func TestGrepFilter_WithRootDirectory(t *testing.T) {
	vfs := NewMockVFS()

	// Create files in different directories
	err := vfs.WriteFile("root.txt", []byte("hello world"))
	require.NoError(t, err)
	err = vfs.WriteFile("subdir/file1.txt", []byte("hello there"))
	require.NoError(t, err)
	err = vfs.WriteFile("subdir/file2.txt", []byte("no match"))
	require.NoError(t, err)
	err = vfs.WriteFile("subdir/nested/file3.txt", []byte("hello nested"))
	require.NoError(t, err)
	err = vfs.WriteFile("other/file4.txt", []byte("hello other"))
	require.NoError(t, err)

	// Search only in "subdir"
	grep, err := NewGrepFilter("hello", vfs, "subdir", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	// Should match only files under subdir
	assert.Len(t, matches, 2)

	paths := make(map[string]bool)
	for _, match := range matches {
		paths[match.Path] = true
	}

	assert.True(t, paths["subdir/file1.txt"])
	assert.True(t, paths["subdir/nested/file3.txt"])
	assert.False(t, paths["root.txt"])
	assert.False(t, paths["other/file4.txt"])
}

func TestGrepFilter_WithGlobFilter(t *testing.T) {
	vfs := NewMockVFS()

	// Create test files with different extensions
	err := vfs.WriteFile("test.go", []byte("package main\nfunc main()"))
	require.NoError(t, err)
	err = vfs.WriteFile("test.txt", []byte("package test"))
	require.NoError(t, err)
	err = vfs.WriteFile("main.go", []byte("package main\nimport fmt"))
	require.NoError(t, err)
	err = vfs.WriteFile("readme.md", []byte("package info"))
	require.NoError(t, err)

	// Create glob filter for only .go files
	globFilter := NewGlobFilter(false, []string{"*.go"})

	// Search for "package" with glob filter
	grep, err := NewGrepFilter("package", vfs, "", globFilter)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	// Should only match .go files
	assert.Len(t, matches, 2)

	for _, match := range matches {
		assert.True(t, match.Path == "test.go" || match.Path == "main.go")
	}
}

func TestGrepFilter_MultipleMatchesPerLine(t *testing.T) {
	vfs := NewMockVFS()

	err := vfs.WriteFile("test.txt", []byte("hello hello hello\nworld\nhello"))
	require.NoError(t, err)

	grep, err := NewGrepFilter("hello", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	require.Len(t, matches, 1)
	// Line 1 and 3 match, even though line 1 has multiple "hello" occurrences
	assert.Equal(t, []int{1, 3}, matches[0].Lines)
}

func TestGrepFilter_EmptyFile(t *testing.T) {
	vfs := NewMockVFS()

	err := vfs.WriteFile("empty.txt", []byte(""))
	require.NoError(t, err)

	grep, err := NewGrepFilter("test", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	// Empty file should not match
	assert.Len(t, matches, 0)
}

func TestGrepFilter_NoMatches(t *testing.T) {
	vfs := NewMockVFS()

	err := vfs.WriteFile("file1.txt", []byte("foo bar baz"))
	require.NoError(t, err)
	err = vfs.WriteFile("file2.txt", []byte("test content"))
	require.NoError(t, err)

	grep, err := NewGrepFilter("nonexistent", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	assert.Len(t, matches, 0)
}

func TestGrepFilter_DirectoriesIgnored(t *testing.T) {
	vfs := NewMockVFS()

	// Create directories and files
	err := vfs.WriteFile("dir1/file.txt", []byte("hello"))
	require.NoError(t, err)
	err = vfs.WriteFile("dir2/subdir/file.txt", []byte("world"))
	require.NoError(t, err)

	grep, err := NewGrepFilter(".*", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	// Should only match files, not directories
	for _, match := range matches {
		assert.True(t, match.Path == "dir1/file.txt" || match.Path == "dir2/subdir/file.txt")
	}
}

func TestGrepFilter_LargeFile(t *testing.T) {
	vfs := NewMockVFS()

	// Create a file with many lines
	var content string
	for i := 1; i <= 1000; i++ {
		if i%100 == 0 {
			content += "match this line\n"
		} else {
			content += "regular line\n"
		}
	}

	err := vfs.WriteFile("large.txt", []byte(content))
	require.NoError(t, err)

	grep, err := NewGrepFilter("match this", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	require.Len(t, matches, 1)
	assert.Equal(t, "large.txt", matches[0].Path)
	// Lines 100, 200, 300, ..., 1000
	expected := []int{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	assert.Equal(t, expected, matches[0].Lines)
}

func TestGrepFilter_SpecialCharacters(t *testing.T) {
	vfs := NewMockVFS()

	err := vfs.WriteFile("special.txt", []byte("line with $var\nline with @symbol\nline with [brackets]"))
	require.NoError(t, err)

	testCases := []struct {
		name          string
		pattern       string
		expectedLines []int
	}{
		{
			name:          "dollar sign",
			pattern:       `\$var`,
			expectedLines: []int{1},
		},
		{
			name:          "at symbol",
			pattern:       "@symbol",
			expectedLines: []int{2},
		},
		{
			name:          "brackets",
			pattern:       `\[brackets\]`,
			expectedLines: []int{3},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			grep, err := NewGrepFilter(tc.pattern, vfs, "", nil)
			require.NoError(t, err)

			matches, err := grep.Search()
			require.NoError(t, err)

			require.Len(t, matches, 1)
			assert.Equal(t, tc.expectedLines, matches[0].Lines)
		})
	}
}

func TestGrepFilter_WithComplexGlobFilter(t *testing.T) {
	vfs := NewMockVFS()

	// Create a realistic project structure
	files := map[string]string{
		"src/main.go":          "package main\nfunc main()",
		"src/util.go":          "package main\nfunc util()",
		"test/main_test.go":    "package main\nfunc TestMain()",
		"vendor/lib.go":        "package vendor\nfunc lib()",
		"README.md":            "# Project\nfunc example()",
		"src/subdir/helper.go": "package subdir\nfunc helper()",
		"test/subdir/test2.go": "package test\nfunc TestHelper()",
	}

	for path, content := range files {
		err := vfs.WriteFile(path, []byte(content))
		require.NoError(t, err)
	}

	// Create glob filter to include only .go files in src/ and test/ (excluding vendor)
	globFilter := NewGlobFilter(false, []string{"src/**/*.go", "test/**/*.go"})

	grep, err := NewGrepFilter("func", vfs, "", globFilter)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	// Should match src/ and test/ .go files, but not vendor/ or README.md
	matchedPaths := make(map[string]bool)
	for _, match := range matches {
		matchedPaths[match.Path] = true
	}

	assert.True(t, matchedPaths["src/main.go"])
	assert.True(t, matchedPaths["src/util.go"])
	assert.True(t, matchedPaths["test/main_test.go"])
	assert.True(t, matchedPaths["src/subdir/helper.go"])
	assert.True(t, matchedPaths["test/subdir/test2.go"])
	assert.False(t, matchedPaths["vendor/lib.go"])
	assert.False(t, matchedPaths["README.md"])
}

func TestGrepFilter_LineNumbers1Based(t *testing.T) {
	vfs := NewMockVFS()

	err := vfs.WriteFile("test.txt", []byte("match\nno\nmatch\nno\nmatch"))
	require.NoError(t, err)

	grep, err := NewGrepFilter("match", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	require.Len(t, matches, 1)
	// Line numbers should be 1-based: lines 1, 3, 5
	assert.Equal(t, []int{1, 3, 5}, matches[0].Lines)
}

func TestGrepFilter_EmptyVFS(t *testing.T) {
	vfs := NewMockVFS()

	grep, err := NewGrepFilter("test", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	assert.Len(t, matches, 0)
}

func TestGrepFilter_RootDirectoryNormalization(t *testing.T) {
	vfs := NewMockVFS()

	err := vfs.WriteFile("dir/file.txt", []byte("hello world"))
	require.NoError(t, err)

	testCases := []struct {
		name    string
		rootDir string
	}{
		{"empty string", ""},
		{"dot", "."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			grep, err := NewGrepFilter("hello", vfs, tc.rootDir, nil)
			require.NoError(t, err)

			matches, err := grep.Search()
			require.NoError(t, err)

			assert.Len(t, matches, 1)
			assert.Equal(t, "dir/file.txt", matches[0].Path)
		})
	}
}

func TestGrepFilter_BinaryContent(t *testing.T) {
	vfs := NewMockVFS()

	// Create a file with binary content (including null bytes)
	binaryContent := []byte{0x00, 0x01, 0x02, 'h', 'e', 'l', 'l', 'o', 0x00, '\n', 'w', 'o', 'r', 'l', 'd'}
	err := vfs.WriteFile("binary.dat", binaryContent)
	require.NoError(t, err)

	grep, err := NewGrepFilter("hello", vfs, "", nil)
	require.NoError(t, err)

	matches, err := grep.Search()
	require.NoError(t, err)

	// Should still be able to search binary files
	require.Len(t, matches, 1)
	assert.Equal(t, "binary.dat", matches[0].Path)
}
