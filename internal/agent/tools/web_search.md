Search the web via the configured default search engine; returns titles, URLs, and snippets. Follow up with web_fetch to get full page content.

<usage>
- Provide a search query to find information on the web
- Returns a list of search results with titles, URLs, and snippets
- Use this to find relevant web pages, then use web_fetch to get full content
- Uses DuckDuckGo by default unless a different default search engine is configured
</usage>

<parameters>
- query: The search query string (required)
- max_results: Maximum number of results to return (default: 10, max: 20)
- search_engine: Optional search engine override for this search (duckduckgo or kagi)
</parameters>

<tips>
- Use specific, targeted search queries for better results
- After getting results, use web_fetch to get the full content of relevant pages
- Combine multiple searches to gather comprehensive information
</tips>
