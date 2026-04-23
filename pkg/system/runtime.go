package system

import (
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/tool"
)

// StartRunSessionParams defines parameters for creating and starting run session runtime.
type StartRunSessionParams struct {
	ModelName              string
	RoleName               string
	Task                   *core.Task
	Thinking               string
	ModelOverridden        bool
	Prompt                 string
	OutputHandler          core.SessionThreadOutput
}

// StartRunSessionResult contains initialized run runtime components.
type StartRunSessionResult struct {
	Thread  *core.SessionThread
	Session *core.SweSession
	Done    <-chan error
}

// StartRunSession creates thread, binds output handler and starts requested flow.
func (s *SweSystem) StartRunSession(params StartRunSessionParams) (StartRunSessionResult, error) {
	var result StartRunSessionResult

	modelName := strings.TrimSpace(params.ModelName)
	if !params.ModelOverridden {
		resolvedRoleName := strings.TrimSpace(params.RoleName)
		if resolvedRoleName != "" && s.Roles != nil {
			if roleConfig, ok := s.Roles.Get(resolvedRoleName); ok && strings.TrimSpace(roleConfig.Model) != "" {
				modelName = strings.TrimSpace(roleConfig.Model)
			}
		}
	}

	session, err := s.newSessionWithOptions(modelName, nil, "", "", params.Thinking, params.RoleName, params.Task)
	if err != nil {
		return result, fmt.Errorf("SweSystem.StartRunSession() [runtime.go]: failed to create session: %w", err)
	}

	done := make(chan error, 1)
	wrappedHandler := &runOutputHandler{delegate: params.OutputHandler, done: done}
	thread := core.NewSessionThreadWithSession(s, session, wrappedHandler)

	if err := thread.UserPrompt(params.Prompt); err != nil {
		return result, fmt.Errorf("SweSystem.StartRunSession() [runtime.go]: failed to send initial message: %w", err)
	}

	result = StartRunSessionResult{Thread: thread, Session: session, Done: done}

	return result, nil
}

// runOutputHandler wraps a SessionThreadOutput to track when processing is done.
type runOutputHandler struct {
	delegate core.SessionThreadOutput
	done     chan error
}

func (h *runOutputHandler) AddAssistantMessage(text string, thinking string) {
	if h.delegate != nil {
		h.delegate.AddAssistantMessage(text, thinking)
	}
}

func (h *runOutputHandler) ShowMessage(message string, messageType string) {
	if h.delegate != nil {
		h.delegate.ShowMessage(message, messageType)
	}
}

func (h *runOutputHandler) AddUserMessage(text string) {
	if h.delegate != nil {
		h.delegate.AddUserMessage(text)
	}
}

func (h *runOutputHandler) AddToolCall(call *tool.ToolCall) {
	if h.delegate != nil {
		h.delegate.AddToolCall(call)
	}
}

func (h *runOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	if h.delegate != nil {
		h.delegate.AddToolCallResult(result)
	}
}

func (h *runOutputHandler) OnRateLimitError(retryAfterSeconds int) {
	if h.delegate != nil {
		h.delegate.OnRateLimitError(retryAfterSeconds)
	}
}

func (h *runOutputHandler) ShouldRetryAfterFailure(message string) bool {
	if h.delegate != nil {
		return h.delegate.ShouldRetryAfterFailure(message)
	}
	return false
}

func (h *runOutputHandler) RunFinished(err error) {
	if h.delegate != nil {
		h.delegate.RunFinished(err)
	}
	select {
	case h.done <- err:
	default:
	}
}
