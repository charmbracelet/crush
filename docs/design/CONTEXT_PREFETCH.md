# Context Prefetch — Persistent Project Index & `map` Tool

> **Status:** Implemented — PR #34 on `feat/context-prefetch`
> (plan doc committed in-branch for reviewers). Part of a plan
> series — continuation: `SEMANTIC_INDEX.md`;
> consumers: `HARNESS_TOPOLOGY.md` (edge evidence), `SESSION_KNOWLEDGE.md`
> (session-hot ranking); gate: `EVAL_HARNESS.md`.
>
> **Shipped:**
>
> - sidecar store + schema (continuation tables reserved)
> - gitignore-aware walker; regex tagger (Go/Py/Rust/TS/JS/Java)
> - `map` tool: skeleton / `path=` / `symbol=` modes
> - lazy mtime+size invalidation, refresh-before-render ordering,
>   rate-limited dirty-scan fallback on symbol miss
> - shared process-wide service (`index.Shared`), single-connection
>   sidecar (`db.OpenDBFile`)
> - background first-build; batched commits (count + time bounds)
>   serve partial results while indexing
> - write-event retag (`index.NotifyWritten`) on all file-mutating
>   tools — agent-created files are visible immediately
> - `project_index` option, default off (`option project-index true`
>   in crushrc); `readOnlyTools` inclusion; tool-level test
>
> **Remaining:** LSP enrichment. Semantic search: `SEMANTIC_INDEX.md`.
> **Now done:** git-churn ranking (one `git log` pass per walk,
> blended with ref-degree), coder prompt hint (gated on the flag),
> `project-index` eval arm (`eval/experiments/project-index.json` +
> `realrepo-*` corpus).

## Goal

Give the model a persistent, per-project map — files, top-level
symbols, and file-to-file references — so "where does X live" is
one local query instead of a chain of discovery roundtrips. The
map is navigation, not content: it says _where_, `view` still
reads before edit.

## Problem

Exploration is purely model-driven. To answer "where does X live"
the model issues `grep`/`glob`/`view` calls — each a full
request-level roundtrip that resends the whole prepared history.
For "create endpoint A" in a large repo this is typically 10-20
discovery calls before the first edit; the cost compounds per step
and the gathered evidence is exactly what stubbing later removes —
which feeds the re-read loop observed in practice (model claims
full context, edits 1-2 files, then re-gathers).

High-end harnesses answer this with a **map**: Aider's ranked
repo-map, Cursor's codebase index. The model starts with a skeleton
of the project and reads surgically instead of grep-blind. Crush has
no equivalent — LSP tools give precision on a _known_ file but
nothing answers "which file" at project scale.

## What exists

- **Project-scoped storage for free:** `Options.DataDirectory`
  defaults to `.crush` resolved against the working dir
  (`defaultDataDirectory` in `internal/config`). An index file
  inside it is automatically per-project — no working_dir hashing,
  no global store.
- **CGO-free SQLite drivers already vendored:** `modernc.org/sqlite`
  and `ncruces/go-sqlite3` (go.mod) — required since builds use
  `CGO_ENABLED=0`. `openDB` (`internal/db/connect.go`) already
  provides the pragma'd connection; `db.OpenDBFile`
  (`internal/db/file.go`) — added by this work — reuses it for a
  sidecar path.
- **Ignore-aware walking already exists:** `fsext.FastGlobWalker`
  respects `.gitignore`/`.crushignore` plus the unconditional
  `fastIgnoreDirs` set — vendored/generated exclusion needs no
  per-project config.
- **The staleness signal already exists:** mtime tracking is the
  established invalidation mechanism (`stampReadMtime` in
  `internal/agent`; observed-mutation pass in `stubs.go`). The index
  reuses `{mtime, size}` — see Invalidation for why the pair.
- **LSP tools are the follow-up layer:** `lsp_symbols`,
  `lsp_references`, `lsp_call_hierarchy`, `lsp_definition` already
  ship — the index answers "where", LSP answers "exactly what".
- **Tool conventions:** `tools.NewXxxTool(deps...)` wired in the
  coordinator's toolset builder; each tool has a `.go` + `.md`
  pair; output capped via truncation helpers; `allToolNames` and
  `readOnlyTools` (both in `internal/config`) are static registries
  a new tool MUST join — the second one puts `map` on the task
  subagent.

## Design — precise navigation index

### Store — sidecar SQLite, disposable by contract

`<DataDirectory>/index.db` (i.e. `<project>/.crush/index.db`). The
index is a **rebuildable cache, not authoritative data** — deleting
it must always be safe. Schema as built (`internal/index/index.go`):

```sql
files   (path PK, mtime, size, indexed_at)
symbols (path, name, kind, line, exported)
refs    (src_path, dst_path, UNIQUE(src_path, dst_path))
        -- dst may be a DIRECTORY (Go package imports); the unique
        -- index pins the dedup invariant ranking relies on, not
        -- just the tagger's refSet convention
-- reserved for the semantic continuation (SEMANTIC_INDEX.md).
-- Honest rationale: they cost nothing, and they pin the contract
-- the continuation builds against — a disposable cache could just
-- as well CREATE TABLE later; these exist so the shape is reviewed
-- now, not improvised then.
chunks     (id PK, path, start_line, end_line, text_hash)
embeddings (chunk_id PK → chunks.id, vector, model_id)
```

`refs` is dedup'd per source file at insert time, so a `count`
column would always be 1 — dropped during review.

`OpenDBFile` mirrors `Connect`'s single-connection rule
(`SetMaxOpenConns(1)`): interleaved multi-conn writes on this
driver caused WAL/header desync (`SQLITE_NOTADB`) under concurrent
sub-agents — the index inherits the same constraint since several
agents share the file. Consequence for the background build: a `map`
query can't read _during_ a batch commit — it serializes behind each
transaction — so the tag loop bounds batches by count (200) AND time
(`walkBatchMax`, 500ms); partial results are genuinely concurrent
between commits. (A separate read pool under WAL might relax this,
but the desync was specifically multi-conn _writes_ — not worth
re-opening.) Multi-process is safe: `openDB` sets WAL +
`busy_timeout=30000`, so two Crush processes on one project can't
corrupt the file. In WAL mode a `map` read isn't blocked by another
process's writer (readers see the pre-commit snapshot); the 30s
bound applies to the writer-vs-writer/checkpoint path — a
_background build_ can wait, queries mostly can't. No reclamation story
beyond "delete the file": freelist pages accumulate over churn, and
since the store is disposable, `rm index.db` is the GC — a
`VACUUM` on build would only polish a cache that's allowed to be
deleted anyway.

`working_dir` keying is implicit: the file lives in the project's
own `.crush/`, so subagent child sessions on the same project share
it automatically. SQLite serializes the small per-file upserts.
Edge case: `options.data_directory` can point at an absolute path —
two projects configured to the same data dir would share one
`index.db` keyed by relative paths, a real collision. Acceptable
(nonstandard config, rebuildable cost) but noted; the fix, if it
ever bites, is keying the file by working-dir hash. Mirror image:
`byWorkingDir` (the `NotifyWritten` registry) is keyed on
workingDir alone while `shared` is keyed on `(dataDir, workingDir)`
— two data dirs on one working dir makes a write notification
last-writer-wins. Same nonstandard-config edge class; noted.
A third route to the same collision needs no config at all:
`LookupClosestBounded` can return an _ancestor's_ `.crush` when
Crush runs in a project subdir — two roots then share one
`index.db` with different path bases, and each walk's reconcile
drops the other's rows (stat-keep doesn't save them — the paths
don't exist under the other root). Rebuildable-cache cost, same
fix path (working-dir keying) if it ever bites.

### Builder — walk once, tag cheaply

Two phases per walk (`walk.go`): collect every candidate file
(fastwalk + `FastGlobWalker` skip rules, 256KB size cap, binary
sniff — zero-byte files are skipped entirely, so empty files are
invisible until they gain content), then reconcile — drop vanished
paths, re-tag only new or `{mtime,size}`-changed files, committing
in bounded batches.
Refs resolve against the complete post-walk path set, so forward
imports still resolve.

`.crush/` itself is excluded — `directoryLister.shouldIgnore` always
skips a built-in dir set (`fastIgnoreDirs`: `.crush`, `.git`,
`node_modules`, …) plus `commonIgnorePatterns` (`vendor`, `bin`,
`build`, `dist`, `out`, `target`, lockfiles, …) independent of the
project's `.gitignore`, so the indexer never walks its own DB,
session artifacts, or vendored deps even in a repo that doesn't
ignore them — and the skeleton's completeness claim carries that
boundary. `web_fetch`'s `page-*.md` scratch files already land
under `DataDirectory` (`crush-fetch-*` temp dirs), so they neither
index nor pollute the tree. And `.crush/` self-protects in git too: Crush writes a
generated `.gitignore` (`*` plus `!skills/` negations) into the
data directory at startup, so `index.db` can't be committed by
accident. Files in untagged
languages (C, Ruby, …) are still indexed — they appear in the
layout and contribute to refs — they just have no symbols; a
skeleton of a mixed-language repo is therefore complete on files,
sparse on declarations.

**Parser decision — `CGO_ENABLED=0` rules out go-tree-sitter**
(it needs cgo). Options, by precision:

| Parser                     | Precision            | Cost                                                           |
| -------------------------- | -------------------- | -------------------------------------------------------------- |
| Per-language regex tagger  | top-level decls only | zero deps, instant — **shipped**                               |
| LSP `documentSymbol` batch | precise              | needs running server; slow cold-start on big repos — remaining |
| tree-sitter via wazero     | precise              | new dep + `.wasm` grammar packaging — escape hatch             |

The map is a navigation hint; precision lives in the follow-up
tools. Known tagger losses, accepted: Go `const ()`/`var ()`
block members, deeply nested decls, macro-generated code,
`export *` re-exports.

### Invalidation — lazy mtime+size, dated output

Nanosecond `mtime` plus `size`, compared as a pair — second-
resolution mtime alone misses rapid successive rewrites (caught by
the roundtrip test). Three paths:

- **Walk-time reconcile**: skip files whose stored pair matches.
- **Query-time refresh**: `refreshPaths` re-tags result paths
  **before** rows are materialized — refreshing after the select
  renders pre-refresh data (review-caught bug; `Subtree`/`Symbol`
  query candidate paths first, `renderTopFiles` already did).
- **Miss fallback**: a `Symbol` miss stat-scans all indexed files
  (`refreshDirty`) and retries once — a symbol added to an existing
  file mid-session resolves without a re-walk. The scan is
  rate-limited (`dirtyScanInterval`, 10s) — a model guessing wrong
  symbol names can't pay a full stat-scan per miss. Cost of the
  window: a symbol added to an existing file by an _external_ edit
  can keep missing until the interval elapses — acceptable, since
  tool-driven writes are covered by `NotifyWritten`. Files _created_
  by tools are covered by `NotifyWritten`; externally-created files
  appear at the next scheduled re-walk. `map.md` tells the model to
  fall back to glob/grep when it suspects something is missing.
- **Drop rule is ENOENT-only**: a path's row is deleted only when
  `fs.ErrNotExist` — on any other stat/read failure the stale row
  is kept. Ghost files would otherwise persist until the next walk,
  and a transient error must not silently un-index a file.
- **Refresh cost is proportional to result-set size**: `Subtree(".")`
  on a big repo stats every returned path (plus a possible re-tag
  each on the single connection), and `knownDir` prefix-probes
  `files` per import on the lazy path — bounded and correct, but a
  root-wide subtree call is the most expensive query the tool has.

Dir-resolving refs (Go package imports) need the exists-probe to
cover directories: the walk builds `knownDirs` from the candidate
set; the refresh path answers dirs via an `instr(path, dir || '/')`
prefix probe on `files` — a files-only probe silently deletes every
Go ref on each lazy re-tag (review-caught).

Ref resolution per language, as built (`tagger.go`): Go imports
resolve to package dirs under `module` path; JS/TS `./foo` probes
extension list then `./foo/index.*`; Python `a.b.c` → `a/b/c.py`
or `a/b/c/__init__.py`; Rust `use crate::…`/`mod` probe `src/` and
sibling paths plus `mod.rs`; Java probes `src/{main,test}/java/`.
Unresolved specifiers (external deps, stdlib) produce no ref —
in-degree ranking is only as good as this resolution, so it
degrades most for JS/TS path aliases (`@/…`, tsconfig paths),
which are unresolvable without reading tsconfig.

### Tool — `map`, idempotent by design

```
map                         → ranked skeleton (default budget 500)
map path=internal/agent     → subtree detail: files + symbols
                              (accepts a file path too — exact match)
map symbol=SessionAgentCall → def sites + referrer files
map semantic="..."          → reserved for SEMANTIC_INDEX; today a
                              guidance message
```

The token budget is a **hard cap on bytes, a soft cap on tokens**:
`capOutput` truncates at `max_tokens × 4` bytes — exact for typical
prose, but dense code or CJK runs ~3 bytes/token, so real output can
overshoot the declared token count by up to ~25%. "~500" in the
example is just the default `max_tokens` — same bound, not a
separate selection budget.

Path hygiene: `path=` accepts project-relative dirs only — absolute
paths (other than `/`, the root alias) and `..` escapes return a
guidance message rather than silently matching nothing.

Name collision: none possible — MCP tools are namespaced
`mcp_<server>_<tool>`, so a server-provided `map` can't shadow the
builtin. The bare word `map` is ambiguous in prose, so `map.md`
opens with "Persistent project index for codebase navigation" —
the model reads it as a tool description, not a verb.

Failure mode: if the walk itself fails (permissions, I/O), the
skeleton header says `index build failed: <err>` and points at
grep/glob — "corruption ⇒ rebuild" covers DB damage; walk failure
is surfaced, not silent.

Idempotent = **stub-safe**: the result always reflects the current
index, so the pruning system may supersede `map` results freely —
re-calling is a local query, not a re-gather.

Division of labor the tool description must state: `map` says
_where_, `view` reads before edit, `lsp_references`/
`lsp_call_hierarchy` give precise follow-ups.

### Ranking — shipped vs remaining

Shipped: `refs` in-degree centrality (files imported by many rank
highest) blended with git-churn (touches over recent history, one
`git log` pass per walk — `internal/index/churn.go`). Remaining:
session-hot files via filetracker, and hydrated hot-file
pre-ranking once `SESSION_KNOWLEDGE` lands.

## Field traps — updated from as-built review

Fixed during review (kept as documentation of the failure shapes):

- **Refresh must precede materialization.** `Subtree`/`Symbol`
  originally refreshed result paths _after_ collecting symbol rows —
  the DB updated but output rendered stale. Now: query candidate
  paths → `refreshPaths` → materialize.
- **Dir-scoped ref resolution.** `refreshIfStale`'s probe must
  answer directories (Go imports resolve to package dirs), or every
  lazy re-tag silently drops a Go file's outgoing refs and in-degree
  ranking degrades with each edit.
- **Single-connection rule applies to sidecars.** `OpenDBFile` sets
  `SetMaxOpenConns(1)` — the WAL-desync bug documented in
  `internal/db/connect.go` would otherwise recur for shared
  index.db.
- **Shared service, not per-agent handles.** `index.Shared` keeps
  one process-wide handle per working dir; per-agent `Open`s would
  accumulate unclosed connections.
- **Claims must match code.** `map.md` originally promised a build
  timestamp nothing rendered and "never silently stale" while new
  files were invisible; both statements now match behavior
  (timestamp rendered; new-file boundary stated).
- **Name tools by their real names.** `map.md` pointed the model at
  `references`/`definition`; the shipped names are `lsp_references`/
  `lsp_definition`. A nav tool that misnames its follow-ups
  manufactures failed calls.
- **Root path must resolve.** `map path=.` / `path=/` returned
  "No indexed files" — both now mean the project root. LIKE/GLOB
  metacharacters in `path=` and stored dir paths are matched
  literally (escaped LIKE, `instr`/`substr` prefix tests instead of
  GLOB).
- **Ranking must be deterministic.** The skeleton's in-degree join
  picked an arbitrary `deg.n` when a file matched several ref
  targets (GLOB `*` crosses `/`); it now SUMs file-level plus
  ancestor-dir refs.
- **Oversized files are recorded, not dropped.** The 256KB cap now
  skips tagging only — large files appear in the map without
  symbols. (The tagger comment had always claimed this; the walker
  actually excluded them.) Lazy re-tag also no longer drops a file
  on a transient read error, matching walk semantics.
- **Declaration regexes must prove declaration.** The JS/TS method
  rule matched any indented `ident(...)` — call sites tagged as
  definitions, the wrong-pointer failure this doc's constraints
  exist to prevent. The rule now requires `{` after the arg list
  (optional TS return type admitted, incl. `,`/space for generics);
  control-flow names stay filtered via `jsKeywords`.
- **Skeleton ranking can't join files × ref-targets.** The
  in-degree query nested-looped every file against every distinct
  `refs` dst with unindexable `substr` — O(F×D) on the single
  connection. Now: `GROUP BY dst_path` counts into a Go map, then
  each file sums its own plus ancestor-dir counts — O(F×depth).
- **fastwalk callbacks are concurrent.** `collect`'s
  `files = append(...)` raced across fastwalk worker goroutines —
  caught by a mid-build race test, not by review. The `files`
  slice is mutex-guarded; `FastGlobWalker`'s `csync` caches were
  already safe.
- **A miss during a build must not read as authoritative.**
  `Symbol`/`Subtree` empty results now carry an "index still
  building" suffix while `indexing` is set — the same
  wrong-pointer-class bug as rendering stale rows, one level up.
- **Import extraction needs block context.** Go string literals
  outside `import (...)` blocks produced candidate refs; the Go
  spec now gates import regexes on an in-block/`import`-line
  state. Rust `use`'s last segment is an _item_, not a file —
  `crate::a::b::Item` resolves by probing `src/a/b.rs` and
  `src/a/b/mod.rs`, which is where virtually all `use` refs live.
- **Write events must mirror the walk's filters.** `touchFile`
  re-checks `ShouldSkip` and drops the row for dirs/empty/ignored
  paths (a file truncated to zero bytes loses its stale symbols
  immediately, matching reconcile). And it no-ops when
  `index.db` doesn't exist — with `map` disabled via
  `disabled_tools`, writes must not create the sidecar; the first
  `map` call's walk covers everything written meanwhile.
- **`impl Trait for Type` names the implementor.** Capturing the
  trait made `map symbol=Foo` miss its impls and `symbol=Display`
  point at every impl site. A `for`-clause rule now runs first.
- **Python imports are relative to the file.** `from .sibling
import x` resolves against the file's dir (dots walk up), and
  absolute imports also probe `src/` — without both, refs stay
  flat on most real Python repos.
- **Exported checks need word boundaries.** Substring `lineHas`
  marked `publish()`/`reexport()` exported; `\b` matching fixes
  the marker and skeleton ordering.
- **Roots can be symlinked.** `touchFile` retries the root check
  with `EvalSymlinks` on both sides (parent-dir fallback for
  deleted files) — a canonicalized LSP path under a symlinked
  workingDir would otherwise skip notifications silently.
- **Process-lifetime caches are the accepted staleness class.**
  `modulePath` (go.mod) and `skipWalker` (ignore rules) are
  `sync.Once` caches — a mid-session edit to either isn't seen by
  `touchFile` until restart, but every re-walk reparses from disk
  so the index self-corrects within `rewalkInterval`.
- **Reconcile must stat before it drops.** A `NotifyWritten` row
  can commit while a background walk is in flight — absent from
  `seen` (its dir was collected pre-write) but live on disk.
  Dropping on `stored \ seen` alone erases the row until the next
  write or re-walk; reconcile now stats the candidate first,
  keeping the same keep-stale-over-erase-live rule as everywhere
  else. Broadening this accepts: a file that _becomes_ ignored
  mid-session (`.gitignore` edit) or externally truncated to zero
  bytes also keeps its row until deleted — for a nav index,
  pointing at a live file isn't a wrong pointer.
- **Refs resolved against a partial index don't self-heal.**
  Files lazily tagged mid-build probed `exists` against
  committed-so-far rows — a not-yet-indexed import target dropped
  the ref, and matching `{mtime,size}` meant it stayed dropped
  forever. Lazy tags now record a `refix` set when `indexing` is
  on, and the walk re-resolves those files against the complete
  `seen` set at build end.
- **"Built" means last walk, not last write.** `MAX(indexed_at)`
  is bumped by every lazy re-tag — a skeleton rendered after a
  session of writes would claim the layout is minutes old. The
  header renders `lastBuild` (process-local) with `indexed_at`
  only as the cross-restart fallback.
- **Referrers refresh too.** `symbol=` refreshed def paths but
  printed referrer files straight from `refs` — a file that
  dropped the import externally still listed. Candidates are
  refreshed then re-queried.
- **Store the stat the tagger actually reads.** `collect` used
  lstat pairs for symlinks while `tagFile` reads the target —
  pairs never matched, so symlinked files re-tagged forever and
  target edits went unseen. Symlinks now stat through.
- **The data dir is only auto-excluded at `.crush`.** A
  `data-directory` configured inside the project (`./.data/`)
  would self-index `index.db` and fetch scratch; `collect` skips
  any in-root `dataDir` explicitly.
- **Failed tags get one retry per content.** An unreadable file
  kept its stale row AND paid a failed open+scan on every query;
  `tagFails` remembers the `{mtime,size}` pair and skips until
  the file changes.

Resolved since the first draft:

- **First `map` call no longer blocks.** The walk runs in a
  background goroutine; the tag loop commits every `walkBatchSize`
  (200) files or `walkBatchMax` (500ms), whichever comes first, so
  queries serve committed-so-far rows while indexing continues and
  a slow batch can't hold the single connection. The skeleton
  header reports "indexing in progress, N files so far" until the
  build completes, and "index build failed" if it did.
  `EnsureIndexed` remains as the blocking API for tests.
- **Re-walks recur on a freshness bound.** Once a build completes,
  the next query after `rewalkInterval` (5m) triggers another
  background walk — incremental, so a quiet tree costs only the
  stat pass. Externally-created files are therefore invisible for
  at most ~5 minutes, not until restart. Each build is bounded by
  `walkTimeout` (10m) — a pathological tree or network FS surfaces
  as `buildErr` instead of hanging the indexer. A failed build
  doesn't suppress retries for the full interval: after a walk
  error the freshness window shrinks to `failRetryInterval` (30s),
  so transient errors retry quickly while persistent ones aren't
  hammered per query.
- **Files created mid-session are covered by write events.**
  `index.NotifyWritten(absPath)` is called on the success path of
  every file-mutating tool (`edit`, `write`, `multiedit`,
  `download`, `lsp_rename` per affected file,
  `lsp_replace_symbol`); `refreshIfStale` tags never-indexed paths,
  so a new file lands in the index on its first write. `touchFile`
  skips when `index.db` doesn't exist, so `index.db` is created by
  the first `map` call — not by writes, and not at all when `map`
  is disabled via `disabled_tools`. Boundary:
  `bash` redirection and external edits rely on the dirty-scan
  fallback / the 5-minute re-walk — same gap class as the stub
  system's observed-mutation pass.
- **`project_index` defaults off** — matches every other
  experimental option; the first-run walk stays an opt-in cost until
  the eval arm justifies flipping. Enabled via
  `option project-index true` in crushrc (added to `optionSpecs`)
  or `project_index: true` in JSON.

Standing constraints (not TODOs — invariants the code must keep):

- **A stale map the model trusts is worse than none.** Shipped:
  the skeleton renders its build timestamp; queried paths are
  re-tagged before render. Per-file stale-marking in output is
  deliberately absent — the lazy refresh means rendered paths are
  fresh; only files _missing_ from the map can be stale by
  omission, and the build timestamp + `map.md` fallback guidance
  carry that caveat.
- **Skeletons lie by omission** — rank conservatively; a missing
  pointer is fine, a wrong one is not.
- **index.db is never authoritative** — corruption or missing
  tables ⇒ rebuild, not error.

Still open:

- **gofumpt** wasn't on PATH during implementation; files are
  gofmt-clean — run `task fmt` before merge.

## Extensions — from orientation toward solving

v1 answers "where". These additions let the map answer the questions
the execution phase of a complex task actually asks — all are
schema-additive or query-only on the existing store (index.db is
rebuildable, so added columns are a re-tag, not a migration):

- **`parent` + `sig` columns on `symbols`.** The Go rule already
  matches `func (r *T) name` but discards the receiver — capture it
  (`parent = "T"`), and record the declaration line text as `sig`.
  Then `map symbol=Ready` renders `method on *Service`, and
  `map symbol=Service` can list its methods. "Which type owns this"
  is the most common follow-up on complex tasks, and the data is
  already in the scanned line. Same for Rust `impl` blocks and
  enclosing JS/Java classes (line ranges make "enclosing type"
  derivable).
- **`map impact=<path>` — transitive reverse refs.** `refs` is
  already a file→file graph; a recursive CTE answers "what breaks
  if I change this file" — the dominant question during refactors.
  ~15 lines of SQL, no new indexing. `map symbol=` can also append
  blast-radius ("transitively reachable from N files").
- **Fuzzy `symbol=` fallback.** Exact `name = ?` misses on
  half-remembered names — precisely when the model reaches for the
  tool — and the miss path still triggers `refreshDirty`
  (rate-limited, so bounded but not free). On miss, fall back to
  `name LIKE ? || '%'` then substring, before reporting nothing.
- ~~**Coder prompt hint.**~~ **Done.** One line in `coder.md.tpl`
  (gated on `project_index` so flag-off prompts are unchanged) —
  "call `map` first on unfamiliar multi-file tasks" — addresses
  pull-only adoption without committing to always-injected skeleton.

Consumers that live in other plans: serving `map` slices inside
edge-retry prompts and binding `PlanItem.Evidence` to index paths
are edge-layer changes — tracked in `HARNESS_TOPOLOGY.md`. Per-file
generated summaries (top-N only, small-model slot) are the cheap
semantic step — tracked in `SEMANTIC_INDEX.md`, which already names
it as the alternative to evaluate before chunk embeddings.

## Non-goals

- No embeddings/vector search — that layer is specced in
  `SEMANTIC_INDEX.md`; here the tables are only reserved.
- No always-injected skeleton section — pull via tool first;
  revisit only if eval shows models won't call `map` unprompted.
- No replacement of `view`-before-edit; the map is navigation, not
  content.
- No cross-project index — `.crush/` per project is the boundary.

## Measurement

`EVAL_HARNESS` arm: same task corpus, map enabled vs not. Metrics:

- **`map` call rate** — did the model reach for the tool at all,
  unprompted? This is the direct input to the pull-vs-inject
  decision (Non-goals defers always-injected skeletons); near-zero
  adoption in the treatment arm means the arm measured non-use,
  not uselessness.
- **Discovery-class tool calls** (grep/glob/view-on-unseen-file)
  before first write — the primary signal.
- **Requests to first edit** and re-read rate during execution —
  the regression this targets.
- **`map`-call token cost** — total input tokens spent on map calls
  themselves. The "token tax" the arm guards against should be
  measured, not just avoided.
- **Wrong-pointer rate** — fraction of `map` pointers (def sites,
  referrer files) the agent followed that led to a `view` of the
  wrong file (immediately abandoned) or a grep re-search. The
  "wrong pointer is worse than a missing one" principle needs a
  measurable proxy, not just a design rule.

A map that doesn't reduce discovery calls is a token tax, not a
feature.

## PR ordering — status

1. ~~`internal/index/`: store + schema (continuation tables),
   walker, regex tagger, mtime+size invalidation, ranked skeleton
   render~~ — **done** (`internal/index/`, `db.OpenDBFile`).
2. ~~`map` tool + `map.md` + coordinator registration behind
   `project_index`; `allToolNames` + `readOnlyTools` entries~~ —
   **done**.
3. ~~Background first-build + partial results; `project_index`
   default (off); new-file visibility (write-event retag);
   tool-level test~~ — **done**.
4. Extensions (`parent`/`sig` columns, `map impact=`, fuzzy
   `symbol=` fallback, coder prompt hint) — see Extensions section.
5. ~~Git-churn ranking~~ — **done** (`churn.go`, one `git log` pass
   per walk). ~~Coder prompt hint~~ — **done**, gated on the flag so
   flag-off prompts are unchanged. Remaining polish: LSP
   `documentSymbol` enrichment when the server is already running.
6. ~~Eval arm~~ — **done**: `eval/experiments/project-index.json` over
   the `realrepo-*` corpus slice; run it, then decide
   skeleton-injection and the semantic continuation
   (`SEMANTIC_INDEX.md`) from data.
