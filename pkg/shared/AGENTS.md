# pkg/shared

`pkg/shared` contains cross-cutting utility code reused across packages. It currently focuses on patch parsing, file-copy helpers, and UUID generation.

## Major files

- `patch.go`: Parser for the custom `*** Begin Patch` format into typed patch operations and hunks.
- `fileutil.go`: Recursive directory/file copy helpers with descriptive error wrapping.
- `uuid.go`: UUIDv7 generator utility for ordered unique IDs.
