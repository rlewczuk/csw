package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBashRunner_RunCommand(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "bash-runner-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		command        string
		timeout        time.Duration
		wantOutputPart string
		wantExitCode   int
		wantErr        bool
		errContains    string
	}{
		{
			name:           "simple echo command",
			command:        "echo 'hello world'",
			timeout:        5 * time.Second,
			wantOutputPart: "hello world",
			wantExitCode:   0,
			wantErr:        false,
		},
		{
			name:           "command with exit code 0",
			command:        "exit 0",
			timeout:        5 * time.Second,
			wantOutputPart: "",
			wantExitCode:   0,
			wantErr:        false,
		},
		{
			name:           "command with non-zero exit code",
			command:        "exit 42",
			timeout:        5 * time.Second,
			wantOutputPart: "",
			wantExitCode:   42,
			wantErr:        false,
		},
		{
			name:           "command writing to stderr",
			command:        "echo 'error message' >&2",
			timeout:        5 * time.Second,
			wantOutputPart: "error message",
			wantExitCode:   0,
			wantErr:        false,
		},
		{
			name:           "command writing to both stdout and stderr",
			command:        "echo 'stdout'; echo 'stderr' >&2",
			timeout:        5 * time.Second,
			wantOutputPart: "stdout",
			wantExitCode:   0,
			wantErr:        false,
		},
		{
			name:         "empty command",
			command:      "",
			timeout:      5 * time.Second,
			wantExitCode: 1,
			wantErr:      true,
			errContains:  "command cannot be empty",
		},
		{
			name:         "command timeout",
			command:      "sleep 10",
			timeout:      100 * time.Millisecond,
			wantExitCode: 124,
			wantErr:      true,
			errContains:  "command timed out",
		},
		{
			name:           "multiline command",
			command:        "echo 'line1'\necho 'line2'",
			timeout:        5 * time.Second,
			wantOutputPart: "line1",
			wantExitCode:   0,
			wantErr:        false,
		},
		{
			name:           "command with pipes",
			command:        "echo 'hello' | tr 'h' 'H'",
			timeout:        5 * time.Second,
			wantOutputPart: "Hello",
			wantExitCode:   0,
			wantErr:        false,
		},
		{
			name:           "command with environment variables",
			command:        "export VAR=test; echo $VAR",
			timeout:        5 * time.Second,
			wantOutputPart: "test",
			wantExitCode:   0,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewBashRunner(tmpDir, tt.timeout)

			output, exitCode, err := runner.RunCommand(tt.command)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantExitCode, exitCode)

			if tt.wantOutputPart != "" {
				assert.Contains(t, output, tt.wantOutputPart)
			}
		})
	}
}

func TestBashRunner_WorkingDirectory(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "bash-runner-workdir-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test file in the temp directory
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	runner := NewBashRunner(tmpDir, 5*time.Second)

	// Test that pwd returns the working directory
	output, exitCode, err := runner.RunCommand("pwd")
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, strings.TrimSpace(output), tmpDir)

	// Test that we can read files relative to the working directory
	output, exitCode, err = runner.RunCommand("cat test.txt")
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "test content")
}

func TestBashRunner_NoTimeout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bash-runner-notimeout-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	runner := NewBashRunner(tmpDir, 0)

	output, exitCode, err := runner.RunCommand("echo 'no timeout'")
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "no timeout")
}

func TestBashRunner_LongRunningCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bash-runner-long-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	runner := NewBashRunner(tmpDir, 2*time.Second)

	start := time.Now()
	output, exitCode, err := runner.RunCommand("sleep 0.5 && echo 'done'")
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "done")
	assert.Less(t, duration, 2*time.Second)
}

func TestBashRunner_ComplexScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bash-runner-script-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	runner := NewBashRunner(tmpDir, 5*time.Second)

	script := `
#!/bin/bash
count=0
for i in {1..5}; do
    count=$((count + i))
done
echo "Sum: $count"
`

	output, exitCode, err := runner.RunCommand(script)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Sum: 15")
}

func TestBashRunner_ErrorHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bash-runner-error-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		command        string
		wantExitCode   int
		wantOutputPart string
	}{
		{
			name:           "command not found",
			command:        "nonexistentcommand123456",
			wantExitCode:   127,
			wantOutputPart: "not found",
		},
		{
			name:           "syntax error",
			command:        "if",
			wantExitCode:   2,
			wantOutputPart: "",
		},
		{
			name:           "file not found",
			command:        "cat /nonexistent/file.txt",
			wantExitCode:   1,
			wantOutputPart: "No such file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewBashRunner(tmpDir, 5*time.Second)

			output, exitCode, err := runner.RunCommand(tt.command)

			// Non-zero exit codes don't cause RunCommand to return error
			// The error is captured in the exit code
			assert.NoError(t, err)

			assert.Equal(t, tt.wantExitCode, exitCode)

			if tt.wantOutputPart != "" {
				assert.Contains(t, output, tt.wantOutputPart)
			}
		})
	}
}

func TestBashRunner_ConcurrentExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bash-runner-concurrent-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	runner := NewBashRunner(tmpDir, 5*time.Second)

	// Run multiple commands concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			output, exitCode, err := runner.RunCommand("echo 'test'")
			assert.NoError(t, err)
			assert.Equal(t, 0, exitCode)
			assert.Contains(t, output, "test")
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent execution timed out")
		}
	}
}
