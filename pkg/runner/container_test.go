package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/rlewczuk/csw/pkg/testutil/cfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shouldRunContainerTests checks if container integration tests should run.
// Returns true if _integ/runc.enabled or _integ/all.enabled exists and contains "yes".
func shouldRunContainerTests(t *testing.T) bool {
	t.Helper()
	return cfg.TestEnabled("runc")
}

// getProjectRoot returns the project root directory.
func getProjectRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	// Go up from pkg/runner to project root
	root := filepath.Join(wd, "../..")
	absRoot, err := filepath.Abs(root)
	require.NoError(t, err)

	return absRoot
}

// containerExists checks if a container with the given ID exists.
func containerExists(ctx context.Context, containerID string) (bool, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return false, err
	}
	defer cli.Close()

	_, err = cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// getContainerID extracts the container ID from a running container runner.
func getContainerID(runner ContainerRunner) (string, error) {
	cr, ok := runner.(*containerRunner)
	if !ok {
		return "", fmt.Errorf("runner is not a containerRunner")
	}
	return cr.container.GetContainerID(), nil
}

// TestContainerRunnerBasic tests basic container runner functionality.
func TestContainerRunnerBasic(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Run a simple command
	output, exitCode, err := runner.RunCommand("echo hello")
	require.NoError(t, err, "Failed to run command")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, strings.TrimSpace(output), "hello")
}

// TestContainerRunnerMountDirectory tests directory mounting in container.
func TestContainerRunnerMountDirectory(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	projectRoot := getProjectRoot(t)

	// Ensure tmp directory exists
	tmpDir := filepath.Join(projectRoot, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create tmp directory: %v", err)
	}

	// Create a test directory with a file
	testDir := cfg.MkTempDir(t, projectRoot, "container_mount_test_*")
	testFile := filepath.Join(testDir, "test.txt")
	testContent := "hello from host"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err, "Failed to create test file")

	// Create container with mount
	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
		MountDirs: map[string]string{
			"/mnt/test": testDir,
		},
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Check if the file is accessible from within container
	output, exitCode, err := runner.RunCommand("cat /mnt/test/test.txt")
	require.NoError(t, err, "Failed to run command")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, strings.TrimSpace(output), testContent)
}

// TestContainerRunnerWorkdir tests working directory option.
func TestContainerRunnerWorkdir(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Run command with workdir
	output, exitCode, err := runner.RunCommandWithOptions("pwd", CommandOptions{
		Workdir: "/tmp",
	})
	require.NoError(t, err, "Failed to run command")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, strings.TrimSpace(output), "/tmp")
}

// TestContainerRunnerTimeout tests command timeout.
func TestContainerRunnerTimeout(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Run command with short timeout
	_, exitCode, err := runner.RunCommandWithOptions("sleep 10", CommandOptions{
		Timeout: 1 * time.Second,
	})
	require.Error(t, err, "Expected timeout error")
	assert.Equal(t, 124, exitCode, "Expected exit code 124 for timeout")
	assert.Contains(t, err.Error(), "timed out")
}

// TestContainerRunnerEmptyCommand tests empty command error.
func TestContainerRunnerEmptyCommand(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Run empty command
	_, exitCode, err := runner.RunCommand("")
	require.Error(t, err, "Expected error for empty command")
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, err.Error(), "command cannot be empty")
}

// TestContainerRunnerClosedRunner tests running command on closed runner.
func TestContainerRunnerClosedRunner(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")

	// Close the runner
	err = runner.Close()
	require.NoError(t, err, "Failed to close runner")

	// Try to run command on closed runner
	_, exitCode, err := runner.RunCommand("echo hello")
	require.Error(t, err, "Expected error on closed runner")
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, err.Error(), "container is closed")
}

// TestContainerRunnerCloseIdempotent tests that Close is idempotent.
func TestContainerRunnerCloseIdempotent(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")

	// Close multiple times
	for i := 0; i < 3; i++ {
		err = runner.Close()
		require.NoError(t, err, "Close should not fail on repeated calls")
	}
}

// TestContainerRunnerRemoval verifies container is removed after Close.
func TestContainerRunnerRemoval(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")

	// Get container ID before closing
	containerID, err := getContainerID(runner)
	require.NoError(t, err, "Failed to get container ID")

	// Verify container exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := containerExists(ctx, containerID)
	require.NoError(t, err, "Failed to check container existence")
	assert.True(t, exists, "Container should exist before Close")

	// Close the runner
	err = runner.Close()
	require.NoError(t, err, "Failed to close runner")

	// Verify container is removed
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	exists, err = containerExists(ctx2, containerID)
	require.NoError(t, err, "Failed to check container existence after Close")
	assert.False(t, exists, "Container should be removed after Close")
}

// TestContainerRunnerMultipleCommands tests running multiple commands.
func TestContainerRunnerMultipleCommands(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Run multiple commands
	tests := []struct {
		cmd      string
		expected string
	}{
		{"echo one", "one"},
		{"echo two", "two"},
		{"expr 1 + 1", "2"},
	}

	for _, tc := range tests {
		output, exitCode, err := runner.RunCommand(tc.cmd)
		require.NoError(t, err, "Failed to run command: %s", tc.cmd)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, strings.TrimSpace(output), tc.expected)
	}
}

// TestContainerRunnerExitCode tests non-zero exit codes.
func TestContainerRunnerExitCode(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Run command that exits with non-zero code
	output, exitCode, err := runner.RunCommand("exit 42")
	// Non-zero exit code doesn't return error
	require.NoError(t, err, "Non-zero exit code should not return error")
	assert.Equal(t, 42, exitCode)
	_ = output
}

// TestContainerRunnerInvalidImage tests error handling for invalid image.
func TestContainerRunnerInvalidImage(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	_, err := NewContainerRunner(ContainerConfig{
		ImageName: "nonexistent:image",
	})
	require.Error(t, err, "Expected error for invalid image")
	assert.Contains(t, err.Error(), "failed to create container")
}

// TestContainerRunnerEmptyImageName tests error handling for empty image name.
func TestContainerRunnerEmptyImageName(t *testing.T) {
	_, err := NewContainerRunner(ContainerConfig{
		ImageName: "",
	})
	require.Error(t, err, "Expected error for empty image name")
	assert.Contains(t, err.Error(), "image name cannot be empty")
}

// TestContainerRunnerWithUIDGID tests that commands run as specified UID/GID.
func TestContainerRunnerWithUIDGID(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	// Create container with specific UID/GID
	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
		UID:       1000,
		GID:       1000,
		UserName:  "testuser",
		GroupName: "testgroup",
		HomeDir:   "/home/testuser",
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Verify command runs as UID 1000
	output, exitCode, err := runner.RunCommand("id -u")
	require.NoError(t, err, "Failed to run command")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, strings.TrimSpace(output), "1000")

	// Verify command runs as GID 1000
	output, exitCode, err = runner.RunCommand("id -g")
	require.NoError(t, err, "Failed to run command")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, strings.TrimSpace(output), "1000")
}

// TestContainerRunnerWithUIDGIDHomeDirWritable tests that home directory is writable by non-root user.
func TestContainerRunnerWithUIDGIDHomeDirWritable(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	projectRoot := getProjectRoot(t)

	// Ensure tmp directory exists
	tmpDir := filepath.Join(projectRoot, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create tmp directory: %v", err)
	}

	// Create a test directory that simulates a user's home directory structure
	testDir := cfg.MkTempDir(t, projectRoot, "container_homedir_test_*")
	containerPath := testDir // Mount at same path

	// Create container with specific UID/GID and mount at same path
	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
		Workdir:   containerPath,
		MountDirs: map[string]string{
			containerPath: testDir,
		},
		UID:            1000,
		GID:            1000,
		UserName:       "testuser",
		GroupName:      "testgroup",
		HomeDir:        filepath.Dir(containerPath),
		ReadOnlyMounts: false,
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Verify the parent directory of workdir (simulating home dir) is writable
	// by creating a file in the parent directory
	parentDir := filepath.Dir(containerPath)
	testFilePath := filepath.Join(parentDir, "test_write.txt")

	output, exitCode, err := runner.RunCommand(fmt.Sprintf("touch %q && echo 'success' > %q && cat %q", testFilePath, testFilePath, testFilePath))
	require.NoError(t, err, "Failed to write to parent directory")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, strings.TrimSpace(output), "success")
}

// TestContainerRunnerWithoutUIDGID runs as root by default.
func TestContainerRunnerWithoutUIDGID(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	// Create container without UID/GID (should run as root)
	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	// Verify command runs as root (UID 0)
	output, exitCode, err := runner.RunCommand("id -u")
	require.NoError(t, err, "Failed to run command")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, strings.TrimSpace(output), "0")
}

// TestContainerRunnerEnv verifies configured environment variables are available in container commands.
func TestContainerRunnerEnv(t *testing.T) {
	if !shouldRunContainerTests(t) {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	runner, err := NewContainerRunner(ContainerConfig{
		ImageName: "busybox:latest",
		Env: map[string]string{
			"GIT_AUTHOR_NAME":  "Test User",
			"GIT_AUTHOR_EMAIL": "test@example.com",
		},
	})
	require.NoError(t, err, "Failed to create container runner")
	defer runner.Close()

	output, exitCode, err := runner.RunCommand("printf '%s|%s' \"$GIT_AUTHOR_NAME\" \"$GIT_AUTHOR_EMAIL\"")
	require.NoError(t, err, "Failed to run command")
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "Test User|test@example.com", strings.TrimSpace(output))
}

func TestParseContainerImageInfo(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		expected  ContainerImageInfo
	}{
		{
			name:      "name with explicit tag",
			reference: "busybox:1.36",
			expected: ContainerImageInfo{
				Reference: "busybox:1.36",
				Name:      "busybox",
				Tag:       "1.36",
				Version:   "1.36",
			},
		},
		{
			name:      "name without tag defaults to latest",
			reference: "busybox",
			expected: ContainerImageInfo{
				Reference: "busybox",
				Name:      "busybox",
				Tag:       "latest",
				Version:   "latest",
			},
		},
		{
			name:      "registry host with port and tag",
			reference: "registry.local:5000/csw/runtime:v2",
			expected: ContainerImageInfo{
				Reference: "registry.local:5000/csw/runtime:v2",
				Name:      "registry.local:5000/csw/runtime",
				Tag:       "v2",
				Version:   "v2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseContainerImageInfo(tt.reference))
		})
	}
}

func TestParseMappedIdentityOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		expectError string
		expected    ContainerIdentity
	}{
		{
			name:   "parses mapped identity marker",
			output: "setup logs\nCSW_IDENTITY\t1000\t50\talice\tstaff\t/home/alice\n",
			expected: ContainerIdentity{
				UID:       1000,
				GID:       50,
				UserName:  "alice",
				GroupName: "staff",
				HomeDir:   "/home/alice",
			},
		},
		{
			name:   "no user group in container maps to requested identity",
			output: "CSW_IDENTITY|1001|1002|hostuser|hostgroup|/home/hostuser\n",
			expected: ContainerIdentity{
				UID:       1001,
				GID:       1002,
				UserName:  "hostuser",
				GroupName: "hostgroup",
				HomeDir:   "/home/hostuser",
			},
		},
		{
			name:   "existing user group with same ids and same names",
			output: "CSW_IDENTITY|1001|1002|hostuser|hostgroup|/home/hostuser\n",
			expected: ContainerIdentity{
				UID:       1001,
				GID:       1002,
				UserName:  "hostuser",
				GroupName: "hostgroup",
				HomeDir:   "/home/hostuser",
			},
		},
		{
			name:   "existing user group with same ids and different names",
			output: "CSW_IDENTITY|1001|1002|containeruser|containergroup|/home/containeruser\n",
			expected: ContainerIdentity{
				UID:       1001,
				GID:       1002,
				UserName:  "containeruser",
				GroupName: "containergroup",
				HomeDir:   "/home/containeruser",
			},
		},
		{
			name:   "parses mapped identity marker with pipe delimiter",
			output: "setup logs\nCSW_IDENTITY|1000|50|alice|staff|/home/alice\n",
			expected: ContainerIdentity{
				UID:       1000,
				GID:       50,
				UserName:  "alice",
				GroupName: "staff",
				HomeDir:   "/home/alice",
			},
		},
		{
			name:   "parses mapped identity marker with escaped tab delimiter",
			output: "setup logs\nCSW_IDENTITY\\t1000\\t50\\talice\\tstaff\\t/home/alice\n",
			expected: ContainerIdentity{
				UID:       1000,
				GID:       50,
				UserName:  "alice",
				GroupName: "staff",
				HomeDir:   "/home/alice",
			},
		},
		{
			name:   "parses marker when prefixed by other output in same line",
			output: "setup logs without newline...CSW_IDENTITY|1000|50|alice|staff|/home/alice\n",
			expected: ContainerIdentity{
				UID:       1000,
				GID:       50,
				UserName:  "alice",
				GroupName: "staff",
				HomeDir:   "/home/alice",
			},
		},
		{
			name:        "missing marker returns error",
			output:      "no identity line\n",
			expectError: "identity marker not found",
		},
		{
			name:        "invalid uid returns error",
			output:      "CSW_IDENTITY\tnot-num\t50\talice\tstaff\t/home/alice\n",
			expectError: "invalid uid value",
		},
		{
			name:        "incomplete details returns error",
			output:      "CSW_IDENTITY\t1000\t50\t\tstaff\t/home/alice\n",
			expectError: "incomplete identity details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, err := parseMappedIdentityOutput(tt.output)
			if tt.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, identity)
		})
	}
}

func TestBuildMappedIdentitySetupScript(t *testing.T) {
	script := buildMappedIdentitySetupScript(ContainerConfig{
		UID:       1000,
		GID:       2000,
		UserName:  "alice",
		GroupName: "staff",
		HomeDir:   "/home/alice",
	})

	assert.Contains(t, script, "target_uid=1000")
	assert.Contains(t, script, "target_gid=2000")
	assert.Contains(t, script, "target_user=\"alice\"")
	assert.Contains(t, script, "target_group=\"staff\"")
	assert.Contains(t, script, "target_home=\"/home/alice\"")
	assert.Contains(t, script, "getent group \"$target_gid\"")
	assert.Contains(t, script, "getent group \"$target_group\"")
	assert.Contains(t, script, "getent passwd \"$target_uid\"")
	assert.Contains(t, script, "getent passwd \"$target_user\"")
	assert.Contains(t, script, "groupadd -g \"$target_gid\" \"$target_group\"")
	assert.Contains(t, script, "addgroup -g \"$target_gid\" \"$target_group\"")
	assert.Contains(t, script, "useradd -m -u \"$target_uid\" -g \"$effective_gid\" -d \"$target_home\" -s /bin/sh \"$target_user\"")
	assert.Contains(t, script, "adduser -D -u \"$target_uid\" -G \"$effective_group\" -h \"$target_home\" -s /bin/sh \"$target_user\"")
	assert.Contains(t, script, "chown -R $effective_uid:$effective_gid \"$target_home\"")
	assert.Contains(t, script, "CSW_IDENTITY|")
	assert.NotContains(t, script, "$effective_user_gid")
	assert.Contains(t, script, "CSW_IDENTITY")
}
