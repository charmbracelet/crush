{{- if .SubagentBody}}
{{.SubagentBody}}
{{- end}}
{{- if .PreloadedSkillsXML}}

{{.PreloadedSkillsXML}}
{{- end}}
{{- if .AvailSkillXML}}

{{.AvailSkillXML}}

<skills_usage>
A skill's `<description>` only says when it applies; its procedure lives in its SKILL.md. If a description matches your task, call the View tool with its `<location>` exactly as shown, read the whole file, and follow it before doing the task. `crush://` locations are internal identifiers the View tool reads natively, not URLs.
</skills_usage>
{{- end}}
{{- if .ContextFiles}}

# Project-Specific Context
Make sure to follow the instructions in the context below.
<project_context>
{{range .ContextFiles}}
<file path="{{.Path}}">
{{.Content}}
</file>
{{end}}
</project_context>
{{- end}}
{{- if .GlobalContextFiles}}

# User context
The following is personal content added by the user that they'd like you to follow no matter what project you're working in.
<user_preferences>
{{range .GlobalContextFiles}}
<file path="{{.Path}}">
{{.Content}}
</file>
{{end}}
</user_preferences>
{{- end}}

<env>
Working directory: {{.WorkingDir}}
Is directory a git repo: {{if .IsGitRepo}} yes {{else}} no {{end}}
Platform: {{.Platform}}
Today's date: {{.Date}}
</env>
