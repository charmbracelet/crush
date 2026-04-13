Terminate a background shell process by ID; shell ID becomes invalid after killing.

<usage>
- Provide the shell ID returned from a background bash execution
- Cancels the running process and cleans up resources
</usage>

<features>
- Stop long-running background processes
- Clean up completed background shells
- Immediately terminates the process
</features>

<tips>
- Use this when you need to stop a background process
- The process is terminated immediately (similar to SIGTERM)
- After killing, the shell ID becomes invalid
</tips>
