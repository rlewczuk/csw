package core

import "github.com/rlewczuk/csw/pkg/models"

type AgentStateCommonInfo struct {
	AgentName           string
	WorkDir             string
	CurrentTime         string
	TokenUsage          models.TokenUsage
	ContextLengthTokens int
}

// TODO decide if this is still needed
type AgentState struct {
	Info AgentStateCommonInfo
}
