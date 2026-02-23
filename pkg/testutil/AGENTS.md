# pkg/testutil

`pkg/testutil` provides reusable test doubles and fixtures used by integration and package tests. It focuses on deterministic mocking of session outputs and HTTP model endpoints.

## Major files

- `soh_mock.go`: Concurrency-safe mock session output handler for capturing assistant/tool/permission events and waiting on async callbacks.
- `llm_mock.go`: HTTP mock server wrapper with queued responses, streaming support, and request capture for assertions.
