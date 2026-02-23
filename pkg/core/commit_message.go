package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/rlewczuk/csw/pkg/models"
)

// CommitPromptData contains data for rendering commit prompt templates.
type CommitPromptData struct {
	Messages []string
}

// CommitMessageTemplateData contains data for rendering the final commit message.
type CommitMessageTemplateData struct {
	Branch  string
	Message string
}

// CommitPromptPaths contains paths to commit prompt template files.
type CommitPromptPaths struct {
	SystemPath  string
	PromptPath  string
	MessagePath string
}

// GenerateCommitMessage generates a short commit message using the active chat model.
// The paths parameter specifies VFS paths to the system prompt, user prompt template,
// and message template files.
// If customMessageTemplate is non-empty, it overrides the message template from paths.
func GenerateCommitMessage(ctx context.Context, sweSystem *SweSystem, session *SweSession, paths CommitPromptPaths, branch string, customMessageTemplate string) (string, error) {
	if sweSystem == nil {
		return "", fmt.Errorf("GenerateCommitMessage() [commit_message.go]: sweSystem cannot be nil")
	}

	if session == nil {
		return "", fmt.Errorf("GenerateCommitMessage() [commit_message.go]: session cannot be nil")
	}

	if sweSystem.VFS == nil {
		return "", fmt.Errorf("GenerateCommitMessage() [commit_message.go]: sweSystem VFS cannot be nil")
	}

	systemPrompt, promptTemplate, messageTemplate, err := LoadCommitPromptTemplates(sweSystem.VFS, paths)
	if err != nil {
		return "", err
	}

	if customMessageTemplate != "" {
		messageTemplate = customMessageTemplate
	}

	userMessages := CollectUserMessages(session.ChatMessages())
	if len(userMessages) == 0 {
		userMessages = []string{"No explicit user task provided"}
	}

	llmPrompt, err := RenderCommitPrompt(promptTemplate, CommitPromptData{Messages: userMessages})
	if err != nil {
		return "", err
	}

	providerName := session.ProviderName()
	provider, ok := sweSystem.ModelProviders[providerName]
	if !ok {
		return "", fmt.Errorf("GenerateCommitMessage() [commit_message.go]: provider not found: %s", providerName)
	}

	chatModel := provider.ChatModel(session.Model(), nil)
	response, err := chatModel.Chat(ctx, []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, systemPrompt),
		models.NewTextMessage(models.ChatRoleUser, llmPrompt),
	}, nil, nil)
	if err != nil {
		return "", fmt.Errorf("GenerateCommitMessage() [commit_message.go]: failed to generate commit message: %w", err)
	}

	shortMessage := LimitWords(strings.TrimSpace(response.GetText()), 10)
	if shortMessage == "" {
		return "", fmt.Errorf("GenerateCommitMessage() [commit_message.go]: generated message is empty")
	}

	finalMessage, err := RenderCommitPrompt(messageTemplate, CommitMessageTemplateData{
		Branch:  branch,
		Message: shortMessage,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(finalMessage), nil
}

// LoadCommitPromptTemplates loads commit prompt templates from the VFS.
func LoadCommitPromptTemplates(vfs interface{ ReadFile(path string) ([]byte, error) }, paths CommitPromptPaths) (string, string, string, error) {
	systemPromptBytes, err := vfs.ReadFile(paths.SystemPath)
	if err != nil {
		return "", "", "", fmt.Errorf("LoadCommitPromptTemplates() [commit_message.go]: failed to read %s: %w", paths.SystemPath, err)
	}
	promptTemplateBytes, err := vfs.ReadFile(paths.PromptPath)
	if err != nil {
		return "", "", "", fmt.Errorf("LoadCommitPromptTemplates() [commit_message.go]: failed to read %s: %w", paths.PromptPath, err)
	}
	messageTemplateBytes, err := vfs.ReadFile(paths.MessagePath)
	if err != nil {
		return "", "", "", fmt.Errorf("LoadCommitPromptTemplates() [commit_message.go]: failed to read %s: %w", paths.MessagePath, err)
	}

	return string(systemPromptBytes), string(promptTemplateBytes), string(messageTemplateBytes), nil
}

// RenderCommitPrompt renders a commit prompt template with the given data.
func RenderCommitPrompt(templateText string, data any) (string, error) {
	tmpl, err := template.New("commit-prompt").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("RenderCommitPrompt() [commit_message.go]: failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("RenderCommitPrompt() [commit_message.go]: failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// CollectUserMessages extracts user text messages from a list of chat messages.
func CollectUserMessages(messages []*models.ChatMessage) []string {
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

// LimitWords limits the input string to a maximum number of words.
func LimitWords(input string, maxWords int) string {
	if maxWords <= 0 {
		return ""
	}
	words := strings.Fields(input)
	if len(words) <= maxWords {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:maxWords], " ")
}
