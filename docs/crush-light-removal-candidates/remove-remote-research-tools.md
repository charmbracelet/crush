# Remove remote research tools

- **Suggested branch:** `draft/remove-remote-research-tools`
- **Risk level:** `medium`

## Short summary
Remove `agentic_fetch`, `web_fetch`, `web_search`, `download`, and related optional remote-research/search helper surfaces used to pull external content into the agent loop.

## Why this is a removal candidate
The light variant explicitly must remove `web_fetch`, `web_search`, and `download`; grouping them with `agentic_fetch` keeps one coherent future PR around external-content acquisition.

## Why the chosen risk level applies
These tools are separate from core local-file tooling, but `agentic_fetch` shares sub-agent machinery and some prompt/template wiring. The code boundary is still clear.

## User-visible behavior affected
Users lose built-in remote search/fetch/download helpers and any agent flow that depends on them for web research.

## Code entrypoints
- `internal/agent/agentic_fetch_tool.go`
- `internal/agent/tools/download.go`
- `internal/agent/tools/web_fetch.go`
- `internal/agent/tools/web_search.go`

## Known touch points
- Files/packages: the tool implementations above, fetch helper/types files, coordinator tool registration, templates/agentic_fetch*
- Config: disabled-tools defaults/help text if these tools are advertised
- Docs/tests: tool markdown, README/help text, agent/tool testdata covering fetch/download/parallel tool calls
- API/server: none direct beyond generic agent execution
- UI: tool output rendering for remote research responses
- Persistence/data model: none specific

## Dependencies on kept features
Must preserve local `fetch`, `view`, `grep`, `glob`, and normal shell/file tools unless explicitly removed elsewhere.

## Things that must be preserved while removing it
Keep local repository inspection tools, the `sourcegraph` tool, and primary agent execution intact.

## Suggested removal order
After or alongside sub-agent removal; before fantasy replacement.

## Acceptance criteria for the future implementation PR
- No `agentic_fetch`, `web_fetch`, `web_search`, or `download` tools remain.
- Associated tool docs/tests/fixtures are removed or updated.
- No UI or README copy advertises remote research/download helpers.
- `sourcegraph` remains untouched for its own follow-up removal PR.
- Core local tools remain functional.

## Open questions / uncertainties
- None currently.
