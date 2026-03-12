
Starts a focused subagent task in a separate child session and waits for it to finish.

When to use subAgent tool:
- When task contains multiple steps that can be executed independently, in parallel, or user explicitly requests certain steps to be run in parallel;
- When a task is large enough to delegate independently, while keeping parent context clean.
 
When NOT to use subAgent tool:
- When whole task is small enough to be executed in the parent session;
- If you want to read a specific file path, use the vfsRead or vfsGrep tool instead of the subAgent tool, to find the match more quickly
- If you are searching for a specific class definition like "class Foo", use the vfsGrep tool instead, to find the match more quickly
- If you are searching for code within a specific file or set of 2-3 files, use the vfsRead tool instead of the subAgent tool, to find the match more quickly
- Other tasks that are not related to the agent descriptions above

Usage notes:
* subagent will start with fresh context and will not see parent session context nor user prompt
* please provide a unique slug for each subagent call in the parent session
* please provide a prompt that fully describes the task being performed by the subagent, include all needed context and information related to task delegated to subagent, 
  * but avoid repeating full parent session context or providing information unrelated to the subagent task
  * clearly tell subagent what you expect to be done in the child session and what should be returned to the parent session (via summary message)
* all subAgent tool calls, will be executed synchronously in the parent session, if you want to run more tasks in parallel, issue multiple subAgent tool calls in a single shot;
  * if you have dependencies between subAgent tool calls (i.e. subAgent A needs output from subAgent B), then run subAgent A first (first shot of subAgent tool calls), then wait for tool response, then subAgent B (second shot of subAgent tool calls)
* subagent created by subAgent tool will reuse current session VFS/workdir/LSP
* subagent will return summary of actions taken by the child session, not the full output of the child session
* if there are many indepentent tasks that can be delegated to subagent, run all subagents at once, do not split them into multiple batches as subagent mechanism has its own rate limiting and queue management (i.e. there is no danger of overloading the subagent mechanism)
  * but be sure to serialize subagent calls if there are clear dependencies between them, eg. one requires output from another

## Parameters

- `slug` (required): very short symbolic subagent session name used in UI/log output.
  - Generate slug from task title the same way as worktree branch symbolic names:
    - lowercase letters, digits, dashes only
    - max 20 characters
    - 2-4 concise words joined with dashes
    - no spaces, slashes, underscores, or punctuation
  - Must be unique among subagent calls in the current parent session.
- `title` (required): short user-facing status title shown while subagent runs.
- `prompt` (required): full initial prompt for the child session.
- `role` (optional): child role override; defaults to parent role.
- `model` (optional): child model override in `provider/model` format; defaults to parent model.
- `thinking` (optional): child thinking override; defaults to parent thinking.

## Output

Returns child session summary result:
- `status`: `completed` or `error`
- `summary`: summary text from child model when completed

On failure, returns diagnostic error details instead of summary.
