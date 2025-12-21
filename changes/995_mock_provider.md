Implement mock model provider in `pkg/models/mock/mock.go`:
* it should implement `models.ModelProvider` interface;
* it should return predefined list of models;
* methods `ChatModel()` and `EmbeddingModel()` should return mock implementations of `models.ChatModel` and `models.EmbeddingModel` interfaces;
* mock implementations should return responses predefined in test code;
* mock implementation of `ChatModel` should support streaming;
* streaming should return fragments predefined in test code;
* mock implementations should support `context.Context` and `Cancel()`;
* write tests for it in `pkg/models/mock/mock_test.go`;
