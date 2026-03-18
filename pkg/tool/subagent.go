package tool

import (
	"fmt"
	"strings"
)

// SubAgentTaskRequest contains input for a delegated subagent run.
type SubAgentTaskRequest struct {
	Slug     string
	Title    string
	Prompt   string
	Role     string
	Model    string
	Thinking string
	// HookFeedbackExecutor enables hookFeedback tool in child session when set.
	HookFeedbackExecutor HookFeedbackExecutor
}

// SubAgentTaskResult contains final subagent execution output.
type SubAgentTaskResult struct {
	Status  string
	Summary string
	Error   string
}

// HookFeedbackRequest defines one hook feedback message payload.
type HookFeedbackRequest struct {
	Fn   string
	Args map[string]any
	ID   string
}

// HookFeedbackResponse defines one processed hook feedback result.
type HookFeedbackResponse struct {
	ID     string
	Fn     string
	OK     bool
	Result any
	Error  string
}

// HookFeedbackExecutor handles hook feedback requests from hook subagent sessions.
type HookFeedbackExecutor interface {
	// ExecuteHookFeedback executes one hook feedback request.
	ExecuteHookFeedback(request HookFeedbackRequest) HookFeedbackResponse
}

// SubAgentExecutor executes delegated subagent tasks.
type SubAgentExecutor interface {
	ExecuteSubAgentTask(request SubAgentTaskRequest) (SubAgentTaskResult, error)
}

// SubAgentTool starts delegated tasks in child sessions.
type SubAgentTool struct {
	executor SubAgentExecutor
}

// NewSubAgentTool creates a new SubAgentTool instance.
func NewSubAgentTool(executor SubAgentExecutor) *SubAgentTool {
	return &SubAgentTool{executor: executor}
}

// GetDescription returns additional dynamic description.
func (t *SubAgentTool) GetDescription() (string, bool) {
	return "", false
}

// Execute executes subagent task in a child session.
func (t *SubAgentTool) Execute(args *ToolCall) *ToolResponse {
	if t.executor == nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("SubAgentTool.Execute() [subagent.go]: executor is nil"),
			Done:  true,
		}
	}

	slug, ok := args.Arguments.StringOK("slug")
	if !ok || strings.TrimSpace(slug) == "" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("SubAgentTool.Execute() [subagent.go]: missing required argument: slug"),
			Done:  true,
		}
	}

	title, ok := args.Arguments.StringOK("title")
	if !ok || strings.TrimSpace(title) == "" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("SubAgentTool.Execute() [subagent.go]: missing required argument: title"),
			Done:  true,
		}
	}

	prompt, ok := args.Arguments.StringOK("prompt")
	if !ok || strings.TrimSpace(prompt) == "" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("SubAgentTool.Execute() [subagent.go]: missing required argument: prompt"),
			Done:  true,
		}
	}

	result, err := t.executor.ExecuteSubAgentTask(SubAgentTaskRequest{
		Slug:     strings.TrimSpace(slug),
		Title:    strings.TrimSpace(title),
		Prompt:   prompt,
		Role:     strings.TrimSpace(args.Arguments.String("role")),
		Model:    strings.TrimSpace(args.Arguments.String("model")),
		Thinking: strings.TrimSpace(args.Arguments.String("thinking")),
	})
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	resultValue := NewToolValue(map[string]any{
		"status":  result.Status,
		"summary": result.Summary,
	})
	if strings.TrimSpace(result.Error) != "" {
		resultValue.Set("error", result.Error)
	}

	return &ToolResponse{Call: args, Result: resultValue, Done: true}
}

// Render returns a string representation of the subagent call.
func (t *SubAgentTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	slug := call.Arguments.String("slug")
	status := call.Arguments.String("status")
	summary := call.Arguments.String("summary")
	errMsg := call.Arguments.String("error")
	oneLiner := truncateString(fmt.Sprintf("subAgent %s (%s)", slug, status), 128)
	jsonl := buildToolRenderJSONL("subAgent", call, map[string]any{"slug": slug, "subagent_status": status, "summary": summary, "error": errMsg})
	if strings.TrimSpace(summary) == "" {
		return oneLiner, oneLiner, jsonl, make(map[string]string)
	}
	return oneLiner, oneLiner + "\n\n" + summary, jsonl, make(map[string]string)
}
