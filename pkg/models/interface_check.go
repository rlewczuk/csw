package models

// Compile-time interface checks
var (
	_ ModelProvider  = (*AnthropicClient)(nil)
	_ ChatModel      = (*AnthropicChatModel)(nil)

	_ ModelProvider  = (*OllamaClient)(nil)
	_ ChatModel      = (*OllamaChatModel)(nil)

	_ ModelProvider  = (*OpenAIClient)(nil)
	_ ChatModel      = (*OpenAIChatModel)(nil)

	_ ModelProvider  = (*ResponsesClient)(nil)
	_ ChatModel      = (*ResponsesChatModel)(nil)

	_ ModelProvider  = (*JetBrainsClient)(nil)
	_ ChatModel      = (*JetBrainsChatModel)(nil)

	_ ModelProvider  = (*MockClient)(nil)
	_ ChatModel      = (*MockChatModel)(nil)

	_ ChatModel = (*RetryChatModel)(nil)
	_ ChatModel = (*FallbackChatModel)(nil)
	_ ChatModel = (*UnstreamingChatModel)(nil)
)
