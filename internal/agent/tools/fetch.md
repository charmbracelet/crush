Fetch raw content from a URL as text, markdown, or html (max 100KB); no AI processing. For analysis or extraction use agentic_fetch.

<parameters>
- url: URL to fetch (required)
- format: text | markdown | html (required)
- timeout: Optional seconds (max 120)
</parameters>

Use text for plain text/API responses, markdown for rendered content, html for raw structure. Not for analysis — use agentic_fetch for that.
