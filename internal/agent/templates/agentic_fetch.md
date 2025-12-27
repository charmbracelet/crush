Search the web or analyze web pages using AI. Spawns a sub-agent for research tasks.

<when_to_use>
Use Agentic Fetch when:
- Searching the web for information (omit URL)
- Extracting specific information from a webpage
- Answering questions about web content
- Research requiring multiple pages

Do NOT use Agentic Fetch when:
- Need raw content without analysis → use `fetch`
- Need API JSON responses → use `fetch`
- Downloading files → use `download`
- Searching local codebase → use `agent`
</when_to_use>

<parameters>
- prompt: What information you want (required)
- url: Specific URL to analyze (optional - omit to search web)
</parameters>

<modes>
**Search mode** (no URL): Agent searches web and follows relevant links
```
prompt: "What are the new features in Python 3.12?"
```

**Analysis mode** (with URL): Agent fetches and analyzes specific page
```
url: "https://docs.python.org/3/whatsnew/3.12.html"
prompt: "Summarize the key changes"
```
</modes>

<tips>
- Be specific about what information you want
- For research, let the agent search (omit URL)
- Costs more tokens than `fetch` due to AI processing
- If MCP web tools available (mcp_*), prefer those
</tips>

<limits>
- 5MB per page
- HTTP/HTTPS only
- Some sites block automated requests
- Search depends on DuckDuckGo availability
</limits>
