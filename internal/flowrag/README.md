# FlowRAG — Intelligent Workflow Memory & Retrieval

FlowRAG is a **workflow learning and replay module** for Crush, the terminal AI coding assistant. It captures successful coding workflows, stores them in a ChromaDB vector database, and retrieves the best past experience via semantic search for future similar tasks.

## How It Works

### Trigger Detection (Dual Layer)

**Primary: Skill-driven (AI semantic understanding)**

FlowRAG is registered as a [Skill](../../skills/builtin/flowrag/SKILL.md) in Crush. The LLM automatically identifies when to trigger it:

- A multi-step task completed with no errors
- User expresses satisfaction or confirms the result
- User explicitly asks to save/remember the workflow

**Secondary: Keyword matching**

The [CompletionDetector](detector.go) recognizes explicit markers:

| Category | Examples |
|----------|----------|
| Colloquial confirmation | "ok", "好的", "done", "搞定了" |
| Business markers | "task complete", "save workflow", "remember this" |

### Data Flow

```
Task completed (Skill detection / keyword match)
  → Prompt: "Save this workflow? (y/n)"
    → Yes: Segmenter extracts successful steps (skips errors/retries)
      → Embedding (OpenAI-compatible API / Trigram Hash)
        → Store in ChromaDB / JSON File
          → Next similar task: retrieve Top-K via semantic search
            → Inject into system prompt for faster execution
```

## Architecture

```
internal/flowrag/
├── detector.go          # Completion marker detection (Skill + keyword)
├── segmenter.go         # Workflow segmentation (filter error steps)
├── store.go             # Vector store (ChromaDB / JSON File backends)
├── retriever.go         # Semantic search + context builder
├── workflow.go          # Orchestrator (unified API)
├── workflow_test.go     # 17 test cases
├── cmd/
│   ├── demo/            # Interactive CLI demo (zero-dependency)
│   └── e2e_test/        # End-to-end semantic search verification
└── README.md
```

## Tech Stack

| Component | Backend | Notes |
|-----------|---------|-------|
| **Vector Store** | ChromaDB (default) | REST API v2, auto-creates `crush_workflows` collection |
| **Fallback Store** | JSON File | Zero-dependency, used when ChromaDB is unavailable |
| **Embedding** | OpenAI-compatible API | `text-embedding-3-small` or similar |
| **Embedding (zero-dep)** | Trigram Hash | Deterministic, content-aware, no API needed |
| **Similarity** | Cosine similarity (ChromaDB built-in) | — |
| **Segmentation** | IsError field filtering | Precisely excludes failed/rollback steps |

## Quick Start

### Option A: ChromaDB (Recommended)

```bash
# Start ChromaDB
docker run -d -p 8000:8000 chromadb/chroma
```

```go
mgr, _ := flowrag.NewWorkflowManager(flowrag.Config{
    ChromaDBURL:      "http://localhost:8000",
    EmbeddingBaseURL: "https://api.openai.com/v1",
    EmbeddingAPIKey:  "sk-xxx",
    EmbeddingModel:   "text-embedding-3-small",
})
```

### Option B: JSON File (Zero External Dependencies)

```go
mgr, _ := flowrag.NewWorkflowManager(flowrag.Config{
    StorePath: "~/.crush/flowrag/workflows.json",
})
// ChromaDBURL left empty → falls back to file backend
```

### API Usage

```go
// Check if this message should trigger FlowRAG
if mgr.Detector().ShouldTriggerFlowRAG(userMessage) {
    // Confirm with user...
}

// Save successful workflow
mgr.SaveSuccessfulWorkflow(ctx, SaveWorkflowInput{
    UserPrompt: "Write an HTTP server",
    Messages:   sessionMessages,
    SessionID:  "session-123",
})

// Retrieve similar past workflows
ctx := mgr.SearchAndBuildContext(ctx, "Create a REST API", 3)
// Inject ctx into system prompt
```

## Interactive Demo

```bash
go run ./internal/flowrag/cmd/demo/
```

Run the demo, type `demo` to load 6 sample workflows spanning auth fixes, REST APIs, Python scripting, DB migrations, OAuth, and Docker Compose — then search with natural language queries and see semantic retrieval in action.

## E2E Verification

```bash
go run ./internal/flowrag/cmd/e2e_test/
```

## Tests

```bash
go test ./internal/flowrag/... -v -count=1
```

17 test cases covering: detector (colloquial + business keywords), segmenter (success/error flow extraction), JSONFileStore (insert/search/empty), cosine similarity precision, retriever context building, and end-to-end integration.
