Extend `ToolInfo` to include JSON schema for tool arguments:
* make sure schema is able to reflect all types supported by `ToolArgs` and `ToolResult`;
* make sure all elements of schema have description fields, so that LLM can understand it;
* make sure schema is compatible with tool calling schemas used by all LLMs APIs (anthropic, openai, ollama etc.) as it will be later integrated into LLM provider API;
* consult ollama, anthropic and openai documentation as reference:
    * see https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview for anthropic
    * see https://platform.openai.com/docs/guides/gpt/function-calling for openai
    * see https://docs.ollama.com/capabilities/tool-calling for ollama
    * make sure schema will be relatively simple and straightforward to translate to all of them when issuing requests to LLMs;
* write tests for it in `pkg/tool/tool_test.go`, use `testify` library for assertions;
* adapt `Tool` interface to include `Info() ToolInfo` method;
* adapt existing tools to implement `Info()` method;
* remove `Name() string` from `Tool` interface as it is redundant with `ToolInfo.Name`
* adapt `ToolRegistry` to implement `ListInfo()` method and return info for all registered tools;
