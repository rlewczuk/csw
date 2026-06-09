Insert text into an existing file immediately after a specific line number. Prefer this tool for adding imports, statements, functions, methods, configuration entries, or documentation to an existing file. First read the relevant file section with vfsRead and use the line numbers from that read result. Use line_number=0 to insert at the beginning of the file before the current first line. Do not use this tool to replace existing lines; use vfsReplaceLines for replacements. Do not use this tool for creating new files; use the file creation/write tool instead. The inserted text is added exactly as provided, so include the correct indentation and trailing newline when needed. The tool verifies the file version when expected_sha256 is provided and rejects stale edits if the file has changed since it was read.

Usage:
- The `path` parameter identifies the existing file to edit.
- `line_number` is the 1-based line number after which to insert text. Use 0 to insert at the beginning of the file, before line 1.
- `content` is the exact text to insert, including indentation and trailing newline when needed.
- `expected_sha256` is optional. When provided, it must match the SHA-256 of the current file content or the edit is rejected.
