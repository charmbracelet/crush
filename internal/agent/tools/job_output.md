Retrieves the current output from a background shell.

<usage>
- Provide the shell ID returned from a background bash execution
- Returns the current stdout and stderr output
- Indicates whether the shell has completed execution
- Optionally wait for the job to complete with max_wait_time parameter
</usage>

<features>
- View output from running background processes
- Check if background process has completed
- Get cumulative output from process start
- Wait for job completion with timeout
- Monitor long-running tasks without continuous polling
</features>

<parameters>
- shell_id (required): The ID of the background shell to retrieve output from
- max_wait_time (optional): Maximum time in seconds to wait for the job to complete. If not set or set to 0, will return immediately without waiting
</parameters>

<tips>
- Use this to monitor long-running processes
- Check the 'done' status to see if process completed
- Can be called multiple times to view incremental output
- Use max_wait_time to avoid polling for long-running tasks
- If timeout is reached, the tool will indicate the task is still running and suggest trying again with a longer wait time
- Set max_wait_time to 0 or omit it for immediate status check
</tips>
