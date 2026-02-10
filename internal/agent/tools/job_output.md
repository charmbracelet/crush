Retrieves the current output from a background shell.

<usage>
- Provide the shell ID returned from a background bash execution
- Returns the current stdout and stderr output
- Indicates whether the shell has completed execution
- Set wait=true to block until the shell completes
</usage>

<features>
- View output from running background processes
- Check if background process has completed
- Get cumulative output from process start
- Optionally wait for process completion before returning
</features>

<tips>
- Use this to monitor long-running processes
- Check the 'done' status to see if process completed
- Can be called multiple times to view incremental output
- Use wait=true when you need the final output and exit status
</tips>
