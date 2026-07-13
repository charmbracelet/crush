Get re.code's current runtime and configuration state without opening `crush.json`.

<usage>
- `detail=summary` (default) shows canonical config targets, staleness, model
  slots, and key options.
- `detail=mcp` shows MCP runtime state and redacted saved configuration shape.
- `detail=skills` shows skill discovery and loaded state.
- `detail=full` shows providers, LSP/MCP state, skills, hooks, permissions,
  disabled tools, and options.
- Every result includes a revision. Pass it as `since_revision` to check for
  changes without repeating unchanged details.
- Shows canonical global `write_target` and optional `project_target`; use the
  project target only when the user explicitly requests a project override
- Use when diagnosing why something isn't working (missing diagnostics,
  provider errors, MCP disconnections)
- Omit parameters for the compact summary; request only the detail needed.
</usage>

<tips>
- Check [lsp] and [mcp] sections for service health
- Use `mcp_manage` for existing MCP servers and `mcp_add` for new or replacement
  definitions. Use `[config_files].write_target` for other global mutations and
  `project_target` only for explicit project scope.
- Check [mcp_config] before opening crush.json when diagnosing MCP command,
  transport, or environment wiring
- Check [providers] to see which providers are enabled and available
- Check [skills] to see which skills are available and whether they have been
  loaded this session
- Check [hooks] to see which hook events are configured and whether the
  hook runner is active
- Pair with the crush-config skill to fix configuration issues
</tips>
