# Plan: `new_session` tool

1. **Tool Definition**: Create a new tool file `internal/agent/tools/new_session.go` (or similar). The tool will accept a `summary` parameter containing the context or instructions for the new session.
2. **Execution Mechanism**: The tool's execute method needs to signal the system to switch sessions. This can be done by returning a specific sentinel error, side-channel callback, or a special response type that the agent coordinator (`internal/agent/coordinator.go`) interprets to trigger a session initialization.
3. **Coordinator Integration**: Update `coordinator.go` (specifically `buildTools()`) to include the new tool. Update the agent message handling loop to intercept the tool's new session signal and process it (e.g., stopping the current generation, creating a new session with the summary as the first user message, and switching the UI to it).
4. **UI Updates**: Add a renderer for the new session tool in the UI layer (`internal/ui/chat/`, e.g., `tools.go`) to inform the user that the agent has started a new session ("Creating new session...").
5. **Testing**: Verify that when the agent invokes the tool, the current completion ends gracefully, a new session appears in the session list, the summary is pre-filled, and the agent auto-runs on the new context.
6. **Tool Allowlist Registration**: Add `"new_session"` to `allToolNames()` in `internal/config/config.go` so the tool is not filtered out before being sent to the LLM. Update corresponding test expectations in `internal/config/load_test.go`.

## Post-Mortem: Steps 1–5 Completed But Tool Not Visible

Steps 1–5 were completed in a prior session. The tool code, coordinator registration, UI renderer, and unit tests all exist and compile. However, the tool was **not visible to the LLM at runtime** because it was missing from `allToolNames()` in `internal/config/config.go:703-726`. This function provides a hardcoded allowlist of tool names. In `coordinator.go:461-466`, `buildTools()` filters the constructed tool list against `agent.AllowedTools` (which is derived from `allToolNames()`). Since `"new_session"` was absent from that allowlist, it was silently dropped before being sent to the LLM provider.
