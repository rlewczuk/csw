package tool

import "fmt"

// HookFeedbackTool forwards hook feedback requests to runtime executor.
type HookFeedbackTool struct {
	executor HookFeedbackExecutor
}

// NewHookFeedbackTool creates a new hook feedback tool.
func NewHookFeedbackTool(executor HookFeedbackExecutor) *HookFeedbackTool {
	return &HookFeedbackTool{executor: executor}
}

// GetDescription returns optional extra description fragment.
func (t *HookFeedbackTool) GetDescription() (string, bool) {
	return "", false
}

// Execute processes one hook feedback request.
func (t *HookFeedbackTool) Execute(args *ToolCall) *ToolResponse {
	if t == nil || t.executor == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("HookFeedbackTool.Execute() [hook_feedback.go]: executor is not configured"), Done: true}
	}

	fn := args.Arguments.String("fn")
	if fn == "" {
		return &ToolResponse{Call: args, Error: fmt.Errorf("HookFeedbackTool.Execute() [hook_feedback.go]: missing required argument: fn"), Done: true}
	}

	request := HookFeedbackRequest{Fn: fn, ID: args.Arguments.String("id")}
	if rawArgs, ok := args.Arguments.Get("args").ObjectOK(); ok {
		request.Args = make(map[string]any, len(rawArgs))
		for key, value := range rawArgs {
			request.Args[key] = value.Raw()
		}
	}

	response := t.executor.ExecuteHookFeedback(request)

	result := NewToolValue(map[string]any{
		"id":     response.ID,
		"fn":     response.Fn,
		"ok":     response.OK,
		"result": response.Result,
		"error":  response.Error,
	})

	return &ToolResponse{Call: args, Result: result, Done: true}
}

// Render renders tool call for textual views.
func (t *HookFeedbackTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	oneLiner := "hook feedback"
	if call != nil {
		if fn := call.Arguments.String("fn"); fn != "" {
			oneLiner = "hook feedback: " + fn
		}
	}
	jsonl := buildToolRenderJSONL("hookFeedback", call, nil)
	return oneLiner, oneLiner, jsonl, make(map[string]string)
}
