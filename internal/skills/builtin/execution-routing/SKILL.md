---
name: execution-routing
description: Use when a substantial re.code task requires choosing among local tools, external research, or an independent sub-agent investigation.
---

# Execution Routing

Preserve the user's objective and choose the smallest tool that can establish
the next missing fact.

## Evidence Order

1. Use the current request, `<env>`, project context, and already returned tool
   results before gathering more context.
2. Use `view`, `grep`, `glob`, `ls`, and LSP tools for repository evidence.
3. Use the shell for bounded host, runtime, package, process, service, and git
   facts. Treat the reported platform as authoritative and change strategy
   after a command-not-found result.
4. Use `recode_info` only when re.code configuration, canonical write targets,
   permissions, or saved provider/MCP definitions are relevant.
5. Use `web_search` and a primary source for unstable external names, package
   identities, APIs, or versions. Do not web-search capabilities already
   present in the live MCP catalog.
6. Delegate only a bounded independent investigation whose result the root
   agent can verify and integrate.

## Failure Handling

- Read the complete result and change the failed assumption, tool, or scope.
- Do not repeat a disproven command or invent a tool, package, path, or reload
  command.
- Keep successful evidence and the original objective across continuations.
- Report a blocker only when the next required fact depends on unavailable
  access, credentials, files, network, or a user decision.

After changes, verify on the same surface that was modified. A narrated next
step is not completion.
