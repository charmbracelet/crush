Persistent project index for codebase navigation — answers "where does X live" without grep roundtrips.

Backed by a per-project index (files, top-level symbols, references) stored on disk and shared across sessions. It survives compaction and new sessions; results reflect the current index, so re-calling is cheap and safe. The skeleton header reports when the index was built; files edited since indexing are re-indexed on access.

Modes:

- `map` (no args): ranked skeleton — directory layout + most-referenced files with their exported symbols. **Call this first on any unfamiliar or multi-file task** to orient before reading files.
- `map path=<dir>`: files and top-level symbols under a directory.
- `map symbol=<name>`: definition sites (file:line) and files that reference them.
- `map semantic=<query>`: reserved for a future semantic index — not yet implemented; use `symbol=`, `path=`, `grep`, or `glob`.

Division of labor:

- `map` says **where** — which files and symbols matter.
- `view` reads the actual code — always read a file before editing it; the map is navigation, not content.
- `lsp_references`, `lsp_definition`, `lsp_call_hierarchy` give precise answers once you know the file.

Limitations: the index tracks top-level declarations only (it may miss deeply nested or generated symbols), and it refreshes incrementally — files you create or edit through tools appear immediately, but files created outside the tools (bash redirection, your own edits) may take up to a few minutes to appear; fall back to glob/grep when you suspect something is missing.
