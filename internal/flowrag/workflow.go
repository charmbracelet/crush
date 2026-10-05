package flowrag

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/google/uuid"
)

type WorkflowManager struct {
	detector  *CompletionDetector
	segmenter *Segmenter
	store     *VectorStore
	retriever *Retriever
}

type Config struct {
	StorePath        string
	ChromaDBURL      string
	EmbeddingBaseURL string
	EmbeddingAPIKey  string
	EmbeddingModel   string
}

func NewWorkflowManager(cfg Config) (*WorkflowManager, error) {
	var embClient EmbeddingClient
	if cfg.EmbeddingBaseURL != "" {
		model := cfg.EmbeddingModel
		if model == "" {
			model = "text-embedding-3-small"
		}
		embClient = NewOpenAIEmbeddingClient(cfg.EmbeddingBaseURL, cfg.EmbeddingAPIKey, model)
	} else {
		embClient = &MockEmbeddingClient{Dim: 1536}
	}

	var store *VectorStore
	if cfg.ChromaDBURL != "" {
		chromaBackend := NewChromaDBStore(cfg.ChromaDBURL)
		store = NewVectorStore(chromaBackend, embClient)
		slog.Info("FlowRAG using ChromaDB backend", "url", cfg.ChromaDBURL)
	} else {
		var err error
		store, err = NewFileVectorStore(cfg.StorePath, embClient)
		if err != nil {
			return nil, fmt.Errorf("create file vector store: %w", err)
		}
		slog.Info("FlowRAG using JSON file backend", "path", cfg.StorePath)
	}

	return &WorkflowManager{
		detector:  NewCompletionDetector(),
		segmenter: NewSegmenter(),
		store:     store,
		retriever: NewRetriever(store),
	}, nil
}

func (m *WorkflowManager) Detector() *CompletionDetector {
	return m.detector
}

func (m *WorkflowManager) Retriever() *Retriever {
	return m.retriever
}

type SaveWorkflowInput struct {
	UserPrompt string
	Messages   []message.Message
	SessionID  string
}

func (m *WorkflowManager) SaveSuccessfulWorkflow(ctx context.Context, input SaveWorkflowInput) error {
	workflow := m.segmenter.Segment(input.UserPrompt, input.Messages)
	if workflow == nil || len(workflow.Steps) == 0 {
		return fmt.Errorf("no successful steps to save")
	}

	workflow.SessionID = input.SessionID

	stepsText := workflow.ToText()

	record := WorkflowRecord{
		ID:         uuid.New().String(),
		UserPrompt: workflow.UserPrompt,
		StepsText:  stepsText,
		Steps:      workflow.Steps,
		SessionID:  workflow.SessionID,
		CreatedAt:  time.Now().Unix(),
	}

	if err := m.store.Insert(ctx, record); err != nil {
		return fmt.Errorf("insert workflow record: %w", err)
	}

	slog.Info("Saved successful workflow to RAG store",
		"workflow_id", record.ID,
		"session_id", input.SessionID,
		"steps_count", len(workflow.Steps),
		"total_records", m.store.Count(),
	)

	return nil
}

func (m *WorkflowManager) SearchAndBuildContext(ctx context.Context, userPrompt string, topK int) string {
	records, err := m.retriever.SearchSimilar(ctx, userPrompt, topK)
	if err != nil {
		slog.Warn("Failed to search similar workflows", "error", err)
		return ""
	}
	return m.retriever.BuildContextPrompt(records)
}
