package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testChatModel is a controllable ChatModel test double for retry tests.
type testChatModel struct {
	mu        sync.Mutex
	responses []testChatResponse
	calls     int
}

type testChatResponse struct {
	response *ChatMessage
	err      error
}

func (m *testChatModel) Chat(_ context.Context, _ []*ChatMessage, _ *ChatOptions, _ []tool.ToolInfo) (*ChatMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++

	if len(m.responses) == 0 {
		return &ChatMessage{Role: ChatRoleAssistant, Parts: []ChatMessagePart{{Text: "default response"}}}, nil
	}

	resp := m.responses[0]
	m.responses = m.responses[1:]
	return resp.response, resp.err
}

func (m *testChatModel) ChatStream(_ context.Context, _ []*ChatMessage, _ *ChatOptions, _ []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		yield(NewTextMessage(ChatRoleAssistant, "stream response"))
	}
}

func (m *testChatModel) Compactor() ChatCompator {
	return nil
}

func (m *testChatModel) getCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// newTestRetryPolicy creates a fast retry policy for testing.
func newTestRetryPolicy(maxRetries int) RetryPolicy {
	return RetryPolicy{
		InitialDelay: 1 * time.Millisecond,
		MaxRetries:   maxRetries,
		MaxDelay:     100 * time.Millisecond,
	}
}

// --- RetryChatModel.Chat tests ---

func TestRetryChatModel_Chat_Success(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{response: NewTextMessage(ChatRoleAssistant, "hello")},
		},
	}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.GetText())
	assert.Equal(t, 1, mock.getCalls())
}

func TestRetryChatModel_Chat_RetriesOnRateLimitAndSucceeds(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: &RateLimitError{Message: "rate exceeded"}},
			{response: NewTextMessage(ChatRoleAssistant, "success")},
		},
	}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.GetText())
	assert.Equal(t, 3, mock.getCalls())
}

func TestRetryChatModel_Chat_RetriesOnRetryableNetworkError(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &NetworkError{Message: "connection reset", IsRetryable: true}},
			{response: NewTextMessage(ChatRoleAssistant, "recovered")},
		},
	}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "recovered", resp.GetText())
	assert.Equal(t, 2, mock.getCalls())
}

func TestRetryChatModel_Chat_DoesNotRetryNonRetryableNetworkError(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &NetworkError{Message: "fatal network error", IsRetryable: false}},
		},
	}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 1, mock.getCalls())

	var networkErr *NetworkError
	assert.True(t, errors.As(err, &networkErr))
}

func TestRetryChatModel_Chat_DoesNotRetryGenericError(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: errors.New("some permanent error")},
		},
	}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 1, mock.getCalls())
	assert.Equal(t, "some permanent error", err.Error())
}

func TestRetryChatModel_Chat_ExhaustsRetries(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: &RateLimitError{Message: "rate exceeded"}},
		},
	}

	policy := newTestRetryPolicy(2) // MaxRetries=2, so totalAttempts=3
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 3, mock.getCalls())
	assert.Contains(t, err.Error(), "temporary LLM API failure after 3 attempts")

	var rateLimitErr *RateLimitError
	assert.True(t, errors.As(err, &rateLimitErr))
}

func TestRetryChatModel_Chat_ZeroRetries(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
		},
	}

	policy := newTestRetryPolicy(0) // MaxRetries=0, so totalAttempts=1
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 1, mock.getCalls())
	assert.Contains(t, err.Error(), "temporary LLM API failure after 1 attempts")
}

func TestRetryChatModel_Chat_ContextCancellation(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: &RateLimitError{Message: "rate exceeded"}},
		},
	}

	policy := RetryPolicy{
		InitialDelay: 5 * time.Second, // Long delay so cancellation kicks in
		MaxRetries:   5,
		MaxDelay:     30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(ctx, nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRetryChatModel_Chat_ContextCancellationDuringBackoff(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
		},
	}

	policy := RetryPolicy{
		InitialDelay: 5 * time.Second,
		MaxRetries:   5,
		MaxDelay:     30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay (during backoff wait)
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(ctx, nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- Logging tests ---

func TestRetryChatModel_Chat_LogsTemporaryErrors(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: &NetworkError{Message: "connection reset", IsRetryable: true}},
			{response: NewTextMessage(ChatRoleAssistant, "success")},
		},
	}

	var logMessages []string
	var logTypes []shared.MessageType
	logFn := func(msg string, msgType shared.MessageType) {
		logMessages = append(logMessages, msg)
		logTypes = append(logTypes, msgType)
	}

	policy := newTestRetryPolicy(5)
	model := NewRetryChatModel(mock, &policy, logFn)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.GetText())

	require.Len(t, logMessages, 4)
	assert.Contains(t, logMessages[0], "attempt 1/6")
	assert.Contains(t, logMessages[0], "rate exceeded")
	assert.Equal(t, shared.MessageTypeError, logTypes[0])
	assert.Contains(t, logMessages[1], "Retrying in")
	assert.Equal(t, shared.MessageTypeWarning, logTypes[1])

	assert.Contains(t, logMessages[2], "attempt 2/6")
	assert.Contains(t, logMessages[2], "connection reset")
	assert.Equal(t, shared.MessageTypeError, logTypes[2])
	assert.Contains(t, logMessages[3], "Retrying in")
	assert.Equal(t, shared.MessageTypeWarning, logTypes[3])
}

func TestRetryChatModel_Chat_DoesNotLogOnSuccess(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{response: NewTextMessage(ChatRoleAssistant, "success")},
		},
	}

	var logMessages []string
	logFn := func(msg string, msgType shared.MessageType) {
		logMessages = append(logMessages, msg)
	}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, logFn)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, logMessages)
	assert.Equal(t, "success", resp.GetText())
}

func TestRetryChatModel_Chat_NilLogFnDoesNotPanic(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
			{response: NewTextMessage(ChatRoleAssistant, "success")},
		},
	}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.GetText())
}

// --- ChatStream passthrough tests ---

func TestRetryChatModel_ChatStream_Passthrough(t *testing.T) {
	mock := &testChatModel{}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, nil)

	ctx := context.Background()
	messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "test")}
	options := &ChatOptions{Temperature: 0.5}
	tools := []tool.ToolInfo{{Name: "tool1"}}

	var fragments []*ChatMessage
	for msg := range model.ChatStream(ctx, messages, options, tools) {
		fragments = append(fragments, msg)
	}

	require.Len(t, fragments, 1)
	assert.Equal(t, "stream response", fragments[0].GetText())
}

// --- Mixed error type sequence tests ---

func TestRetryChatModel_Chat_MixedErrorSequence(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: &NetworkError{Message: "timeout", IsRetryable: true}},
			{err: ErrEndpointUnavailable},
			{err: io.EOF},
			{response: NewTextMessage(ChatRoleAssistant, "finally")},
		},
	}

	policy := newTestRetryPolicy(5)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "finally", resp.GetText())
	assert.Equal(t, 5, mock.getCalls())
}

func TestRetryChatModel_Chat_NonRetryableErrorStopsRetries(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: errors.New("permanent error")},
			{response: NewTextMessage(ChatRoleAssistant, "should not reach")},
		},
	}

	policy := newTestRetryPolicy(5)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "permanent error", err.Error())
	assert.Equal(t, 2, mock.getCalls())
}

// --- Wrapped error tests ---

func TestRetryChatModel_Chat_WrappedRateLimitError(t *testing.T) {
	innerErr := &RateLimitError{RetryAfterSeconds: 0, Message: "rate exceeded"}
	wrappedErr := fmt.Errorf("request failed: %w", innerErr)

	mock := &testChatModel{
		responses: []testChatResponse{
			{err: wrappedErr},
			{response: NewTextMessage(ChatRoleAssistant, "success")},
		},
	}

	policy := newTestRetryPolicy(3)
	model := NewRetryChatModel(mock, &policy, nil)

	resp, err := model.Chat(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.GetText())
	assert.Equal(t, 2, mock.getCalls())
}

// --- Interface compliance test ---

func TestRetryChatModel_ImplementsChatModel(t *testing.T) {
	// Compile-time check is in interface_check.go; this runtime check confirms it.
	var _ ChatModel = NewRetryChatModel(&testChatModel{}, &RetryPolicy{}, nil)
}

// --- Rate limit with RetryAfterSeconds integration test ---

func TestRetryChatModel_Chat_RateLimitRetryAfterDelaysCorrectly(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{RetryAfterSeconds: 0, Message: "rate exceeded"}},
			{response: NewTextMessage(ChatRoleAssistant, "success")},
		},
	}

	policy := RetryPolicy{
		InitialDelay: 10 * time.Millisecond,
		MaxRetries:   3,
		MaxDelay:     200 * time.Millisecond,
	}

	model := NewRetryChatModel(mock, &policy, nil)

	start := time.Now()
	resp, err := model.Chat(context.Background(), nil, nil, nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "success", resp.GetText())
	// Should have waited at least InitialDelay (10ms) for exponential backoff
	assert.True(t, elapsed >= 10*time.Millisecond, "expected delay of at least 10ms, got %v", elapsed)
}

// --- Error message format test ---

func TestRetryChatModel_Chat_ExhaustsRetriesErrorMessage(t *testing.T) {
	mock := &testChatModel{
		responses: []testChatResponse{
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: &RateLimitError{Message: "rate exceeded"}},
			{err: &RateLimitError{Message: "rate exceeded"}},
		},
	}

	policy := newTestRetryPolicy(2)
	model := NewRetryChatModel(mock, &policy, nil)

	_, err := model.Chat(context.Background(), nil, nil, nil)
	require.Error(t, err)

	errMsg := err.Error()
	assert.True(t, strings.HasPrefix(errMsg, "RetryChatModel.Chat()"), "error should start with function name, got: %s", errMsg)
	assert.Contains(t, errMsg, "retry.go")
	assert.Contains(t, errMsg, "temporary LLM API failure after 3 attempts")
}
