# Package `pkg/testutil` Overview

Package `pkg/testutil` provides reusable testing helpers in package `pkg/testutil`.

## Important files

* `integ.go` - Integration config helper wrappers.
* `llm_mock.go` - Mock HTTP server utilities.
* `soh_mock.go` - Mock session output handler.
* `soh_mock_test.go` - Session output handler tests.

## Important public API objects

* `AssistantMessageRecord` - Captured assistant message payload.
* `MockHTTPServer` - In-memory HTTP endpoint mock.
* `CapturedRequest` - Recorded incoming HTTP request.
* `NewMockHTTPServer` - Creates mock HTTP server.
* `MockSessionOutputHandler` - Captures session output events.
* `NewMockSessionOutputHandler` - Creates session output mock.
* `IntegCfgDir` - Returns `_integ` directory path.
* `IntegCfgReadFile` - Reads `_integ` file content.
* `IntegTestEnabled` - Checks feature `.enabled` flag.
