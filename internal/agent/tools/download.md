Download binary files from URL to local disk.

<when_to_use>
Use Download when:
- Downloading images, PDFs, archives, binaries
- Saving files to disk (not just viewing content)
- Need the actual file, not just its content

Do NOT use Download when:
- Just reading web content → use `fetch`
- Need to analyze content → use `agentic_fetch`
- Downloading text/HTML to view → use `fetch`
</when_to_use>

<parameters>
- url: URL to download from (required)
- file_path: Local path to save file (required)
- timeout: Seconds to wait (optional, max 600)
</parameters>

<behavior>
- Creates parent directories automatically
- Overwrites existing files without warning
- Streams large files efficiently
</behavior>

<limits>
- Max file size: 100MB
- HTTP/HTTPS only
- No authentication or cookies
</limits>

<example>
Download an image:
```
url: "https://example.com/logo.png"
file_path: "/project/assets/logo.png"
```
</example>
