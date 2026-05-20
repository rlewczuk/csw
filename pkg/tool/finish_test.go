package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type finishSessionMock struct {
	requested bool
	summary   string
}

func (s *finishSessionMock) RequestFinish(summary string) {
	s.requested = true
	s.summary = summary
}

func TestFinishTool_ExecuteRequestsFinish(t *testing.T) {
	session := &finishSessionMock{}
	finishTool := NewFinishTool(session)

	call := &ToolCall{ID: "finish-1", Function: "finish", Arguments: NewToolValue(map[string]any{"summary": "Implemented the requested change."})}
	response := finishTool.Execute(call)

	require.NotNil(t, response)
	require.NoError(t, response.Error)
	assert.True(t, session.requested)
	assert.Equal(t, "Implemented the requested change.", session.summary)
	assert.True(t, response.Done)
	assert.Equal(t, "success", response.Result.Get("status").AsString())
	assert.Equal(t, "Session finish requested.", response.Result.Get("message").AsString())
	assert.Equal(t, "Implemented the requested change.", response.Result.Get("summary").AsString())
}

func TestFinishTool_ExecuteRequiresSummary(t *testing.T) {
	tests := []struct {
		name      string
		arguments map[string]any
	}{
		{name: "missing summary", arguments: map[string]any{}},
		{name: "empty summary", arguments: map[string]any{"summary": "   "}},
		{name: "invalid summary", arguments: map[string]any{"summary": 42}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session := &finishSessionMock{}
			finishTool := NewFinishTool(session)

			call := &ToolCall{ID: "finish-1", Function: "finish", Arguments: NewToolValue(tc.arguments)}
			response := finishTool.Execute(call)

			require.NotNil(t, response)
			require.Error(t, response.Error)
			assert.Contains(t, response.Error.Error(), "summary")
			assert.False(t, session.requested)
			assert.True(t, response.Done)
		})
	}
}

func TestFinishTool_ExecuteWithoutSession(t *testing.T) {
	finishTool := NewFinishTool(nil)

	call := &ToolCall{ID: "finish-1", Function: "finish", Arguments: NewToolValue(map[string]any{"summary": "Finished without a bound session."})}
	response := finishTool.Execute(call)

	require.NotNil(t, response)
	require.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, "success", response.Result.Get("status").AsString())
	assert.Equal(t, "Finished without a bound session.", response.Result.Get("summary").AsString())
}
