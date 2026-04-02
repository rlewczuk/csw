package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vcs"
)

// SessionSummaryBuildResult stores runtime fields required to build and persist session summaries.
type SessionSummaryBuildResult struct {
	LogsDir        string
	WorkDirRoot    string
	WorkDir        string
	LSPServer      string
	ContainerImage string
}

// SessionSummaryJSON is a JSON representation of completed session summary details.
type SessionSummaryJSON struct {
	BaseCommitID     string                `json:"base_commit_id,omitempty"`
	HeadCommitID     string                `json:"head_commit_id,omitempty"`
	EditedFiles      []string              `json:"edited_files"`
	FinalTodoList    []tool.TodoItem       `json:"final_todo_list"`
	FinalTokenUsage  SessionTokenUsageJSON `json:"final_token_usage"`
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

// SessionTokenUsageJSON stores final token usage values in summary JSON.
type SessionTokenUsageJSON struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
	Cached int `json:"cached"`
}

// SubAgentSummaryJSON is a JSON representation of completed subagent session summary details.
type SubAgentSummaryJSON struct {
	SessionID       string          `json:"session_id"`
	ParentSessionID string          `json:"parent_session_id,omitempty"`
	Status          string          `json:"status"`
	Summary         string          `json:"summary,omitempty"`
	FinalTodoList   []tool.TodoItem `json:"final_todo_list"`
	ModelUsed       string          `json:"model_used,omitempty"`
	ThinkingLevel   string          `json:"thinking_level,omitempty"`
	CompletedAt     string          `json:"completed_at"`
}

// BuildSessionSummaryJSON builds a JSON summary payload from completed session state.
func BuildSessionSummaryJSON(session *SweSession, buildResult SessionSummaryBuildResult, startTime time.Time, endTime time.Time, baseCommitID string, headCommitID string) SessionSummaryJSON {
	usage := session.TokenUsage()
	duration := endTime.Sub(startTime)

	return SessionSummaryJSON{
		BaseCommitID:     strings.TrimSpace(baseCommitID),
		HeadCommitID:     strings.TrimSpace(headCommitID),
		EditedFiles:      vcs.CollectEditedFiles(buildResult.WorkDirRoot, buildResult.WorkDir, baseCommitID, headCommitID),
		FinalTodoList:    session.GetTodoList(),
		FinalTokenUsage:  SessionTokenUsageJSON{Input: usage.InputTokens, Output: usage.OutputTokens, Total: usage.TotalTokens, Cached: usage.InputCachedTokens},
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

// SaveSessionSummaryJSON saves JSON summary for completed session to logs directory.
func SaveSessionSummaryJSON(logsDir string, session *SweSession, buildResult SessionSummaryBuildResult, startTime time.Time, endTime time.Time, baseCommitID string, headCommitID string) error {
	if session == nil {
		return fmt.Errorf("SaveSessionSummaryJSON() [session_summary.go]: session is nil")
	}
	if strings.TrimSpace(logsDir) == "" {
		return fmt.Errorf("SaveSessionSummaryJSON() [session_summary.go]: logsDir is empty")
	}

	sessionLogDir := filepath.Join(logsDir, "sessions", session.ID())
	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("SaveSessionSummaryJSON() [session_summary.go]: failed to create session log directory: %w", err)
	}

	summary := BuildSessionSummaryJSON(session, buildResult, startTime, endTime, baseCommitID, headCommitID)
	content, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("SaveSessionSummaryJSON() [session_summary.go]: failed to marshal summary json: %w", err)
	}

	filePath := filepath.Join(sessionLogDir, "summary.json")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("SaveSessionSummaryJSON() [session_summary.go]: failed to write summary json file: %w", err)
	}

	return nil
}

// EmitSessionSummary persists and emits final session summary.
func EmitSessionSummary(startTime time.Time, endTime time.Time, session *SweSession, buildResult SessionSummaryBuildResult, showMessage func(string, shared.MessageType), sessionRunErr error, baseCommitID string, headCommitID string) error {
	duration := endTime.Sub(startTime)
	sessionInfo := BuildSessionSummaryMessage(duration, session, buildResult)

	if err := SaveSessionSummaryMarkdownFunc(buildResult.LogsDir, session, sessionInfo); err != nil {
		if sessionRunErr == nil {
			return fmt.Errorf("EmitSessionSummary() [session_summary.go]: failed to save session summary: %w", err)
		}
		if showMessage != nil {
			showMessage(fmt.Sprintf("Failed to save session summary: %v", err), shared.MessageTypeWarning)
		}
	}

	if err := SaveSessionSummaryJSONFunc(buildResult.LogsDir, session, buildResult, startTime, endTime, baseCommitID, headCommitID); err != nil {
		if sessionRunErr == nil {
			return fmt.Errorf("EmitSessionSummary() [session_summary.go]: failed to save session summary JSON: %w", err)
		}
		if showMessage != nil {
			showMessage(fmt.Sprintf("Failed to save session summary JSON: %v", err), shared.MessageTypeWarning)
		}
	}

	if showMessage != nil {
		showMessage(sessionInfo, shared.MessageTypeInfo)
	}

	return sessionRunErr
}

// SaveSessionSummaryMarkdownFunc is a test hook for markdown summary saver.
var SaveSessionSummaryMarkdownFunc = SaveSessionSummaryMarkdown

// SaveSessionSummaryJSONFunc is a test hook for JSON summary saver.
var SaveSessionSummaryJSONFunc = SaveSessionSummaryJSON

// BuildSessionSummaryMessage builds user-facing multi-line session summary text.
func BuildSessionSummaryMessage(duration time.Duration, session *SweSession, buildResult SessionSummaryBuildResult) string {
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

// SaveSessionSummaryMarkdown saves markdown summary for completed session to logs directory.
func SaveSessionSummaryMarkdown(logsDir string, session *SweSession, sessionInfo string) error {
	if session == nil {
		return fmt.Errorf("SaveSessionSummaryMarkdown() [session_summary.go]: session is nil")
	}
	if strings.TrimSpace(logsDir) == "" {
		return fmt.Errorf("SaveSessionSummaryMarkdown() [session_summary.go]: logsDir is empty")
	}

	sessionLogDir := filepath.Join(logsDir, "sessions", session.ID())
	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("SaveSessionSummaryMarkdown() [session_summary.go]: failed to create session log directory: %w", err)
	}

	filePath := filepath.Join(sessionLogDir, "summary.md")
	content := BuildSessionSummaryMarkdown(session, sessionInfo)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("SaveSessionSummaryMarkdown() [session_summary.go]: failed to write summary file: %w", err)
	}

	return nil
}

// BuildSessionSummaryMarkdown builds markdown content containing summary and session details.
func BuildSessionSummaryMarkdown(session *SweSession, sessionInfo string) string {
	summary := strings.TrimSpace(LastAssistantMessageText(session))

	var builder strings.Builder
	builder.WriteString("# Summary\n\n")
	builder.WriteString(summary)
	builder.WriteString("\n\n# Session Info\n\n")
	builder.WriteString(strings.TrimSpace(sessionInfo))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("Session ID: %s\n", session.ID()))

	return builder.String()
}

// LastAssistantMessageText returns the latest assistant text content from session history.
func LastAssistantMessageText(session *SweSession) string {
	if session == nil {
		return ""
	}

	messages := session.ChatMessages()
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message == nil || message.Role != models.ChatRoleAssistant {
			continue
		}

		var textBuilder strings.Builder
		for _, part := range message.Parts {
			if part.Text != "" {
				textBuilder.WriteString(part.Text)
			}
		}
		if textBuilder.Len() > 0 {
			return textBuilder.String()
		}

		for _, part := range message.Parts {
			if part.ReasoningContent != "" {
				textBuilder.WriteString(part.ReasoningContent)
			}
		}

		return textBuilder.String()
	}

	return ""
}

// FormatEditedFilesSummary formats changed and untracked files as summary list.
func FormatEditedFilesSummary(workDirRoot string, workDir string) string {
	diffDir := vcs.ChooseGitDiffDir(workDirRoot, workDir)
	cmd := exec.Command("git", "diff", "--numstat")
	cmd.Dir = diffDir

	output, err := cmd.Output()
	if err != nil {
		return "-"
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	lines := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}

		lines = append(lines, fmt.Sprintf("- %s (+%s/-%s)", parts[2], parts[0], parts[1]))
	}

	untrackedCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	untrackedCmd.Dir = diffDir
	untrackedOutput, untrackedErr := untrackedCmd.Output()
	if untrackedErr == nil {
		untrackedScanner := bufio.NewScanner(bytes.NewReader(untrackedOutput))
		for untrackedScanner.Scan() {
			path := strings.TrimSpace(untrackedScanner.Text())
			if path == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s (new file)", path))
		}
	}

	if len(lines) == 0 {
		return "-"
	}

	return strings.Join(lines, "\n")
}

// WriteSubAgentSummary persists JSON and markdown summary files for a subagent session.
func WriteSubAgentSummary(logBaseDir string, session *SweSession, summary SubAgentSummaryJSON) error {
	if session == nil || strings.TrimSpace(logBaseDir) == "" {
		return nil
	}

	dir := filepath.Join(logBaseDir, "sessions", session.ID())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("WriteSubAgentSummary() [session_summary.go]: failed to create session summary dir: %w", err)
	}

	jsonData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("WriteSubAgentSummary() [session_summary.go]: failed to marshal summary json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), jsonData, 0644); err != nil {
		return fmt.Errorf("WriteSubAgentSummary() [session_summary.go]: failed to write summary json: %w", err)
	}

	markdown := strings.TrimSpace(summary.Summary)
	if markdown == "" {
		markdown = "(no summary)"
	}
	content := fmt.Sprintf("# Summary\n\n%s\n\n# Session Info\n\nSession ID: %s\nParent Session ID: %s\nStatus: %s\n", markdown, summary.SessionID, summary.ParentSessionID, summary.Status)
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("WriteSubAgentSummary() [session_summary.go]: failed to write summary markdown: %w", err)
	}

	return nil
}
