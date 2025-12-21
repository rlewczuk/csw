Extend `ChatModel` interface with streaming chat method:
* it should be equivalend of `Chat()` method but return iterator returning fragments of response as they arrive;
* end of stream is indicated by error `ErrEndOfStream`;
* make sure `context.Context` is properly handled;
* modify `OllamaChatModel` to implement new method;
* add tests for new method for ollama client in `ollama_client_test.go`;


