package vfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGlobFilter_EmptyPatterns(t *testing.T) {
	filter := NewGlobFilter(true, nil)

	testCases := []struct {
		path     string
		expected bool
	}{
		{"file.txt", true},
		{"dir/file.txt", true},
		{"any/path/here.go", true},
		{"", true},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			result := filter.Matches(tc.path)
			assert.Equal(t, tc.expected, result, "path: %s", tc.path)
		})
	}
}

func TestGlobFilter_SimplePatterns(t *testing.T) {
	testCases := []struct {
		name     string
		patterns []string
		path     string
		expected bool
	}{
		// Simple filename patterns
		{"exact match", []string{"file.txt"}, "file.txt", true},
		{"exact match - no match", []string{"file.txt"}, "other.txt", false},
		{"wildcard extension", []string{"*.txt"}, "file.txt", true},
		{"wildcard extension - no match", []string{"*.txt"}, "file.go", false},
		{"wildcard name", []string{"test*"}, "test_file.txt", true},
		{"wildcard name - no match", []string{"test*"}, "file.txt", false},

		// Directory patterns
		{"directory pattern", []string{"dir/*.txt"}, "dir/file.txt", true},
		{"directory pattern - no match", []string{"dir/*.txt"}, "other/file.txt", false},
		{"nested directory", []string{"a/b/c.txt"}, "a/b/c.txt", true},
		{"nested directory - no match", []string{"a/b/c.txt"}, "a/c.txt", false},

		// Double star patterns
		{"double star all", []string{"**"}, "any/path/file.txt", true},
		{"double star extension", []string{"**/*.go"}, "pkg/vfs/glob.go", true},
		{"double star extension - no match", []string{"**/*.go"}, "file.txt", false},
		{"double star prefix", []string{"**/test/*.go"}, "pkg/test/file.go", true},
		{"double star prefix - no match", []string{"**/test/*.go"}, "pkg/other/file.go", false},

		// Character classes
		{"char class", []string{"file[123].txt"}, "file1.txt", true},
		{"char class - no match", []string{"file[123].txt"}, "file4.txt", false},
		{"char range", []string{"file[a-c].txt"}, "fileb.txt", true},
		{"char range - no match", []string{"file[a-c].txt"}, "filed.txt", false},

		// Question mark
		{"question mark", []string{"file?.txt"}, "file1.txt", true},
		{"question mark - no match", []string{"file?.txt"}, "file12.txt", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewGlobFilter(false, tc.patterns)
			result := filter.Matches(tc.path)
			assert.Equal(t, tc.expected, result, "patterns: %v, path: %s", tc.patterns, tc.path)
		})
	}
}

func TestGlobFilter_MultiplePatterns(t *testing.T) {
	testCases := []struct {
		name     string
		patterns []string
		tests    map[string]bool
	}{
		{
			name:     "multiple extensions",
			patterns: []string{"*.go", "*.txt"},
			tests: map[string]bool{
				"file.go":  true,
				"file.txt": true,
				"file.md":  false,
			},
		},
		{
			name:     "multiple directories",
			patterns: []string{"pkg/**/*.go", "cmd/**/*.go"},
			tests: map[string]bool{
				"pkg/vfs/glob.go": true,
				"cmd/csw/main.go": true,
				"test/helper.go":  false,
				"internal/foo.go": false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewGlobFilter(false, tc.patterns)
			for path, expected := range tc.tests {
				result := filter.Matches(path)
				assert.Equal(t, expected, result, "path: %s", path)
			}
		})
	}
}

func TestGlobFilter_NegationPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		patterns []string
		path     string
		expected bool
	}{
		// Basic negation
		{"include then exclude", []string{"*.txt", "!secret.txt"}, "file.txt", true},
		{"include then exclude match", []string{"*.txt", "!secret.txt"}, "secret.txt", false},

		// Re-inclusion
		{"exclude then include", []string{"!*.go"}, "file.go", false},
		{"double negation", []string{"*.txt", "!secret.txt", "!secret.txt"}, "secret.txt", false},

		// Complex negation patterns
		{"include directory then exclude file", []string{"dir/**", "!dir/keep.txt"}, "dir/keep.txt", false},
		{"include directory then exclude file - other file", []string{"dir/**", "!dir/keep.txt"}, "dir/remove.txt", true},

		// Multiple negations
		{"multiple negations", []string{"*.log", "!important.log", "!critical.log"}, "debug.log", true},
		{"multiple negations - excluded", []string{"*.log", "!important.log", "!critical.log"}, "important.log", false},
		{"multiple negations - excluded 2", []string{"*.log", "!important.log", "!critical.log"}, "critical.log", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewGlobFilter(false, tc.patterns)
			result := filter.Matches(tc.path)
			assert.Equal(t, tc.expected, result, "patterns: %v, path: %s", tc.patterns, tc.path)
		})
	}
}

func TestGlobFilter_GitignoreContent(t *testing.T) {
	gitignoreContent := `# This is a comment
*.log
!important.log

# Build directories
build/
dist/

# Nested patterns
**/node_modules/**
**/*.tmp

# Negation example
*.txt
!README.txt
`

	filter := NewGlobFilter(false, nil, gitignoreContent)

	testCases := []struct {
		path     string
		expected bool
	}{
		// Log files
		{"debug.log", true},
		{"error.log", true},
		{"important.log", false}, // Negated

		// Build directories
		{"build/output.js", true},
		{"dist/app.js", true},

		// Node modules (anywhere)
		{"node_modules/package.json", true},
		{"project/node_modules/lib.js", true},

		// Temp files
		{"cache.tmp", true},
		{"src/temp.tmp", true},

		// Text files with negation
		{"file.txt", true},
		{"README.txt", false}, // Negated

		// Non-matching paths
		{"file.go", false},
		{"src/main.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			result := filter.Matches(tc.path)
			assert.Equal(t, tc.expected, result, "path: %s", tc.path)
		})
	}
}

func TestGlobFilter_GitignoreWithPatterns(t *testing.T) {
	gitignoreContent := `*.tmp
build/`

	patterns := []string{"*.go", "*.txt"}

	filter := NewGlobFilter(false, patterns, gitignoreContent)

	testCases := []struct {
		path     string
		expected bool
	}{
		// From patterns
		{"file.go", true},
		{"file.txt", true},

		// From gitignore
		{"cache.tmp", true},
		{"build/app.js", true},

		// Non-matching
		{"file.md", false},
		{"src/readme.md", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			result := filter.Matches(tc.path)
			assert.Equal(t, tc.expected, result, "path: %s", tc.path)
		})
	}
}

func TestGlobFilter_EmptyLinesAndComments(t *testing.T) {
	gitignoreContent := `
# Comment line
   # Indented comment

*.log

   
# Another comment
*.tmp
`

	filter := NewGlobFilter(false, nil, gitignoreContent)

	assert.True(t, filter.Matches("debug.log"))
	assert.True(t, filter.Matches("cache.tmp"))
	assert.False(t, filter.Matches("file.go"))
}

func TestGlobFilter_PathNormalization(t *testing.T) {
	filter := NewGlobFilter(false, []string{"dir/file.txt"})

	testCases := []struct {
		path     string
		expected bool
	}{
		{"dir/file.txt", true},
		{"/dir/file.txt", true}, // Leading slash removed
		{"dir/other.txt", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			result := filter.Matches(tc.path)
			assert.Equal(t, tc.expected, result, "path: %s", tc.path)
		})
	}
}

func TestGlobFilter_ComplexScenario(t *testing.T) {
	// Simulate a real .gitignore with various patterns
	gitignoreContent := `# Dependencies
node_modules/
vendor/

# Build outputs
*.o
*.a
*.so
build/
dist/

# IDE files
.vscode/
.idea/
*.swp

# Keep specific files
!vendor/important.go
!.vscode/settings.json

# Logs
*.log
!debug.log

# Temporary
**/*.tmp
**/tmp/
`

	filter := NewGlobFilter(false, nil, gitignoreContent)

	testCases := []struct {
		path     string
		expected bool
		reason   string
	}{
		// Dependencies
		{"node_modules/package.json", true, "node_modules excluded"},
		{"vendor/lib.go", true, "vendor excluded"},
		{"vendor/important.go", false, "vendor/important.go re-included"},

		// Build outputs
		{"main.o", true, "*.o excluded"},
		{"lib.a", true, "*.a excluded"},
		{"build/app", true, "build/ excluded"},
		{"dist/bundle.js", true, "dist/ excluded"},

		// IDE files
		{".vscode/launch.json", true, ".vscode/ excluded"},
		{".vscode/settings.json", false, ".vscode/settings.json re-included"},
		{".idea/workspace.xml", true, ".idea/ excluded"},
		{"file.swp", true, "*.swp excluded"},

		// Logs
		{"app.log", true, "*.log excluded"},
		{"debug.log", false, "debug.log re-included"},

		// Temporary
		{"cache.tmp", true, "**/*.tmp excluded"},
		{"src/temp.tmp", true, "**/*.tmp excluded"},
		{"tmp/data.txt", true, "**/tmp/ excluded"},
		{"src/tmp/cache", true, "**/tmp/ excluded"},

		// Source files (not excluded)
		{"main.go", false, "not matched"},
		{"src/lib.go", false, "not matched"},
		{"README.md", false, "not matched"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			result := filter.Matches(tc.path)
			assert.Equal(t, tc.expected, result, "%s - path: %s", tc.reason, tc.path)
		})
	}
}

func TestParseGlobPattern(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected *globPattern
	}{
		{"simple pattern", "*.go", &globPattern{pattern: "*.go", negate: false}},
		{"negation pattern", "!*.log", &globPattern{pattern: "*.log", negate: true}},
		{"comment", "# comment", nil},
		{"empty line", "", nil},
		{"whitespace only", "   ", nil},
		{"pattern with whitespace", "  *.txt  ", &globPattern{pattern: "*.txt", negate: false}},
		{"negation with whitespace", "  !*.log  ", &globPattern{pattern: "*.log", negate: true}},
		{"negation with space after !", "! *.log", &globPattern{pattern: "*.log", negate: true}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseGlobPattern(tc.input)
			if tc.expected == nil {
				assert.Nil(t, result, "input: %q", tc.input)
			} else {
				assert.NotNil(t, result, "input: %q", tc.input)
				assert.Equal(t, tc.expected.pattern, result.pattern, "input: %q", tc.input)
				assert.Equal(t, tc.expected.negate, result.negate, "input: %q", tc.input)
			}
		})
	}
}

func TestGlobFilter_PatternOrdering(t *testing.T) {
	// Test that later patterns override earlier ones
	testCases := []struct {
		name     string
		patterns []string
		path     string
		expected bool
	}{
		{
			name:     "include then exclude",
			patterns: []string{"*.txt", "!secret.txt"},
			path:     "secret.txt",
			expected: false,
		},
		{
			name:     "exclude then include then exclude again",
			patterns: []string{"*.txt", "!secret.txt", "secret.txt"},
			path:     "secret.txt",
			expected: true,
		},
		{
			name:     "multiple overrides",
			patterns: []string{"*.log", "!debug.log", "debug.log", "!debug.log"},
			path:     "debug.log",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewGlobFilter(false, tc.patterns)
			result := filter.Matches(tc.path)
			assert.Equal(t, tc.expected, result, "patterns: %v, path: %s", tc.patterns, tc.path)
		})
	}
}

func TestGlobFilter_DefaultMatch(t *testing.T) {
	testCases := []struct {
		name         string
		defaultMatch bool
		patterns     []string
		tests        map[string]bool
	}{
		{
			name:         "empty patterns with defaultMatch true",
			defaultMatch: true,
			patterns:     nil,
			tests: map[string]bool{
				"any/file.txt": true,
				"another.go":   true,
				"path/to/file": true,
			},
		},
		{
			name:         "empty patterns with defaultMatch false",
			defaultMatch: false,
			patterns:     nil,
			tests: map[string]bool{
				"any/file.txt": false,
				"another.go":   false,
				"path/to/file": false,
			},
		},
		{
			name:         "non-matching paths with defaultMatch true",
			defaultMatch: true,
			patterns:     []string{"*.txt"},
			tests: map[string]bool{
				"file.txt":    true, // matches pattern
				"file.go":     true, // doesn't match, returns defaultMatch=true
				"dir/file.md": true, // doesn't match, returns defaultMatch=true
			},
		},
		{
			name:         "non-matching paths with defaultMatch false",
			defaultMatch: false,
			patterns:     []string{"*.txt"},
			tests: map[string]bool{
				"file.txt":    true,  // matches pattern
				"file.go":     false, // doesn't match, returns defaultMatch=false
				"dir/file.md": false, // doesn't match, returns defaultMatch=false
			},
		},
		{
			name:         "mixed patterns with defaultMatch true",
			defaultMatch: true,
			patterns:     []string{"*.go", "!test.go"},
			tests: map[string]bool{
				"main.go":  true,  // matches *.go
				"test.go":  false, // matches *.go but negated by !test.go
				"file.txt": true,  // doesn't match any pattern, returns defaultMatch=true
			},
		},
		{
			name:         "mixed patterns with defaultMatch false",
			defaultMatch: false,
			patterns:     []string{"*.go", "!test.go"},
			tests: map[string]bool{
				"main.go":  true,  // matches *.go
				"test.go":  false, // matches *.go but negated by !test.go
				"file.txt": false, // doesn't match any pattern, returns defaultMatch=false
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewGlobFilter(tc.defaultMatch, tc.patterns)
			for path, expected := range tc.tests {
				result := filter.Matches(path)
				assert.Equal(t, expected, result, "path: %s, defaultMatch: %v", path, tc.defaultMatch)
			}
		})
	}
}

func TestGlobFilter_ExcludeAllOptimization(t *testing.T) {
	testCases := []struct {
		name     string
		patterns []string
		tests    map[string]bool
		comment  string
	}{
		{
			name:     "!** alone excludes everything",
			patterns: []string{"!**"},
			tests: map[string]bool{
				"file.txt":         false,
				"dir/file.go":      false,
				"any/path/here.md": false,
			},
			comment: "!** should exclude all files",
		},
		{
			name:     "!** discards all previous patterns",
			patterns: []string{"*.txt", "*.go", "!**"},
			tests: map[string]bool{
				"file.txt":    false,
				"file.go":     false,
				"file.md":     false,
				"dir/test.go": false,
			},
			comment: "All patterns before !** should be discarded",
		},
		{
			name:     "patterns after !** are still effective",
			patterns: []string{"*.txt", "!**", "*.go"},
			tests: map[string]bool{
				"file.txt":    false, // excluded by !**
				"file.go":     true,  // included by *.go after !**
				"file.md":     false, // not matched
				"dir/test.go": true,  // included by *.go after !**
			},
			comment: "Patterns after !** should still work",
		},
		{
			name:     "multiple !** - last one wins",
			patterns: []string{"*.txt", "!**", "*.go", "!**"},
			tests: map[string]bool{
				"file.txt": false,
				"file.go":  false,
				"file.md":  false,
			},
			comment: "Second !** should discard all previous patterns including first !** and *.go",
		},
		{
			name:     "!** followed by negation patterns",
			patterns: []string{"*.txt", "!**", "**", "!test.go"},
			tests: map[string]bool{
				"file.txt":    true,  // matched by **
				"file.go":     true,  // matched by **
				"test.go":     false, // matched by ** but excluded by !test.go
				"dir/test.go": false, // matched by ** but excluded by !test.go
			},
			comment: "!** followed by ** and negation should work correctly",
		},
		{
			name:     "complex scenario with !**",
			patterns: []string{"**/*.go", "!vendor/**", "!**", "*.txt", "!secret.txt"},
			tests: map[string]bool{
				"file.go":        false, // *.go excluded by !**
				"vendor/lib.go":  false, // excluded by !**
				"file.txt":       true,  // matched by *.txt
				"secret.txt":     false, // matched by *.txt but excluded by !secret.txt
				"dir/readme.txt": true,  // matched by *.txt
				"dir/secret.txt": false, // matched by *.txt but excluded by !secret.txt
				"file.md":        false, // not matched
			},
			comment: "All patterns before !** should be discarded, only *.txt and !secret.txt apply",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewGlobFilter(false, tc.patterns)
			for path, expected := range tc.tests {
				result := filter.Matches(path)
				assert.Equal(t, expected, result, "%s - path: %s", tc.comment, path)
			}
		})
	}
}
