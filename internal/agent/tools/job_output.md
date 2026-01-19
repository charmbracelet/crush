Get output from a background shell process.

<usage>
- Provide shell_id from a background bash execution
- Returns current stdout/stderr
- Shows if process is still running or completed
</usage>

<behavior>
- Returns cumulative output from process start
- Check "done" field to see if process completed
- Can call multiple times to see incremental output
</behavior>

<example>
After starting a server with `run_in_background=true`:
```
shell_id: "abc123"
```
→ Returns server output and whether it's still running.
</example>
