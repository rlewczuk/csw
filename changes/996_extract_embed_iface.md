Modify Ollama provider so that it contains following method:

```
    EmbeddingModel(model string) EmbeddingModel
```

This method should return an implementation of `models.EmbeddingModel` interface.
Provider itself should not implement `models.EmbeddingModel` interface but rather 
return an implementation of it. Also, modify tests in `ollama_client_test.go` to and make them running using
 model tests use `nomic-embed-text:latest`.

