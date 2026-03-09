package tool

import (
	"encoding/json"
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
}

// SubAgentTaskResult contains final subagent execution output.
type SubAgentTaskResult struct {
	Status        string
	Summary       string
	FinalTodoList []TodoItem
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

	finalTodoList := make([]any, 0, len(result.FinalTodoList))
	todoJSON, marshalErr := json.Marshal(result.FinalTodoList)
	if marshalErr == nil {
		_ = json.Unmarshal(todoJSON, &finalTodoList)
	}

	resultValue := NewToolValue(map[string]any{
		"status":          result.Status,
		"summary":         result.Summary,
		"final_todo_list": finalTodoList,
	})

	return &ToolResponse{Call: args, Result: resultValue, Done: true}
}

// Render returns a string representation of the subagent call.
func (t *SubAgentTool) Render(call *ToolCall) (string, string, map[string]string) {
	slug := call.Arguments.String("slug")
	status := call.Arguments.String("status")
	summary := call.Arguments.String("summary")
	oneLiner := truncateString(fmt.Sprintf("subAgent %s (%s)", slug, status), 128)
	if strings.TrimSpace(summary) == "" {
		return oneLiner, oneLiner, make(map[string]string)
	}
	return oneLiner, oneLiner+"\n\n"+summary, make(map[string]string)
}
