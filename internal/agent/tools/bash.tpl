Execute bash commands. Cross-platform via mvdan/sh interpreter.

<rules>
- Dangerous commands ({{ .BannedCommands }}) require user approval even in YOLO mode
- Use Grep/Glob instead of 'find'/'grep', View/LS instead of 'cat'/'ls'
- Output truncated at {{ .MaxOutputLength }} chars
- Commands >1min auto-convert to background jobs
</rules>

<background>
Use `run_in_background=true` for servers/watchers. Use job_output/job_kill to manage. Never use `&`.
</background>

<git_commits>
1. Run git status, git diff, git log in parallel
2. Stage relevant files, analyze in <commit_analysis> tags
3. Commit using HEREDOC:
   git commit -m "$(cat <<'EOF2'
   Message here.
{{ if .Attribution.GeneratedWith }}
   💘 Generated with Crush
{{ end }}{{ if eq .Attribution.TrailerStyle "assisted-by" }}
   Assisted-by: {{ .ModelName }} via Crush <crush@charm.land>
{{ else if eq .Attribution.TrailerStyle "co-authored-by" }}
   Co-Authored-By: Crush <crush@charm.land>
{{ end }}
   EOF2
   )"
4. Run git status to verify. Don't push.
</git_commits>

<pull_requests>
1. Run git status, git diff, git log, git diff main...HEAD in parallel
2. Create branch/commit/push if needed
3. Analyze in <pr_analysis> tags
4. Create PR:
   gh pr create --title "title" --body "$(cat <<'EOF2'
   ## Summary
   <bullets>
   ## Test plan
   <checklist>
{{ if .Attribution.GeneratedWith }}
   💘 Generated with Crush
{{ end }}
   EOF2
   )"
</pull_requests>
