---
name: flowrag
description: Use when the user's coding task has been successfully completed and the agent should save the successful workflow for future reuse. FlowRAG captures the sequence of successful tool calls (excluding errors and retries), stores them in a vector database (ChromaDB), and retrieves similar past workflows via semantic search to accelerate future tasks. Trigger this skill when the user expresses satisfaction with the result, confirms task completion, or when a multi-step coding task has been executed without errors.
---

# FlowRAG — Workflow Memory & Retrieval

FlowRAG is a workflow memory system for Crush. It captures **successful coding
workflows** — the sequence of tool calls, their inputs/outputs — and stores
them in a vector database so that **similar future tasks** can be completed
faster by recalling past successful approaches.

## When to Trigger This Skill

Invoke FlowRAG when any of these conditions are true:

1. **The user signals task completion** — they say the task is done, the
   solution works, or they confirm the result is acceptable.
2. **A multi-step workflow completed without errors** — the agent ran multiple
   tools (read → edit → write → test) and all steps succeeded.
3. **The user explicitly asks to save or remember** — they request the
   workflow be stored for future reference.
4. **The agent is about to start a new task** — before executing, check if a
   similar workflow exists in the RAG store to accelerate execution.

## Saving a Workflow

When a task completes successfully, ask the user:
"Does this solution achieve the desired effect? Should I save this workflow
for future reuse? (y/n)"

If the user confirms (y), the system will:

1. **Extract the successful flow** — take the current session's messages and
   filter out all steps where `IsError` is true (failed tool calls, retries).
2. **Generate an embedding** — send the workflow text to the configured
   embedding API (default: OpenAI `text-embedding-3-small`).
3. **Store in ChromaDB** — persist the embedding + metadata in a ChromaDB
   collection named `crush_workflows`.

Only the final successful path is saved — intermediate failures and retries
are automatically excluded.

## Retrieving Past Workflows

Before executing a new task, search the FlowRAG store:

1. Generate an embedding of the user's request.
2. Query ChromaDB for the top-K most similar past workflows.
3. Inject the retrieved workflows into the system prompt as
   `<past_successful_workflows>` context, allowing the agent to reference
   proven approaches.

## ChromaDB Backend

FlowRAG uses **ChromaDB** (the most widely adopted open-source vector
database) as its default backend. ChromaDB runs as a local service on
`http://localhost:8000`.

Key ChromaDB characteristics:
- Collection name: `crush_workflows`
- Distance function: cosine similarity
- Embeddings are generated client-side and sent to ChromaDB for storage

## Configuration

FlowRAG stores its configuration in `crush.json` under `flowrag`:

```json
{
  "flowrag": {
    "enabled": true,
    "chromadb_url": "http://localhost:8000",
    "embedding_base_url": "https://api.openai.com/v1",
    "embedding_api_key": "$OPENAI_API_KEY",
    "embedding_model": "text-embedding-3-small",
    "top_k": 3
  }
}
```

If `chromadb_url` is omitted or unreachable, FlowRAG falls back to a local
JSON file store (`~/.crush/flowrag/workflows.json`).

## E2E Testing

To verify the RAG pipeline end-to-end, run:

```bash
go run ./internal/flowrag/cmd/e2e_test/main.go
```

This script will:
1. Start a local ChromaDB container (if Docker available)
2. Insert several test workflows into ChromaDB
3. Run semantic queries and display similarity scores
4. Verify retrieval quality and ranking
