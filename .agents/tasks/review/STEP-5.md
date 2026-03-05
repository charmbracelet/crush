# Add trailing newline to `new_session.md`

Status: COMPLETED

## Sub tasks

1. [x] Verify file lacks trailing newline
2. [x] Add trailing newline to `internal/agent/tools/new_session.md`
3. [x] Verify fix with `od -c`

## NOTES

File ended with `</notes>` and no `\n`. Added trailing newline. Confirmed with `od -c` showing `> \n` at end.
