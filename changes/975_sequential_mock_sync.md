Modify `MockHttpServer` in `pkg/testutil/llm_mock.go` to support multiple sequential responses:
* existing method `AddRestResponse()` should be modified so that it appends response to existing one instead of overwriting it;
* enqueued response should be returned in FIFO order;
* look through anthropic tests to fix cases where tests are skipped because they require multiple sequential responses;
* also look through ollama tests to fix cases where tests are skipped because they require multiple sequential responses;


