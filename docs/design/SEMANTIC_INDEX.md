# Semantic Index — Chunk Embeddings & `map semantic=` Search

> **Status:** Spec. Continuation of `CONTEXT_PREFETCH.md` — that
> plan ships the per-project index store, the symbol spine, and
> the `map` tool this layer plugs into. Its schema already
> reserves the `chunks`/`embeddings` tables, so this is an
> additive query mode, not a rework. Do not start until the parent
> lands — the reserved schema is the contract.

## Goal

`map semantic="<query>"` answers natural-language questions —
"where is retry handled", "what owns locking" — that don't resolve
to a symbol name. The symbol spine says _where_ when you know the
name; this layer covers the fuzzy half.

## Problem

A navigation index keyed on declarations can't answer conceptual
queries. Today the model falls back to iterative grep for these —
the same discovery roundtrips the parent plan exists to eliminate,
just triggered by a different query shape. The field pattern
(Sourcegraph SCIP + embeddings, Cursor) is precise index first,
fuzzy layer on top: embeddings complement the symbol spine, they
never replace it.

## Design

Same store (`<DataDirectory>/index.db`); the tables are already
created:

```sql
chunks     (path, start_line, end_line, text_hash)
embeddings (chunk_id PK, vector, model_id)
```

- **Chunk pass** — function-level where LSP symbols exist,
  line-window fallback. `text_hash` per chunk so only dirty
  chunks re-embed; the parent's `{mtime, size}` invalidation
  decides which files re-chunk.
- **Embeddings** via the configured provider API behind
  `options.semantic_index` — **opt-in**: local/offline users pay
  nothing, and code leaving the machine is a consent decision,
  not a default.
- **`map semantic="<query>"`** becomes a real query mode — today
  it returns a guidance message pointing at `symbol=`/`path=`.
- **Cheap alternative to evaluate first:** generated one-line
  summaries for the top-N files by refs in-degree (~50), stored as
  `files.summary`, via the existing small-model + mem0 path. The
  skeleton then renders `path (N refs) — "what this file is"`,
  capturing most of the fuzzy-orientation value on the ranking
  layer the parent already built. Try it before committing to a
  chunk-embedding pipeline.

## Non-goals

- No replacement of the symbol spine or ranked skeleton —
  `semantic:` is one more `map` mode, not a new front end.
- Nothing embedded for users who don't opt in; no default-on
  cost, no network calls behind a default.
- No new store or per-project boundary change — the parent's
  sidecar `index.db` remains the boundary.

## Measurement

Same `EVAL_HARNESS` arm as the parent, second axis: map vs
map+semantic. Discovery calls to first edit should drop further
than the skeleton alone achieves, and `semantic:` hits should
convert into targeted `view`s — if they don't, it's a cost
center, not a feature. Track re-embed volume too: `text_hash`
hit rate on repeat sessions is the cost control.
