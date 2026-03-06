package models

// Compile-time interface checks
var (
	_ ModelProvider  = (*AnthropicClient)(nil)
	_ ChatModel      = (*AnthropicChatModel)(nil)
	_ EmbeddingModel = (*AnthropicEmbeddingModel)(nil)

	_ ModelProvider  = (*OllamaClient)(nil)
	_ ChatModel      = (*OllamaChatModel)(nil)
	_ EmbeddingModel = (*OllamaEmbeddingModel)(nil)

	_ ModelProvider  = (*OpenAIClient)(nil)
	_ ChatModel      = (*OpenAIChatModel)(nil)
	_ EmbeddingModel = (*OpenAIEmbeddingModel)(nil)

	_ ModelProvider  = (*ResponsesClient)(nil)
	_ ChatModel      = (*ResponsesChatModel)(nil)
	_ EmbeddingModel = (*ResponsesEmbeddingModel)(nil)

	_ ModelProvider  = (*JetBrainsClient)(nil)
	_ ChatModel      = (*JetBrainsChatModel)(nil)
	_ EmbeddingModel = (*JetBrainsEmbeddingModel)(nil)

	_ ModelProvider  = (*MockClient)(nil)
	_ ChatModel      = (*MockChatModel)(nil)
	_ EmbeddingModel = (*MockEmbeddingModel)(nil)
)
