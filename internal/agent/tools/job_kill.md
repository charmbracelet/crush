Stop a background shell process.

<usage>
- Provide shell_id from a background bash execution
- Immediately terminates the process (SIGTERM)
- Shell ID becomes invalid after killing
</usage>

<when_to_use>
- Stop servers or watchers you started
- Clean up processes no longer needed
- Cancel long-running commands
</when_to_use>

<example>
```
shell_id: "abc123"
```
→ Stops the background process.
</example>
