Fetch a URL or search the web using an AI sub-agent that can extract, summarize, and answer questions. Slower and costlier than fetch; use fetch for raw content or API responses.
{{- if .GhAvailable }}

Do NOT use this tool for GitHub URLs (repos, issues, PRs, actions, CI runs). Use `gh` CLI in bash instead — agentic_fetch cannot run shell commands so `gh` is unavailable inside it.
{{- end }}
