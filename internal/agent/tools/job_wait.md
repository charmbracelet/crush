Waits for a background shell to finish and returns its final output.

<usage>
- Provide the shell ID returned from a background bash execution
- Blocks until the background job completes or the request context is done
- Returns the final cumulative output, completion status, and exit code
</usage>

<features>
- Wait for a long-running background task to fully finish
- Get final stdout/stderr output in one call
- Returns the final exit code when available
</features>

<tips>
- Use this only when you truly need to wait for completion
- Use job_output to inspect current logs without blocking
- Use job_kill to stop a background job before it finishes
</tips>
