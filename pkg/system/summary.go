package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/vcs"
)

type sessionSummaryJSON struct {
	BaseCommitID     string                `json:"base_commit_id,omitempty"`
	HeadCommitID     string                `json:"head_commit_id,omitempty"`
	EditedFiles      []string              `json:"edited_files"`
	FinalTodoList    []tool.TodoItem       `json:"final_todo_list"`
	FinalTokenUsage  sessionTokenUsageJSON `json:"final_token_usage"`
	FinalContext     int                   `json:"final_context_length"`
	FinalCompactions int                   `json:"final_compaction_count"`
	FinalDuration    string                `json:"final_session_duration"`
	FinalTimestamp   string                `json:"final_session_timestamp"`
	ToolsUsed        []string              `json:"tools_used"`
	RolesUsed        []string              `json:"roles_used"`
	ModelUsed        string                `json:"model_used"`
	ThinkingLevel    string                `json:"thinking_level"`
	LSPServer        string                `json:"lsp_server,omitempty"`
	ContainerImage   string                `json:"container_image,omitempty"`
	TimeSpentSeconds float64               `json:"time_spent_seconds"`
	SessionID        string                `json:"session_id"`
}

type sessionTokenUsageJSON struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
	Cached int `json:"cached"`
}

func BuildSessionSummaryJSON(session *core.SweSession, buildResult BuildSystemResult, startTime time.Time, endTime time.Time, baseCommitID string, headCommitID string) sessionSummaryJSON {
	usage := session.TokenUsage()
	duration := endTime.Sub(startTime)
	return sessionSummaryJSON{
		BaseCommitID:     strings.TrimSpace(baseCommitID),
		HeadCommitID:     strings.TrimSpace(headCommitID),
		EditedFiles:      vcs.CollectEditedFiles(buildResult.WorkDirRoot, buildResult.WorkDir, baseCommitID, headCommitID),
		FinalTodoList:    session.GetTodoList(),
		FinalTokenUsage:  sessionTokenUsageJSON{Input: usage.InputTokens, Output: usage.OutputTokens, Total: usage.TotalTokens, Cached: usage.InputCachedTokens},
		FinalContext:     session.ContextLengthTokens(),
		FinalCompactions: session.CompactionCount(),
		FinalDuration:    duration.String(),
		FinalTimestamp:   endTime.Format(time.RFC3339Nano),
		ToolsUsed:        shared.SortedList(session.UsedTools()),
		RolesUsed:        shared.SortedList(session.UsedRoles()),
		ModelUsed:        strings.TrimSpace(session.ModelWithProvider()),
		ThinkingLevel:    strings.TrimSpace(session.ThinkingLevel()),
		LSPServer:        strings.TrimSpace(buildResult.LSPServer),
		ContainerImage:   strings.TrimSpace(buildResult.ContainerImage),
		TimeSpentSeconds: duration.Seconds(),
		SessionID:        session.ID(),
	}
}

func SaveSessionSummaryJSON(logsDir string, session *core.SweSession, buildResult BuildSystemResult, startTime time.Time, endTime time.Time, baseCommitID string, headCommitID string) error {
	if session == nil {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: session is nil")
	}

	if strings.TrimSpace(logsDir) == "" {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: logsDir is empty")
	}

	sessionLogDir := filepath.Join(logsDir, "sessions", session.ID())
	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: failed to create session log directory: %w", err)
	}

	summary := BuildSessionSummaryJSON(session, buildResult, startTime, endTime, baseCommitID, headCommitID)
	content, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: failed to marshal summary json: %w", err)
	}

	filePath := filepath.Join(sessionLogDir, "summary.json")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: failed to write summary json file: %w", err)
	}

	return nil
}

func EmitSessionSummary(startTime time.Time, endTime time.Time, session *core.SweSession, buildResult BuildSystemResult, showMessage func(string, shared.MessageType), sessionRunErr error, baseCommitID string, headCommitID string) error {
	duration := endTime.Sub(startTime)
	sessionInfo := BuildSessionSummaryMessage(duration, session, buildResult)
	if err := SaveSessionSummaryMarkdownFunc(buildResult.LogsDir, session, sessionInfo); err != nil {
		if sessionRunErr == nil {
			return fmt.Errorf("EmitSessionSummary() [cli.go]: failed to save session summary: %w", err)
		}

		if showMessage != nil {
			showMessage(fmt.Sprintf("Failed to save session summary: %v", err), ui.MessageTypeWarning)
		}
	}

	if err := SaveSessionSummaryJSONFunc(buildResult.LogsDir, session, buildResult, startTime, endTime, baseCommitID, headCommitID); err != nil {
		if sessionRunErr == nil {
			return fmt.Errorf("EmitSessionSummary() [cli.go]: failed to save session summary JSON: %w", err)
		}

		if showMessage != nil {
			showMessage(fmt.Sprintf("Failed to save session summary JSON: %v", err), ui.MessageTypeWarning)
		}
	}

	if showMessage != nil {
		showMessage(sessionInfo, ui.MessageTypeInfo)
	}

	return sessionRunErr
}

var SaveSessionSummaryMarkdownFunc = SaveSessionSummaryMarkdown
var SaveSessionSummaryJSONFunc = SaveSessionSummaryJSON

func BuildSessionSummaryMessage(duration time.Duration, session *core.SweSession, buildResult BuildSystemResult) string {
	base := fmt.Sprintf("Session completed in %s", duration.Round(time.Second))
	if session == nil {
		return base
	}

	usage := session.TokenUsage()
	primary := fmt.Sprintf(
		"%s | tokens(input=%d[cached=%d,noncached=%d], output=%d, total=%d) | context=%d",
		base,
		usage.InputTokens,
		usage.InputCachedTokens,
		usage.InputNonCachedTokens,
		usage.OutputTokens,
		usage.TotalTokens,
		session.ContextLengthTokens(),
	)

	lines := []string{primary}
	lines = append(lines, fmt.Sprintf("Model: %s", shared.NullValue(session.ModelWithProvider())))
	lines = append(lines, fmt.Sprintf("Thinking: %s", shared.NullValue(session.ThinkingLevel())))
	lines = append(lines, fmt.Sprintf("LSP server: %s", shared.NullValue(strings.TrimSpace(buildResult.LSPServer))))
	lines = append(lines, fmt.Sprintf("Container image: %s", shared.NullValue(strings.TrimSpace(buildResult.ContainerImage))))
	lines = append(lines, fmt.Sprintf("Roles used: %s", shared.FormatList(session.UsedRoles())))
	lines = append(lines, fmt.Sprintf("Tools used: %s", shared.FormatList(session.UsedTools())))
	lines = append(lines, "")
	lines = append(lines, "Edited files:")
	lines = append(lines, FormatEditedFilesSummary(buildResult.WorkDirRoot, buildResult.WorkDir))

	return strings.Join(lines, "\n")
}

func SaveSessionSummaryMarkdown(logsDir string, session *core.SweSession, sessionInfo string) error {
	if session == nil {
		return fmt.Errorf("saveSessionSummaryMarkdown() [cli.go]: session is nil")
	}

	if strings.TrimSpace(logsDir) == "" {
		return fmt.Errorf("saveSessionSummaryMarkdown() [cli.go]: logsDir is empty")
	}

	sessionLogDir := filepath.Join(logsDir, "sessions", session.ID())
	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("saveSessionSummaryMarkdown() [cli.go]: failed to create session log directory: %w", err)
	}

	filePath := filepath.Join(sessionLogDir, "summary.md")
	content := BuildSessionSummaryMarkdown(session, sessionInfo)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("saveSessionSummaryMarkdown() [cli.go]: failed to write summary file: %w", err)
	}

	return nil
}

func BuildSessionSummaryMarkdown(session *core.SweSession, sessionInfo string) string {
	summary := strings.TrimSpace(core.LastAssistantMessageText(session))

	var builder strings.Builder
	builder.WriteString("# Summary\n\n")
	builder.WriteString(summary)
	builder.WriteString("\n\n# Session Info\n\n")
	builder.WriteString(strings.TrimSpace(sessionInfo))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("Session ID: %s\n", session.ID()))

	return builder.String()
}
