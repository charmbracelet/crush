Search code across public GitHub repositories via Sourcegraph. Supports regex, language/repo/file filters, and symbol search (max 20 results). Only searches public repos.

<parameters>
- query: Sourcegraph search query (required)
- count: Results to return (default 10, max 20)
- context_window: Lines of context around matches (default 10)
- timeout: Optional timeout in seconds (max 120)
</parameters>

<syntax>
- "fmt.Println" — exact match
- "file:.go fmt.Println" — filter by file
- "repo:^github\.com/golang/go$ fmt.Println" — filter by repo
- "lang:go fmt.Println" — filter by language
- "fmt\.(Print|Printf|Println)" — regex
- "fmt.Println AND log.Fatal" — boolean AND
- "fmt.Println OR log.Fatal" — boolean OR
- "term1 NOT term2" — exclude
- "type:symbol" — symbol search
- "case:yes" — case-sensitive
</syntax>
