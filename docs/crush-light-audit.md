# crush-light feature-removal audit

## Audit scope
- Repository root: `/home/runner/work/crush-light/crush-light`
- Coverage basis: tracked repository content excluding `.git/` and `.github/agents/`
- Reviewed paths: **982** total (**123** directories, **859** files)
- Audit artifacts:
  - `docs/crush-light-reviewed-paths.tsv`
  - `docs/crush-light-fantasy-usage.md`
  - `docs/crush-light-removal-candidates/*.md`
  - `docs/crush-light-draft-prs/*.md`

## Baseline validation before audit edits
- `go build .` ✅ passed
- `go test -race -failfast ./...` ✅ passed
- `./scripts/check_log_capitalization.sh && golangci-lint run --path-mode=abs --config=.golangci.yml --timeout=5m` ⚠️ stopped at a pre-existing log-lint failure in `internal/agent/tools/mcp/init.go` (`failed to initialize mcp client`, `skipping disabled mcp`, `error closing mcp session`).

## Required-kept features confirmed during review
- Multiple named sessions per project remain required.
- Per-session file history/version snapshots remain required.
- File modification-time tracking/stale-write protections remain required.
- `internal/oauth` and provider OAuth flows remain required alongside BYOK.
- Model picker and mid-session model switching remain required.
- `internal/server` and generated Swagger docs remain required, with removed-feature endpoints/docs to be deleted later.

## Future removal draft PR inventory
| Feature | Risk | Draft PR status / link / branch | Candidate doc |
| --- | --- | --- | --- |
| Remove MCP support | `medium` | Created: [PR #1](https://github.com/JTRNS/crush-light/pull/1) on `copilot/prepare-feature-removal-audit` | `docs/crush-light-removal-candidates/remove-mcp-support.md` |
| Remove LSP support | `high` | Planned branch: `draft/remove-lsp-support` (body prepared in repo; remote PR creation blocked by single-branch PR tooling) | `docs/crush-light-removal-candidates/remove-lsp-support.md` |
| Remove PostHog analytics | `low` | Planned branch: `draft/remove-posthog-analytics` (body prepared in repo; remote PR creation blocked by single-branch PR tooling) | `docs/crush-light-removal-candidates/remove-posthog-analytics.md` |
| Remove sub-agent orchestration | `high` | Planned branch: `draft/remove-sub-agent-orchestration` (body prepared in repo; remote PR creation blocked by single-branch PR tooling) | `docs/crush-light-removal-candidates/remove-sub-agent-orchestration.md` |
| Remove remote research tools | `medium` | Planned branch: `draft/remove-remote-research-tools` (body prepared in repo; remote PR creation blocked by single-branch PR tooling) | `docs/crush-light-removal-candidates/remove-remote-research-tools.md` |
| Remove parallel tool execution | `high` | Planned branch: `draft/remove-parallel-tool-execution` (body prepared in repo; remote PR creation blocked by single-branch PR tooling) | `docs/crush-light-removal-candidates/remove-parallel-tool-execution.md` |
| Remove out-of-working-dir permission gate | `medium` | Planned branch: `draft/remove-out-of-working-dir-permission-gate` (body prepared in repo; remote PR creation blocked by single-branch PR tooling) | `docs/crush-light-removal-candidates/remove-out-of-working-dir-permission-gate.md` |
| Remove todo support while keeping sessions | `medium` | Planned branch: `draft/remove-todo-support` (body prepared in repo; remote PR creation blocked by single-branch PR tooling) | `docs/crush-light-removal-candidates/remove-todo-support.md` |
| Remove NixOS and Home Manager module support | `low` | Planned branch: `draft/remove-nix-home-manager-support` (body prepared in repo; remote PR creation blocked by single-branch PR tooling) | `docs/crush-light-removal-candidates/remove-nix-home-manager-support.md` |
| Track charm.land/fantasy usage | `high` | Planned branch: `draft/track-fantasy-usage` (references prepared in repo; dedicated remote PR still pending because only the current task branch can be opened from this environment) | `docs/crush-light-removal-candidates/track-fantasy-usage.md` |

## Additional removal candidates identified during review
These are plausible light-variant follow-ons but are not in the mandatory removal set above.

- `sourcegraph` tool surface if it is not folded into the remote-research-tools PR.
- Desktop notifications (`internal/ui/notification/**`, `options.disable_notifications`) if further UX simplification is desired.
- Update-checking and stats/dashboard surfaces (`internal/update/**`, `internal/cmd/stats*`) if the fork wants a smaller non-core command/UI set.
- Hyper-specific provider support (`internal/agent/hyper/**`, `internal/oauth/hyper/**`) if the fork decides to trim provider-specific integrations beyond the protected OAuth/BYOK baseline.

## Explicit removal-target mapping
- **MCP support** → `docs/crush-light-removal-candidates/remove-mcp-support.md`
- **LSP support + configurable per-language LSP servers** → `docs/crush-light-removal-candidates/remove-lsp-support.md`
- **PostHog analytics** → `docs/crush-light-removal-candidates/remove-posthog-analytics.md`
- **Sub-agents** → `docs/crush-light-removal-candidates/remove-sub-agent-orchestration.md`
- **Parallel tool execution** → `docs/crush-light-removal-candidates/remove-parallel-tool-execution.md`
- **Out-of-working-dir permission gate** → `docs/crush-light-removal-candidates/remove-out-of-working-dir-permission-gate.md`
- **Todos tool + todo persistence** → `docs/crush-light-removal-candidates/remove-todo-support.md`
- **`web_fetch`, `web_search`, `download`** → `docs/crush-light-removal-candidates/remove-remote-research-tools.md`
- **NixOS/Home Manager module support** → `docs/crush-light-removal-candidates/remove-nix-home-manager-support.md`
- **`charm.land/fantasy` tracking only** → `docs/crush-light-removal-candidates/track-fantasy-usage.md`

## Fantasy tracking notes
- Usage/dependency references are collected in `docs/crush-light-fantasy-usage.md`.
- The required dedicated tracking PR should mirror those references.
- This environment exposes only a single branch-scoped PR creation path, so the fantasy tracker body is prepared in-repo but still needs a separate remote PR created from a dedicated branch.
- This environment does not expose a PR-comment write tool, so the future tracking PR should include the same references in its description and receive follow-up comments when comment-writing access is available.

## Draft PR creation limitation encountered
- `create_pull_request` and `report_progress` both target the current task branch (`copilot/prepare-feature-removal-audit`) instead of arbitrary local branches.
- Because of that constraint, only one actual remote draft PR could be opened during this task: MCP removal ([PR #1](https://github.com/JTRNS/crush-light/pull/1)).
- The remaining future PR descriptions are fully prepared in `docs/crush-light-removal-candidates/*.md` with suggested branch names so a follow-up agent or human can open them without redoing discovery.

## Review coverage notes
- `docs/crush-light-reviewed-paths.tsv` records every reviewed directory and tracked file with status, notes, and category flags.
- Testdata and generated artifacts were reviewed as grouped coverage surfaces; notes identify the owning subsystem and whether the paths support removable features, kept features, fantasy usage, configuration, or build/docs/test surfaces.
- Non-code repository assets (README, schema, workflows, scripts, release config, fixtures, and generated Swagger/schema artifacts) are included in the coverage file.

## Open questions
- Should `sourcegraph` be removed together with the remote-research tools, or kept as the lone external search integration?
- After LSP removal, which `view`/`edit` affordances need explicit non-LSP fallbacks documented or tested?
- Is any remaining permission behavior still expected to enforce a working-directory boundary after the explicit gate is removed?
- Should Hyper-specific provider integration stay in scope for a later light-variant cut, given the requirement to keep OAuth/BYOK support generically?
- If PR-comment access remains unavailable, what repository-approved workaround should be used for future fantasy usage annotations beyond the PR description and audit docs?
