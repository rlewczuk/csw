package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
)

// buildAdditionalAgentMessages builds user messages with extra AGENTS.md instructions for vfsRead/vfsGrep tool calls.
func (s *SweSession) buildAdditionalAgentMessages(toolCall *tool.ToolCall, response *tool.ToolResponse) ([]*models.ChatMessage, []string, error) {
	if s == nil || s.promptGenerator == nil || toolCall == nil || response == nil || response.Error != nil {
		return nil, nil, nil
	}

	var dirs []string
	switch toolCall.Function {
	case "vfsRead":
		path, ok := toolCall.Arguments.StringOK("path")
		if !ok || strings.TrimSpace(path) == "" {
			return nil, nil, nil
		}
		dirs = append(dirs, filepath.Dir(path))
	case "vfsGrep":
		dirs = append(dirs, parseDirsFromGrepResult(response.Result.Get("content").AsString())...)
	default:
		return nil, nil, nil
	}

	messages := make([]*models.ChatMessage, 0)
	loadedPaths := make([]string, 0)
	for _, dir := range uniqueStrings(dirs) {
		dirMessages, dirLoadedPaths, err := s.buildAdditionalAgentMessageForDir(dir)
		if err != nil {
			return nil, nil, err
		}
		if len(dirMessages) > 0 {
			messages = append(messages, dirMessages...)
		}
		if len(dirLoadedPaths) > 0 {
			loadedPaths = append(loadedPaths, dirLoadedPaths...)
		}
	}

	return messages, loadedPaths, nil
}

// buildAdditionalAgentMessageForDir creates user messages from AGENTS.md files in the
// provided directory and its parent directories if not loaded yet.
func (s *SweSession) buildAdditionalAgentMessageForDir(dir string) ([]*models.ChatMessage, []string, error) {
	rootPath := ""
	if s.VFS != nil {
		rootPath = s.VFS.WorktreePath()
	}
	if strings.TrimSpace(rootPath) == "" {
		rootPath = s.workDir
	}
	workDirAbs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, nil, fmt.Errorf("SweSession.buildAdditionalAgentMessageForDir() [session_agents.go]: failed to resolve root path %q: %w", rootPath, err)
	}

	resolvedDir := dir
	if strings.TrimSpace(resolvedDir) == "" {
		resolvedDir = "."
	}
	if !filepath.IsAbs(resolvedDir) {
		resolvedDir = filepath.Join(workDirAbs, resolvedDir)
	}
	resolvedDir, err = filepath.Abs(resolvedDir)
	if err != nil {
		return nil, nil, fmt.Errorf("SweSession.buildAdditionalAgentMessageForDir() [session_agents.go]: failed to resolve dir %q: %w", dir, err)
	}

	relDir, err := filepath.Rel(workDirAbs, resolvedDir)
	if err != nil {
		return nil, nil, fmt.Errorf("SweSession.buildAdditionalAgentMessageForDir() [session_agents.go]: failed to get relative dir for %q: %w", resolvedDir, err)
	}
	if relDir == "." || relDir == "" || strings.HasPrefix(relDir, "..") || filepath.IsAbs(relDir) {
		return nil, nil, nil
	}

	if s.loadedAgentFiles == nil {
		s.loadedAgentFiles = make(map[string]struct{})
	}

	files, err := s.promptGenerator.GetAgentFiles(relDir)
	if err != nil {
		return nil, nil, fmt.Errorf("SweSession.buildAdditionalAgentMessageForDir() [session_agents.go]: failed to get agent files for %q: %w", relDir, err)
	}
	if len(files) == 0 {
		return nil, nil, nil
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		depthI := strings.Count(filepath.Clean(paths[i]), string(filepath.Separator))
		depthJ := strings.Count(filepath.Clean(paths[j]), string(filepath.Separator))
		if depthI != depthJ {
			return depthI > depthJ
		}
		return paths[i] < paths[j]
	})

	messages := make([]*models.ChatMessage, 0, len(paths))
	loadedPaths := make([]string, 0, len(paths))
	for _, agentsPath := range paths {
		if _, loaded := s.loadedAgentFiles[agentsPath]; loaded {
			continue
		}
		s.loadedAgentFiles[agentsPath] = struct{}{}
		loadedPaths = append(loadedPaths, agentsPath)
		wrapped := "<system>\n" + files[agentsPath] + "\n</system>"
		messages = append(messages, models.NewTextMessage(models.ChatRoleUser, wrapped))
	}

	if len(messages) == 0 {
		return nil, nil, nil
	}

	return messages, loadedPaths, nil
}

// parseDirsFromGrepResult extracts directories from vfsGrep result content.
func parseDirsFromGrepResult(content string) []string {
	lines := strings.Split(content, "\n")
	dirs := make([]string, 0, len(lines))
	linePattern := regexp.MustCompile(`^(.+):(\d+)$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "(") || line == "No files found" {
			continue
		}
		matches := linePattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		dirs = append(dirs, filepath.Dir(matches[1]))
	}
	return dirs
}

// uniqueStrings returns a deduplicated slice preserving input order.
func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// buildAgentFileLoadNotifications creates notifications for newly loaded AGENTS.md files.
func buildAgentFileLoadNotifications(agentsPaths []string) []tool.ToolNotification {
	if len(agentsPaths) == 0 {
		return nil
	}

	notifications := make([]tool.ToolNotification, 0, len(agentsPaths))
	for _, agentsPath := range uniqueStrings(agentsPaths) {
		trimmedPath := strings.TrimSpace(agentsPath)
		if trimmedPath == "" {
			continue
		}

		notifications = append(notifications, tool.ToolNotification{
			Type:    "agents_auto_loaded",
			Path:    trimmedPath,
			Message: fmt.Sprintf("AGENTS.md from %q was automatically loaded.", filepath.Dir(trimmedPath)),
		})
	}

	if len(notifications) == 0 {
		return nil
	}

	return notifications
}
