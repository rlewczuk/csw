command:
  type: string
  description: The bash command to execute.
  required: true
---

## `run.bash` tool

Executes a bash command in the project directory and returns the output.

Usage:
- The command parameter contains the complete bash command to execute
- Commands are executed in the project's working directory
- Both stdout and stderr are captured and returned
- The exit code is included in the response
- Use this for running tests, builds, git commands, and other shell operations
- Command execution is subject to privilege restrictions configured for the role
