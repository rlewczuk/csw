package openai

import (
	"github.com/codesnort/codesnort-swe/pkg/models"
)

// Compile-time check to ensure OpenAIClient implements models.ModelProvider interface
var _ models.ModelProvider = (*OpenAIClient)(nil)

// Compile-time check to ensure OpenAIChatModel implements models.ChatModel interface
var _ models.ChatModel = (*OpenAIChatModel)(nil)

// Compile-time check to ensure OpenAIEmbeddingModel implements models.EmbeddingModel interface
var _ models.EmbeddingModel = (*OpenAIEmbeddingModel)(nil)
