package system

import (
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/tool"
)

// StartCLISessionParams defines parameters for creating and starting CLI session runtime.
type StartCLISessionParams struct {
	ModelName            string
	RoleName             string
	TaskInfo             *core.TaskInfo
	Thinking             string
	ModelOverridden      bool
	RoleOverridden       bool
	ThinkingOverridden   bool
	Prompt               string
	ResumeTarget         string
	ContinueSession      bool
	ForceResume          bool
	ForceCompact         bool
	OutputHandler        core.SessionThreadOutput
}

// StartCLISessionResult contains initialized CLI runtime components.
type StartCLISessionResult struct {
	Thread  *core.SessionThread
	Session *core.SweSession
	Done    <-chan error
}

// StartCLISession creates thread, binds output handler and starts requested flow.
func (s *SweSystem) StartCLISession(params StartCLISessionParams) (StartCLISessionResult, error) {
	var result StartCLISessionResult

	var (
		thread  *core.SessionThread
		session *core.SweSession
		err     error
	)

	if params.ResumeTarget != "" {
		if params.ResumeTarget == "last" {
			session, err = s.LoadLastSession(nil)
			if err != nil {
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to load last session: %w", err)
			}
		} else {
			session, err = s.LoadSession(params.ResumeTarget, nil)
			if err != nil {
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to load session: %w", err)
			}
		}
		thread = core.NewSessionThreadWithSession(s, session, nil)
	} else {
		thread = core.NewSessionThread(s, nil)
		if err := thread.StartSession(params.ModelName); err != nil {
			return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to start session: %w", err)
		}
		session = thread.GetSession()
	}

	if session == nil {
		return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: session is not available")
	}

	if params.ResumeTarget != "" {
		if params.ModelOverridden {
			if err := session.SetModel(params.ModelName); err != nil {
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to override model: %w", err)
			}
		}
		if params.RoleOverridden {
			if err := session.SetRole(params.RoleName); err != nil {
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to override role: %w", err)
			}
			if !params.ModelOverridden {
				if roleConfig := session.Role(); roleConfig != nil && strings.TrimSpace(roleConfig.Model) != "" {
					if err := session.SetModel(roleConfig.Model); err != nil {
						return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to apply role model override: %w", err)
					}
				}
			}
		}
		if params.ThinkingOverridden {
			session.SetThinkingLevel(params.Thinking)
		}
		if params.ForceCompact {
			if err := session.ForceCompactContext(); err != nil {
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to compact resumed session: %w", err)
			}
		}
	}

	if params.ResumeTarget == "" {
		if err := session.SetRole(params.RoleName); err != nil {
			return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to set role: %w", err)
		}
		if !params.ModelOverridden {
			if roleConfig := session.Role(); roleConfig != nil && strings.TrimSpace(roleConfig.Model) != "" {
				if err := session.SetModel(roleConfig.Model); err != nil {
					return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to apply role model: %w", err)
				}
			}
		}
		session.SetWorkDir(s.WorkDir)
	}

	session.SetTaskInfo(params.TaskInfo)

	done := make(chan error, 1)
	wrappedHandler := &cliOutputHandler{delegate: params.OutputHandler, done: done}
	thread.SetOutputHandler(wrappedHandler)

	if params.ResumeTarget != "" {
		if params.ContinueSession {
			if err := thread.UserPrompt(params.Prompt); err != nil {
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to send continue message: %w", err)
			}
		} else {
			if !params.ForceResume && !session.HasPendingWork() {
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: resumed session has no pending work (provide prompt with --resume to continue or use --force to run anyway)")
			}
			if err := thread.ResumePending(); err != nil {
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to resume pending work: %w", err)
			}
		}
	} else {
		if err := thread.UserPrompt(params.Prompt); err != nil {
			return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to send initial message: %w", err)
		}
	}

	result = StartCLISessionResult{Thread: thread, Session: session, Done: done}

	return result, nil
}

// cliOutputHandler wraps a SessionThreadOutput to track when processing is done.
type cliOutputHandler struct {
	delegate core.SessionThreadOutput
	done     chan error
}

func (h *cliOutputHandler) AddAssistantMessage(text string, thinking string) {
	if h.delegate != nil {
		h.delegate.AddAssistantMessage(text, thinking)
	}
}

func (h *cliOutputHandler) ShowMessage(message string, messageType string) {
	if h.delegate != nil {
		h.delegate.ShowMessage(message, messageType)
	}
}

func (h *cliOutputHandler) AddToolCall(call *tool.ToolCall) {
	if h.delegate != nil {
		h.delegate.AddToolCall(call)
	}
}

func (h *cliOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	if h.delegate != nil {
		h.delegate.AddToolCallResult(result)
	}
}

func (h *cliOutputHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	if h.delegate != nil {
		h.delegate.OnPermissionQuery(query)
	}
}

func (h *cliOutputHandler) OnRateLimitError(retryAfterSeconds int) {
	if h.delegate != nil {
		h.delegate.OnRateLimitError(retryAfterSeconds)
	}
}

func (h *cliOutputHandler) ShouldRetryAfterFailure(message string) bool {
	if h.delegate != nil {
		return h.delegate.ShouldRetryAfterFailure(message)
	}
	return false
}

func (h *cliOutputHandler) RunFinished(err error) {
	if h.delegate != nil {
		h.delegate.RunFinished(err)
	}
	select {
	case h.done <- err:
	default:
	}
}
