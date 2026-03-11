# Package `pkg/testutil` Overview

Package `pkg/testutil` provides reusable test doubles and fixtures used by integration and package tests. It focuses on deterministic mocking of session outputs and HTTP model endpoints.

## Important files

* `integ.go` - integration test configuration helpers
* `llm_mock.go` - mock HTTP server for testing REST endpoints
* `soh_mock.go` - mock session output handler for tests

## Important public API objects

* `MockHTTPServer` - mock HTTP server for testing REST endpoints
* `NewMockHTTPServer` - creates a new mock HTTP server
* `CapturedRequest` - represents a captured HTTP request
* `MockSessionOutputHandler` - mock session output handler capturing events
* `NewMockSessionOutputHandler` - creates a new mock session output handler
* `AssistantMessageRecord` - stores assistant output captured from handler
* `IntegCfgDir` - returns path to _integ directory at project root
* `IntegCfgReadFile` - reads file from _integ directory
* `IntegTestEnabled` - checks if feature is enabled via .enabled file
