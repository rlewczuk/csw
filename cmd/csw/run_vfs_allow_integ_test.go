package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVFSAllowPaths(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "single path",
			input:    []string{"/path/to/dir"},
			expected: []string{"/path/to/dir"},
		},
		{
			name:     "multiple flags",
			input:    []string{"/path/one", "/path/two"},
			expected: []string{"/path/one", "/path/two"},
		},
		{
			name:     "colon separated",
			input:    []string{"/path/one:/path/two"},
			expected: []string{"/path/one", "/path/two"},
		},
		{
			name:     "mixed flags and colon separated",
			input:    []string{"/path/one:/path/two", "/path/three"},
			expected: []string{"/path/one", "/path/two", "/path/three"},
		},
		{
			name:     "paths with spaces",
			input:    []string{"  /path/one  :  /path/two  "},
			expected: []string{"/path/one", "/path/two"},
		},
		{
			name:     "empty strings in colon separated",
			input:    []string{"/path/one::/path/two"},
			expected: []string{"/path/one", "/path/two"},
		},
		{
			name:     "single empty string",
			input:    []string{""},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVFSAllowPaths(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCLIVFSAllowIntegration(t *testing.T) {
	// Create temporary directories to use as allowed paths
	allowedDir1, err := os.MkdirTemp("", "csw-vfs-allow-1-*")
	require.NoError(t, err)
	defer os.RemoveAll(allowedDir1)

	allowedDir2, err := os.MkdirTemp("", "csw-vfs-allow-2-*")
	require.NoError(t, err)
	defer os.RemoveAll(allowedDir2)

	// Create test files in allowed directories
	err = os.WriteFile(filepath.Join(allowedDir1, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(allowedDir2, "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	t.Run("VFSAllowPathsParsedAndPassedToRuntime", func(t *testing.T) {
		// This test verifies that the VFSAllow paths are correctly parsed and
		// passed through RunParams to runtime code

		// Create a mock runFunc to capture the params
		var capturedParams *RunParams
		originalRunCLIFunc := runFunc
		runFunc = func(params *RunParams) error {
			capturedParams = params
			return nil
		}
		defer func() {
			runFunc = originalRunCLIFunc
		}()

		// Get the CLI command
		cmd := RunCommand()

		// Execute with vfs-allow flags
		cmd.SetArgs([]string{
			"--vfs-allow", allowedDir1,
			"--vfs-allow", allowedDir2,
			"test prompt",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify that VFSAllow contains both paths
		require.NotNil(t, capturedParams)
		assert.Len(t, capturedParams.VFSAllow, 2)
		assert.Contains(t, capturedParams.VFSAllow, allowedDir1)
		assert.Contains(t, capturedParams.VFSAllow, allowedDir2)
	})

	t.Run("VFSAllowColonSeparated", func(t *testing.T) {
		// This test verifies that colon-separated paths work correctly

		var capturedParams *RunParams
		originalRunCLIFunc := runFunc
		runFunc = func(params *RunParams) error {
			capturedParams = params
			return nil
		}
		defer func() {
			runFunc = originalRunCLIFunc
		}()

		cmd := RunCommand()

		// Execute with colon-separated vfs-allow flag
		cmd.SetArgs([]string{
			"--vfs-allow", allowedDir1 + ":" + allowedDir2,
			"test prompt",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify that VFSAllow contains both paths
		require.NotNil(t, capturedParams)
		assert.Len(t, capturedParams.VFSAllow, 2)
		assert.Contains(t, capturedParams.VFSAllow, allowedDir1)
		assert.Contains(t, capturedParams.VFSAllow, allowedDir2)
	})

	t.Run("VFSAllowMixedFlagsAndColon", func(t *testing.T) {
		// Create a third directory
		allowedDir3, err := os.MkdirTemp("", "csw-vfs-allow-3-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir3)

		var capturedParams *RunParams
		originalRunCLIFunc := runFunc
		runFunc = func(params *RunParams) error {
			capturedParams = params
			return nil
		}
		defer func() {
			runFunc = originalRunCLIFunc
		}()

		cmd := RunCommand()

		// Execute with mixed flags and colon-separated
		cmd.SetArgs([]string{
			"--vfs-allow", allowedDir1 + ":" + allowedDir2,
			"--vfs-allow", allowedDir3,
			"test prompt",
		})

		err = cmd.Execute()
		require.NoError(t, err)

		// Verify that VFSAllow contains all three paths
		require.NotNil(t, capturedParams)
		assert.Len(t, capturedParams.VFSAllow, 3)
		assert.Contains(t, capturedParams.VFSAllow, allowedDir1)
		assert.Contains(t, capturedParams.VFSAllow, allowedDir2)
		assert.Contains(t, capturedParams.VFSAllow, allowedDir3)
	})
}

func TestVFSAllowFlagInHelp(t *testing.T) {
	cmd := RunCommand()

	// Capture help output
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	helpOutput := buf.String()

	// Verify that --vfs-allow is mentioned in help
	assert.Contains(t, helpOutput, "--vfs-allow")
	assert.Contains(t, helpOutput, "Additional path to allow VFS access")
	assert.Contains(t, helpOutput, "--shadow-dir")
}

// require is a helper to assert no error
func requireNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
