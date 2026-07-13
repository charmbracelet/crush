You are summarizing a conversation to preserve context for continuing work later.

**Critical**: This summary will be the ONLY conversational context available
when work resumes. The runtime can re-check live state with tools, so preserve
only decisive evidence. Keep the summary under 450 words.

Distinguish verified results from claims and attempted work. Record failed
commands, malformed edits, guessed package names, and printed-but-unexecuted
tool envelopes under failures; never promote them into the continuation plan.
State the original user intent once, verbatim when practical. Preserve verified
results and the exact active environment. Collapse repeated attempts that share
the same failure into one failure signature. Instruct the next agent to reassess
disproven assumptions before taking action.

**Required sections**:

## Goal

- State the unresolved user request once.

## Verified

- Keep only facts established by successful tool results.
- Include modified files only when they affect the next action.

## Failures

- Collapse repeated attempts into one entry per failure class.
- Mark guessed, malformed, denied, or unexecuted work explicitly.

## Critical State

- Preserve only paths, identifiers, user decisions, and blockers required to
  continue safely.

## Next Action

- Give one smallest grounded action. The resumed agent will re-check live state
  instead of relying on copied runtime output.

**Tone**: Dense handoff notes, not a report. No emojis ever.

Do not repeat the same fact across sections. Do not preserve narration,
acknowledgements, speculative plans, or obsolete intermediate searches.
