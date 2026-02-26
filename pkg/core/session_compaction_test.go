package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type compactionOutputHandler struct {
	messages []string
}

func (h *compactionOutputHandler) ShowMessage(message string, messageType string) {
	h.messages = append(h.messages, messageType+":"+message)
}

func (h *compactionOutputHandler) AddAssistantMessage(text string, thinking string) {}

func (h *compactionOutputHandler) AddToolCall(call *tool.ToolCall) {}

func (h *compactionOutputHandler) AddToolCallResult(result *tool.ToolResponse) {}

func (h *compactionOutputHandler) RunFinished(err error) {}

func (h *compactionOutputHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {}

func (h *compactionOutputHandler) OnRateLimitError(retryAfterSeconds int) {}

func (h *compactionOutputHandler) ShouldRetryAfterFailure(message string) bool {
	return false
}

func TestSweSessionMaybeCompactContext(t *testing.T) {
	t.Run("compacts messages and writes pre/post snapshots", func(t *testing.T) {
		tmpDir := t.TempDir()

		configStore := impl.NewMockConfigStore()
		configStore.SetGlobalConfig(&conf.GlobalConfig{ContextCompactionThreshold: 0.95})

		provider := models.NewMockProvider(nil)
		provider.Config = &conf.ModelProviderConfig{ContextLengthLimit: 100}

		handler := &compactionOutputHandler{}
		session := &SweSession{
			id: "session-1",
			system: &SweSystem{
				LogBaseDir:  tmpDir,
				ConfigStore: configStore,
			},
			provider:      provider,
			contextLength: 96,
			messages: []*models.ChatMessage{
				models.NewTextMessage(models.ChatRoleSystem, "system"),
				models.NewTextMessage(models.ChatRoleUser, "hello"),
			},
			outputHandler: handler,
		}

		err := session.maybeCompactContext()
		require.NoError(t, err)

		assert.Equal(t, 1, session.compactionCount)
		require.NotEmpty(t, handler.messages)
		assert.Contains(t, handler.messages[0], "Compacting messages")

		prePath := filepath.Join(tmpDir, "sessions", "session-1", "messages-pre-1.jsonl")
		postPath := filepath.Join(tmpDir, "sessions", "session-1", "messages-post-1.jsonl")

		preBytes, err := os.ReadFile(prePath)
		require.NoError(t, err)
		postBytes, err := os.ReadFile(postPath)
		require.NoError(t, err)

		assert.Contains(t, string(preBytes), `"role":"system"`)
		assert.Contains(t, string(postBytes), `"role":"user"`)
	})

	t.Run("skips compaction when below configured threshold", func(t *testing.T) {
		tmpDir := t.TempDir()

		configStore := impl.NewMockConfigStore()
		configStore.SetGlobalConfig(&conf.GlobalConfig{ContextCompactionThreshold: 0.95})

		provider := models.NewMockProvider(nil)
		provider.Config = &conf.ModelProviderConfig{ContextLengthLimit: 100}

		handler := &compactionOutputHandler{}
		session := &SweSession{
			id: "session-2",
			system: &SweSystem{
				LogBaseDir:  tmpDir,
				ConfigStore: configStore,
			},
			provider:      provider,
			contextLength: 94,
			messages: []*models.ChatMessage{
				models.NewTextMessage(models.ChatRoleUser, "hello"),
			},
			outputHandler: handler,
		}

		err := session.maybeCompactContext()
		require.NoError(t, err)

		assert.Equal(t, 0, session.compactionCount)
		assert.Empty(t, handler.messages)
		_, statErr := os.Stat(filepath.Join(tmpDir, "sessions", "session-2", "messages-pre-1.jsonl"))
		assert.Error(t, statErr)
	})

	t.Run("uses default threshold when configured value is invalid", func(t *testing.T) {
		configStore := impl.NewMockConfigStore()
		configStore.SetGlobalConfig(&conf.GlobalConfig{ContextCompactionThreshold: 1.5})

		provider := models.NewMockProvider(nil)
		provider.Config = &conf.ModelProviderConfig{ContextLengthLimit: 100}

		session := &SweSession{
			system: &SweSystem{ConfigStore: configStore},
			provider:      provider,
			contextLength: 96,
			messages: []*models.ChatMessage{
				models.NewTextMessage(models.ChatRoleUser, "hello"),
			},
		}

		err := session.maybeCompactContext()
		require.NoError(t, err)
		assert.Equal(t, 1, session.compactionCount)
	})
}
