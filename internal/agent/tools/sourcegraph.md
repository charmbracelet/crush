Search code across public repositories using Sourcegraph's GraphQL API.

<usage>
- Provide search query using Sourcegraph syntax
- Optional result count (default: 10, max: 20)
- Optional timeout for request
</usage>

<basic_syntax>
- "fmt.Println" - exact matches
- "file:.go fmt.Println" - limit to Go files
- "repo:^github\.com/golang/go$ fmt.Println" - specific repos
- "lang:go fmt.Println" - limit to Go code
- "fmt.Println AND log.Fatal" - combined terms
- "fmt\.(Print|Printf|Println)" - regex patterns
- "\"exact phrase\"" - exact phrase matching
- "-file:test" or "-repo:forks" - exclude matches
</basic_syntax>

<key_filters>
Repository: repo:name, repo:^exact$, repo:org/repo@branch, -repo:exclude, fork:yes, archived:yes, visibility:public
File: file:\.js$, file:internal/, -file:test, file:has.content(text)
Content: content:"exact", -content:"unwanted", case:yes
Type: type:symbol, type:file, type:path, type:diff, type:commit
Time: after:"1 month ago", before:"2023-01-01", author:name, message:"fix"
Result: select:repo, select:file, select:content, count:100, timeout:30s
</key_filters>

<examples>
- "file:.go context.WithTimeout" - Go code using context.WithTimeout
- "lang:typescript useState type:symbol" - TypeScript React useState hooks
- "repo:^github\.com/kubernetes/kubernetes$ pod list type:file" - Kubernetes pod files
- "file:Dockerfile (alpine OR ubuntu) -content:alpine:latest" - Dockerfiles with base images
</examples>

<boolean_operators>
- "term1 AND term2" - both terms
- "term1 OR term2" - either term
- "term1 NOT term2" - term1 but not term2
- "term1 and (term2 or term3)" - grouping with parentheses
</boolean_operators>

<limitations>
- Only searches public repositories
- Rate limits may apply
- Complex queries take longer
- Max 20 results per query
</limitations>

<tips>
- Use specific file extensions to narrow results
- Add repo: filters for targeted searches
- Use type:symbol for function/method definitions
- Use type:file to find relevant files
</tips>
