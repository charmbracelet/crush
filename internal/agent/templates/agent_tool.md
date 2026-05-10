Launch a new agent that has access to the following tools: glob, grep, ls, view. When you are searching for a keyword or file and are not confident that you will find the right match on the first try, use the agent tool to perform the search for you.

<usage>
- If searching for a keyword or "which file does X?", use the Agent tool
- For a specific file path, use View or Glob directly
- Launch multiple agents concurrently in a single message
- Each agent is stateless — provide a detailed task description and specify exactly what to return
- The agent cannot use Bash, Edit, or Write; use those directly instead
</usage>
