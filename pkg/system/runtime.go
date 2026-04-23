package system

import (
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
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

	thread := core.NewSessionThreadWithSession(s, session, params.OutputHandler)

	if err := thread.UserPrompt(params.Prompt); err != nil {
		return result, fmt.Errorf("SweSystem.StartRunSession() [runtime.go]: failed to send initial message: %w", err)
	}

	result = StartRunSessionResult{Thread: thread, Session: session, Done: thread.Done()}

	return result, nil
}
