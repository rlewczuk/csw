package core

import (
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
