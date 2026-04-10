# Package `pkg/io` Overview

The `pkg/io` package provides text and JSONL input/output adapters that bridge external readers and writers to the core session thread interfaces. It implements `TextSessionInput` and `JsonlSessionInput` for reading commands, and `TextSessionOutput` and `JsonlSessionOutput` for rendering messages, tool results, and permission queries.

## Important files

* `jsonl_input.go` - JSONL input adapter for session commands
* `jsonl_output.go` - JSONL output adapter for session responses
* `text_input.go` - Plain text input adapter for session commands
* `text_output.go` - Text output adapter with slug prefixing

## Important public API objects

* `JsonlSessionInput` - reads JSONL commands and forwards them
* `NewJsonlSessionInput` - creates a JSONL input adapter
* `JsonlSessionOutput` - writes session output in JSONL mode
* `NewJsonlSessionOutput` - creates a JSONL output adapter
* `TextSessionInput` - reads plain text lines and forwards them
* `NewTextSessionInput` - creates a text input adapter
* `TextSessionOutput` - writes human-readable session output
* `NewTextSessionOutput` - creates a text output adapter
* `NewTextSessionOutputWithSlug` - creates a text output adapter with slug
