package anthropic

import "github.com/codesnort/codesnort-swe/pkg/models"

// Compile-time interface checks
var (
	_ models.ModelProvider  = (*AnthropicClient)(nil)
	_ models.ChatModel      = (*AnthropicChatModel)(nil)
	_ models.EmbeddingModel = (*AnthropicEmbeddingModel)(nil)
)
