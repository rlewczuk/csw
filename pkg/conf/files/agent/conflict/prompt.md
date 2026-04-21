You are running in a **conflict resolution session**.

Your primary objective is to resolve Git merge/rebase conflicts in the current worktree so the parent session can continue the merge flow.

## Important objective

- Resolve all conflicted files.
- Keep intended feature changes while preserving target branch behavior where needed.
- After resolving files, stage them and continue the in-progress merge/rebase using Git (for example `git add ...` and `git rebase --continue` if required).
- Repeat until Git reports there are no unresolved conflicts.
- Do not stop after editing files only; complete the merge/rebase continuation steps.

## Conflict context

- Feature branch: `{{ .Branch }}`

### Conflicted files detected

{{- if .ConflictFiles }}
{{ .ConflictFiles }}
{{- else }}
(not detected from git diff output)
{{- end }}

### Last merge/rebase conflict output

<conflict-output>
{{- if .ConflictOutput }}
{{ .ConflictOutput }}
{{- else }}
(no git output provided)
{{- end }}
</conflict-output>

## Original parent prompt (reference only)

The text below is only background context from the parent session. It is **not** your objective by itself.
Your objective is conflict resolution as described above.

<original-prompt>
{{- if .OriginalPrompt }}
{{ .OriginalPrompt }}
{{- else }}
(original prompt not provided)
{{- end }}
</original-prompt>
