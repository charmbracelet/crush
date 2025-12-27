Search public repositories on Sourcegraph. Use for finding examples in open source code.

<when_to_use>
Use Sourcegraph when:
- Looking for usage examples of a library/API
- Finding how others solved similar problems
- Searching open source codebases for patterns
- Need code examples from well-known projects

Do NOT use Sourcegraph when:
- Searching the current project → use `grep` or `agent`
- Need private/local code → use local tools
</when_to_use>

<parameters>
- query: Sourcegraph search query (required)
- count: Number of results (default: 10, max: 20)
</parameters>

<query_syntax>
Basic: `"fmt.Println"` - exact match
File filter: `file:.go fmt.Println` - only Go files
Repo filter: `repo:kubernetes/kubernetes pod` - specific repo
Language: `lang:typescript useState` - by language
Exclude: `-file:test -repo:forks` - exclude patterns
Regex: `"fmt\.(Print|Printf)"` - pattern matching
</query_syntax>

<examples>
Find Go error handling patterns:
```
query: "file:.go errors.Wrap lang:go"
```

Find React hook usage:
```
query: "lang:typescript useEffect cleanup return"
```

Find in specific repo:
```
query: "repo:^github.com/golang/go$ context.WithTimeout"
```
</examples>

<limits>
- Public repositories only
- Max 20 results per query
- Rate limits may apply
</limits>
