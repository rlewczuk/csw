Implement OpenAI provider in `pkg/models/openai/openai_client.go`:
* it should implement `models.ModelProvider` interface (see `pkg/models/api.go`)
* it should use http client from standard library
* it should use DTOs in `pkg/models/openai/openai_dto.go` for request and response objects
* for information about OpenAI API see https://docs.ollama.com/api/openai-compatibility
* it should generate implementation in `pkg/models/openai/openai_client.go`
* make sure `context.Context` is properly handled
* make sure errors are properly returned
* implement tests in `pkg/models/openai/openai_client_test.go`, use `testify` library for assertions
* tests should use environment variable `OPENAI_URL` to determine host url, if no variable is provided, tests should be skipped;
* when running test use URL `http://heha:11434/v1` and model `devstral-small-2:latest`;
* take test-first approach - write tests first, then implement code
* handle relevant errors defined in `pkg/models/api.go`
