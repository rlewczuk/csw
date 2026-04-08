package system

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
)

// ChatPresenter is runtime presenter contract required by system runtime wiring.
type ChatPresenter interface {
	ui.IChatPresenter
	core.SessionThreadOutput
}

// ChatView is runtime chat view contract required by system runtime wiring.
type ChatView interface {
	ui.IChatView
	StartReadingInput()
}

type sessionLoggerChatView interface {
	SetSessionLogger(logger *slog.Logger)
}

// ChatPresenterFactory builds presenter for session thread.
type ChatPresenterFactory func(system core.SessionFactory, thread *core.SessionThread) ChatPresenter

// ChatViewFactory builds chat view bound to presenter.
type ChatViewFactory func(presenter ui.IChatPresenter, output io.Writer, input io.Reader, interactive bool, allowAllPerms bool, outputFormat string) ChatView

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
	Interactive          bool
	AllowAllPerms        bool
	OutputFormat         string
	ChatOutput           io.Writer
	ChatInput            io.Reader
	ChatPresenterFactory ChatPresenterFactory
	ChatViewFactory      ChatViewFactory
}

// StartCLISessionResult contains initialized CLI runtime components.
type StartCLISessionResult struct {
	ChatView ui.IChatView
	Thread   *core.SessionThread
	Session  *core.SweSession
	Done     <-chan error
}

// StartCLISession creates thread, presenter, chat view and starts requested flow.
func (s *SweSystem) StartCLISession(params StartCLISessionParams) (StartCLISessionResult, error) {
	var result StartCLISessionResult

	if params.ChatPresenterFactory == nil {
		return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: chat presenter factory is nil")
	}
	if params.ChatViewFactory == nil {
		return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: chat view factory is nil")
	}

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

	chatPresenter := params.ChatPresenterFactory(s, thread)
	chatView := params.ChatViewFactory(chatPresenter, params.ChatOutput, params.ChatInput, params.Interactive, params.AllowAllPerms, params.OutputFormat)
	if loggerAwareView, ok := chatView.(sessionLoggerChatView); ok {
		loggerAwareView.SetSessionLogger(logging.GetSessionLogger(session.ID(), logging.LogTypeSession))
	}

	if err := chatPresenter.SetView(chatView); err != nil {
		return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to set view: %w", err)
	}

	if params.Interactive {
		chatView.StartReadingInput()
	}

	done := make(chan error, 1)
	wrappedHandler := &cliOutputHandler{delegate: chatPresenter, done: done}
	thread.SetOutputHandler(wrappedHandler)

	if params.ResumeTarget != "" {
		if params.ContinueSession {
			userMsg := &ui.ChatMessageUI{Role: ui.ChatRoleUser, Text: params.Prompt}
			if err := chatPresenter.SendUserMessage(userMsg); err != nil {
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
		userMsg := &ui.ChatMessageUI{Role: ui.ChatRoleUser, Text: params.Prompt}
		if err := chatPresenter.SendUserMessage(userMsg); err != nil {
			return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to send initial message: %w", err)
		}
	}

	result = StartCLISessionResult{ChatView: chatView, Thread: thread, Session: session, Done: done}

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
