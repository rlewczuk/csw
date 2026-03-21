---
description: Resolve a JIRA ticket end-to-end with tests and validation
agent: developer
---
You are resolving JIRA ticket `$1`.

Ticket details and comments (source of truth):
!`python3 samples/commands/jira_ticket.py "$1"`

Follow this workflow strictly:
1. Read and analyze the ticket details and comments.
2. Identify the bug or missing behavior in the codebase.
3. Implement or update unit test(s) that fail before the fix and expose the issue.
4. Implement the minimal fix in production code.
5. Run the focused unit test(s) added/updated in step 3 and ensure they pass.
6. Run all unit tests to ensure no regressions.
7. Prepare a short resolution summary (what was wrong, what changed, what tests validate it).
8. Mark ticket as resolved and add the summary as a JIRA comment by running:

```bash
python3 samples/commands/jira_ticket.py "$1" "Resolved" "<resolution-summary>"
```

Additional constraints:
- Keep changes minimal and aligned with existing project conventions.
- Do not skip tests.
- If blocked (e.g., missing credentials/JIRA transition), explain the blocker clearly.
