Fetches content from URL and returns it in specified format.

<usage>
- Provide URL to fetch content from
- Specify desired output format (text, markdown, or html)
- Optional timeout for request
</usage>

<features>
- Supports three output formats: text, markdown, html
- Auto-handles HTTP redirects
- Sets reasonable timeouts to prevent hanging
- Validates input parameters before requests
</features>

<limitations>
- Max response size: 5MB
- Only supports HTTP and HTTPS protocols
- Cannot handle authentication or cookies
- Some websites may block automated requests
</limitations>

<tips>
- Use text format for plain text content or simple API responses
- Use markdown format for content that should be rendered with formatting
- Use html format when you need raw HTML structure
- Set appropriate timeouts for potentially slow websites
</tips>
