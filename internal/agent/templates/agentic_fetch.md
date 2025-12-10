Fetches content from a URL or searches the web, then processes it using an AI model to extract information or answer questions.

<when_to_use>
Use this tool when you need to:
- Search the web for information (omit the url parameter)
- Extract specific information from a webpage (provide a url)
- Answer questions about web content
- Summarize or analyze web pages
- Research topics by searching and following links

DO NOT use this tool when:
- You just need raw content without analysis (use fetch instead - faster and cheaper)
- You want direct access to API responses or JSON (use fetch instead)
- You don't need the content processed or interpreted (use fetch instead)
</when_to_use>

<usage>
- Provide a prompt describing what information you want to find or extract (required)
- Optionally provide a URL to fetch and analyze specific content
- If no URL is provided, the agent will search the web to find relevant information
- The tool spawns a sub-agent with web_search, web_fetch, and analysis tools
- Returns the agent's response about the content
</usage>

<parameters>
- prompt: What information you want to find or extract (required)
- url: The URL to fetch content from (optional - if not provided, agent will search the web)
</parameters>

<usage_notes>
- IMPORTANT: If an MCP-provided web fetch tool is available, prefer using that tool instead of this one, as it may have fewer restrictions. All MCP-provided tools start with "mcp_".
- When using URL mode: The URL must be a fully-formed valid URL. HTTP URLs will be automatically upgraded to HTTPS.
- When searching: Just provide the prompt describing what you want to find - the agent will search and fetch relevant pages.
- The sub-agent can perform multiple searches and fetch multiple pages to gather comprehensive information.
- This tool is read-only and does not modify any files.
- Results will be summarized if the content is very large.
- This tool uses AI processing and costs more tokens than the simple fetch tool.
</usage_notes>

<limitations>
- Max response size: 5MB per page
- Only supports HTTP and HTTPS protocols
- Cannot handle authentication or cookies
- Some websites may block automated requests
- Uses additional tokens for AI processing
- Search results depend on DuckDuckGo availability
</limitations>

<tips>
- Be specific in your prompt about what information you want
- For research tasks, omit the URL and let the agent search and follow relevant links
- For complex pages, ask the agent to focus on specific sections
- The agent has access to web_search, web_fetch, grep, and view tools
- If you just need raw content, use the fetch tool instead to save tokens
</tips>

<examples>
Search for information:
- prompt: "What are the main new features in the latest Python release?"

Fetch and analyze a URL:
- url: "https://docs.python.org/3/whatsnew/3.12.html"
- prompt: "Summarize the key changes in Python 3.12"
</examples>
