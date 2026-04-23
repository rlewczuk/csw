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
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSessionMaybeCompactContext(t *testing.T) {
	t.Run("compacts messages and writes pre/post snapshots", func(t *testing.T) {
		tmpDir := t.TempDir()

		configStore := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{ContextCompactionThreshold: 0.95}}

		provider := models.NewMockProvider(nil)
		provider.Config = &conf.ModelProviderConfig{ContextLengthLimit: 100}

		handler := testutil.NewMockSessionOutputHandler()
		session := &SweSession{
			id:         "session-1",
			logBaseDir: tmpDir,
			config:      configStore,
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
		require.NotEmpty(t, handler.StatusMessages)
		assert.Contains(t, handler.StatusMessages[0].Message, "Compacting messages")

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

		configStore := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{ContextCompactionThreshold: 0.95}}

		provider := models.NewMockProvider(nil)
		provider.Config = &conf.ModelProviderConfig{ContextLengthLimit: 100}

		handler := testutil.NewMockSessionOutputHandler()
		session := &SweSession{
			id:          "session-2",
			logBaseDir:  tmpDir,
			config:      configStore,
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
		assert.Empty(t, handler.StatusMessages)
		_, statErr := os.Stat(filepath.Join(tmpDir, "sessions", "session-2", "messages-pre-1.jsonl"))
		assert.Error(t, statErr)
	})

	t.Run("uses default threshold when configured value is invalid", func(t *testing.T) {
		configStore := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{ContextCompactionThreshold: 1.5}}

		provider := models.NewMockProvider(nil)
		provider.Config = &conf.ModelProviderConfig{ContextLengthLimit: 100}

		session := &SweSession{
			config:      configStore,
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

func (m *tokenLimitChatModel) Compactor() models.ChatCompator {
	return nil
}

func TestSweSessionRunNonStreamingChat_CompactsOnTokenLimitError(t *testing.T) {
	t.Run("compacts context and retries", func(t *testing.T) {
		handler := testutil.NewMockSessionOutputHandler()
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
		require.NotEmpty(t, handler.StatusMessages)
		assert.Contains(t, handler.StatusMessages[0].Message, "too large")
		assert.Contains(t, handler.StatusMessages[1].Message, "Context exceeded model input token limit")
	})

	t.Run("returns error after reaching max attempts", func(t *testing.T) {
		handler := testutil.NewMockSessionOutputHandler()
		configStore := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{LLMRetryMaxAttempts: 2}}
		session := &SweSession{
			messages: []*models.ChatMessage{
				models.NewTextMessage(models.ChatRoleSystem, "system"),
				models.NewTextMessage(models.ChatRoleUser, "first"),
				models.NewTextMessage(models.ChatRoleAssistant, "first reply"),
				models.NewTextMessage(models.ChatRoleUser, "second"),
			},
			outputHandler: handler,
			config:        configStore,
		}

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
		handler := testutil.NewMockSessionOutputHandler()
		provider := models.NewMockProvider(nil)
		provider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 1}
		configStore := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 30}}

		session := &SweSession{
			messages: []*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "hello")},
			outputHandler: handler,
			provider: provider,
			config:      configStore,
			llmRetryPolicyOverride: &models.RetryPolicy{
				InitialDelay: time.Millisecond,
				MaxRetries:   1,
				MaxDelay:     time.Millisecond,
			},
		}

		chatModel := &tokenLimitChatModel{
			errors: []error{
				&models.RateLimitError{RetryAfterSeconds: 0, Message: "The usage limit has been reached"},
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
		assert.GreaterOrEqual(t, elapsed, time.Millisecond)

		messages := collectSessionMessages(handler.StatusMessages)
		assert.Contains(t, messages, "Retrying in")
	})
}

func collectSessionMessages(records []testutil.SessionMessageRecord) string {
	parts := make([]string, 0, len(records))
	for _, record := range records {
		parts = append(parts, record.Message)
	}

	return strings.Join(parts, "\n")
}
