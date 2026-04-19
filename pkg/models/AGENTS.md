# Package `pkg/models` Overview

Package `pkg/models` provides model/provider abstractions and clients for `pkg/models`.

## Important files

* `api.go` - Core interfaces, messages, roles, and options.
* `providers.go` - Provider registry and config-based factory.
* `errors.go` - Shared API, rate-limit, and network errors.
* `tags.go` - Model tag mappings and lookup registry.
* `model_chain_factory.go` - Provider/model chain parsing and construction.
* `retry.go` - Retry wrapper policy and backoff handling.
* `fallback.go` - Multi-provider fallback chat model wrapper.
* `unstreaming_chat_model.go` - Stream aggregation for sync chat calls.
* `oauth.go` - OAuth PKCE, token exchange, renewal helpers.
* `config_updater.go` - Config update callback implementation.
* `openai_client.go` - OpenAI-compatible provider client implementation.
* `anthropic_client.go` - Anthropic provider client implementation.
* `ollama_client.go` - Ollama provider client implementation.
* `responses_client.go` - OpenAI Responses provider client implementation core (provider, model listing, shared helpers).
* `responses_client_chat.go` - Responses chat model implementation (`ResponsesChatModel`) and chat/stream request handling.
* `responses_client_compactor.go` - Responses chat compactor implementation and compact endpoint URL helper.
* `responses_client_errors.go` - Responses client HTTP/network error mapping and raw-response formatting helpers.
* `responses_client_oauth_refresh.go` - OAuth2 token refresh lifecycle helpers for Responses client.
* `responses_client_response_conversion.go` - Responses request/response conversion and stream parsing helpers.
* `responses_client_compactor_test.go` - Compactor behavior and compact URL helper tests.
* `responses_client_errors_test.go` - Error-handling and rate-limit tests for Responses client helpers.
* `responses_client_oauth_refresh_test.go` - OAuth2 token refresh and 401 retry tests for Responses client.
* `responses_client_chat_test.go` - Chat-model-specific tests for Responses client (`ResponsesChatModel` chat/stream behavior).
* `jetbrains_client.go` - JetBrains provider client implementation.
* `mock.go` - Mock provider and chat/embedding models.

## Important public API objects

* `ChatModel` - Interface for sync and streaming chat requests.
* `EmbeddingModel` - Interface for text embedding generation.
* `ModelProvider` - Interface for model listing and builders.
* `ChatRole` - Enum: Assistant, Developer, System, User.
* `ModelType` - Enum: `ModelTypeChat`, `ModelTypeEmbed`.
* `ChatMessage` - Chat message with role and parts.
* `ChatMessagePart` - Text, reasoning, tool call, or response part.
* `ChatOptions` - Runtime options and request headers.
* `TokenUsage` - Input/output/total token accounting.
* `ModelInfo` - Provider model metadata entry.
* `ProviderRegistry` - Lazy-loading provider cache from config store.
* `ModelTagRegistry` - Resolves tags using regex mappings.
* `ConfigUpdater` - Callback type for persisting provider config.
* `ConfigUpdaterImpl` - Writable config-store callback adapter.
* `LLMRequestError` - Request error with optional raw response.
* `RateLimitError` - Retryable rate-limit error details.
* `NetworkError` - Retryable network error details.
* `APIRequestError` - Structured request validation error.
* `RetryPolicy` - Retry delays, caps, and limits.
* `RetryChatModel` - Chat wrapper adding retry behavior.
* `FallbackChatModel` - Chat wrapper switching between models.
* `UnstreamingChatModel` - Aggregates stream fragments into one response.
* `OpenAIClient` - OpenAI-compatible provider client.
* `AnthropicClient` - Anthropic provider client.
* `OllamaClient` - Ollama provider client.
* `ResponsesClient` - OpenAI Responses provider client.
* `JetBrainsClient` - JetBrains provider client.
* `MockClient` - In-memory test provider implementation.
* `ProviderModelRef` - Parsed provider/model chain entry.
* `NewProviderRegistry` - Creates registry backed by config store.
* `ModelFromConfig` - Creates provider from config type.
* `NewModelTagRegistry` - Creates empty model tag registry.
* `NewConfigUpdater` - Creates provider config updater.
* `NewTextMessage` - Builds text-only chat message.
* `NewToolCallMessage` - Builds assistant tool-call message.
* `NewToolResponseMessage` - Builds user tool-response message.
* `NewRetryChatModel` - Wraps chat model with retries.
* `DefaultRetryPolicy` - Returns standard retry policy values.
* `NewFallbackChatModel` - Wraps model chain for fallback.
* `NewUnstreamingChatModel` - Wraps model for sync aggregation.
* `NewChatModelFromProviderChain` - Builds wrapped model chain from spec.
* `ParseProviderModelChain` - Parses provider/model comma-chain.
* `ExpandProviderModelChain` - Expands aliases in provider/model chain.
* `ComposeProviderModelSpec` - Joins refs into provider/model spec.
* `NormalizeModelAliasMap` - Converts config aliases to chain aliases.
* `NewMockProvider` - Creates mock provider with model list.
* `IsOAuth2Provider` - Reports whether provider uses OAuth2.
* `ExtractJWTExpiry` - Extracts expiry timestamp from JWT.
