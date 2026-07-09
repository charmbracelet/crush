Fetch a web URL and return full page content as markdown. Use this for explicit URLs or URLs returned by web_search. Large pages (>50KB) are saved to a scratch file for grep/view. Never guess URLs.
{{- if .GhAvailable }} For GitHub content when an exact repo, issue, or PR link is provided, use `gh` CLI in bash instead.{{- end }}
