---
description: Analyses your task specification for ambiguities, errors and missing parts
agent: critic
---

Analyze and edit current task description for errors, ambiguities and missing parts.

When processing task, take into account:
* for obvious changes without ambiguities or questions, you are free to edit relevant portions of task description
  * obvious missing parts can also be added directly
* for ambiguities or unclear missing parts you can either add annotations or questions
  * annotations should be clearly marked with `CSW:` prefix
  * questions should appear in a section at the end of the file
* look for answered questions in the questions section, apply changes according to answers, and if question/answer is not necessary anymore, remove it from the file
* look for `TBD`/`TODO` marked comments and try to fill them in; if there are more questions or ambiguities or missing parts for given TBD comment, ask questions or add annotations

You can read current task decription using `taskGet` tool with `promptOnly` parameter set to `true` and no `uuid` nor `name` parameteres.
Results of analysis should be saved in current task description using `taskEdit` tool (no `name` or `uuid` parameters are required).

Please DO NOT edit or change any other files in this project, just `task.md`.

At the end use `taskUpdate` tool to change task status:
* if there are still ambiguities or questions, change status to `draft`
* if there are no ambiguities or questions, change status to `created`
