Read stdout/stderr from a background job by ID, without waiting for it to finish.

<usage>
- `shell_id` (required): the ID reported when the job was backgrounded.
- `offset`: byte offset to start reading from. Pass the `next_offset` value
  returned by the previous call to get only what has been printed since.
  Defaults to 0, which returns the whole transcript.
- `wait`: block until the job finishes before reading. Defaults to false.
</usage>

<features>
- Returns immediately by default, so you can check on a long-running job and
  keep working.
- Reports `Status: running` or `Status: completed`, plus the exit code when a
  finished job failed.
- Output larger than the display budget is elided in the middle, and the full
  transcript path is included so you can grep or view the whole thing.
</features>

<tips>
- Poll with `offset` rather than re-reading from 0. Watching a chatty build
  otherwise costs you its entire history on every check.
- You do not need to poll just to learn that a job finished: a completed job
  reports back on its own.
- Use `wait: true` only when you genuinely cannot proceed without the result.
- Lost the ID? Use `job_list`.
</tips>
