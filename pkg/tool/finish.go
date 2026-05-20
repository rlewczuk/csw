package tool

import (
	"fmt"
	"strings"
)

// FinishSession is implemented by sessions that can be finished by tool call.
type FinishSession interface {
	RequestFinish(summary string)
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
	summary, ok := args.Arguments.Get("summary").AsStringOK()
	if !ok || strings.TrimSpace(summary) == "" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("FinishTool.Execute() [finish.go]: missing required argument: summary"),
			Done:  true,
		}
	}
	summary = strings.TrimSpace(summary)

	if t.session != nil {
		t.session.RequestFinish(summary)
	}

	var result ToolValue
	result.Set("status", "success")
	result.Set("message", "Session finish requested.")
	result.Set("summary", summary)

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns tool execution summary output.
func (t *FinishTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	summary := strings.TrimSpace(call.Arguments.Get("summary").AsString())
	oneLiner := "finish requested"
	full := "Session finish requested."
	if summary != "" {
		full = full + "\n\nSummary:\n" + summary
	}
	jsonl := buildToolRenderJSONL("finish", call, map[string]any{"action": "finish", "summary": summary})
	return oneLiner, full, jsonl, map[string]string{}
}

// GetDescription returns dynamic tool description.
func (t *FinishTool) GetDescription() (string, bool) {
	return "", false
}
