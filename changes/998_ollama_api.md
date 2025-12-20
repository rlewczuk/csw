Implement Ollama chat client in `pkg/models/ollama/ollama.go`: 
* client should be an implementation of `models.ChatModel` interface;
* prefer http client from standard library;
* use DTOs in `pkg/models/ollama/dto.go` for request and response objects;
* use `pkg/models/ollama/ollama.go` for client implementation;
* make sure `context.Context` is properly handled;
* make sure errors are properly returned;
* make sure fields present in `models.ChatOptions` are properly mapped to request DTO;
* make sure fields present in `models.ChatMessage` are properly mapped to request DTO;
* make sure fields present in `models.ChatMessage` are properly mapped from response DTO;
* implement tests in `pkg/models/ollama/ollama_test.go`, use `testify` library for assertions;


```
	ChatModel(mode string, options *ChatOptions) ChatModel

	EmbeddingModel(mode string) EmbeddingModel
```

