---
description: Analyses your task specification for ambiguities, errors and missing parts
agent: critic
---

Analyze below task specification for errors, ambiguities and missing parts:

<task-specification>
{{.Task.Prompt}}
</task-specification>

Results of analysis should be saved as annotations to `{{.Task.TaskDir}}/task.md` file. 
Please edit this file and add either inline annotations in relevant places or bottomline comments and questions at the end of this document.
Please DO NOT edit or change any other files in this project, just this one.
