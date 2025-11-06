Downloads binary data from URL and saves to local file.

<usage>
- Provide URL to download from
- Specify local file path where content should be saved
- Optional timeout for request
</usage>

<features>
- Downloads any file type (binary or text)
- Auto-creates parent directories if missing
- Handles large files efficiently with streaming
- Sets reasonable timeouts to prevent hanging
- Validates input parameters before requests
</features>

<limitations>
- Max file size: 100MB
- Only supports HTTP and HTTPS protocols
- Cannot handle authentication or cookies
- Some websites may block automated requests
- Will overwrite existing files without warning
</limitations>

<tips>
- Use absolute paths or paths relative to working directory
- Set appropriate timeouts for large files or slow connections
</tips>
