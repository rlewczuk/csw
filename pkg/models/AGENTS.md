# pkg/models

`pkg/models` is the model/provider abstraction layer for chat and embedding backends. It defines common model interfaces, provider registry/factory behavior, concrete provider clients, OAuth/token helpers, tagging logic, and shared DTO/error/logging utilities.

## Major files

- `api.go`: Core public model API (`ChatModel`, `EmbeddingModel`, `ModelProvider`, message and option types).
- `providers.go`: Provider registry and config-backed factory wiring with caching and invalidation.
- `openai_client.go`: OpenAI-compatible provider implementation (chat, streaming, embeddings, model listing, tool calls).
- `anthropic_client.go`: Anthropic provider implementation with tool and streaming conversion.
- `ollama_client.go`: Ollama provider implementation for chat/stream/embed/list operations.
- `responses_client.go`: Responses API provider implementation with event-stream parsing and tool-call assembly.
- `oauth.go`: OAuth utility APIs (PKCE/state generation, auth URL, code exchange, refresh, token-expiry helpers).
- `tags.go`: Model-tag resolution from global/provider config mappings.
- `errors.go`: Shared model/provider error taxonomy used by retry and UI logic.
- `mock.go`: Test doubles for provider/chat/embedding behavior with programmable outputs and failures.
