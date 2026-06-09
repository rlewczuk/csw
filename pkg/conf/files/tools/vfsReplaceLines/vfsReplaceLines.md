Replace an inclusive range of lines in a file. Use this instead of exact string replacement when editing existing files. The file must be read first. Provide the line range from the last vfsRead output and the replacement text. The tool verifies the file version before editing.

Usage:
- The `path` parameter identifies the file to edit.
- `start_line` and `end_line` are 1-based line numbers from the latest `vfsRead` output; both endpoints are inclusive.
- `replacement` is the exact text that should replace the selected line range.
- `expected_sha256` is optional. When provided, it must match the SHA-256 of the current file content or the edit is rejected.
- The selected range must exist in the current file.
