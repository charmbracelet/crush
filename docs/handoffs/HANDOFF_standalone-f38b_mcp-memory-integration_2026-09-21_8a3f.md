---
schema_version: 1
handoff_id: 8a3f
parent_handoff_ids: []
lineage: none
chain: [standalone-f38b]
repo: djbclark/crush
workspace: ~/src/crush
branch: yolo-true-unblock
head_sha: 60606c7fb5e4b4ea9844239b1334b20c1472852d
created_at: 2026-09-21T10:18:00-04:00
writer: crush
---
# Handoff — YOLO mode testing, MCP/memory integration for Crush

## The Goal
Wire Crush (djbclark fork, branch `yolo-true-unblock`, PR charmbracelet/crush#3909) into
the same memory and MCP stack Claude Code and Hermes use — so Crush sessions feed
Hindsight long-term memory and all shared MCP tools are available. Validate the YOLO mode
feature branch end-to-end. Track the upstream PR via GitHub Actions to Hermes Telegram.

## Where We Are

**Crush fork** — tree clean at `60606c7f`, fully built and tested:
- `go build .` + `go test ./internal/...` — all pass.
- `~/opt/crush-yolo/crush` rebuilt from current tree (wrapper `~/.local/bin/crush` execs it).
- YOLO mode: `blockFuncs` returns nil when `permissions.SkipRequests()`; blocklist lifted;
  description omits banned commands; tests cover all three behaviors.
- PR #3909 open on charmbracelet/crush, tracked in site-djbclark
  `.github/workflows/track-issue-activity.yml` matrix (committed+pushed).

**~/.config/crush/crushrc** — 11 MCP servers + YOLO permissions (no files changed in crush repo):
- Memory: `hindsight`, `hindsight-shared`, `basic-memory`
- From Claude: `hermes`, `cow`, `ghost-os`, `beeper` (--oauth), `graft` (global CLI), `bezalel-fixed` (proxy :8765)
- From Hermes: `fieldy`, `cloudflare` (both --oauth, unauth'd until first use)
- All verified via `crush run` — 8 OK, 2 awaiting OAuth, 1 (hindsight) no-tools in disabled-bank dirs.

**Graft fix** — root cause: `tree-sitter-kotlin` has no prebuild for node 26 ABI 147.
Global install (`/opt/homebrew/lib/node_modules/@nanonets/graft`) has source-built
binding; npx cache (`~/.npm/_npx/b7f45974acf31384/`) now also has it. Both Claude's
`~/.claude.json` and crushrc point to `/opt/homebrew/bin/graft mcp` (durable). After
`graft upgrade` or npx cache wipes, re-run `node-gyp rebuild` in the kotlin module.

**Bezalel proxy hardened** — `~/.local/share/bezalel-proxy/proxy.py` (:8765, launchd
`sh.bezalel.proxy`): thread-safe per-session `MCP-Protocol-Version` tracking,
batch-aware notification interception (202 locally), SSE streaming pass-through,
502 on upstream unreachability, config-search prefers `mcpServers.bezalel` first.
Claude Code (`bezalel-fixed: ✔ Connected`) and Crush both verified.

**Hindsight write-back** — `~/.local/bin/crush-hindsight-sync` + LaunchAgent
`com.djbclark.crush-hindsight-sync` (15 min):
- Converts Crush `session show` → Claude JSONL transcript.
- Feeds `claude-stop-hook.js` retain pipeline (per-run temp variant tagged harness `crush`).
- Sessions retained to `coding-agent::crush` bank; diag log confirms `crush retain_ok`.
- Sweeps failed extraction ops on all `coding-agent::*` banks when LiteLLM is healthy.
- State: `~/.local/share/crush-hindsight/sync-state.json`, log `sync.log`.

**Memory** — `site-private/memory/reference_crush_hindsight_integration.md` (committed+pushed)
covers the graft rebuild, proxy patches, and write-back sync agent, with check/rebuild
commands. Numbered bullet preference recorded in `feedback_numbered_bullets.md`.

## What We Tried

- **npx @nanonets/graft mcp**: failed on node 26 with `No native build found for ABI 147`
  (tree-sitter-kotlin). Tried direct node-gyp rebuild in npx cache → works, but cache is
  evictable. Switched both configs to global CLI for durability.

- **Bezalel-fixed no-op**: upstream 400s all `notifications/initialized` POSTs, and 400s
  tools/list without `MCP-Protocol-Version` header. Claude tolerates the 400 but Crush
  rejected it. Proxy now intercepts notifications locally and injects the header per-session.

- **Hindsight `tools/list`: Method not found** in /tmp — by design: the `coding-agent::tmp`
  bank is disabled in `~/.hindsight/coding-agent.json`, so zero tools register. Works in
  real repos.

- **Extraction stuck**: all 6 ops failed with Clinepass 429 (weekly cap). Self-healing:
  sweep retries via `/v1/default/banks/{bank}/operations/{id}/retry` when litellm healthy
  (~07:00 tomorrow).

- **StructuredClone on HOOK_HARNESSES**: can't clone functions. Switched to spread `{...}`
  shallow copy.

## Key Decisions

1. **LaunchAgent poller over Crush hook events**: Crush only supports PreToolUse hooks;
   implementing Stop/SessionEnd is a feature that would pollute the yolo-true-unblock PR
   branch. A launcher-independent sync job is the right shape until the fork (or upstream)
   gains lifecycle hooks. The same claude-stop-hook retain pipeline can be wired directly
   once events exist.

2. **Harness tag via temp hook file**: The plugin's harness name is hardcoded in the entry
   (`runHarnessRetain("claude-code")`). Generating a temp copy that clones the harness entry
   and overrides `retain.harness` to `"crush"` avoids forking the plugin and doesn't touch
   the real file. Cost: one sed + file write per sync run (<1ms).

3. **Extraction sweep with LiteLLM health guard**: Retrying failed ops while the quota cap
   is up wastes backoff cycles. Only sweeping when `<litellm>/health` reports healthy
   endpoints means retries fire precisely when capacity returns.

4. **Graft: global CLI over npx**: The npx path works post-rebuild but can't survive
   cache eviction. Global install + rebuild-after-upgrade procedure is durable and
   documented.

## Evidence & Data

- Crush fork: `go test ./internal/...` — 0 failures. YOLO-specific tests in
  `internal/agent/tools/bash_test.go` and `internal/config/shellconfig_permissions_test.go`.
- MCP: 8 servers OK, 2 awaiting OAuth, 1 disabled-bank no-tools (by design).
- Retain: `crush retain_ok` in `/tmp/hindsight-plugin.log` — 8 sessions total
  (ebead1dbedeb2b40, 64f4957655563f7e, 4d8d43f714a425a3, 26e4cedf75159e48,
  2d67c18d163175ee, bb5d46c0a7ce6c7e, c401715239886c8e, de8348dc-d691...).
- Extraction: 6 failed ops on `coding-agent::crush` (clinepass 429). Pending sweep.
- Site-djbclark matrix: 17 jobs, all success, baseline cache seeded for crush-3909.
- Git: tree clean, no files changed in crush repo this session.

## Operator Feedback

- "Remember to always number bullet list items" → recorded in `feedback_numbered_bullets.md`.
- "Track the PR/Issue for this via github actions to hermes" → added to matrix in site-djbclark.
- "But be sure to merge any local changes" → graft upgrade rebuild procedure documented;
  bezalel proxy patches preserve/layer; crush fork built from current tree.
- "Feel free to upgrade things" → graft already at latest (0.18.0).

## Where We're Going

1. Wait for Clinepass weekly cap reset (~07:00 UTC 2026-09-22); next sync sweep will
   re-kick extraction. Verify pages build: `hindsight_list_knowledge_pages` on
   `coding-agent::crush`.
2. Complete fieldy/cloudflare OAuth (first real tool use will pop browser flow).
3. Upstream PR #3909 — monitor via GitHub Actions tracker; handle review feedback.

## Quick Start

```bash
# Rebuild crush fork after pulling changes
cd ~/src/crush && go build -o ~/opt/crush-yolo/crush .

# Manual hindsight sync (or the LaunchAgent runs every 15 min)
~/.local/bin/crush-hindsight-sync

# Check extraction status
curl -s http://127.0.0.1:8888/v1/default/banks/coding-agent%3A%3Acrush/operations | python3 -c "
import json,sys; d=json.load(sys.stdin)
for o in d['operations']: print(o['task_type'], o['status'], o['updated_at'][:19])"

# Rebuild graft kotlin after upgrade/cache wipe
cd /opt/homebrew/lib/node_modules/@nanonets/graft/node_modules/tree-sitter-kotlin && npx -y node-gyp rebuild

# Restart bezalel proxy after edits
launchctl kickstart -k gui/$(id -u)/sh.bezalel.proxy
```