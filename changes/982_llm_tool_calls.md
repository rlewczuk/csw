Extend LLM providers in `pkg/models` to support tool calling:
* it should be possible to pass list of tools to LLM provider, `ChatModel` methods should accept slice of `ToolInfo` objects;
* if LLM returns tool call, `ChatStream` should yield `ToolCall` object;
* it should be possible to pass tool call results back to LLM;
* functionality should work in both blocking and streaming modes;
* ChatMessage parts should be extended with ToolCall and ToolResponse to support both tool calls and responses;
* as chat message can also carry text chunks of response, it should be possible to distinguish between them;
* adapt openai, anthropic and ollama clients to support tool calling according to API documentation:
    * see https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview for anthropic
    * see https://platform.openai.com/docs/guides/gpt/function-calling for openai
    * see https://docs.ollama.com/capabilities/tool-calling for ollama
* make sure to add tests for tool calling to all clients;
* if DTO structures don't have fields for tool calling, add them according to provider API documentation;
* modify mock provider to support simulated tool calling:
    * give test code way to obtain tool call returned by LLM (i.e. store incoming calls in mock);
    * give test code way to obtain tool response returned to LLM (i.e. store outgoing responses in mock);
* add tests for tool calling to existing test suites for all above providers;

