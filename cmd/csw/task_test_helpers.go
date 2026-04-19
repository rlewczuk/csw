package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func setTaskStatusForTest(t *testing.T, taskPath string, status string) {
	t.Helper()

	contents, err := os.ReadFile(taskPath)
	require.NoError(t, err)

	taskData := &core.Task{}
	require.NoError(t, yaml.Unmarshal(contents, taskData))
	taskData.Status = status

	updatedContents, err := yaml.Marshal(taskData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(taskPath, updatedContents, 0o644))
}

func setTaskYMLModTimeForTest(t *testing.T, taskPath string, modTime time.Time) {
	t.Helper()
	require.NoError(t, os.Chtimes(taskPath, modTime, modTime))
}

func nonEmptyLines(input string) []string {
	raw := strings.Split(strings.TrimSpace(input), "\n")
	result := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		result = append(result, line)
	}
	return result
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer

	fn()

	require.NoError(t, writer.Close())
	os.Stdout = oldStdout

	var buffer bytes.Buffer
	_, readErr := buffer.ReadFrom(reader)
	require.NoError(t, readErr)
	require.NoError(t, reader.Close())

	return buffer.String()
}
