# notebook_search

List and search notebook entries for the current session. Returns titles
and tags only — use `recall` to get full content of any entry.

Usage:
  notebook_search()           — list all entries
  notebook_search("decision") — filter to decision entries
  notebook_search("file:auth.go") — filter to entries about auth.go
  notebook_search("command")  — filter to command entries
  notebook_search("turn:5")   — filter to entries from turn 5
