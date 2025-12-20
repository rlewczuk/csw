package ollama

import "github.com/codesnort/codesnort-swe/pkg/models"

// Compile-time interface checks
var (
	_ models.ModelProvider  = (*OllamaClient)(nil)
	_ models.ChatModel      = (*OllamaClient)(nil)
	_ models.EmbeddingModel = (*OllamaClient)(nil)
)
