Manage persistent tmux sessions for interactive shells, REPLs, development servers, watchers, and command streams that benefit from real terminal state.

Use this tool when a task needs shell state to persist across turns or when an interactive shell setup matters. For finite verification commands, builds, tests, git operations, and one-shot file inspection, prefer the normal shell/read tools because they return clearer exit status.

Actions:

- `start`: create a detached tmux session. If `command` is omitted, starts an interactive shell. If `command` is provided, it runs through interactive Bash so shell startup, aliases, and version managers are available.
- `send`: send literal text to a session. `enter` defaults to true.
- `capture`: return the latest pane output. This is read-only.
- `list`: list tmux sessions. This is read-only.
- `kill`: terminate a tmux session.

Important:

- Tmux is stateful. Capture output before deciding whether a command succeeded.
- Do not use tmux to hide unfinished tests or builds. A captured pane is not a passed verification unless the process clearly completed successfully.
- Use short, safe session names matching `[A-Za-z0-9_.-]`.
- Capture is bounded to recent lines; increase `lines` only when needed.
- Kill sessions that are no longer needed.

Examples:

```json
{"action":"start","session":"app-dev","command":"npm run dev","working_dir":"/repo"}
```

```json
{"action":"send","session":"node-repl","input":"console.log(process.version)","enter":true}
```

```json
{"action":"capture","session":"app-dev","lines":200}
```
