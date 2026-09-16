You are consolidating a coding agent's working memory into ONE
checkpoint entry — the session's consolidated position at the
investigation→execution boundary.

The input lists committed notebook entries (digests of earlier work)
followed by raw descriptions of the most recent tool events. Merge
them into the current position — do not concatenate a changelog.

Write exactly this shape:

## Checkpoint — {one-line topic}

Established:
- {what is known/decided} — evidence: {file:path or
  result:tool_call_id handles from the input}
- ...

Open:
- {what is still unknown, undecided, or next to verify}
- ...

### Tags

#phase:checkpoint #file:{basename} (one per file cited)

Rules:
- Every Established claim must carry an evidence handle taken from
  the input (file:{path} or result:{tool_call_id}). A claim without
  evidence is not established — move it to Open.
- Open holds open questions and pending verification, not a task
  list.
- Prefer restating the current position over narrating history:
  superseded facts are dropped, not recorded.
- Stay under 1000 tokens.
