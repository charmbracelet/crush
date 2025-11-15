Search the web using various search engines.

<when_to_use>
Use this tool when you need to:
- Find current information beyond your knowledge cutoff
- Research documentation, libraries, or APIs
- Verify information that may have changed since your training
- Get up-to-date statistics, news, or examples
- Find solutions to current issues or bugs

DO NOT use this tool when:
- The information is well-known and in your training data
- You need to access a specific, known URL (use fetch instead)
- You need to download or access content from a URL

Examples of good use cases:
- "What's the latest version of React?"
- "Find examples of how to use context.WithTimeout in Go"
- "What are the current best practices for error handling in TypeScript?"

Examples of when NOT to use:
- "What is 2+2?" (use your knowledge)
- "Read https://example.com/docs" (use fetch instead)
- "Download a file from example.com" (use download instead)
</when_to_use>

<usage>
Provide:
- query: Your search query (required)
- provider: Search engine - "duckduckgo", "brave", or "google" (default: duckduckgo)
- max_results: Number of results to return, 1-20 (default: 10)
- site: Optional domain restriction (e.g., "docs.python.org")

Example queries:
- "Golang context timeout examples"
- "Python list comprehension site:docs.python.org"
- "JavaScript async await error handling"
</usage>

<features>
- DuckDuckGo: Works without API keys (default and recommended)
- Brave: Alternative search engine option
- Google: Requires API key configuration
- Domain filtering with site: parameter
- Rate limiting to avoid overwhelming services
- Structured results with titles, URLs, and snippets
</features>

<limitations>
- DuckDuckGo may occasionally fail due to rate limiting or HTML changes
- Brave uses HTML scraping and may be less reliable than API-based search
- Google requires API key setup
- Requires internet connection
- Cannot search behind paywalls or authentication
- Results limited to publicly available information
</limitations>

<tips>
- Be specific in your query for better results
- Use the site: parameter to search within documentation
- Start with DuckDuckGo (default) - it doesn't require API keys
- For technical questions, include language or framework names
- Check the URLs in results - prefer official documentation sources
- Try different providers if one doesn't give good results