package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
)

const (
	commitPromptSystemPath  = "pkg/conf/impl/conf/agent/commit/system.md"
	commitPromptPromptPath  = "pkg/conf/impl/conf/agent/commit/prompt.md"
	commitPromptMessagePath = "pkg/conf/impl/conf/agent/commit/message.md"
)

type commitPromptData struct {
	Messages []string
}

type commitMessageTemplateData struct {
	Branch  string
	Message string
}

// generateWorktreeCommitMessage generates a short commit message using the active chat model.
func generateWorktreeCommitMessage(ctx context.Context, sweSystem *core.SweSystem, session *core.SweSession, branch string, customTemplate string) (string, error) {
	if sweSystem == nil {
		return "", fmt.Errorf("generateWorktreeCommitMessage() [commit_message.go]: sweSystem cannot be nil")
	}
	if session == nil {
		return "", fmt.Errorf("generateWorktreeCommitMessage() [commit_message.go]: session cannot be nil")
	}
	if sweSystem.VFS == nil {
		return "", fmt.Errorf("generateWorktreeCommitMessage() [commit_message.go]: sweSystem VFS cannot be nil")
	}

	systemPrompt, promptTemplate, messageTemplate, err := loadCommitPromptTemplates(sweSystem)
	if err != nil {
		return "", err
	}

	if customTemplate != "" {
		messageTemplate = customTemplate
	}

	userMessages := collectUserMessages(session.ChatMessages())
	if len(userMessages) == 0 {
		userMessages = []string{"No explicit user task provided"}
	}

	llmPrompt, err := renderCommitPrompt(promptTemplate, commitPromptData{Messages: userMessages})
	if err != nil {
		return "", err
	}

	providerName := session.ProviderName()
	provider, ok := sweSystem.ModelProviders[providerName]
	if !ok {
		return "", fmt.Errorf("generateWorktreeCommitMessage() [commit_message.go]: provider not found: %s", providerName)
	}

	chatModel := provider.ChatModel(session.Model(), nil)
	response, err := chatModel.Chat(ctx, []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, systemPrompt),
		models.NewTextMessage(models.ChatRoleUser, llmPrompt),
	}, nil, nil)
	if err != nil {
		return "", fmt.Errorf("generateWorktreeCommitMessage() [commit_message.go]: failed to generate commit message: %w", err)
	}

	shortMessage := limitWords(strings.TrimSpace(response.GetText()), 10)
	if shortMessage == "" {
		return "", fmt.Errorf("generateWorktreeCommitMessage() [commit_message.go]: generated message is empty")
	}

	finalMessage, err := renderCommitPrompt(messageTemplate, commitMessageTemplateData{
		Branch:  branch,
		Message: shortMessage,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(finalMessage), nil
}

func loadCommitPromptTemplates(sweSystem *core.SweSystem) (string, string, string, error) {
	systemPromptBytes, err := sweSystem.VFS.ReadFile(commitPromptSystemPath)
	if err != nil {
		return "", "", "", fmt.Errorf("loadCommitPromptTemplates() [commit_message.go]: failed to read %s: %w", commitPromptSystemPath, err)
	}
	promptTemplateBytes, err := sweSystem.VFS.ReadFile(commitPromptPromptPath)
	if err != nil {
		return "", "", "", fmt.Errorf("loadCommitPromptTemplates() [commit_message.go]: failed to read %s: %w", commitPromptPromptPath, err)
	}
	messageTemplateBytes, err := sweSystem.VFS.ReadFile(commitPromptMessagePath)
	if err != nil {
		return "", "", "", fmt.Errorf("loadCommitPromptTemplates() [commit_message.go]: failed to read %s: %w", commitPromptMessagePath, err)
	}

	return string(systemPromptBytes), string(promptTemplateBytes), string(messageTemplateBytes), nil
}

func renderCommitPrompt(templateText string, data any) (string, error) {
	tmpl, err := template.New("commit-prompt").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("renderCommitPrompt() [commit_message.go]: failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("renderCommitPrompt() [commit_message.go]: failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func collectUserMessages(messages []*models.ChatMessage) []string {
	result := make([]string, 0)
	for _, msg := range messages {
		if msg == nil || msg.Role != models.ChatRoleUser {
			continue
		}
		text := strings.TrimSpace(msg.GetText())
		if text == "" {
			continue
		}
		result = append(result, text)
	}
	return result
}

func limitWords(input string, maxWords int) string {
	if maxWords <= 0 {
		return ""
	}
	words := strings.Fields(input)
	if len(words) <= maxWords {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:maxWords], " ")
}
