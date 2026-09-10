# recall

Search and retrieve full details from previous events that were compacted
into the notebook. Each entry covers one specific event (file read, file
edit, command, decision).

Usage:
recall("file:auth.go") — retrieve all events that touched auth.go
recall("turn:5") — retrieve all events from turn 5
recall("command") — retrieve all command events
recall("decision") — retrieve all decision events
recall("auth.go") — fuzzy text search for auth.go
recall("cross:auth.go") — search across sessions via mem0 (requires sync enabled)

Returns: full notebook entries with all preserved details.
Each entry is about ONE event — no noise from unrelated events.

Do NOT re-read files with notebook entries — use recall first.
It is 16x cheaper than re-reading the file.
