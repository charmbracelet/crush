Execute shell commands; long-running commands automatically move to background and return a shell ID.

<cross_platform>
This tool is named `bash` for compatibility, but it is Crush's cross-platform shell tool, not a guarantee that GNU Bash, WSL, cmd.exe, or PowerShell is available.
It uses the mvdan/sh interpreter with portable shell syntax on all platforms, including Windows.
Use forward slashes for paths: "ls C:/foo/bar" not "ls C:\foo\bar".
Prefer portable commands and built-in file/search tools over OS-specific shell features. Only use cmd.exe, PowerShell, WSL, or Unix-only commands when the user or environment explicitly requires them.
Common shell builtins and core utils are available on Windows.
</cross_platform>

<execution_steps>
1. Directory Verification: If creating directories/files, use LS tool to verify parent exists
2. Policy Check: Safe read-only commands execute directly. Other commands use the configured permission mode and PreToolUse hooks
3. Command Execution: Execute with proper quoting, capture output
4. Auto-Background: Commands exceeding 60 seconds (default, configurable via `auto_background_after`) automatically move to background and return a shell ID
5. Output Processing: Truncate if exceeds {{ .MaxOutputLength }} characters
6. Return Result: Include errors, metadata with <cwd></cwd> tags
</execution_steps>

<usage_notes>
- Command required, working_dir optional (defaults to current directory)
- For repository file discovery and content search, prefer the native Grep/Glob/View/LS/Agent tools
- For host/runtime facts such as disk usage, cache sizes, process state, server state, git output, package-manager output, or finite system snapshots, use bounded shell commands with specific paths plus timeout/head/maxdepth when useful
- When the user asks for measured system facts, return command-backed measurements. Directory trees, globs, and MCP listings can identify candidates, but they are not evidence of size, free space, running state, or completion by themselves
- Do not run broad unbounded filesystem walks; narrow the path or use native tools when the target is inside the repository
- Chain with ';' or '&&', avoid newlines except in quoted strings
- Each command runs in independent shell (no state persistence between calls)
- Prefer absolute paths over 'cd' (use 'cd' only if user explicitly requests)
- For log readers and other commands that may follow indefinitely, prefer the command's finite/non-follow mode when available. Otherwise set run_in_background=true and inspect snapshots with job_output
{{- if .RgAvailable }}
- Ripgrep (`rg`) is available; prefer it over `grep` for faster, more intuitive searching
{{- end }}
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

<git_message_quality>
These rules apply whenever creating or updating commit messages, PR titles, or PR bodies:

- Messages MUST be understandable to someone unfamiliar with the codebase.
- Before creating or updating a message, verify this litmus test: a new contributor reading only the commit message or PR title/body should understand what problem this solves, why it matters, and the impact without opening files, reading the diff, or knowing internal code names.
- Avoid code identifiers, filenames, function names, and implementation details unless they are necessary for understanding the user-facing impact.
- Bad: "Add NameFromHex with sync.Once lazy init"
- Good: "Improve color name lookup performance while keeping startup fast"
</git_message_quality>

<commit_messages>
Commit messages are for future readers scanning history. Before committing:

- Follow <git_message_quality>.
- Draft a concise 1-2 sentence message focusing on why the change exists and what outcome it enables, not a list of files or implementation details.
- Use clear, accurate verbs ("add"=new capability, "update"=enhancement, "fix"=bug fix) and avoid generic messages.
- The first line MUST be under 72 characters.
- Add a body only when it is needed to explain the reasoning, tradeoffs, or important context; wrap body lines at 72 characters.
- If the change is internal-only, still describe the benefit or maintenance outcome rather than naming private code.
- Bad: "fix: nil pointer in session.go"
- Good: "fix: prevent session loading from crashing on missing metadata"
- Bad: "refactor: move PromptBuilder into internal/agent"
- Good: "refactor: make prompt assembly easier to maintain"
</commit_messages>

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
   - Don't use tools beyond the context of git

4. Draft a commit message:
   - Follow <commit_messages>
   - Review draft against the litmus test before committing

5. Create commit{{ if or (eq .Attribution.TrailerStyle "assisted-by") (eq .Attribution.TrailerStyle "co-authored-by")}} with attribution{{ end }} using HEREDOC:
   git commit -m "$(cat <<'EOF'
Commit message here.

{{ if .Attribution.GeneratedWith }}
💘 Generated with Crush
{{ end}}
{{if eq .Attribution.TrailerStyle "assisted-by" }}

Assisted-by: Crush:{{ .ModelID }}
{{ else if eq .Attribution.TrailerStyle "co-authored-by" }}

Co-Authored-By: Crush <crush@charm.land>
{{ end }}
EOF
)"

6. If pre-commit hook fails, retry ONCE. If fails again, hook preventing commit. If succeeds but files modified, MUST amend.

7. Run git status to verify.

Notes: Use "git commit -am" when possible, don't stage unrelated files, NEVER update config, don't push, no -i flags, no empty commits, return empty response, when rebasing always use -m.
</git_commits>

<pull_requests>
{{ if .GhAvailable -}}
   Use the `gh` command for ALL GitHub tasks.
{{- end }}

When user asks you to create or update a PR:

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

6. Draft a PR message:
   - Follow <git_message_quality>
   - Draft concise (1-2 bullet points) PR summary focusing on "why"
   - Ensure summary reflects ALL changes since main divergence
   - Use clear, concise language
   - Provide an accurate reflection of changes and purpose
   - Avoid generic summaries; messages should be thoughtful
   - Review draft against the litmus test before creating or updating the PR

7. Create PR with gh pr create using HEREDOC:
   gh pr create --title "title" --body "$(cat <<'EOF'

<summary>

{{ if .Attribution.GeneratedWith -}}
💘 Generated with Crush
{{- end }}

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
