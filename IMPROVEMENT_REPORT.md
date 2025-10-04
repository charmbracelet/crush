# Cliffy Improvement Report

## Project Snapshot
- Cliffy delivers a streamlined CLI on top of Crush, but several advertised switches (`--show-thinking`, `--stats`, `--output-format json`) never influence execution, and the runtime still depends on legacy Crush conventions.
- Implementation docs in `docs/` outline richer behavior (structured output formatter, diff emission, telemetry), yet the production code path stops short of those plans.
- Testing focuses on the scheduler, leaving the CLI surfaces, progress renderer, and output formatter without automated coverage.

## High-Priority Improvements

### 1. Wire CLI Flags into Execution
- Location: `cmd/cliffy/main.go:23-214`, `internal/runner/runner.go:19-178`
- Flags such as `--show-thinking`, `--thinking-format`, and `--stats` are parsed but never forwarded to the volley scheduler or agent, so users see no effect.
- Recommendation: extend `volley.DefaultVolleyOptions` or introduce a request-scoped struct to carry these options, and reuse the existing rendering logic in `internal/runner` so thinking traces and stats flow through the CLI path.

### 2. Implement Structured Output Modes
- Location: `cmd/cliffy/main.go:157-214`, `cmd/cliffy/main.go:217-273`, `internal/output/formatter.go:1-103`
- `--output-format json` is accepted but `outputVolleyResults` always prints plain text. Docs describe JSON/diff support, but the formatter is never invoked.
- Recommendation: branch on `opts.OutputFormat`, serialize `[]volley.TaskResult` + `VolleySummary`, and delegate to helpers in `internal/output`. Add CLI tests to lock behavior down.

### 3. Surface Token & Cost Stats
- Location: `cmd/cliffy/main.go:31`, `internal/volley/scheduler.go:214-273`
- Token totals are tracked per task/summary, but `--stats` never toggles anything. Quiet mode also suppresses the summary entirely, hiding failures in multi-task runs.
- Recommendation: introduce a stats renderer that respects `Quiet`/`Verbose`, prints totals when requested, and exits with rich context for failed tasks.

### 4. Remove Remaining Crush Persistence
- Location: `internal/runner/runner.go:54-60`, `internal/config/load.go:45-110`
- Despite the “zero persistence” claim, the fallback log path still writes to `.crush/cliffy.log`, and config defaults reference `.crush`.
- Recommendation: move logging and data directories to `.cliffy` (config already passes this in) and gate log creation behind opt-in debug mode so the binary remains stateless by default.

## Core Functionality & DX Enhancements

### 5. Support Scripts and Pipelines via STDIN / Task Files
- Location: `cmd/cliffy/main.go:147-154`
- Tasks can only be passed as CLI args, making it awkward to stream prompts from other tools or reuse saved prompts.
- Recommendation: accept `-` to read tasks from STDIN, add `--tasks-file` to load newline-separated prompts, and allow JSON task batches so Cliffy fits naturally into shell pipelines.

### 6. Expose Per-Task Metadata for Automation
- Location: `internal/volley/task.go:14-76`, `cmd/cliffy/main.go:217-273`
- Results lack fields like exit codes, retry counts, and tool traces when printed, limiting scriptability.
- Recommendation: enrich `TaskResult` output with status, retry attempts, token totals, and tool invocation summaries; provide `--emit-tool-trace` that streams `tools.ExecutionMetadata` as NDJSON for downstream log aggregation.

### 7. Consolidate Single-Task and Volley Execution Paths
- Location: `internal/runner/runner.go:19-178`, `cmd/cliffy/main.go:123-215`
- The legacy `runner` package streams thinking and tool calls nicely, but the CLI now uses only the volley path, so single-task DX regressed.
- Recommendation: fold the streaming logic from `runner` into the volley scheduler (or call `runner.Execute` when only one task is supplied) so users get consistent live feedback without learning two flags.

### 8. Improve First-Run Experience with Interactive Checks
- Location: `internal/config/init.go:11-91`, `internal/config/load.go:45-166`
- Missing API keys or config errors surface as stack traces; `MarkProjectInitialized` hints at a guided flow that never materialized.
- Recommendation: implement `cliffy init`/`cliffy doctor` commands that verify provider keys, write sample `cliffy.json`, and explain fallback options. Cache success in `.cliffy/init` to skip repeats.

### 9. Offer Built-In Task Templates & Model Presets
- Location: `internal/config/config.go:61-164`
- Users must hand-edit JSON to toggle models/tools; there are no presets for “fast QA”, “deep review”, etc.
- Recommendation: ship curated presets (YAML or JSON) alongside the binary, expose `cliffy preset list|apply`, and allow per-task overrides like `cliffy --preset sec-review "audit auth.go"` to smooth the DX for non-experts.

### 10. Provide Shell Completions and Discoverability
- Location: `cmd/cliffy/main.go:38-114`
- Cobra already supports completions, but Cliffy never registers them, so flags like `--context-file` stay hidden.
- Recommendation: add `completion` subcommands and document them in `README.md`. Pair with `--help` examples to highlight multi-task usage, presets, and JSON output.

## Medium-Priority Improvements

### 11. Finish Volley Resilience Story
- Location: `internal/volley/scheduler.go:21-35`, `internal/volley/scheduler.go:158-209`
- Fields like `currentConcurrent`/`failureCount` are never consulted, and retry backoff lacks jitter. Fail-fast cancellation works but never drains queues cleanly.
- Recommendation: either remove the dead state or complete the adaptive concurrency/anti-thundering herd logic the fields hint at. Add jitter and per-error retry policies.

### 12. Align Docs With Implementation
- Location: `docs/architecture.md`, `docs/implementation-guide.md`
- Design docs promise features (output formatter, diff mode, telemetry) that are absent. Contributors will chase ghosts unless the docs or code converge.
- Recommendation: update the docs to reflect current scope, then track missing pieces in issues/roadmap so new work is deliberate.

### 13. Expand Test Coverage
- Location: `cmd/cliffy/`, `internal/output/`, `internal/volley/progress.go`
- Only the scheduler has substantive tests; regressions in CLI flag plumbing, tool trace formatting, or progress rendering would go unnoticed.
- Recommendation: add cobra command tests covering flag combinations, unit tests for `FormatToolTrace`, and snapshot-based checks for the progress tracker under different verbosity levels.

## Quick Wins
- Document expected environment variables (OpenRouter) in `schema.json` to match README guidance.
- Add `--max-concurrent` override so users can tune throughput without editing config.
- Provide a `Taskfile` target for `go test ./...` + lint to encourage routine validation.

## Suggested Next Steps
1. Prioritize wiring the CLI options, structured output, and single-task streaming so users immediately feel the improvements.
2. Ship the new DX affordances (`--tasks-file`, presets, doctor command) alongside updated docs to smooth onboarding.
3. Backfill tests around the new surfaces to stabilize the faster release cadence your teammate wants.
