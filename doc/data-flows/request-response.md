# Data Flow: Request-Response

This diagram illustrates the flow of a user prompt through the system until a response is rendered in the TUI.

```mermaid
sequenceDiagram
    participant U as User
    participant TUI as TUI (Bubble Tea)
    participant App as App Orchestrator
    participant AC as Agent Coordinator
    participant SA as Session Agent
    participant F as Fantasy Abstraction
    participant LLM as AI Provider (e.g. Anthropic)

    U->>TUI: Enter Prompt
    TUI->>App: Send Prompt Message
    App->>AC: Run(sessionID, prompt)
    AC->>SA: Run(call)
    
    rect rgb(240, 240, 240)
        Note over SA, LLM: Generation Loop
        SA->>F: Stream(request)
        F->>LLM: API Call (Streaming)
        LLM-->>F: Text Deltas / Tool Calls
        F-->>SA: OnTextDelta / OnToolCall
        SA-->>App: PubSub Event (Message Update)
        App-->>TUI: tea.Msg
        TUI-->>U: Render Update
    end

    SA->>AC: Return AgentResult
    AC->>App: Return
```

## Data Transformation
1. **Prompt Sanitization:** The prompt is combined with system reminders and any project-specific context (from `AGENTS.md` or `.cursorrules`).
2. **Context Assembly:** Previous messages in the session are fetched from SQLite and converted into the provider's message format.
3. **Streaming Deltas:** Responses are received as small chunks (deltas), which are appended to the message state and persisted to the DB incrementally.
