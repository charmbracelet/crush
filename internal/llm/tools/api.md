# API Interaction Tool (api)

The `api` tool allows you to make HTTP requests to APIs with configurable headers, body content, and authentication. It supports all standard HTTP methods and provides flexible response formatting.

## Usage

```json
{
  "url": "https://api.example.com/users",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer token123"
  },
  "body": {
    "name": "John Doe",
    "email": "john@example.com"
  },
  "format": "json"
}
```

## Parameters

- `url` (required): The URL to make the request to
- `method` (optional): The HTTP method to use (default: GET)
  - `GET`: Retrieve data
  - `POST`: Create new resources
  - `PUT`: Update existing resources
  - `DELETE`: Remove resources
  - `PATCH`: Partial updates
  - `HEAD`: Retrieve headers only
  - `OPTIONS`: Retrieve communication options
- `headers` (optional): HTTP headers to include in the request
- `body` (optional): The request body (for POST, PUT, etc.)
- `timeout` (optional): Timeout in seconds (max 120, default: 30)
- `format` (optional): The format to return the response in (default: json)
  - `json`: Pretty-printed JSON
  - `text`: Plain text
  - `raw`: Raw response

## Examples

### GET request with headers

```json
{
  "url": "https://api.github.com/user",
  "method": "GET",
  "headers": {
    "Authorization": "token ghp_xyz123",
    "User-Agent": "blush-agent"
  },
  "format": "json"
}
```

### POST request with JSON body

```json
{
  "url": "https://jsonplaceholder.typicode.com/posts",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json"
  },
  "body": {
    "title": "My Post",
    "body": "This is the post content",
    "userId": 1
  },
  "format": "json"
}
```

### PUT request to update a resource

```json
{
  "url": "https://jsonplaceholder.typicode.com/posts/1",
  "method": "PUT",
  "headers": {
    "Content-Type": "application/json"
  },
  "body": {
    "id": 1,
    "title": "Updated Post",
    "body": "This is the updated content",
    "userId": 1
  },
  "format": "json"
}
```

### DELETE request

```json
{
  "url": "https://jsonplaceholder.typicode.com/posts/1",
  "method": "DELETE",
  "format": "json"
}
```

## Response Format

The tool returns the response in the specified format along with metadata:
- Status code
- Response headers
- Formatted content based on the requested format

## Notes

- The tool automatically sets `Content-Type: application/json` for requests with a body if not explicitly provided
- The tool automatically sets `User-Agent: blush/1.0` if not explicitly provided
- Response size is limited to 5MB
- The tool handles JSON serialization/deserialization automatically
- For complex authentication flows, consider using the `bash` tool with `curl` or similar