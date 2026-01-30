package core

type AgentStateCommonInfo struct {
	AgentName   string
	WorkDir     string
	CurrentTime string
}

// TODO decide if this is still needed
type AgentState struct {
	Info AgentStateCommonInfo
}
