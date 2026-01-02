Implement Anthropic provider in `pkg/models/anthropic/anthropic_client.go`:
* it should implement `models.ModelProvider` interface (see `pkg/models/api.go`)
* it should use http client from standard library
* it should use DTOs in `pkg/models/anthropic/anthropic_dto.go` for request and response objects
* for information about Anthropic API see:
  * https://platform.claude.com/docs/en/api/beta/messages/create for chat
  * https://platform.claude.com/docs/en/api/beta/models/list for list of models
  * look for other pages in https://platform.claude.com/docs/en/api/beta if needed
* it should generate implementation in `pkg/models/anthropic/anthropic_client.go`
* there is no embedding model in Anthropic API, so `EmbeddingModel` method should return error `ErrNotImplemented`
* make sure `context.Context` is properly handled
* make sure errors are properly returned
* implement tests in `pkg/models/anthropic/anthropic_client_test.go`, use `testify` library for assertions
* tests should use file `.anthropic_api_key` which will contain API key for Anthropic API
* if file exist, test should read its content and trim spaces and use it as API key
* if file does not exist, tests should be skipped
* use standard endpoint URL for Anthropic API
* use model `claude-sonnet-4-5-20250929` for testing
* take test-first approach - write tests first, then implement code
* handle relevant errors defined in `pkg/models/api.go`
