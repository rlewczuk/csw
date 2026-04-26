// session_core_integ_agent_files_test.go contains integration tests for AGENTS.md
// discovery and injection into chat context after tool calls.
package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type agentFilePromptGenerator struct {
	vfs apis.VFS
}

func (g *agentFilePromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	return "You are a helpful assistant.", nil
}

func (g *agentFilePromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{Name: toolName, Description: "Mock tool for testing", Schema: schema}, nil
}

func (g *agentFilePromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	result := make(map[string]string)
	path := filepath.Join(dir, "AGENTS.md")
	content, err := g.vfs.ReadFile(path)
	if err != nil {
		if strings.Contains(err.Error(), apis.ErrFileNotFound.Error()) {
			return result, nil
		}
		return nil, err
	}
	result[path] = string(content)
	return result, nil
}

func TestSessionAdditionalAgentFiles(t *testing.T) {
	t.Run("injects subdirectory AGENTS.md once across multiple vfsRead tool calls", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a helpful assistant.")
		system := fixture.system
		vfsInstance := fixture.vfs
		system.PromptGenerator = &agentFilePromptGenerator{vfs: vfsInstance}

		require.NoError(t, vfsInstance.WriteFile("src/AGENTS.md", []byte("Subdir instructions")))
		require.NoError(t, vfsInstance.WriteFile("src/a.txt", []byte("file a")))
		require.NoError(t, vfsInstance.WriteFile("src/b.txt", []byte("file b")))

		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model", Model: "test-model"}})
		mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{{ToolCall: &tool.ToolCall{
				ID:       "read-1",
				Function: "vfsRead",
				Arguments: tool.NewToolValue(map[string]any{
					"path": "src/a.txt",
				}),
			}}},
		}})
		mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{{ToolCall: &tool.ToolCall{
				ID:       "read-2",
				Function: "vfsRead",
				Arguments: tool.NewToolValue(map[string]any{
					"path": "src/b.txt",
				}),
			}}},
		}})
		mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
			Role:  models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{{Text: "done"}},
		}})

		system.ModelProviders = map[string]models.ModelProvider{"mock": mockProvider}

		session, err := system.NewSession("mock/test-model", testutil.NewMockSessionOutputHandler())
		require.NoError(t, err)
		require.NoError(t, session.UserPrompt("Read files"))
		require.NoError(t, session.Run(context.Background()))

		require.GreaterOrEqual(t, len(mockProvider.RecordedMessages), 3)
		systemMessageCount := 0
		for _, msg := range session.ChatMessages() {
			if msg.Role == models.ChatRoleUser && strings.Contains(msg.GetText(), "<system>") {
				systemMessageCount++
				assert.Contains(t, msg.GetText(), "Subdir instructions")
			}
		}
		assert.Equal(t, 1, systemMessageCount)
	})

	t.Run("injects AGENTS.md for vfsGrep matches", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a helpful assistant.")
		system := fixture.system
		vfsInstance := fixture.vfs
		system.PromptGenerator = &agentFilePromptGenerator{vfs: vfsInstance}

		require.NoError(t, vfsInstance.WriteFile("src/AGENTS.md", []byte("Grep instructions")))
		require.NoError(t, vfsInstance.WriteFile("src/main.go", []byte("package main\nfunc main(){}")))

		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model", Model: "test-model"}})
		mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{{ToolCall: &tool.ToolCall{
				ID:       "grep-1",
				Function: "vfsGrep",
				Arguments: tool.NewToolValue(map[string]any{
					"pattern": "main",
					"path":    "src",
				}),
			}}},
		}})
		mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
			Role:  models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{{Text: "done"}},
		}})

		system.ModelProviders = map[string]models.ModelProvider{"mock": mockProvider}

		session, err := system.NewSession("mock/test-model", testutil.NewMockSessionOutputHandler())
		require.NoError(t, err)
		require.NoError(t, session.UserPrompt("Search code"))
		require.NoError(t, session.Run(context.Background()))

		require.GreaterOrEqual(t, len(mockProvider.RecordedMessages), 2)
		found := false
		for _, msg := range mockProvider.RecordedMessages[1] {
			if msg.Role == models.ChatRoleUser && strings.Contains(msg.GetText(), "<system>") {
				found = true
				assert.Contains(t, msg.GetText(), "Grep instructions")
			}
		}
		assert.True(t, found)
	})

	t.Run("does not inject root AGENTS.md for root file reads", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a helpful assistant.")
		system := fixture.system
		vfsInstance := fixture.vfs
		system.PromptGenerator = &agentFilePromptGenerator{vfs: vfsInstance}

		require.NoError(t, vfsInstance.WriteFile("AGENTS.md", []byte("Root instructions")))
		require.NoError(t, vfsInstance.WriteFile("main.go", []byte("package main")))

		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model", Model: "test-model"}})
		mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{{ToolCall: &tool.ToolCall{
				ID:       "read-root",
				Function: "vfsRead",
				Arguments: tool.NewToolValue(map[string]any{
					"path": "main.go",
				}),
			}}},
		}})
		mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
			Role:  models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{{Text: "done"}},
		}})

		system.ModelProviders = map[string]models.ModelProvider{"mock": mockProvider}

		session, err := system.NewSession("mock/test-model", testutil.NewMockSessionOutputHandler())
		require.NoError(t, err)
		require.NoError(t, session.UserPrompt("Read root file"))
		require.NoError(t, session.Run(context.Background()))

		require.GreaterOrEqual(t, len(mockProvider.RecordedMessages), 2)
		for _, msg := range mockProvider.RecordedMessages[1] {
			assert.NotContains(t, msg.GetText(), "Root instructions")
		}
	})
}
