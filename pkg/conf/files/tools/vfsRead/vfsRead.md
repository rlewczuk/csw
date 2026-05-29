Reads a file from the local filesystem. You can access any file directly by using this tool.
Assume this tool is able to read all files on the machine. If the User provides a path to a file assume that path is valid. It is okay to read a file that does not exist; an error will be returned.

Usage:
- The `path` parameter must be an absolute path, not a relative path
- By default, it reads up to 384 lines
- The `offset` parameter is required and specifies how many lines to skip before reading. Use `0` to read from the beginning of the file
- Any lines longer than 2000 characters will be truncated
- Results are returned using cat -n format, with line numbers starting at 1
- If you read a file that exists but has empty contents you will receive a system reminder warning in place of file contents.
