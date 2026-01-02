Rework `models.ChatModel` method `ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions) (ChatStreamIterator, error)`
so that instead of returning custom iterator interface it will return standard `iter.Seq[ChatMessage]`, so that it can be 
used with standard iterator utilities.

* be sure to update tests as well;
* be sure to update all implementations of `models.ChatModel` interface: `ollama`, `openai`, `mock`;

