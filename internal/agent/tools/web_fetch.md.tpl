Fetch a web URL and return content as markdown; for use inside sub-agents. Large pages (>50KB) are saved to a temp file for grep/view.

<usage>
- Provide a URL to fetch
- The tool fetches the content and returns it as markdown
- Use this when you need to follow links from the current page
- After fetching, analyze the content to answer the user's question
</usage>

<features>
- Automatically converts HTML to markdown for easier analysis
- For large pages (>50KB), saves content to a temporary file and provides the path
- You can then use grep/view tools to search through the file
- Handles UTF-8 content validation
</features>

<limitations>
- Max response size: 5MB
- Only supports HTTP and HTTPS protocols
- Cannot handle authentication or cookies
- Some websites may block automated requests
{{- if .GhAvailable }}
- For GitHub content when an exact repo, issue, or PR link is provided, use `gh` CLI in bash instead
{{- end }}
</limitations>

<tips>
- For large pages saved to files, use grep to find relevant sections first
- Don't fetch unnecessary pages - only when needed to answer the question
- Focus on extracting specific information from the fetched content
</tips>
