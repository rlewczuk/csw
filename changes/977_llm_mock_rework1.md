Rework `OllamaMockServer` client in `ollama_mock_test.go` into a set of functions converting Ollama DTO structures into 
JSON strings and then use `MockHttpServer` implemented in `llm_mock.go` along with those functions in ollmama tests:
* remove `OllamaMockServer` structure and all its methods;
* adapt tests to use `MockHttpServer` and directly, using new functions to convert DTOs to JSON;
* aside of switching to low level mock, don't change tests logic;
* ensure that all tests work with both real and mock http server as they are now;

