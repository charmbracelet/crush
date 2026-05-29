{{- if .SubagentBody}}
{{.SubagentBody}}
{{- end}}
{{- if .PreloadedSkillsXML}}

{{.PreloadedSkillsXML}}
{{- end}}
{{- if .AvailSkillXML}}

{{.AvailSkillXML}}
{{- end}}

<env>
Working directory: {{.WorkingDir}}
Is directory a git repo: {{if .IsGitRepo}} yes {{else}} no {{end}}
Platform: {{.Platform}}
Today's date: {{.Date}}
</env>
