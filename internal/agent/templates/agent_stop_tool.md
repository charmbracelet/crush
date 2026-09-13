Stop a running sub-agent that you dispatched with `agent_dispatch`.

Use it when a sub-agent is stuck, working on the wrong thing, or no longer needed. Check what it is doing with `agent_status` first if you are not sure.

Stopping ends the sub-agent's current run. Its session is kept, so `agent_send` with the same label resumes it later with its earlier work still in context. A stopped sub-agent does not send a report.

If it needs correcting rather than stopping, prefer `agent_send`, which redirects it without losing its place.
