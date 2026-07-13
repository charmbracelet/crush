Get re.code's current runtime state: active model, provider, LSP/MCP status, skills, hooks, permissions, and disabled tools. No parameters needed.

<usage>
- Shows active model and provider, LSP/MCP server status, redacted MCP
  configuration shape, skills, hooks, permissions mode, disabled tools, and
  key options
- Shows canonical global `write_target` and optional `project_target`; use the
  project target only when the user explicitly requests a project override
- Use when diagnosing why something isn't working (missing diagnostics,
  provider errors, MCP disconnections)
- No parameters needed — always returns the full current state
</usage>

<tips>
- Check [lsp] and [mcp] sections for service health
- Use `mcp_add` for MCP mutations. Use `[config_files].write_target` for
  other global mutations and `project_target` only for explicit project scope
- Check [mcp_config] before opening crush.json when diagnosing MCP command,
  transport, or environment wiring
- Check [providers] to see which providers are enabled and available
- Check [skills] to see which skills are available and whether they have been
  loaded this session
- Check [hooks] to see which hook events are configured and whether the
  hook runner is active
- Pair with the crush-config skill to fix configuration issues
</tips>
