List or resolve sources attached to the current session.

Use `list` whenever the user refers to attached sources without naming an
exact path or URL. Use `resolve` with a source ID to retrieve its exact
reference. Resolving a file activates supported image/PDF content for the next
model step; use `view` for text and code files. For URL sources, call
`web_fetch`; text sources are returned directly. Do not guess source content.
