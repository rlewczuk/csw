package tool

import (
	"errors"
	"fmt"
	"strings"
)

// HookFeedbackTool sends hook feedback messages from hook-started subagent sessions.
type HookFeedbackTool struct {
	executor         HookFeedbackExecutor
	modelResolver    func() string
	thinkingResolver func() string
}

// NewHookFeedbackTool creates a new HookFeedbackTool instance.
func NewHookFeedbackTool(executor HookFeedbackExecutor, modelResolver func() string, thinkingResolver func() string) *HookFeedbackTool {
	return &HookFeedbackTool{
		executor:         executor,
		modelResolver:    modelResolver,
		thinkingResolver: thinkingResolver,
	}
}

// GetDescription returns additional dynamic description.
func (t *HookFeedbackTool) GetDescription() (string, bool) {
	return "", false
}

// Execute processes hook feedback payload and returns execution result.
func (t *HookFeedbackTool) Execute(args *ToolCall) *ToolResponse {
	if t.executor == nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("HookFeedbackTool.Execute() [hook_feedback.go]: executor is nil"),
			Done:  true,
		}
	}

	fn, ok := args.Arguments.StringOK("fn")
	if !ok || strings.TrimSpace(fn) == "" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("HookFeedbackTool.Execute() [hook_feedback.go]: missing required argument: fn"),
			Done:  true,
		}
	}

	argsMap := map[string]any{}
	if rawArgs, exists := args.Arguments.GetOK("args"); exists && !rawArgs.IsNil() {
		objectValue, objectOK := rawArgs.ObjectOK()
		if !objectOK {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("HookFeedbackTool.Execute() [hook_feedback.go]: args must be an object"),
				Done:  true,
			}
		}
		argsMap = make(map[string]any, len(objectValue))
		for key, value := range objectValue {
			argsMap[key] = value.Raw()
		}
	}

	request := HookFeedbackRequest{
		Fn:   strings.TrimSpace(fn),
		Args: argsMap,
		ID:   strings.TrimSpace(args.Arguments.String("id")),
	}

	if strings.EqualFold(request.Fn, "llm") {
		if strings.TrimSpace(hookFeedbackArgString(request.Args, "prompt")) == "" {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("HookFeedbackTool.Execute() [hook_feedback.go]: missing args.prompt"),
				Done:  true,
			}
		}
		if strings.TrimSpace(hookFeedbackArgString(request.Args, "model")) == "" && t.modelResolver != nil {
			request.Args["model"] = strings.TrimSpace(t.modelResolver())
		}
		if strings.TrimSpace(hookFeedbackArgString(request.Args, "thinking")) == "" && t.thinkingResolver != nil {
			request.Args["thinking"] = strings.TrimSpace(t.thinkingResolver())
		}
	}

	response := t.executor.ExecuteHookFeedback(request)
	responseValue := NewToolValue(map[string]any{
		"fn":     response.Fn,
		"ok":     response.OK,
		"result": response.Result,
		"error":  response.Error,
	})
	if strings.TrimSpace(response.ID) != "" {
		responseValue.Set("id", response.ID)
	}

	if !response.OK {
		message := strings.TrimSpace(response.Error)
		if message == "" {
			message = fmt.Sprintf("HookFeedbackTool.Execute() [hook_feedback.go]: hook feedback request failed for fn=%s", response.Fn)
		}
		return &ToolResponse{Call: args, Error: errors.New(message), Result: responseValue, Done: true}
	}

	return &ToolResponse{Call: args, Result: responseValue, Done: true}
}

// Render returns a string representation of the hook feedback call.
func (t *HookFeedbackTool) Render(call *ToolCall) (string, string, map[string]string) {
	fn := strings.TrimSpace(call.Arguments.String("fn"))
	if fn == "" {
		fn = "unknown"
	}
	oneLiner := truncateString("hookFeedback "+fn, 128)
	return oneLiner, oneLiner, map[string]string{}
}

func hookFeedbackArgString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, exists := args[key]
	if !exists {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprintf("%v", value)
}
