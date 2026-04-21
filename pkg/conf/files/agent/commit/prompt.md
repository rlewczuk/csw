Generate a commit message summary from the user requests below.

Requirements:
- Maximum ten words.
- Focus on requested code changes and intent.
- Output message text only.

User messages:
<user_messages>
{{- range .Messages }}
- {{ . }}
{{- end }}
</user_messages>