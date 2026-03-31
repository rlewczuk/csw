package core

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

type capturedSessionMessage struct {
	message     string
	messageType string
}

type retryOutputHandler struct {
	messages []capturedSessionMessage
}

func (h *retryOutputHandler) ShowMessage(message string, messageType string) {
	h.messages = append(h.messages, capturedSessionMessage{message: message, messageType: messageType})
}

func (h *retryOutputHandler) AddAssistantMessage(text string, thinking string) {}

func (h *retryOutputHandler) AddToolCall(call *tool.ToolCall) {}

func (h *retryOutputHandler) AddToolCallResult(result *tool.ToolResponse) {}

func (h *retryOutputHandler) RunFinished(err error) {}

func (h *retryOutputHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {}

func (h *retryOutputHandler) OnRateLimitError(retryAfterSeconds int) {}

func (h *retryOutputHandler) ShouldRetryAfterFailure(message string) bool {
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
			id:         "session-1",
			logBaseDir: tmpDir,
			configStore: configStore,
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
			id:          "session-2",
			logBaseDir:  tmpDir,
			configStore: configStore,
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
			configStore: configStore,
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

type tokenLimitChatModel struct {
	errors    []error
	responses []*models.ChatMessage
	callSizes []int
	callIndex int
}

func (m *tokenLimitChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) (*models.ChatMessage, error) {
	_ = ctx
	_ = options
	_ = tools
	m.callSizes = append(m.callSizes, len(messages))

	if m.callIndex < len(m.errors) {
		err := m.errors[m.callIndex]
		if err != nil {
			m.callIndex++
			return nil, err
		}
	}

	if m.callIndex < len(m.responses) {
		response := m.responses[m.callIndex]
		m.callIndex++
		return response, nil
	}

	m.callIndex++
	return models.NewTextMessage(models.ChatRoleAssistant, "ok"), nil
}

func (m *tokenLimitChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) iter.Seq[*models.ChatMessage] {
	_ = ctx
	_ = messages
	_ = options
	_ = tools
	return func(yield func(*models.ChatMessage) bool) {
		_ = yield
	}
}

func TestSweSessionRunNonStreamingChat_CompactsOnTokenLimitError(t *testing.T) {
	t.Run("compacts context and retries", func(t *testing.T) {
		handler := &compactionOutputHandler{}
		session := &SweSession{
			messages: []*models.ChatMessage{
				models.NewTextMessage(models.ChatRoleSystem, "system"),
				models.NewTextMessage(models.ChatRoleUser, "first"),
				models.NewTextMessage(models.ChatRoleAssistant, "first reply"),
				models.NewTextMessage(models.ChatRoleUser, "second"),
				models.NewTextMessage(models.ChatRoleAssistant, "second reply"),
			},
			outputHandler: handler,
		}

		chatModel := &tokenLimitChatModel{
			errors: []error{models.ErrTooManyInputTokens, nil},
			responses: []*models.ChatMessage{
				nil,
				models.NewTextMessage(models.ChatRoleAssistant, "done"),
			},
		}

		retryPolicy := session.llmRetryPolicy()
		retryingChatModel := models.NewRetryChatModel(chatModel, &retryPolicy, session.handleRetryChatModelMessage)
		response, err := session.runNonStreamingChat(t.Context(), retryingChatModel, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, "done", response.GetText())
		assert.Equal(t, 1, session.compactionCount)
		require.Len(t, chatModel.callSizes, 2)
		require.NotEmpty(t, handler.messages)
		assert.Contains(t, handler.messages[0], "too large")
		assert.Contains(t, handler.messages[1], "Context exceeded model input token limit")
	})

	t.Run("returns error after reaching max attempts", func(t *testing.T) {
		handler := &compactionOutputHandler{}
		session := &SweSession{
			messages: []*models.ChatMessage{
				models.NewTextMessage(models.ChatRoleSystem, "system"),
				models.NewTextMessage(models.ChatRoleUser, "first"),
				models.NewTextMessage(models.ChatRoleAssistant, "first reply"),
				models.NewTextMessage(models.ChatRoleUser, "second"),
			},
			outputHandler: handler,
			configStore:   impl.NewMockConfigStore(),
		}
		session.configStore.(*impl.MockConfigStore).SetGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2})

		chatModel := &tokenLimitChatModel{
			errors: []error{models.ErrTooManyInputTokens, models.ErrTooManyInputTokens},
		}

		retryPolicy := session.llmRetryPolicy()
		retryingChatModel := models.NewRetryChatModel(chatModel, &retryPolicy, session.handleRetryChatModelMessage)
		response, err := session.runNonStreamingChat(t.Context(), retryingChatModel, nil, nil)
		require.Nil(t, response)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too many input tokens after 2 attempts")
		assert.Equal(t, 2, session.compactionCount)
		require.Len(t, chatModel.callSizes, 2)
	})
}

func TestSweSessionRunNonStreamingChat_UsageLimitWait(t *testing.T) {
	t.Run("waits retry-after plus buffer before retrying", func(t *testing.T) {
		handler := &retryOutputHandler{}
		provider := models.NewMockProvider(nil)
		provider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: time.Millisecond}

		session := &SweSession{
			messages: []*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "hello")},
			outputHandler: handler,
			provider: provider,
			configStore: impl.NewMockConfigStore(),
		}
		session.configStore.(*impl.MockConfigStore).SetGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 30})

		chatModel := &tokenLimitChatModel{
			errors: []error{
				&models.RateLimitError{RetryAfterSeconds: 2, Message: "The usage limit has been reached"},
				nil,
			},
			responses: []*models.ChatMessage{nil, models.NewTextMessage(models.ChatRoleAssistant, "done")},
		}

		start := time.Now()
		retryPolicy := session.llmRetryPolicy()
		retryingChatModel := models.NewRetryChatModel(chatModel, &retryPolicy, session.handleRetryChatModelMessage)
		response, err := session.runNonStreamingChat(t.Context(), retryingChatModel, nil, nil)
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, "done", response.GetText())
		assert.GreaterOrEqual(t, elapsed, 12*time.Millisecond)

		messages := collectSessionMessages(handler.messages)
		assert.Contains(t, messages, "Usage limit has been reached. Reset expected at")
		assert.Contains(t, messages, "Retrying in")
	})
}

func collectSessionMessages(records []capturedSessionMessage) string {
	parts := make([]string, 0, len(records))
	for _, record := range records {
		parts = append(parts, record.message)
	}

	return strings.Join(parts, "\n")
}
