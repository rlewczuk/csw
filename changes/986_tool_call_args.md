Extend `ToolCall` arguments to support more complex types than just map of strings:
* it should follow JSON schema used by LLMs for function calling, use anthropic and openai documentation as reference:
  * see https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview for anthropic
  * see https://platform.openai.com/docs/guides/gpt/function-calling for openai
  * see https://docs.ollama.com/capabilities/tool-calling for ollama
  * it should support nested objects and arrays;
  * it should support primitive types (string, int, float, bool);
* it should be compatible across schemas used by different LLMs APIs (anthropic, openai, ollama etc.);
* it should be possible to serialize and deserialize it to JSON;
* it should be easy to introspect and convert to other data structures (eg. APIs for LLM providers);
* follow good practices on golang data structure design and keep it simple;
* adapt `ToolResponse` to support complex types as well;
* adapt already implemented tools in `vfs.go` to use new types;
* write tests for it in `pkg/tool/tool_test.go`, use `testify` library for assertions;
