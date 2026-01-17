Fetch web content (for sub-agents). Converts HTML to markdown.

<usage>
- Provide URL to fetch
- Returns content as markdown
- Use when following links during research
</usage>

<behavior>
- Converts HTML to markdown for analysis
- Large pages (>50KB) saved to temp file
- Use grep/view on temp files for large content
</behavior>

<limits>
- Max: 5MB
- HTTP/HTTPS only
- No auth/cookies
</limits>

<tip>
Only fetch pages needed to answer the question. Don't fetch unnecessarily.
</tip>
