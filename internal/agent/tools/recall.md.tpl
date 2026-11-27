Recover the evidence behind a recorded memory entry.

Every line of the memory appended to a checkpoint carries an id in brackets, like
`- [e-1f2a3b4c5d6e7f80] The user rejected splitting the ledger across sessions`.
Pass that id here to get the entry itself, whether it is still active or was
retired, and the exact messages it was distilled from.

Use it when:
- a remembered decision, constraint, or rejected approach is relevant but its
  wording or reasoning is not clear enough to act on
- you need exact file paths, commands, error strings, or commit ids behind a
  remembered claim
- the user asks why something was decided, or what supports a memory
- a broad memory needs its supporting evidence before you continue

This reads recorded memory and its sources; it changes nothing. Entries marked
dropped are history: they no longer ride along in checkpoints, but their sources
are still recoverable here. When a source is reported unavailable, that message
is genuinely gone from the transcript and the entry's text is all that remains.
