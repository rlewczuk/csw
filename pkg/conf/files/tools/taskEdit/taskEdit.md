Performs exact string replacements in task prompt.

- Task can be identified by: `uuid` or`name` or current session task when neither is provided.
- Use this tool to apply focused prompt updates with exact string replacement semantics.
- The edit will FAIL if `oldString` is not found in the task prompt with an error "oldString not found in content".
- The edit will FAIL if `oldString` is found multiple times in the task prompt with an error "oldString found multiple times and requires more code context to uniquely identify the intended match". Either provide a larger string with more surrounding context to make it unique or use `replaceAll` to change every instance of `oldString`.
- Use `replaceAll` for replacing and renaming strings across task prompt. This parameter is useful if you want to rename a variable for instance.