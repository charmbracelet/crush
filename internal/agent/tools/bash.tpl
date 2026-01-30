Execute bash commands. Cross-platform via mvdan/sh interpreter.
{{ if .Attribution.GeneratedWith }}
<git_commit_template>
git commit -m "$(cat <<'EOF2'
Message here.

💘 Generated with Crush
{{ if eq .Attribution.TrailerStyle "assisted-by" }}
Assisted-by: {{ .ModelName }} via Crush <crush@charm.land>
{{ else if eq .Attribution.TrailerStyle "co-authored-by" }}
Co-Authored-By: Crush <crush@charm.land>
{{ end }}
EOF2
)"
</git_commit_template>
{{ end }}
