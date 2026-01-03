In `ollama_client_test.go` implement additional integration tests for tool calling:
* it should test if tool calls are properly passed to LLM;
* it should test if tool responses are properly passed back to LLM;
* it should test if tool calls and responses are properly interleaved with text chunks;
* fix any issues if tests expose errors in client implementation;
* make sure that if tool call is returned by LLM in multiple chunks, it is properly reassembled inside client before returning to caller;
* note that this test works with real LLM, it is not mocked;
* use some sample tool call and response, for example weather tool;
* as other tests, it should be run only if ollama integration tests are enabled (see `pkg/testutil/integ.go`);
* you can use openai tests as reference;

