package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type finishSessionMock struct {
	requested bool
}

func (s *finishSessionMock) RequestFinish() {
	s.requested = true
}

func TestFinishTool_ExecuteRequestsFinish(t *testing.T) {
	session := &finishSessionMock{}
	finishTool := NewFinishTool(session)

	call := &ToolCall{ID: "finish-1", Function: "finish", Arguments: NewToolValue(map[string]any{})}
	response := finishTool.Execute(call)

	require.NotNil(t, response)
	assert.True(t, session.requested)
	assert.True(t, response.Done)
	assert.Equal(t, "success", response.Result.Get("status").AsString())
	assert.Equal(t, "Session finish requested.", response.Result.Get("message").AsString())
}

func TestFinishTool_ExecuteWithoutSession(t *testing.T) {
	finishTool := NewFinishTool(nil)

	call := &ToolCall{ID: "finish-1", Function: "finish", Arguments: NewToolValue(map[string]any{})}
	response := finishTool.Execute(call)

	require.NotNil(t, response)
	assert.True(t, response.Done)
	assert.Equal(t, "success", response.Result.Get("status").AsString())
}
