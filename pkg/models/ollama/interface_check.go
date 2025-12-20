package ollama

import "github.com/codesnort/codesnort-swe/pkg/models"

// Compile-time interface checks
var (
	_ models.ModelProvider  = (*OllamaClient)(nil)
	_ models.ChatModel      = (*OllamaChatModel)(nil)
	_ models.EmbeddingModel = (*OllamaEmbeddingModel)(nil)
)
