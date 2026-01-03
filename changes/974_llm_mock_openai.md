Adapt existing openai tests in `openai_client_test.go` to use mock server in addition to real openai:
* add new constructor function to openai client that accepts `*http.Client`, so that it can be used with mock server;
* ensure that tests still run with real LLM if integration testing mode is enabled (i.e. `_integ/openai.enabled` or `_integ/all.enabled` exists and contains `yes`);
* run tests with mock server if integration testing mode is disabled;
* do not change tests logic except for replacing LLM client to one with mock http server, both real and mock clients should be transparent to tests;
* use `MockHTTPServer` from `pkg/testutil/llm_mock.go`;
* you can look at ollama tests for inspiration;
