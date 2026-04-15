package core

import (
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
)

type AgentStateCommonInfo struct {
	AgentName           string
	WorkDir             string
	ShadowDir           string
	CurrentTime         string
	TokenUsage          models.TokenUsage
	ContextLengthTokens int
}

// TODO decide if this is still needed
type AgentState struct {
	Info AgentStateCommonInfo
	Role *conf.AgentRoleConfig
	Task *Task
}

// Clone returns a deep copy of AgentState.
func (s AgentState) Clone() AgentState {
	cloned := s
	if s.Role != nil {
		cloned.Role = s.Role.Clone()
	}
	if s.Task != nil {
		cloned.Task = cloneTask(s.Task)
	}

	return cloned
}

// SetHookContextValue sets one hook context field.
func (s *AgentState) SetHookContextValue(key string, value string) {
	if s == nil || strings.TrimSpace(key) == "" {
		return
	}
}
