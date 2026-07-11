# btw — ephemeral side questions

Ask a quick "by the way" question about the current session without
touching its history.

## How to open

1. Open the command palette (`/` on an empty editor, or `ctrl+p`).
2. Type `btw` (aliases: `ask`, `question`, `sidequestion`) and press enter.
3. Type your question and press enter.

The answer appears in an overlay. Close with `esc` — nothing is saved
to the session or database.

## Follow-ups

From the answer view, press enter to ask another question. Prior Q/A
pairs in this open dialog are sent as context so the side model can
continue the side conversation. Closing the dialog discards them.

## Context rules

- The side model sees the full session conversation so far (after the
  last summary, if any), including tool calls and results.
- It has **no tools** and answers only from that context.
- It can run while the main agent is busy; it does not interrupt or
  queue against the main run.
- Token spend from btw is reported in the dialog footer only and is not
  added to the session cost.

## When it fails

If the session is too large for the available models, btw returns an
error. Run `/summarize` (palette: Summarize Session) and try again.
