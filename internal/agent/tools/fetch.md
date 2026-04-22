Fetch raw content from a URL as text, markdown, or html (max 5MB); no AI processing. Optional `jq` parameter filters JSON responses server-side — use it for counting, extracting, or aggregating API data instead of loading the full payload. For analysis or extraction of prose/HTML use agentic_fetch.

<when_to_use>
Use this tool when you need:
- Raw, unprocessed content from a URL
- Direct access to API responses or JSON data
- HTML/text/markdown content without interpretation
- Simple, fast content retrieval without analysis
- To save tokens by avoiding AI processing
- To count, sum, or extract fields from a JSON API response (use the `jq` parameter)

DO NOT use this tool when you need to:
- Extract specific information from a webpage (use agentic_fetch instead)
- Answer questions about web content (use agentic_fetch instead)
- Analyze or summarize web pages (use agentic_fetch instead)
</when_to_use>

<usage>
- Provide URL to fetch content from
- Specify desired output format (text, markdown, or html) — optional when `jq` is set
- Optional timeout for request
- Optional `jq` expression to filter JSON responses. When set, the body is parsed as JSON and the expression is applied server-side; `format` is ignored. Examples:
  - `jq: "length"` — count items in a top-level array
  - `jq: "[.[].name]"` — extract names from an array of objects
  - `jq: "[.[].models | length] | add"` — sum nested array lengths
  - `jq: ".data | keys"` — list keys of a nested object

If a jq filter fails because it assumed the wrong shape, the error message
will include an `(input shape: ...)` hint describing the actual top-level
structure (e.g. `array of 32 items; first item is object with keys: id,
name, models`). Use that hint to fix the filter — do NOT fall back to
fetching the raw payload.

When fetching a large JSON response without a `jq` filter, the tool
appends a trailing `[crush-hint: ...]` banner suggesting you re-issue
the call with a `jq` expression. Heed it — dumping big JSON into context
causes context-overflow errors on many providers. The banner is at the
end of the body so that parsing from the start still works until it.
</usage>

<features>
- Supports three output formats: text, markdown, html
- Auto-handles HTTP redirects
- Fast and lightweight - no AI processing
- Sets reasonable timeouts to prevent hanging
- Validates input parameters before requests
</features>

<limitations>
- Max response size: 5MB
- Only supports HTTP and HTTPS protocols
- Cannot handle authentication or cookies
- Some websites may block automated requests
- Returns raw content only - no analysis or extraction
</limitations>

<tips>
- Use text format for plain text content or simple API responses
- Use markdown format for content that should be rendered with formatting
- Use html format when you need raw HTML structure
- Set appropriate timeouts for potentially slow websites
- If the user asks to analyze or extract from a page, use agentic_fetch instead
</tips>
