Restart LSP (Language Server Protocol) clients.

<usage>
- Restart all running LSP clients or a specific LSP client by name
- Useful when LSP servers become unresponsive or need to be reloaded
- Parameters:
  - name (optional): Specific LSP client name to restart. If not provided, all clients will be restarted.
</usage>

<features>
- Gracefully shuts down all LSP clients
- Restarts them with their original configuration
- Reports success/failure for each client
</features>

<limitations>
- Only restarts clients that were successfully started
- Does not modify LSP configurations
- Requires LSP clients to be already running
</limitations>

<tips>
- Use when LSP diagnostics are stale or unresponsive
- Call this tool if you notice LSP features not working properly
</tips>
