# pkg/logging

`pkg/logging` implements structured logging for global runtime events and per-session logs. It manages logger creation/caching, file-backed JSONL logging, in-memory fallback behavior, flush/close lifecycle, and standardized event helpers for chat and tool activity.

## Major files

- `logger.go`: Primary logging implementation and API (`GetGlobalLogger`, `GetSessionLogger`, `SetLogsDirectory`, structured event logging helpers).
- `test_logger.go`: In-memory logger utilities for tests, including per-session buffer capture and retrieval helpers.
