package models

import (
	"context"
	"errors"
	"iter"
	"sync"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fallbackTestResult struct {
	response *ChatMessage
	err      error
}

type fallbackTestModel struct {
	mu      sync.Mutex
	name    string
	results []fallbackTestResult
	calls   int
}

func (m *fallbackTestModel) Chat(_ context.Context, _ []*ChatMessage, _ *ChatOptions, _ []tool.ToolInfo) (*ChatMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++

	if len(m.results) == 0 {
		return NewTextMessage(ChatRoleAssistant, m.name+"-default"), nil
	}

	result := m.results[0]
	m.results = m.results[1:]
	return result.response, result.err
}

func (m *fallbackTestModel) ChatStream(_ context.Context, _ []*ChatMessage, _ *ChatOptions, _ []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		yield(NewTextMessage(ChatRoleAssistant, m.name+"-stream"))
	}
}

func (m *fallbackTestModel) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestFallbackChatModel_Chat_NoModels(t *testing.T) {
	model := NewFallbackChatModel(nil, 1)

	response, err := model.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "no chat models configured")
}

func TestFallbackChatModel_Chat_PersistsSelectedModelAfterTemporaryFailure(t *testing.T) {
	m1 := &fallbackTestModel{
		name: "m1",
		results: []fallbackTestResult{
			{response: NewTextMessage(ChatRoleAssistant, "m1-response")},
			{err: &NetworkError{Message: "temporary network error", IsRetryable: true}},
		},
	}
	m2 := &fallbackTestModel{
		name: "m2",
		results: []fallbackTestResult{
			{response: NewTextMessage(ChatRoleAssistant, "m2-response-1")},
			{response: NewTextMessage(ChatRoleAssistant, "m2-response-2")},
		},
	}

	fallback := NewFallbackChatModel([]ChatModel{m1, m2}, 3)

	var waitCalls int
	var waitedFor time.Duration
	fallback.waitWithContext = func(_ context.Context, d time.Duration) error {
		waitCalls++
		waitedFor = d
		return nil
	}

	first, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "m1-response", first.GetText())

	second, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "m2-response-1", second.GetText())

	third, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "m2-response-2", third.GetText())

	assert.Equal(t, 2, m1.Calls())
	assert.Equal(t, 2, m2.Calls())
	assert.Equal(t, 1, waitCalls)
	assert.Equal(t, 3*time.Second, waitedFor)
}

func TestFallbackChatModel_Chat_RateLimitSwitchesWithoutDelay(t *testing.T) {
	m1 := &fallbackTestModel{
		name: "m1",
		results: []fallbackTestResult{{err: &RateLimitError{RetryAfterSeconds: 12, Message: "m1 rate limit"}}},
	}
	m2 := &fallbackTestModel{
		name: "m2",
		results: []fallbackTestResult{
			{response: NewTextMessage(ChatRoleAssistant, "m2-response-1")},
			{response: NewTextMessage(ChatRoleAssistant, "m2-response-2")},
		},
	}

	fallback := NewFallbackChatModel([]ChatModel{m1, m2}, 10)

	var waitCalls int
	fallback.waitWithContext = func(_ context.Context, _ time.Duration) error {
		waitCalls++
		return nil
	}

	first, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "m2-response-1", first.GetText())

	second, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "m2-response-2", second.GetText())

	assert.Equal(t, 1, m1.Calls())
	assert.Equal(t, 2, m2.Calls())
	assert.Equal(t, 0, waitCalls)
}

func TestFallbackChatModel_Chat_CyclesWhenLastModelFails(t *testing.T) {
	m1 := &fallbackTestModel{
		name: "m1",
		results: []fallbackTestResult{{response: NewTextMessage(ChatRoleAssistant, "m1-success")}},
	}
	m2 := &fallbackTestModel{
		name: "m2",
		results: []fallbackTestResult{{err: &RateLimitError{RetryAfterSeconds: 8, Message: "m2 rate limit"}}},
	}

	fallback := NewFallbackChatModel([]ChatModel{m1, m2}, 0)
	fallback.setSelectedIndex(1)

	response, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "m1-success", response.GetText())

	next, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "m1-default", next.GetText())
	assert.Equal(t, 1, m2.Calls())
	assert.Equal(t, 2, m1.Calls())
}

func TestFallbackChatModel_Chat_AllRateLimitedReturnsShortestRetry(t *testing.T) {
	models := []ChatModel{
		&fallbackTestModel{name: "m1", results: []fallbackTestResult{{err: &RateLimitError{RetryAfterSeconds: 15, Message: "m1"}}}},
		&fallbackTestModel{name: "m2", results: []fallbackTestResult{{err: &RateLimitError{RetryAfterSeconds: 4, Message: "m2"}}}},
		&fallbackTestModel{name: "m3", results: []fallbackTestResult{{err: &RateLimitError{RetryAfterSeconds: 0, Message: "m3"}}}},
	}

	fallback := NewFallbackChatModel(models, 0)

	response, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, response)

	var rateErr *RateLimitError
	require.True(t, errors.As(err, &rateErr))
	assert.Equal(t, 4, rateErr.RetryAfterSeconds)
	assert.Equal(t, "m2", rateErr.Message)
}

func TestFallbackChatModel_Chat_AllModelsFailWithNonRateLimitReturnsLastError(t *testing.T) {
	err1 := errors.New("provider one failure")
	err2 := errors.New("provider two failure")

	m1 := &fallbackTestModel{name: "m1", results: []fallbackTestResult{{err: err1}}}
	m2 := &fallbackTestModel{name: "m2", results: []fallbackTestResult{{err: err2}}}

	fallback := NewFallbackChatModel([]ChatModel{m1, m2}, 0)

	response, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, err2, err)
	assert.Equal(t, 1, m1.Calls())
	assert.Equal(t, 1, m2.Calls())
}

func TestFallbackChatModel_Chat_MixedErrorsAndRateLimitReturnsShortestRateLimit(t *testing.T) {
	errPermanent := errors.New("permanent failure")
	m1 := &fallbackTestModel{name: "m1", results: []fallbackTestResult{{err: errPermanent}}}
	m2 := &fallbackTestModel{name: "m2", results: []fallbackTestResult{{err: &RateLimitError{RetryAfterSeconds: 20, Message: "slow"}}}}
	m3 := &fallbackTestModel{name: "m3", results: []fallbackTestResult{{err: &RateLimitError{RetryAfterSeconds: 3, Message: "fast"}}}}

	fallback := NewFallbackChatModel([]ChatModel{m1, m2, m3}, 0)

	response, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, response)

	var rateErr *RateLimitError
	require.True(t, errors.As(err, &rateErr))
	assert.Equal(t, 3, rateErr.RetryAfterSeconds)
	assert.Equal(t, "fast", rateErr.Message)
}

func TestNewFallbackChatModel_NegativeDelayIsClampedToZero(t *testing.T) {
	fallback := NewFallbackChatModel([]ChatModel{&fallbackTestModel{name: "m1"}}, -5)
	assert.Equal(t, time.Duration(0), fallback.switchDelay)
}

func TestFallbackChatModel_Chat_ContextCancelledWhileWaitingForSwitch(t *testing.T) {
	m1 := &fallbackTestModel{name: "m1", results: []fallbackTestResult{{err: &NetworkError{Message: "temporary", IsRetryable: true}}}}
	m2 := &fallbackTestModel{name: "m2", results: []fallbackTestResult{{response: NewTextMessage(ChatRoleAssistant, "m2")}}}

	fallback := NewFallbackChatModel([]ChatModel{m1, m2}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	response, err := fallback.Chat(ctx, nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, m1.Calls())
	assert.Equal(t, 0, m2.Calls())
}

func TestFallbackChatModel_ChatStream_UsesCurrentlySelectedModel(t *testing.T) {
	m1 := &fallbackTestModel{name: "m1", results: []fallbackTestResult{{err: &RateLimitError{RetryAfterSeconds: 5, Message: "m1 rl"}}}}
	m2 := &fallbackTestModel{name: "m2", results: []fallbackTestResult{{response: NewTextMessage(ChatRoleAssistant, "m2-ok")}}}

	fallback := NewFallbackChatModel([]ChatModel{m1, m2}, 0)

	_, err := fallback.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)

	var fragments []*ChatMessage
	for fragment := range fallback.ChatStream(context.Background(), nil, nil, nil) {
		fragments = append(fragments, fragment)
	}

	require.Len(t, fragments, 1)
	assert.Equal(t, "m2-stream", fragments[0].GetText())
}
