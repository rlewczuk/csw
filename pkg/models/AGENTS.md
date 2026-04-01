# Package `pkg/models` Overview

Package `pkg/models` is the model/provider abstraction layer for chat and embedding backends. It defines common model interfaces, provider registry/factory behavior, concrete provider clients, OAuth/token helpers, tagging logic, and shared DTO/error/logging utilities.

## Important files

* `api.go` - Core public model API and message types
* `providers.go` - Provider registry and config factory
* `tags.go` - Model-tag resolution from config mappings
* `errors.go` - Shared model/provider error taxonomy
* `logging.go` - HTTP request/response logging utilities
* `oauth.go` - OAuth2 PKCE, token refresh, JWT helpers
* `useragent.go` - User-Agent header generation
* `verbose.go` - Verbose HTTP logging with obfuscation
* `config_updater.go` - Config persistence callback for providers
* `mock.go` - Test doubles for provider/chat/embedding
* `unstreaming_chat_model.go` - Wrapper for non-streaming chat
* `fallback.go` - Multi-model fallback wrapper
* `retry.go` - Retry wrapper with exponential backoff
* `model_chain_factory.go` - Provider/model chain parser and factory
* `anthropic_client.go` - Anthropic API client implementation
* `anthropic_dto.go` - Anthropic API DTO types
* `openai_client.go` - OpenAI-compatible API client
* `openai_dto.go` - OpenAI API DTO types
* `ollama_client.go` - Ollama API client implementation
* `ollama_dto.go` - Ollama API DTO types
* `jetbrains_client.go` - JetBrains AI client implementation
* `responses_client.go` - OpenAI Responses API client
* `responses_dto.go` - Responses API DTO types
* `interface_check.go` - Compile-time interface assertions

## Important public API objects

* `ChatModel` - Interface for chat-capable models
* `EmbeddingModel` - Interface for embedding generation
* `ModelProvider` - Interface for model provider factories
* `ProviderRegistry` - Manages provider instances from config
* `ModelTagRegistry` - Resolves model tags from config mappings
* `ChatMessage` - Message structure for chat conversations
* `ChatMessagePart` - Part of a message (text, tool call, response)
* `ChatOptions` - Options for chat requests (temperature, etc.)
* `TokenUsage` - Token accounting from LLM responses
* `ChatRole` - Role constants: `ChatRoleAssistant`, `ChatRoleDeveloper`, `ChatRoleSystem`, `ChatRoleUser`
* `ModelInfo` - Information about available models
* `ModelType` - Model type enum: `ModelTypeChat`, `ModelTypeEmbed`
* `ConfigUpdater` - Callback for persisting config changes
* `LLMRequestError` - Error type with raw HTTP response
* `RateLimitError` - Rate limit error with retry info
* `NetworkError` - Retryable network error wrapper
* `APIRequestError` - Structured API request validation error
* `RetryPolicy` - Controls retry behavior for LLM errors
* `RetryChatModel` - Wrapper adding retry logic to chat models
* `FallbackChatModel` - Multi-model fallback wrapper
* `UnstreamingChatModel` - Wrapper for sync chat over streaming models
* `OpenAIClient` - OpenAI-compatible provider client
* `AnthropicClient` - Anthropic API provider client
* `OllamaClient` - Ollama API provider client
* `ResponsesClient` - OpenAI Responses API client
* `JetBrainsClient` - JetBrains AI provider client
* `MockClient` - Test double for ModelProvider
* `NewProviderRegistry` - Creates provider registry from config
* `ModelFromConfig` - Factory for creating providers from config
* `NewTextMessage` - Creates text-only chat message
* `NewToolCallMessage` - Creates message with tool calls
* `NewToolResponseMessage` - Creates message with tool responses
* `NewUnstreamingChatModel` - Wraps streaming model for sync use
* `NewRetryChatModel` - Wraps chat model with retry logic
* `NewFallbackChatModel` - Creates fallback wrapper for multiple models
* `NewChatModelFromProviderChain` - Creates chat model from provider chain spec
* `NewMockProvider` - Creates mock provider for testing
* `DefaultRetryPolicy` - Returns default retry policy configuration
* `IsRetryableError` - Checks if error is retryable
* `IsOAuth2Provider` - Checks if provider uses OAuth2
* `ExtractJWTExpiry` - Extracts expiry from JWT token
