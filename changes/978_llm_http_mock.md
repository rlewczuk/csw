Implement mock HTTP server for Ollama API in `pkg/testutil/llm_mock.go`:
* use `httptest` package from standard library;
* structure mock into two layers: 
    * low-level mock that provides simple and concise way for catching and responding to requests to REST endpoints;
    * high-level mock that provides ollama API-compatible interface for responding to requests (simple wrapper designed to make test code more concise and readable);
* functions creating low level mock server should return object containing following methods:
  * `Client() *http.Client` returning http client that can be used to interact with mock server;
  * `Close()` to stop mock server;
  * `AddRestResponse(path string, method string, response string)` to add response for specific REST endpoint;
  * `AddStreamingResponse(path string, method string, closeAfter bool, responses...string)` to add streaming response for specific REST endpoint;
    * this method called again on the same path and HTTP method will add new pieces of streaming response to already existing response and if there is client waiting for reading from the stream, it will append new responses to the stream;
* function creating high level mock for Ollama should return object containing following methods:
  * `Client() *http.Client` returning http client that can be used to interact with mock server;
  * `Close()` to stop mock server;
  * `AddChatResponse(model string, response ollama.ChatResponse)` to add response for specific model;
  * `AddEmbeddingResponse(model string, closeAfter bool, responses...ollama.ChatResponse)` to add streaming responses for specific model;
      * this method called again add new pieces of streaming response (formatteed as proper Ollama API stream response) to already existing response and there is client waiting for reading from the stream, it will append new responses to the stream;
* add new constructor function to ollama client that accepts `*http.Client`, so that it can be used with mock server;
* adapt existing ollama tests to use mock server in addition to real ollama:
  * ensure that tests still run with real LLM if integration testing mode is enabled (i.e. `_integ/ollama.enabled` or `_integ/all.enabled` exists and contains `yes`)
  * run tests with mock server if integration testing mode is disabled;
  * do not change tests logic except for replacing LLM client to one with mock http server, both real and mock clients should be transparent to tests;
