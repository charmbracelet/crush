# Use Case: Interactive Coding Assistance

## Actor
Developer

## Trigger
User enters a prompt in the Crush TUI chat interface.

## Preconditions
- Crush is installed and configured with at least one LLM provider.
- Crush is running in a project directory.

## Main Flow
1. **Input:** User types a request (e.g., "Add a new method to the `User` struct that calculates age").
2. **Context Gathering:** 
   - Agent uses `ls` or `view` to find relevant files.
   - Agent may use LSP (`references`, `diagnostics`) to understand dependencies.
3. **Reasoning:** The LLM processes the prompt and the gathered context.
4. **Tool Execution:**
   - Agent proposes an edit using the `edit` or `write` tool.
   - User reviews and approves/denies the permission request.
5. **Feedback:** Agent reports the result of the operation.
6. **Iteration:** User provides follow-up instructions if needed.

## Alternative Flows
- **Context Window Full:** If the history exceeds the model's limit, Crush automatically triggers the `Summarize` flow to condense history before continuing.
- **Error in execution:** If a tool fails (e.g., syntax error), the agent receives the error output and can attempt a fix.

## Postconditions
- The codebase is updated as requested.
- The session history is saved in the SQLite database.
