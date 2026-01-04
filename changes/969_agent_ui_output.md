Implement mock UI output handler for SweSession in `pkg/ui/ui.go`:
* it should implement `SessionOutputHandler` interface;
* it should keep all output in memory;
* it should be possible to access output data to verify it in tests;
* integrate it into `SweSession` in `pkg/core/core.go` so that it can be used to capture output of agent operation;
* implement `SessionUiFactory` interface in `pkg/ui/ui.go` that can create mock UI output handler;
* integrate it into `SweSystem` do that it can be used to create sessions with UI created by factory;
* all parts related to mock UI should be implemeted in `pkg/ui/mock.go`;
* write tests for it in `pkg/ui/ui_test.go`, use `testify` library for assertions;

---

Integration tests in `pkg/core/simple_test.go` should be modified to use mock UI output from `pkg/ui/mock.go` instead of custom handler to capture and verify output of agent operation.
Modify tests, also you can modify mock ui implementation if needed by tests. Try preserving logic of modified tests.
