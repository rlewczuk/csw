package vfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePatch_EmptyPatch(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError string
	}{
		{
			name:      "empty string",
			input:     "",
			wantError: "missing or invalid Begin/End markers",
		},
		{
			name:      "only whitespace",
			input:     "   \n\t\n  ",
			wantError: "missing or invalid Begin/End markers",
		},
		{
			name:      "missing begin marker",
			input:     "*** End Patch",
			wantError: "missing or invalid Begin/End markers",
		},
		{
			name:      "missing end marker",
			input:     "*** Begin Patch",
			wantError: "missing or invalid Begin/End markers",
		},
		{
			name:      "begin after end",
			input:     "*** End Patch\n*** Begin Patch",
			wantError: "missing or invalid Begin/End markers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePatch(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestParsePatch_AddFile(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPath     string
		wantContents string
	}{
		{
			name: "add single line file",
			input: `*** Begin Patch
*** Add File: hello.txt
+Hello world
*** End Patch`,
			wantPath:     "hello.txt",
			wantContents: "Hello world",
		},
		{
			name: "add multi line file",
			input: `*** Begin Patch
*** Add File: src/main.go
+package main
+
+func main() {
+	println("Hello")
+}
*** End Patch`,
			wantPath: "src/main.go",
			wantContents: `package main

func main() {
	println("Hello")
}`,
		},
		{
			name: "add empty file",
			input: `*** Begin Patch
*** Add File: empty.txt
*** End Patch`,
			wantPath:     "empty.txt",
			wantContents: "",
		},
		{
			name: "add file with path containing spaces",
			input: `*** Begin Patch
*** Add File: path with spaces/file.txt
+content
*** End Patch`,
			wantPath:     "path with spaces/file.txt",
			wantContents: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := ParsePatch(tt.input)
			require.NoError(t, err)
			require.Len(t, patch.Hunks, 1)

			hunk, ok := patch.Hunks[0].(AddFile)
			require.True(t, ok, "expected AddFile hunk")
			assert.Equal(t, tt.wantPath, hunk.Path)
			assert.Equal(t, tt.wantContents, hunk.Contents)
		})
	}
}

func TestParsePatch_DeleteFile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPath string
	}{
		{
			name: "delete simple file",
			input: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
			wantPath: "old.txt",
		},
		{
			name: "delete file with path",
			input: `*** Begin Patch
*** Delete File: src/obsolete.go
*** End Patch`,
			wantPath: "src/obsolete.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := ParsePatch(tt.input)
			require.NoError(t, err)
			require.Len(t, patch.Hunks, 1)

			hunk, ok := patch.Hunks[0].(DeleteFile)
			require.True(t, ok, "expected DeleteFile hunk")
			assert.Equal(t, tt.wantPath, hunk.Path)
		})
	}
}

func TestParsePatch_UpdateFile(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPath     string
		wantMovePath string
		wantChunks   []UpdateFileChunk
	}{
		{
			name: "simple line replacement",
			input: `*** Begin Patch
*** Update File: app.py
@@ def greet():
-print("Hi")
+print("Hello, world!")
*** End Patch`,
			wantPath:     "app.py",
			wantMovePath: "",
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{`print("Hi")`},
					NewLines:      []string{`print("Hello, world!")`},
					ChangeContext: "def greet():",
					IsEndOfFile:   false,
				},
			},
		},
		{
			name: "update with move",
			input: `*** Begin Patch
*** Update File: src/app.py
*** Move to: src/main.py
@@ def main():
-old content
+new content
*** End Patch`,
			wantPath:     "src/app.py",
			wantMovePath: "src/main.py",
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{"old content"},
					NewLines:      []string{"new content"},
					ChangeContext: "def main():",
					IsEndOfFile:   false,
				},
			},
		},
		{
			name: "update with context lines",
			input: `*** Begin Patch
*** Update File: test.go
@@ func test() {
 context line
-old line
+new line
 another context
*** End Patch`,
			wantPath:     "test.go",
			wantMovePath: "",
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{"context line", "old line", "another context"},
					NewLines:      []string{"context line", "new line", "another context"},
					ChangeContext: "func test() {",
					IsEndOfFile:   false,
				},
			},
		},
		{
			name: "update with end of file anchor",
			input: `*** Begin Patch
*** Update File: file.txt
@@
-old end line
+new end line
*** End of File
*** End Patch`,
			wantPath:     "file.txt",
			wantMovePath: "",
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{"old end line"},
					NewLines:      []string{"new end line"},
					ChangeContext: "",
					IsEndOfFile:   true,
				},
			},
		},
		{
			name: "multiple chunks",
			input: `*** Begin Patch
*** Update File: multi.go
@@ func first():
-old1
+new1
@@ func second():
-old2
+new2
*** End Patch`,
			wantPath:     "multi.go",
			wantMovePath: "",
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{"old1"},
					NewLines:      []string{"new1"},
					ChangeContext: "func first():",
					IsEndOfFile:   false,
				},
				{
					OldLines:      []string{"old2"},
					NewLines:      []string{"new2"},
					ChangeContext: "func second():",
					IsEndOfFile:   false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := ParsePatch(tt.input)
			require.NoError(t, err)
			require.Len(t, patch.Hunks, 1)

			hunk, ok := patch.Hunks[0].(UpdateFile)
			require.True(t, ok, "expected UpdateFile hunk")
			assert.Equal(t, tt.wantPath, hunk.Path)
			assert.Equal(t, tt.wantMovePath, hunk.MovePath)
			assert.Equal(t, tt.wantChunks, hunk.Chunks)
		})
	}
}

func TestParsePatch_MultipleOperations(t *testing.T) {
	input := `*** Begin Patch
*** Add File: new.txt
+new content
*** Delete File: old.txt
*** Update File: existing.txt
@@ context
-old
+new
*** End Patch`

	patch, err := ParsePatch(input)
	require.NoError(t, err)
	require.Len(t, patch.Hunks, 3)

	// First: Add
	addHunk, ok := patch.Hunks[0].(AddFile)
	require.True(t, ok)
	assert.Equal(t, "new.txt", addHunk.Path)
	assert.Equal(t, "new content", addHunk.Contents)

	// Second: Delete
	deleteHunk, ok := patch.Hunks[1].(DeleteFile)
	require.True(t, ok)
	assert.Equal(t, "old.txt", deleteHunk.Path)

	// Third: Update
	updateHunk, ok := patch.Hunks[2].(UpdateFile)
	require.True(t, ok)
	assert.Equal(t, "existing.txt", updateHunk.Path)
}

func TestParsePatch_WithHeredoc(t *testing.T) {
	input := `cat <<'EOF'
*** Begin Patch
*** Add File: test.txt
+content
*** End Patch
EOF`

	patch, err := ParsePatch(input)
	require.NoError(t, err)
	require.Len(t, patch.Hunks, 1)

	hunk, ok := patch.Hunks[0].(AddFile)
	require.True(t, ok)
	assert.Equal(t, "test.txt", hunk.Path)
	assert.Equal(t, "content", hunk.Contents)
}

func TestParsePatch_EmptyHeaderPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "empty add file path",
			input: `*** Begin Patch
*** Add File:
+content
*** End Patch`,
		},
		{
			name: "empty delete file path",
			input: `*** Begin Patch
*** Delete File:
*** End Patch`,
		},
		{
			name: "empty update file path",
			input: `*** Begin Patch
*** Update File:
@@ context
-old
+new
*** End Patch`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePatch(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "empty file path")
		})
	}
}

func TestParsePatch_WhitespaceHandling(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContents string
	}{
		{
			name: "leading spaces in content",
			input: `*** Begin Patch
*** Add File: test.txt
+    indented line
+  another indented
*** End Patch`,
			wantContents: "    indented line\n  another indented",
		},
		{
			name: "trailing spaces in content",
			input: `*** Begin Patch
*** Add File: test.txt
+line with trailing spaces   
+normal line
*** End Patch`,
			wantContents: "line with trailing spaces   \nnormal line",
		},
		{
			name: "empty content lines",
			input: `*** Begin Patch
*** Add File: test.txt
+line1
+
+line2
*** End Patch`,
			wantContents: "line1\n\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := ParsePatch(tt.input)
			require.NoError(t, err)
			require.Len(t, patch.Hunks, 1)

			hunk, ok := patch.Hunks[0].(AddFile)
			require.True(t, ok)
			assert.Equal(t, tt.wantContents, hunk.Contents)
		})
	}
}

func TestParsePatch_UpdateFileChunkEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantChunks []UpdateFileChunk
	}{
		{
			name: "chunk with only additions",
			input: `*** Begin Patch
*** Update File: test.txt
@@ marker
+new line 1
+new line 2
*** End Patch`,
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{},
					NewLines:      []string{"new line 1", "new line 2"},
					ChangeContext: "marker",
					IsEndOfFile:   false,
				},
			},
		},
		{
			name: "chunk with only deletions",
			input: `*** Begin Patch
*** Update File: test.txt
@@ marker
-old line 1
-old line 2
*** End Patch`,
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{"old line 1", "old line 2"},
					NewLines:      []string{},
					ChangeContext: "marker",
					IsEndOfFile:   false,
				},
			},
		},
		{
			name: "chunk without context",
			input: `*** Begin Patch
*** Update File: test.txt
@@
-old
+new
*** End Patch`,
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{"old"},
					NewLines:      []string{"new"},
					ChangeContext: "",
					IsEndOfFile:   false,
				},
			},
		},
		{
			name: "chunk with mixed changes",
			input: `*** Begin Patch
*** Update File: test.txt
@@ context
 context1
-old1
+new1
 context2
-old2
+new2
 context3
*** End Patch`,
			wantChunks: []UpdateFileChunk{
				{
					OldLines:      []string{"context1", "old1", "context2", "old2", "context3"},
					NewLines:      []string{"context1", "new1", "context2", "new2", "context3"},
					ChangeContext: "context",
					IsEndOfFile:   false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := ParsePatch(tt.input)
			require.NoError(t, err)
			require.Len(t, patch.Hunks, 1)

			hunk, ok := patch.Hunks[0].(UpdateFile)
			require.True(t, ok)
			assert.Equal(t, tt.wantChunks, hunk.Chunks)
		})
	}
}

func TestParsePatch_ComplexExample(t *testing.T) {
	// This is the example from the format documentation
	input := `*** Begin Patch
*** Add File: hello.txt
+Hello world
*** Update File: src/app.py
*** Move to: src/main.py
@@ def greet():
-print("Hi")
+print("Hello, world!")
*** Delete File: obsolete.txt
*** End Patch`

	patch, err := ParsePatch(input)
	require.NoError(t, err)
	require.Len(t, patch.Hunks, 3)

	// Check Add File
	addHunk, ok := patch.Hunks[0].(AddFile)
	require.True(t, ok)
	assert.Equal(t, "hello.txt", addHunk.Path)
	assert.Equal(t, "Hello world", addHunk.Contents)

	// Check Update File with Move
	updateHunk, ok := patch.Hunks[1].(UpdateFile)
	require.True(t, ok)
	assert.Equal(t, "src/app.py", updateHunk.Path)
	assert.Equal(t, "src/main.py", updateHunk.MovePath)
	require.Len(t, updateHunk.Chunks, 1)
	assert.Equal(t, "def greet():", updateHunk.Chunks[0].ChangeContext)
	assert.Equal(t, []string{`print("Hi")`}, updateHunk.Chunks[0].OldLines)
	assert.Equal(t, []string{`print("Hello, world!")`}, updateHunk.Chunks[0].NewLines)

	// Check Delete File
	deleteHunk, ok := patch.Hunks[2].(DeleteFile)
	require.True(t, ok)
	assert.Equal(t, "obsolete.txt", deleteHunk.Path)
}

func TestStripHeredoc(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no heredoc",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "simple heredoc",
			input:    "cat <<EOF\ncontent\nEOF",
			expected: "content",
		},
		{
			name:     "heredoc with quotes",
			input:    "cat <<'EOF'\ncontent\nEOF",
			expected: "content",
		},
		{
			name:     "heredoc with double quotes",
			input:    "cat \u003c\u003c\"EOF\"\ncontent\nEOF",
			expected: "content",
		},
		{
			name:     "heredoc without cat",
			input:    "<<EOF\ncontent\nEOF",
			expected: "content",
		},
		{
			name:     "heredoc with trailing whitespace",
			input:    "cat <<EOF  \ncontent\nEOF",
			expected: "content",
		},
		{
			name:     "no heredoc - just content with delimiter",
			input:    "some text EOF more text",
			expected: "some text EOF more text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHeredoc(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
