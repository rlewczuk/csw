package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
)

// executeToolCalls executes the given tool calls and appends the results to the conversation.
func (s *SweSession) executeToolCalls(toolCalls []*tool.ToolCall) error {
	if s.logger != nil {
		s.logger.Debug("execute_tool_calls_start", "count", len(toolCalls))
	}

	toolResponses := make([]*tool.ToolResponse, 0, len(toolCalls)+len(s.pendingToolResponses))
	agentMessages := make([]*models.ChatMessage, 0)
	if len(s.pendingToolResponses) > 0 {
		toolResponses = append(toolResponses, s.pendingToolResponses...)
	}
	for _, toolCall := range toolCalls {
		s.toolsUsed = appendUniqueString(s.toolsUsed, toolCall.Function)
	}

	type toolExecutionResult struct {
		index    int
		toolCall *tool.ToolCall
		response *tool.ToolResponse
	}

	results := make([]toolExecutionResult, len(toolCalls))

	maxToolThreads := s.maxToolThreadsLimit()
	if maxToolThreads > len(toolCalls) {
		maxToolThreads = len(toolCalls)
	}
	if maxToolThreads <= 0 {
		maxToolThreads = 1
	}

	type indexedToolCall struct {
		index int
		call  *tool.ToolCall
	}

	jobs := make(chan indexedToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		jobs <- indexedToolCall{index: i, call: toolCall}
	}
	close(jobs)

	var (
		wg            sync.WaitGroup
		startGateMu   sync.Mutex
		lastStartTime time.Time
	)

	waitForStartSlot := func() {
		startGateMu.Lock()
		defer startGateMu.Unlock()
		if !lastStartTime.IsZero() {
			nextStartAt := lastStartTime.Add(s.toolExecutionStartDelayLimit())
			if sleepFor := time.Until(nextStartAt); sleepFor > 0 {
				time.Sleep(sleepFor)
			}
		}
		lastStartTime = time.Now()
	}

	for i := 0; i < maxToolThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				waitForStartSlot()

				if s.logger != nil {
					s.logger.Info("executing_tool_call", "tool", job.call.Function, "args", job.call.Arguments)
				}

				response := s.Tools.Execute(job.call)

				if s.logger != nil {
					s.logger.Info("tool_call_executed", "tool", job.call.Function, "response", response)
				}

				results[job.index] = toolExecutionResult{index: job.index, toolCall: job.call, response: response}
			}
		}()
	}

	wg.Wait()

	for _, result := range results {
		response := result.response
		if response == nil {
			continue
		}

		toolResponses = append(toolResponses, response)
		logging.LogToolResult(s.logger, response)

		newAgentMessages, loadedAgentPaths, err := s.buildAdditionalAgentMessages(result.toolCall, response)
		if err != nil {
			return fmt.Errorf("SweSession.executeToolCalls() [session_tools.go]: failed to load additional AGENTS.md instructions: %w", err)
		}
		agentMessages = append(agentMessages, newAgentMessages...)
		response.Notifications = append(response.Notifications, buildAgentFileLoadNotifications(loadedAgentPaths)...)

		if s.outputHandler != nil {
			s.decorateToolResponseForOutput(response)
			s.outputHandler.AddToolCallResult(response)
		}
	}

	// Add tool responses to the conversation
	s.appendConversationMessage(models.NewToolResponseMessage(toolResponses...), "incoming", "tool_response")

	if len(agentMessages) > 0 {
		for _, agentMessage := range agentMessages {
			s.appendConversationMessage(agentMessage, "incoming", "agent_instructions")
		}
	}

	s.pendingToolResponses = nil
	s.persistSessionState()

	return nil
}

// decorateToolResponseForOutput enriches tool responses with rendered output fields.
func (s *SweSession) decorateToolResponseForOutput(response *tool.ToolResponse) {
	if s == nil || s.Tools == nil || response == nil || response.Call == nil {
		return
	}

	renderCall := copyToolCallWithResultForRender(response.Call, response)
	summary, details, jsonl, meta := s.Tools.Render(renderCall)

	if strings.TrimSpace(summary) != "" {
		response.Result.Set("summary", summary)
	}
	if strings.TrimSpace(details) != "" {
		response.Result.Set("details", details)
	}
	if strings.TrimSpace(jsonl) != "" {
		response.Result.Set("jsonl", jsonl)
	}
	if len(meta) > 0 {
		metaAny := make(map[string]any, len(meta))
		for key, value := range meta {
			metaAny[key] = value
		}
		response.Result.Set("meta", metaAny)
	}
}

// copyToolCallWithResultForRender creates a render call containing original arguments and result fields.
func copyToolCallWithResultForRender(call *tool.ToolCall, response *tool.ToolResponse) *tool.ToolCall {
	if call == nil {
		return nil
	}

	args := make(map[string]any)
	if obj := call.Arguments.Object(); obj != nil {
		for key, value := range obj {
			args[key] = value.Raw()
		}
	}

	if response != nil {
		if obj := response.Result.Object(); obj != nil {
			for key, value := range obj {
				args[key] = value.Raw()
			}
		}
		if response.Error != nil {
			args["error"] = response.Error.Error()
		}
	}

	return &tool.ToolCall{
		ID:        call.ID,
		Function:  call.Function,
		Arguments: tool.NewToolValue(args),
		Access:    call.Access,
	}
}
