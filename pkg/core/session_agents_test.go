package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roleLimitedToolStub struct{}

func (s *roleLimitedToolStub) GetDescription() (string, bool) {
	return "", false
}

func (s *roleLimitedToolStub) Execute(args *tool.ToolCall) *tool.ToolResponse {
	return &tool.ToolResponse{Call: args, Done: true}
}

func (s *roleLimitedToolStub) Render(call *tool.ToolCall) (string, string, string, map[string]string) {
	return "stub", "stub", "{}", map[string]string{}
}

func (s *roleLimitedToolStub) IsRoleAllowed(roleName string) bool {
	return roleName == "developer"
}

type sessionAgentTestPromptGenerator struct {
	vfs apis.VFS
}

func (g *sessionAgentTestPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	return "", nil
}

func (g *sessionAgentTestPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error) {
	return tool.ToolInfo{}, nil
}

func (g *sessionAgentTestPromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	result := make(map[string]string)
	for current := filepath.Clean(dir); current != "." && current != ""; current = filepath.Dir(current) {
		path := filepath.Join(current, "AGENTS.md")
		content, err := g.vfs.ReadFile(path)
		if err != nil {
			if errors.Is(err, apis.ErrFileNotFound) {
				if filepath.Dir(current) == current {
					break
				}
				continue
			}
			return nil, err
		}
		result[path] = string(content)

		if filepath.Dir(current) == current {
			break
		}
	}
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
	require.NoError(t, mockVFS.WriteFile("pkg/AGENTS.md", []byte("pkg instructions")))
	require.NoError(t, mockVFS.WriteFile("pkg/foo/AGENTS.md", []byte("foo instructions")))
	require.NoError(t, mockVFS.WriteFile("pkg/foo/bar/AGENTS.md", []byte("bar instructions")))
	require.NoError(t, mockVFS.WriteFile("pkg/foo/baz/AGENTS.md", []byte("baz instructions")))
	require.NoError(t, mockVFS.WriteFile("AGENTS.md", []byte("root instructions")))

	session := &SweSession{
		promptGenerator: &sessionAgentTestPromptGenerator{vfs: mockVFS},
		VFS:             mockVFS,
		workDir:         ".",
	}

	t.Run("loads messages for directory and parents excluding root", func(t *testing.T) {
		msgs, loadedPaths, err := session.buildAdditionalAgentMessageForDir("pkg/foo/bar")
		require.NoError(t, err)
		require.Len(t, msgs, 3)
		require.Len(t, loadedPaths, 3)

		joined := strings.Builder{}
		for _, msg := range msgs {
			joined.WriteString(msg.GetText())
			joined.WriteString("\n")
			assert.Contains(t, msg.GetText(), "<system>")
			assert.Contains(t, msg.GetText(), "</system>")
		}

		joinedText := joined.String()
		assert.Contains(t, joinedText, "bar instructions")
		assert.Contains(t, joinedText, "foo instructions")
		assert.Contains(t, joinedText, "pkg instructions")
		assert.NotContains(t, joinedText, "root instructions")

		nextMsgs, nextLoadedPaths, err := session.buildAdditionalAgentMessageForDir("pkg/foo/bar")
		require.NoError(t, err)
		assert.Nil(t, nextMsgs)
		assert.Nil(t, nextLoadedPaths)
	})

	t.Run("deduplicates parent files across subsequent directory reads", func(t *testing.T) {
		freshSession := &SweSession{
			promptGenerator: &sessionAgentTestPromptGenerator{vfs: mockVFS},
			VFS:             mockVFS,
			workDir:         ".",
		}

		firstMsgs, firstLoadedPaths, err := freshSession.buildAdditionalAgentMessageForDir("pkg/foo/bar")
		require.NoError(t, err)
		require.Len(t, firstMsgs, 3)
		require.Len(t, firstLoadedPaths, 3)

		secondMsgs, secondLoadedPaths, err := freshSession.buildAdditionalAgentMessageForDir("pkg/foo/baz")
		require.NoError(t, err)
		require.Len(t, secondMsgs, 1)
		require.Len(t, secondLoadedPaths, 1)
		assert.Contains(t, secondMsgs[0].GetText(), "baz instructions")
		assert.Equal(t, "pkg/foo/baz/AGENTS.md", secondLoadedPaths[0])
	})

	t.Run("ignores root directory", func(t *testing.T) {
		msgs, loadedPaths, err := session.buildAdditionalAgentMessageForDir(".")
		require.NoError(t, err)
		assert.Nil(t, msgs)
		assert.Nil(t, loadedPaths)
	})
}

func TestFilterToolsForRole(t *testing.T) {
	registry := tool.NewToolRegistry()
	registry.Register("always", &roleLimitedToolStub{})
	registry.Register("open", tool.NewTodoReadTool(nil))

	dev := &conf.AgentRoleConfig{Name: "developer"}
	readonly := &conf.AgentRoleConfig{Name: "readonly"}

	devFiltered := filterToolsForRole(registry, dev)
	assert.Contains(t, devFiltered.List(), "always")
	assert.Contains(t, devFiltered.List(), "open")

	roFiltered := filterToolsForRole(registry, readonly)
	assert.NotContains(t, roFiltered.List(), "always")
	assert.Contains(t, roFiltered.List(), "open")
}
