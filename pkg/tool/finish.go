package tool

// FinishSession is implemented by sessions that can be finished by tool call.
type FinishSession interface {
	RequestFinish()
}

// FinishTool requests normal session loop completion.
type FinishTool struct {
	session FinishSession
}

// NewFinishTool creates a new finish tool instance.
func NewFinishTool(session FinishSession) *FinishTool {
	return &FinishTool{session: session}
}

// Execute requests session completion and returns success response.
func (t *FinishTool) Execute(args *ToolCall) *ToolResponse {
	if t.session != nil {
		t.session.RequestFinish()
	}

	var result ToolValue
	result.Set("status", "success")
	result.Set("message", "Session finish requested.")

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns tool execution summary output.
func (t *FinishTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	oneLiner := "finish requested"
	full := "Session finish requested."
	jsonl := buildToolRenderJSONL("finish", call, map[string]any{"action": "finish"})
	return oneLiner, full, jsonl, map[string]string{}
}

// GetDescription returns dynamic tool description.
func (t *FinishTool) GetDescription() (string, bool) {
	return "", false
}
