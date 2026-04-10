# Package `pkg/testutil` Overview

Package `pkg/testutil` provides reusable test utilities and helpers for integration tests, mock HTTP servers, and session output handling in `pkg/testutil`.

## Important files

* `integ.go` - Integration config helper wrappers.
* `llm_mock.go` - Mock HTTP server utilities.
* `soh_mock.go` - Mock session output handler.
* `soh_mock_test.go` - Session output handler tests.

## Important public API objects

* `AssistantMessageRecord` - Captured assistant message payload.
* `CapturedRequest` - Recorded incoming HTTP request.
* `MockHTTPServer` - In-memory mock HTTP server.
* `NewMockHTTPServer` - Creates new mock HTTP server.
* `MockSessionOutputHandler` - Captures session output events.
* `NewMockSessionOutputHandler` - Creates session output mock.
* `IntegCfgDir` - Returns `_integ` directory path.
* `IntegCfgReadFile` - Reads `_integ` file content.
* `IntegTestEnabled` - Checks feature `.enabled` flag.
