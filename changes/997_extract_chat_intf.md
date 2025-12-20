Modify Ollama provider so that it contains following method:

```
    ChatModel(model string, options *ChatOptions) ChatModel
```

This method should return an implementation of `models.ChatModel` interface.
Provider itself should not implement `models.ChatModel` interface, but rather
return a separate implementation in `ChatModel` method.

Be sure to update tests as well.
