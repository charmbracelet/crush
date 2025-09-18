Search code across public repositories using Sourcegraph's GraphQL API.

WHEN TO USE THIS TOOL:

- Use when you need to find code examples or implementations across public repositories
- Helpful for researching how others have solved similar problems
- Useful for discovering patterns and best practices in open source code

HOW TO USE:

- Provide a search query using Sourcegraph's query syntax
- Optionally specify the number of results to return (default: 10)
- Optionally set a timeout for the request

QUERY SYNTAX:

- Basic search: "fmt.Println" searches for exact matches
- File filters: "file:.go fmt.Println" limits to Go files
- Repository filters: "repo:^github\.com/golang/go$ fmt.Println" limits to specific repos
- Language filters: "lang:go fmt.Println" limits to Go code
- Boolean operators: "fmt.Println AND log.Fatal" for combined terms
- Regular expressions: "fmt\.(Print|Printf|Println)" for pattern matching
- Quoted strings: "\"exact phrase\"" for exact phrase matching
- Exclude filters: "-file:test" or "-repo:forks" to exclude matches

ADVANCED FILTERS:

- Repository filters:
  - "repo:name" - Match repositories with name containing "name"
  - "repo:^github\.com/org/repo$" - Exact repository match
  - "repo:org/repo@branch" - Search specific branch
  - "repo:org/repo rev:branch" - Alternative branch syntax
  - "-repo:name" - Exclude repositories
  - "fork:yes" or "fork:only" - Include or only show forks
  - "archived:yes" or "archived:only" - Include or only show archived repos
  - "visibility:public" or "visibility:private" - Filter by visibility

- File filters:
  - "file:\.js$" - Files with .js extension
  - "file:internal/" - Files in internal directory
  - "-file:test" - Exclude test files
  - "file:has.content(Copyright)" - Files containing "Copyright"
  - "file:has.contributor([email protected])" - Files with specific contributor

- Content filters:
  - "content:\"exact string\"" - Search for exact string
  - "-content:\"unwanted\"" - Exclude files with unwanted content
  - "case:yes" - Case-sensitive search

- Type filters:
  - "type:symbol" - Search for symbols (functions, classes, etc.)
  - "type:file" - Search file content only
  - "type:path" - Search filenames only
  - "type:diff" - Search code changes
  - "type:commit" - Search commit messages

- Commit/diff search:
  - "after:\"1 month ago\"" - Commits after date
  - "before:\"2023-01-01\"" - Commits before date
  - "author:name" - Commits by author
  - "message:\"fix bug\"" - Commits with message

- Result selection:
  - "select:repo" - Show only repository names
  - "select:file" - Show only file paths
  - "select:content" - Show only matching content
  - "select:symbol" - Show only matching symbols

- Result control:
  - "count:100" - Return up to 100 results
  - "count:all" - Return all results
  - "timeout:30s" - Set search timeout

EXAMPLES:

- "file:.go context.WithTimeout" - Find Go code using context.WithTimeout
- "lang:typescript useState type:symbol" - Find TypeScript React useState hooks
- "repo:^github\.com/kubernetes/kubernetes$ pod list type:file" - Find Kubernetes files related to pod listing
- "repo:sourcegraph/sourcegraph$ after:\"3 months ago\" type:diff database" - Recent changes to database code
- "file:Dockerfile (alpine OR ubuntu) -content:alpine:latest" - Dockerfiles with specific base images
- "repo:has.path(\.py) file:requirements.txt tensorflow" - Python projects using TensorFlow

BOOLEAN OPERATORS:

- "term1 AND term2" - Results containing both terms
- "term1 OR term2" - Results containing either term
- "term1 NOT term2" - Results with term1 but not term2
- "term1 and (term2 or term3)" - Grouping with parentheses

LIMITATIONS:

- Only searches public repositories
- Rate limits may apply
- Complex queries may take longer to execute
- Maximum of 20 results per query

TIPS:

- Use specific file extensions to narrow results
- Add repo: filters for more targeted searches
- Use type:symbol to find function/method definitions
- Use type:file to find relevant files
