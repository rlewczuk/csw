package system

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
)

// SessionLoggerAppView is app view supporting session logger binding.
type SessionLoggerAppView interface {
	ui.IAppView
	SetSessionLogger(logger *slog.Logger)
}

// ChatPresenter is runtime presenter contract required by system runtime wiring.
type ChatPresenter interface {
	ui.IChatPresenter
	core.SessionThreadOutput
	SetAppView(view ui.IAppView)
}

// ChatView is runtime chat view contract required by system runtime wiring.
type ChatView interface {
	ui.IChatView
	StartReadingInput()
}

// AppViewFactory builds app view for CLI runtime.
type AppViewFactory func(output io.Writer) SessionLoggerAppView

// ChatPresenterFactory builds presenter for session thread.
type ChatPresenterFactory func(system core.SessionFactory, thread *core.SessionThread) ChatPresenter

// ChatViewFactory builds chat view bound to presenter.
type ChatViewFactory func(presenter ui.IChatPresenter, output io.Writer, input io.Reader, interactive bool, allowAllPerms bool, verbose bool) ChatView

// StartCLISessionParams defines parameters for creating and starting CLI session runtime.
type StartCLISessionParams struct {
	ModelName            string
	RoleName             string
	Prompt               string
	ResumeTarget         string
	ContinueSession      bool
	ForceResume          bool
	Interactive          bool
	AllowAllPerms        bool
	Verbose              bool
	AppOutput            io.Writer
	ChatOutput           io.Writer
	ChatInput            io.Reader
	AppViewFactory       AppViewFactory
	ChatPresenterFactory ChatPresenterFactory
	ChatViewFactory      ChatViewFactory
}

// StartCLISessionResult contains initialized CLI runtime components.
type StartCLISessionResult struct {
	AppView ui.IAppView
	Thread  *core.SessionThread
	Session *core.SweSession
	Done    <-chan error
}

// StartCLISession creates app view, thread, presenter, chat view and starts requested flow.
func (s *SweSystem) StartCLISession(params StartCLISessionParams) (StartCLISessionResult, error) {
	var result StartCLISessionResult

	if params.AppViewFactory == nil {
		return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: app view factory is nil")
	}
	if params.ChatPresenterFactory == nil {
		return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: chat presenter factory is nil")
	}
	if params.ChatViewFactory == nil {
		return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: chat view factory is nil")
	}

	appView := params.AppViewFactory(params.AppOutput)

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

	appView.SetSessionLogger(logging.GetSessionLogger(session.ID(), logging.LogTypeSession))

	if params.ResumeTarget == "" {
		if err := session.SetRole(params.RoleName); err != nil {
			return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: failed to set role: %w", err)
		}
		session.SetWorkDir(s.WorkDir)
	}

	chatPresenter := params.ChatPresenterFactory(s, thread)
	chatPresenter.SetAppView(appView)
	chatView := params.ChatViewFactory(chatPresenter, params.ChatOutput, params.ChatInput, params.Interactive, params.AllowAllPerms, params.Verbose)

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
				return result, fmt.Errorf("SweSystem.StartCLISession() [runtime.go]: resumed session has no pending work (use --resume-continue to add a prompt or --force to run anyway)")
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

	result = StartCLISessionResult{AppView: appView, Thread: thread, Session: session, Done: done}

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
