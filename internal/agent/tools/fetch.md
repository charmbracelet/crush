Fetches raw content from URL and returns it in specified format without any AI processing.

<when_to_use>
Use this tool when you need:
- Raw, unprocessed content from a URL
- Direct access to API responses or JSON data
- HTML/text/markdown content without interpretation
- Simple, fast content retrieval without analysis
- To save tokens by avoiding AI processing

DO NOT use this tool when you need to:
- Extract specific information from a webpage (use agentic_fetch instead)
- Answer questions about web content (use agentic_fetch instead)
- Analyze or summarize web pages (use agentic_fetch instead)
</when_to_use>

<usage>
- Provide URL to fetch content from
- Specify desired output format (text, markdown, or html)
- Optional timeout for request
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
