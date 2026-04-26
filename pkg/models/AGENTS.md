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
* `openai_client.go` - OpenAI-compatible provider client implementation core (client setup, listing, and sync chat).
* `openai_client_stream.go` - OpenAI chat streaming implementation and streaming raw-chunk logging helper.
* `openai_client_tool_calling_test.go` - OpenAI tool-calling tests split from the main OpenAI client test file.
* `openai_client_reasoning_test.go` - OpenAI reasoning-content tests split from the main OpenAI client test file.
* `openai_client_logging_test.go` - OpenAI logging and raw request/response callback tests split from the main OpenAI client test file.
* `openai_client_stream_test.go` - OpenAI streaming chat tests split from the main OpenAI client test file.
* `openai_client_errors.go` - OpenAI client HTTP/network error mapping and usage-limit retry-after parsing helpers.
* `openai_client_errors_test.go` - Tests for OpenAI client error handling and rate-limit retry behavior.
* `openai_client_conversion.go` - OpenAI chat/tool payload conversion and prompt-cache request-stability helpers.
* `openai_client_conversion_test.go` - Tests for OpenAI conversion and request-stability helpers.
* `anthropic_client.go` - Anthropic provider client implementation core (client setup, listing, and sync chat).
* `anthropic_client_stream.go` - Anthropic chat streaming implementation and streaming raw-chunk logging helper.
* `anthropic_client_errors.go` - Anthropic client HTTP/network error mapping and rate-limit retry-after parsing helpers.
* `anthropic_client_errors_test.go` - Tests for Anthropic client error handling and token-limit detection behavior.
* `anthropic_client_prompt_caching.go` - Anthropic prompt-caching request shaping and tool schema normalization helpers.
* `anthropic_client_prompt_caching_test.go` - Tests for Anthropic prompt-caching request stability and cache-control breakpoints.
* `anthropic_client_stream_test.go` - Anthropic streaming chat and raw stream-chunk callback tests split from the main Anthropic client test file.
* `ollama_client.go` - Ollama provider client implementation core (client setup, listing, chat, and streaming).
* `ollama_client_conversion.go` - Ollama chat/tool message and tool schema conversion helpers.
* `ollama_client_errors.go` - Ollama client HTTP/network error mapping and rate-limit retry-after parsing helpers.
* `ollama_client_tool_calling_test.go` - Ollama client tool-calling tests split from the main Ollama client test file.
* `ollama_client_conversion_test.go` - Tests for Ollama conversion helpers and generated tool-call IDs.
* `ollama_client_raw_request_logging_test.go` - Ollama client raw request/response logging tests split from the main Ollama client test file.
* `ollama_client_stream_test.go` - Ollama streaming chat behavior tests split from the main Ollama client test file.
* `ollama_client_request_options_test.go` - Ollama client request-options tests split from the main Ollama client test file (max tokens, headers, query params).
* `ollama_client_errors_test.go` - Tests for Ollama client error handling and endpoint error mapping.
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
* `responses_client_chat_test_request_payload.go` - Request-payload-focused chat tests (headers, instructions, codex payload compatibility, and request stability).
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
