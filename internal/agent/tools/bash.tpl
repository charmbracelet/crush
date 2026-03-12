Executes bash commands with automatic background conversion for long-running tasks.

<cross_platform>
Uses mvdan/sh interpreter (Bash-compatible on all platforms including Windows).
Use forward slashes for paths: "ls C:/foo/bar" not "ls C:\foo\bar".
Common shell builtins and core utils available on Windows.
</cross_platform>

<execution_steps>
1. Directory Verification: If creating directories/files, use LS tool to verify parent exists
2. Security Check: Banned commands ({{ .BannedCommands }}) return error - explain to user. Safe read-only commands execute without prompts
3. Command Execution: Execute with proper quoting, capture output
4. Auto-Background: Commands exceeding 1 minute automatically move to background and return shell ID
5. Output Processing: Truncate if exceeds {{ .MaxOutputLength }} characters
6. Return Result: Include errors, metadata with <cwd></cwd> tags
</execution_steps>

<usage_notes>
- Command required, working_dir optional (defaults to current directory)
- IMPORTANT: Use Grep/Glob/Agent tools instead of 'find'/'grep'. Use View/LS tools instead of 'cat'/'head'/'tail'/'ls'
- Chain with ';' or '&&', avoid newlines except in quoted strings
- Each command runs in independent shell (no state persistence between calls)
- Prefer absolute paths over 'cd' (use 'cd' only if user explicitly requests)
</usage_notes>

<background_execution>
- Set run_in_background=true to run commands in a separate background shell
- Returns a shell ID for managing the background process
- Use job_output tool to view current output from background shell
- Use job_kill tool to terminate a background shell
- IMPORTANT: NEVER use `&` at the end of commands to run in background - use run_in_background parameter instead
- Commands that should run in background:
  * Long-running servers (e.g., `npm start`, `python -m http.server`, `node server.js`)
  * Watch/monitoring tasks (e.g., `npm run watch`, `tail -f logfile`)
  * Continuous processes that don't exit on their own
  * Any command expected to run indefinitely
- Commands that should NOT run in background:
  * Build commands (e.g., `npm run build`, `go build`)
  * Test suites (e.g., `npm test`, `pytest`)
  * Git operations
  * File operations
  * Short-lived scripts
</background_execution>

<git_commits>
When user asks to create git commit:

1. Single message with three tool_use blocks (IMPORTANT for speed):
   - git status (untracked files)
   - git diff (staged/unstaged changes)
   - git log (recent commit message style)

2. Add relevant untracked files to staging. Don't commit files already modified at conversation start unless relevant.

3. Analyze staged changes in <commit_analysis> tags:
   - List changed/added files, summarize nature (feature/enhancement/bug fix/refactoring/test/docs)
   - Brainstorm purpose/motivation, assess project impact, check for sensitive info
   - Don't use tools beyond git context
   - Draft concise (1-2 sentences) message focusing on "why" not "what"
   - Use clear language, accurate reflection ("add"=new feature, "update"=enhancement, "fix"=bug fix)
   - Avoid generic messages, review draft

4. Create commit{{ if or (eq .Attribution.TrailerStyle "assisted-by") (eq .Attribution.TrailerStyle "co-authored-by")}} with attribution{{ end }} using HEREDOC:
   git commit -m "$(cat <<'EOF'
   Commit message here.

{{ if .Attribution.GeneratedWith }}
   💘 Generated with Crush
{{ end}}
{{if eq .Attribution.TrailerStyle "assisted-by" }}

   Assisted-by: {{ .ModelName }} via Crush <crush@charm.land>
{{ else if eq .Attribution.TrailerStyle "co-authored-by" }}

   Co-Authored-By: Crush <crush@charm.land>
{{ end }}

   EOF
   )"

5. If pre-commit hook fails, retry ONCE. If fails again, hook preventing commit. If succeeds but files modified, MUST amend.

6. Run git status to verify.

Notes: Use "git commit -am" when possible, don't stage unrelated files, NEVER update config, don't push, no -i flags, no empty commits, return empty response.
</git_commits>

<pull_requests>
Use gh command for ALL GitHub tasks. When user asks to create PR:

1. Single message with multiple tool_use blocks (VERY IMPORTANT for speed):
   - git status (untracked files)
   - git diff (staged/unstaged changes)
   - Check if branch tracks remote and is up to date
   - git log and 'git diff main...HEAD' (full commit history from main divergence)

2. Create new branch if needed
3. Commit changes if needed
4. Push to remote with -u flag if needed

5. Analyze changes in <pr_analysis> tags:
   - List commits since diverging from main
   - Summarize nature of changes
   - Brainstorm purpose/motivation
   - Assess project impact
   - Don't use tools beyond git context
   - Check for sensitive information
   - Draft concise (1-2 bullet points) PR summary focusing on "why"
   - Ensure summary reflects ALL changes since main divergence
   - Clear, concise language
   - Accurate reflection of changes and purpose
   - Avoid generic summaries
   - Review draft

6. Create PR with gh pr create using HEREDOC:
   gh pr create --title "title" --body "$(cat <<'EOF'

   ## Summary

   <1-3 bullet points>

   ## Test plan

   [Checklist of TODOs...]

{{ if .Attribution.GeneratedWith}}
   💘 Generated with Crush
{{ end }}

   EOF
   )"

Important:

- Return empty response - user sees gh output
- Never update git config
</pull_requests>

<examples>
Good: pytest /foo/bar/tests
Bad: cd /foo/bar && pytest tests
</examples>
