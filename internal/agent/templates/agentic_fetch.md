Fetch a URL or search the web using an AI sub-agent that can extract, summarize, and answer questions. Slower and costlier than fetch; use fetch for raw content or API responses.

<when_to_use>
Use for searching the web or extracting information from web pages. For raw content without analysis, use fetch instead.
</when_to_use>

<usage>
- prompt: What information to find or extract (required)
- url: Optional URL to fetch; omit to search the web
</usage>

<parameters>
- prompt: What information you want to find or extract (required)
- url: The URL to fetch content from (optional - if not provided, agent will search the web)
</parameters>
