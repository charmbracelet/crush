Execute shell commands; long-running commands automatically move to background and return a shell ID.

<cross_platform>
Uses mvdan/sh interpreter (Bash-compatible on all platforms including Windows).
Use forward slashes for paths: "ls C:/foo/bar" not "ls C:\foo\bar".
Common shell builtins and core utils available on Windows.
</cross_platform>

<execution_steps>
1. Directory Verification: If creating directories/files, use LS tool to verify parent exists
2. Security Check: Banned commands ({{ .BannedCommands }}) return error - explain to user. Safe read-only commands execute without prompts
3. Command Execution: Execute with proper quoting, capture output
4. Auto-Background: Commands exceeding 1 minute (default, configurable via `auto_background_after`) automatically move to background and return shell ID
5. Output Processing: Truncate if exceeds {{ .MaxOutputLength }} characters
6. Return Result: Include errors, metadata with <cwd></cwd> tags
</execution_steps>

<usage_notes>
- Command required, working_dir optional (current dir)
- Use Grep/Glob/Agent instead of 'find'/'grep'. Use View/LS instead of 'cat'/'head'/'tail'/'ls'
- Chain with ';' or '&&', avoid newlines in quoted strings
- Each command runs in independent shell
- Prefer absolute paths over 'cd'
</usage_notes>

<background_execution>
- Set run_in_background=true to run in a background shell; NEVER use `&`
- Returns a shell ID; use job_output/job_kill to manage
- Long-running servers/watches/generators → background. Build/test/git/file ops → not background
</background_execution>

<examples>
Good: pytest /foo/bar/tests
Bad: cd /foo/bar && pytest tests
</examples>
