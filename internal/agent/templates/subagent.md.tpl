{{- if .SubagentBody}}
{{.SubagentBody}}
{{- end}}
{{- if .PreloadedSkillsXML}}

{{.PreloadedSkillsXML}}
{{- end}}
{{- if .AvailSkillXML}}

{{.AvailSkillXML}}
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

<env>
Working directory: {{.WorkingDir}}
Is directory a git repo: {{if .IsGitRepo}} yes {{else}} no {{end}}
Platform: {{.Platform}}
Today's date: {{.Date}}
</env>
