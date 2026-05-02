Use this tool to finish current session loop normally.

When called, the session stops processing further LLM turns after current tool handling,
and finalization flow runs as in normal completion (same behavior as when assistant
returns message without tool calls).

Usage:
- This tool takes no parameters. Leave input empty.
