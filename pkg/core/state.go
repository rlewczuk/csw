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
	Info        AgentStateCommonInfo
	Role        *conf.AgentRoleConfig
	Task        *Task
	HookContext HookContext
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
	if s.HookContext != nil {
		cloned.HookContext = make(HookContext, len(s.HookContext))
		for key, value := range s.HookContext {
			cloned.HookContext[key] = value
		}
	}

	return cloned
}

// SetHookContextValue sets one hook context field.
func (s *AgentState) SetHookContextValue(key string, value string) {
	if s == nil || strings.TrimSpace(key) == "" {
		return
	}
	if s.HookContext == nil {
		s.HookContext = make(HookContext)
	}
	s.HookContext[key] = value
}

// MergeHookContext merges values into hook context.
func (s *AgentState) MergeHookContext(values map[string]string) {
	if s == nil {
		return
	}
	if s.HookContext == nil {
		s.HookContext = make(HookContext)
	}
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		s.HookContext[trimmedKey] = value
	}
}

// HookContextData returns a copy of hook context map.
func (s *AgentState) HookContextData() HookContext {
	if s == nil || len(s.HookContext) == 0 {
		return HookContext{}
	}
	result := make(HookContext, len(s.HookContext))
	for key, value := range s.HookContext {
		result[key] = value
	}

	return result
}
