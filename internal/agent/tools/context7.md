Get up-to-date documentation for any library, framework, SDK, or CLI tool from context7.com. Use this whenever the user asks about a third-party library — even ones you think you know — because your training data may not reflect recent changes.

Pass the library name (e.g. "react", "Next.js", "Prisma") and a specific question. The tool resolves the name to a context7 library ID and queries the docs in one call. To pin a version, pass `library_id` directly in the format `/owner/repo` or `/owner/repo/v1.2.3`.

Do not use for: refactoring, writing scripts from scratch, debugging business logic, code review, or general programming concepts. Set `CONTEXT7_API_KEY` (starts with `ctx7sk-`) for higher rate limits; works without it.
