Fetches content from a specified URL and processes it using an AI model to extract information or answer questions.

<when_to_use>
Use this tool when you need to:
- Extract specific information from a webpage (e.g., "get pricing info")
- Answer questions about web content (e.g., "what does this article say about X?")
- Summarize or analyze web pages
- Find specific data within large pages
- Interpret or process web content with AI

DO NOT use this tool when:
- You just need raw content without analysis (use fetch instead - faster and cheaper)
- You want direct access to API responses or JSON (use fetch instead)
- You don't need the content processed or interpreted (use fetch instead)
</when_to_use>

<usage>
- Takes a URL and a prompt as input
- Fetches the URL content, converts HTML to markdown
- Processes the content with the prompt using a small, fast model
- Returns the model's response about the content
- Use this tool when you need to retrieve and analyze web content
</usage>

<usage_notes>

- IMPORTANT: If an MCP-provided web fetch tool is available, prefer using that tool instead of this one, as it may have fewer restrictions. All MCP-provided tools start with "mcp_".
- The URL must be a fully-formed valid URL
- HTTP URLs will be automatically upgraded to HTTPS
- The prompt should describe what information you want to extract from the page
- This tool is read-only and does not modify any files
- Results will be summarized if the content is very large
- For very large pages, the content will be saved to a temporary file and the agent will have access to grep/view tools to analyze it
- When a URL redirects to a different host, the tool will inform you and provide the redirect URL. You should then make a new fetch request with the redirect URL to fetch the content.
- This tool uses AI processing and costs more tokens than the simple fetch tool
  </usage_notes>

<limitations>
- Max response size: 5MB
- Only supports HTTP and HTTPS protocols
- Cannot handle authentication or cookies
- Some websites may block automated requests
- Uses additional tokens for AI processing
</limitations>

<tips>
- Be specific in your prompt about what information you want to extract
- For complex pages, ask the agent to focus on specific sections
- The agent has access to grep and view tools when analyzing large pages
- If you just need raw content, use the fetch tool instead to save tokens
</tips>
