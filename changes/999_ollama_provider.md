Implement Ollama provider in `pkg/models/ollama/ollama_client.go`:
* it should implement `models.ModelProvider` interface (see `pkg/models/api.go`)
* it should use http client from standard library
* it should use DTOs in `pkg/models/ollama/dto.go` for request and response objects
* it should generate implementation in `pkg/models/ollama/ollama_client.go`
* make sure `context.Context` is properly handled
* make sure errors are properly returned
* implement tests in `pkg/models/ollama/ollama_client_test.go`, use `testify` library for assertions
* take test-first approach - write tests first, then implement code
* for test use ollama running on `http://beha:11434` and model `devstral-small-2:latest`
* handle relevant errors defined in `pkg/models/api.go`
* do not implement full client yet, only methods in `models.ModelProvider` interface
