Run shell commands. Use dedicated tools when available (view, grep, glob, ls).

<when_to_use>
Use Bash when:
- Running build/test commands (go build, npm test, pytest)
- Git operations (git status, git commit, git push)
- Installing dependencies
- Running project scripts
- Commands that don't have dedicated tools

Do NOT use Bash when:
- Reading files → use `view`
- Searching file contents → use `grep`
- Finding files → use `glob`
- Listing directories → use `ls`
- Fetching URLs → use `fetch`
</when_to_use>

<execution>
- Each command runs in independent shell (no state between calls)
- Use absolute paths rather than cd
- Commands >1 minute auto-convert to background
- Output truncated at {{ .MaxOutputLength }} chars
</execution>

<background_jobs>
For servers, watchers, or long-running processes:
- Set `run_in_background=true` - do NOT use `&`
- Returns shell_id for management
- Use `job_output` to check output
- Use `job_kill` to stop

**Run in background:**
- npm start, npm run dev
- python -m http.server
- go run main.go (servers)
- tail -f, watch commands

**Do NOT run in background:**
- npm run build, go build
- npm test, pytest, go test
- git commands
- One-time scripts
</background_jobs>

<banned_commands>
These commands are blocked for security:
{{ .BannedCommands }}
</banned_commands>

<git_commits>
When creating a commit:
1. Run git status, git diff, git log (parallel calls for speed)
2. Stage relevant files (don't stage unrelated changes)
3. Write clear commit message focusing on "why"
4. Use HEREDOC for multi-line messages:
```bash
git commit -m "$(cat <<'EOF'
Commit message here
{{ if .Attribution.GeneratedWith }}
💘 Generated with Crush
{{ end }}
{{if eq .Attribution.TrailerStyle "assisted-by" }}
Assisted-by: {{ .ModelName }} via Crush <crush@charm.land>
{{ else if eq .Attribution.TrailerStyle "co-authored-by" }}
Co-Authored-By: Crush <crush@charm.land>
{{ end }}
EOF
)"
```

Notes:
- Use `git commit -am` when possible
- Don't commit unrelated files
- Don't amend unless asked
- Don't push unless asked
</git_commits>

<pull_requests>
Use gh CLI for GitHub operations. When creating PR:
1. Check git status, diff, log (parallel)
2. Create branch if needed
3. Commit and push
4. Create PR with gh pr create

Keep PR descriptions focused on "why" not "what".
</pull_requests>

<tips>
- Combine related commands: `git status && git diff`
- Use absolute paths: `pytest /project/tests` not `cd /project && pytest tests`
- Chain with `&&` for dependent commands
- Avoid interactive commands (use -y flags)
</tips>
