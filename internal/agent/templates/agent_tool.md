Launch a sub-agent to perform complex searches across the codebase. The agent has access to Glob, Grep, LS, and View tools.

<when_to_use>
Use Agent when:
- Searching for a concept and unsure where to look ("where is authentication handled?")
- Need to explore multiple files to answer a question
- Looking for patterns across the codebase
- Question requires iterative searching (find X, then look for Y in those files)

Do NOT use Agent when:
- You know the file path → use `view` directly
- Searching for exact text → use `grep` directly
- Finding files by name → use `glob` directly
- Looking up symbol references → use `lsp_references`
</when_to_use>

<how_it_works>
- Agent runs autonomously with its own tool calls
- Returns a single final message with findings
- Cannot modify files (read-only tools only)
- Stateless: each invocation starts fresh
- Results not visible to user until you summarize them
</how_it_works>

<prompt_guidelines>
Write detailed prompts—the agent works independently:
- Be specific about what to find and what to return
- Include context about the codebase if relevant
- Specify the format you want for the response
- Ask for file paths and line numbers in results
</prompt_guidelines>

<examples>
Good: "Find where user sessions are created and validated. Return the file paths, function names, and a brief description of the flow."

Good: "Search for all usages of the Config struct. List each file and how it uses Config."

Bad: "Find the config" → Too vague, doesn't specify what to return.

Bad: "Look in src/auth.go for the login function" → Just use `view` directly.
</examples>

<parallel_execution>
Launch multiple agents concurrently when you have independent questions:
```
[agent: "Where is database connection handled?"]
[agent: "Where are API routes defined?"]
```
</parallel_execution>
