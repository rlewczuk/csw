Implement simple coding agent in `pkg/core/agent.go`:
* see `pkg/core/core.go` for partial implementation, fill in missing parts (incl data structures if needed);
* see `pkg/core/simple_test.go` for partially implemented test code, fill in missing parts;
* agent implementation should satisfy mentioned test
* use `MockHttpServer` from `pkg/testutil/llm_mock.go` and ollama client to simulate LLM responses;
* session should maintain conversation context as a list of chat messages;
* use streaming chat API, not blocking one;