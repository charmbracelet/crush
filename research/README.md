# Architecture research

This directory keeps external implementation research pinned to exact upstream
revisions. Research material is evidence for design comparison, not a source of
truth for Crush behavior.

## Claude Code from Source

`claude-code-from-source` is pinned as a Git submodule from
<https://github.com/alejandrobalderas/claude-code-from-source>.

The upstream project explicitly describes itself as an educational
reconstruction containing original pseudocode, not Anthropic's production
Claude Code source. Treat architectural claims from it as informed secondary
research and cross-check user-visible behavior against Anthropic's public
documentation or the official repository at
<https://github.com/anthropics/claude-code>.

The chapters most relevant to re:configured are:

- `book/ch02-bootstrap.md` for startup and configuration snapshots.
- `book/ch05-agent-loop.md` for bounded recovery and continuation behavior.
- `book/ch06-tools.md` for deterministic tool validation and execution.
- `book/ch08-sub-agents.md` for parent and sub-agent context boundaries.
- `book/ch12-extensibility.md` for skills, hooks, and trust boundaries.
- `book/ch15-mcp.md` for MCP lifecycle and runtime state.

Do not copy pseudocode directly into production. First verify the intended
behavior against Crush's actual config service, agent loop, tool contracts, and
tests.

## Official Claude Code cross-check

Use Anthropic's public documentation as the higher-authority source for
user-visible behavior:

- <https://code.claude.com/docs/en/settings> documents resolved configuration
  scopes, precedence, validation, and active-source inspection.
- <https://code.claude.com/docs/en/mcp> documents arbitrary server names,
  local/project/user scopes, schema validation, and connection-status checks.
- <https://code.claude.com/docs/en/hooks> documents blocking Stop hooks and the
  `stop_hook_active` guard required to prevent continuation loops.
- <https://code.claude.com/docs/en/debug-your-config> distinguishes a server
  being configured from it actually being connected and providing tools.
- <https://code.claude.com/docs/en/sessions> documents project-scoped JSONL
  transcripts, resume by session ID, and `/compact` replacing active history
  with a summary while keeping the stored transcript.

These sources support deterministic schema validation, explicit runtime status,
and bounded completion checks. They do not support hardcoding an MCP server or
changing the user's requested objective. They also show that configuration can
be reloaded and watched, so the reconstruction's snapshot discussion should be
applied only to security-sensitive state such as hook execution—not to the
entire live Crush configuration.

## Session and compaction cross-check

The official Codex app-server documentation at
<https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md>
documents persisted threads, reconstructed turn history, and `thread/resume`.
Its protocol documentation at
<https://github.com/openai/codex/blob/main/codex-rs/docs/protocol_v1.md>
also records the response ID used to continue a thread between turns.

Claude Code, Codex, and Crush therefore share the same boundary: the host
persists a session, but the model itself is stateless and only sees the active
context supplied for its current request. Compaction must preserve a compact
checkpoint rather than replaying the full transcript. Runtime-owned facts such
as active config paths and MCP connection state should be re-read after
compaction instead of copied into a long summary.

For MCP setup, instructions alone are advisory. The built-in `mcp-setup` skill
teaches the generic workflow, while `mcp_add` enforces transport shape,
approval, rollback, and connection verification for every MCP server without
provider-specific hardcoding. Source URLs are optional approval context rather
than a hard prerequisite because documentation hosts and registries can reject
automated fetches even when the proposed configuration is exact.

Claude Code, Codex CLI, Amp, and other clients documented by Playwright expose
a dedicated MCP-add command rather than asking the coding model to parse and
rewrite the client's full configuration. Crush follows that boundary with
`mcp_add`: `recode_info` supplies a redacted structured state snapshot, the
model verifies a primary source, and the tool owns the exact scoped mutation.
Sub-agent delegation does not improve this path because it duplicates the same
tool schemas and runtime discovery cost.
