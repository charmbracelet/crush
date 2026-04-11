Get information about Crush's current runtime configuration and service
state.

<usage>
- Shows active model and provider, LSP/MCP server status, skills,
  permissions mode, disabled tools, and key options
- Use when diagnosing why something isn't working (missing diagnostics,
  provider errors, MCP disconnections)
- No parameters needed — always returns the full current state
</usage>

<tips>
- Check [lsp] and [mcp] sections for service health
- Check [providers] to see which providers are enabled and available
- Check [skills] to see which skills are available and whether they have been
  loaded this session
- Pair with the crush-config skill to fix configuration issues
</tips>
