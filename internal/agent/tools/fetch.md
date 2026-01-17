Fetch raw content from a URL. Fast and lightweight - no AI processing.

<when_to_use>
Use Fetch when:
- Need raw HTML, JSON, or text from a URL
- Accessing API endpoints directly
- Want content without interpretation
- Saving tokens (no AI processing)

Do NOT use Fetch when:
- Need to extract specific information → use `agentic_fetch`
- Need to analyze or summarize content → use `agentic_fetch`
- Want to search the web → use `agentic_fetch` without URL
- Downloading binary files → use `download`
</when_to_use>

<parameters>
- url: URL to fetch (required)
- format: "text", "markdown", or "html" (required)
- timeout: Seconds to wait (optional, max 120)
</parameters>

<format_guide>
- `text`: Plain text, best for APIs or simple content
- `markdown`: Converted from HTML, good for documentation
- `html`: Raw HTML structure
</format_guide>

<limits>
- Max response: 5MB
- HTTP/HTTPS only
- No authentication or cookies
- Some sites block automated requests
</limits>

<example>
Fetch API response:
```
url: "https://api.github.com/repos/owner/repo"
format: "text"
```

Fetch documentation as markdown:
```
url: "https://docs.example.com/api"
format: "markdown"
```
</example>
