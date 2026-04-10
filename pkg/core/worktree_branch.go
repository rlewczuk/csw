package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
)

const (
	worktreeBranchTargetLength = 20
	worktreeBranchMaxLength    = 25
)

// WorktreeBranchPromptData contains data for rendering worktree branch prompt templates.
type WorktreeBranchPromptData struct {
	Input string
}

// GenerateWorktreeBranchName generates a symbolic worktree branch suffix using the active chat model.
func GenerateWorktreeBranchName(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore conf.ConfigStore, model string, inputPrompt string) (string, error) {
	if modelProviders == nil {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: model providers cannot be nil")
	}

	if configStore == nil {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: config store cannot be nil")
	}

	aliasValues, err := configStore.GetModelAliases()
	if err != nil {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: failed to load model aliases: %w", err)
	}
	aliases, err := models.NormalizeModelAliasMap(aliasValues)
	if err != nil {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: failed to normalize model aliases: %w", err)
	}

	modelRefs, err := models.ExpandProviderModelChain(model, aliases)
	if err != nil {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: failed to resolve model spec: %w", err)
	}
	if len(modelRefs) == 0 {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: resolved model spec is empty")
	}
	providerName := modelRefs[0].Provider
	modelName := modelRefs[0].Model
	for _, ref := range modelRefs {
		if _, ok := modelProviders[ref.Provider]; !ok {
			return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: provider not found: %s", ref.Provider)
		}
	}

	provider, ok := modelProviders[providerName]
	if !ok {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: provider not found: %s", providerName)
	}

	systemPrompt, messageTemplate, err := LoadWorktreeBranchPromptTemplates(configStore)
	if err != nil {
		return "", err
	}

	userPrompt, err := RenderWorktreeBranchPrompt(messageTemplate, WorktreeBranchPromptData{Input: inputPrompt})
	if err != nil {
		return "", err
	}

	chatModel := provider.ChatModel(modelName, nil)
	response, err := chatModel.Chat(ctx, []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, systemPrompt),
		models.NewTextMessage(models.ChatRoleUser, userPrompt),
	}, nil, nil)
	if err != nil {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: failed to generate branch name: %w", err)
	}

	branch := NormalizeWorktreeBranchSymbolicName(response.GetText())
	if branch == "" {
		return "", fmt.Errorf("GenerateWorktreeBranchName() [worktree_branch.go]: generated branch name is empty")
	}

	return branch, nil
}

// LoadWorktreeBranchPromptTemplates loads worktree branch prompts from configuration store.
func LoadWorktreeBranchPromptTemplates(configStore conf.ConfigStore) (string, string, error) {
	systemPromptBytes, err := configStore.GetAgentConfigFile("worktree", "system.md")
	if err != nil {
		return "", "", fmt.Errorf("LoadWorktreeBranchPromptTemplates() [worktree_branch.go]: failed to read worktree/system.md: %w", err)
	}

	messageTemplateBytes, err := configStore.GetAgentConfigFile("worktree", "message.md")
	if err != nil {
		return "", "", fmt.Errorf("LoadWorktreeBranchPromptTemplates() [worktree_branch.go]: failed to read worktree/message.md: %w", err)
	}

	return string(systemPromptBytes), string(messageTemplateBytes), nil
}

// RenderWorktreeBranchPrompt renders a worktree branch prompt template with the given data.
func RenderWorktreeBranchPrompt(templateText string, data any) (string, error) {
	tmpl, err := template.New("worktree-branch-prompt").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("RenderWorktreeBranchPrompt() [worktree_branch.go]: failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("RenderWorktreeBranchPrompt() [worktree_branch.go]: failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// NormalizeWorktreeBranchSymbolicName converts raw model output into a branch-safe symbolic name.
func NormalizeWorktreeBranchSymbolicName(input string) string {
	raw := strings.ToLower(strings.TrimSpace(input))
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range raw {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			builder.WriteRune(r)
			lastDash = false
			continue
		}

		if builder.Len() == 0 || lastDash {
			continue
		}

		builder.WriteByte('-')
		lastDash = true
	}

	normalized := strings.Trim(builder.String(), "-")
	if len(normalized) > worktreeBranchTargetLength {
		normalized = truncateWorktreeBranchByWords(normalized)
	}

	return normalized
}

// truncateWorktreeBranchByWords trims a normalized branch suffix at word boundaries.
func truncateWorktreeBranchByWords(normalized string) string {
	if len(normalized) <= worktreeBranchTargetLength {
		return normalized
	}

	words := strings.Split(normalized, "-")
	if len(words) == 0 {
		return ""
	}

	current := ""
	for _, word := range words {
		if word == "" {
			continue
		}

		candidate := word
		if current != "" {
			candidate = current + "-" + word
		}

		if len(candidate) <= worktreeBranchTargetLength {
			current = candidate
			continue
		}

		if len(candidate) <= worktreeBranchMaxLength {
			return candidate
		}

		if current != "" {
			return current
		}

		return strings.Trim(candidate[:worktreeBranchMaxLength], "-")
	}

	return current
}
