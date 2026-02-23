package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sessionAgentTestPromptGenerator struct {
	vfs vfs.VFS
}

func (g *sessionAgentTestPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	return "", nil
}

func (g *sessionAgentTestPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error) {
	return tool.ToolInfo{}, nil
}

func (g *sessionAgentTestPromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	result := make(map[string]string)
	path := filepath.Join(dir, "AGENTS.md")
	content, err := g.vfs.ReadFile(path)
	if err != nil {
		if strings.Contains(err.Error(), vfs.ErrFileNotFound.Error()) {
			return result, nil
		}
		return nil, err
	}
	result[path] = string(content)
	return result, nil
}

func TestParseDirsFromGrepResult(t *testing.T) {
	t.Run("extracts directories from grep output lines", func(t *testing.T) {
		content := "src/main.go:10\nsrc/pkg/file.go:2\nREADME.md:1"
		dirs := parseDirsFromGrepResult(content)
		assert.Equal(t, []string{"src", "src/pkg", "."}, dirs)
	})

	t.Run("ignores non-match and truncation lines", func(t *testing.T) {
		content := "No files found\n(Results are truncated. Consider using a more specific path or pattern.)\nsrc/main.go:3"
		dirs := parseDirsFromGrepResult(content)
		assert.Equal(t, []string{"src"}, dirs)
	})
}

func TestBuildAdditionalAgentMessageForDir(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	require.NoError(t, mockVFS.WriteFile("src/AGENTS.md", []byte("subdir instructions")))

	session := &SweSession{
		system: &SweSystem{
			PromptGenerator: &sessionAgentTestPromptGenerator{vfs: mockVFS},
		},
		VFS:     mockVFS,
		workDir: ".",
	}

	t.Run("loads message once for directory", func(t *testing.T) {
		msg, err := session.buildAdditionalAgentMessageForDir("src")
		require.NoError(t, err)
		require.NotNil(t, msg)
		assert.Contains(t, msg.GetText(), "<system>")
		assert.Contains(t, msg.GetText(), "subdir instructions")

		nextMsg, err := session.buildAdditionalAgentMessageForDir("src")
		require.NoError(t, err)
		assert.Nil(t, nextMsg)
	})

	t.Run("ignores root directory", func(t *testing.T) {
		msg, err := session.buildAdditionalAgentMessageForDir(".")
		require.NoError(t, err)
		assert.Nil(t, msg)
	})
}
