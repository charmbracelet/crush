Downloads binary data from a URL and saves it to a local file.

WHEN TO USE THIS TOOL:

- Use when you need to download files, images, or other binary data from URLs
- Helpful for downloading assets, documents, or any file type
- Useful for saving remote content locally for processing or storage

HOW TO USE:

- Provide the URL to download from
- Specify the local file path where the content should be saved
- Optionally set a timeout for the request

FEATURES:

- Downloads any file type (binary or text)
- Automatically creates parent directories if they don't exist
- Handles large files efficiently with streaming
- Sets reasonable timeouts to prevent hanging
- Validates input parameters before making requests

LIMITATIONS:

- Maximum file size is 100MB
- Only supports HTTP and HTTPS protocols
- Cannot handle authentication or cookies
- Some websites may block automated requests
- Will overwrite existing files without warning

TIPS:

- Use absolute paths or paths relative to the working directory
- Set appropriate timeouts for large files or slow connections
