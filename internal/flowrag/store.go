package flowrag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Vector []float64

type WorkflowRecord struct {
	ID         string         `json:"id"`
	UserPrompt string         `json:"user_prompt"`
	StepsText  string         `json:"steps_text"`
	Embedding  Vector         `json:"embedding,omitempty"`
	Steps      []WorkflowStep `json:"steps"`
	SessionID  string         `json:"session_id"`
	CreatedAt  int64          `json:"created_at"`
}

type EmbeddingClient interface {
	GetEmbedding(ctx context.Context, text string) (Vector, error)
}

type HTTPSEmbeddingClient struct {
	BaseURL string
	APIKey  string
	Model   string
	client  *http.Client
}

func NewOpenAIEmbeddingClient(baseURL, apiKey, model string) *HTTPSEmbeddingClient {
	return &HTTPSEmbeddingClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func (c *HTTPSEmbeddingClient) GetEmbedding(ctx context.Context, text string) (Vector, error) {
	reqBody := embeddingRequest{
		Model: c.Model,
		Input: text,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	url := c.BaseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding request returned status %d", resp.StatusCode)
	}

	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	return Vector(embResp.Data[0].Embedding), nil
}

type MockEmbeddingClient struct {
	Dim int
}

func (m *MockEmbeddingClient) GetEmbedding(_ context.Context, text string) (Vector, error) {
	v := make(Vector, m.Dim)
	for i := range v {
		v[i] = float64(len(text)+i) / float64(m.Dim+len(text))
	}
	return v, nil
}

type HashEmbeddingClient struct {
	Dim int
}

func NewHashEmbeddingClient(dim int) *HashEmbeddingClient {
	if dim <= 0 {
		dim = 256
	}
	return &HashEmbeddingClient{Dim: dim}
}

func (h *HashEmbeddingClient) GetEmbedding(_ context.Context, text string) (Vector, error) {
	v := make(Vector, h.Dim)
	if len(text) == 0 {
		return v, nil
	}

	const ngramSize = 3
	runes := []rune(text)
	for i := 0; i <= len(runes)-ngramSize; i++ {
		ngram := string(runes[i : i+ngramSize])
		hash := hashString(ngram)
		idx := int(hash % uint32(h.Dim))
		v[idx] += 1.0
	}

	norm := 0.0
	for _, val := range v {
		norm += val * val
	}
	if norm > 0 {
		norm = norm / float64(len(runes))
		for i := range v {
			v[i] = v[i] / norm
		}
	}

	return v, nil
}

func hashString(s string) uint32 {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

type VectorStoreBackend interface {
	Insert(ctx context.Context, record WorkflowRecord) error
	Search(ctx context.Context, queryEmbedding Vector, topK int) ([]WorkflowRecord, error)
	Count() int
}

type JSONFileStore struct {
	mu        sync.RWMutex
	records   []WorkflowRecord
	storePath string
}

func NewJSONFileStore(storePath string) (*JSONFileStore, error) {
	dir := filepath.Dir(storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}

	store := &JSONFileStore{
		records:   make([]WorkflowRecord, 0),
		storePath: storePath,
	}

	if err := store.load(); err != nil {
		slog.Warn("Failed to load vector store, starting fresh", "error", err)
	}

	return store, nil
}

func (s *JSONFileStore) load() error {
	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.records)
}

func (s *JSONFileStore) save() error {
	data, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.storePath, data, 0644)
}

func (s *JSONFileStore) Insert(_ context.Context, record WorkflowRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record.CreatedAt = time.Now().Unix()
	s.records = append(s.records, record)
	return s.save()
}

func (s *JSONFileStore) Search(_ context.Context, queryEmbedding Vector, topK int) ([]WorkflowRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		record WorkflowRecord
		score  float64
	}

	var results []scored
	for _, record := range s.records {
		if record.Embedding == nil {
			continue
		}
		score := cosineSimilarity(queryEmbedding, record.Embedding)
		results = append(results, scored{record: record, score: score})
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > len(results) {
		topK = len(results)
	}

	top := make([]WorkflowRecord, topK)
	for i := 0; i < topK; i++ {
		top[i] = results[i].record
	}

	return top, nil
}

func (s *JSONFileStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

type ChromaDBStore struct {
	baseURL    string
	client     *http.Client
	collection string
	mu         sync.Mutex
	created    bool
}

func NewChromaDBStore(baseURL string) *ChromaDBStore {
	return &ChromaDBStore{
		baseURL:    baseURL,
		client:     &http.Client{Timeout: 30 * time.Second},
		collection: "crush_workflows",
	}
}

func (s *ChromaDBStore) ensureCollection(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.created {
		return nil
	}

	createURL := fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections", s.baseURL)
	body := map[string]interface{}{
		"name":     s.collection,
		"metadata": map[string]string{"description": "Crush FlowRAG workflow records"},
	}

	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Warn("ChromaDB unreachable, collection not created", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		s.created = true
		slog.Info("ChromaDB collection ready", "collection", s.collection)
		return nil
	}

	var errResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errResp)
	slog.Warn("ChromaDB create collection response", "status", resp.StatusCode, "body", errResp)
	return fmt.Errorf("chromadb create collection returned status %d", resp.StatusCode)
}

func (s *ChromaDBStore) Insert(ctx context.Context, record WorkflowRecord) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}

	if record.Embedding == nil {
		return fmt.Errorf("record has no embedding")
	}

	metalJSON, _ := json.Marshal(map[string]string{
		"user_prompt": record.UserPrompt,
		"session_id":  record.SessionID,
		"steps_json":  mustMarshalSteps(record.Steps),
	})

	addURL := fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections/%s/add",
		s.baseURL, s.collection)

	body := map[string]interface{}{
		"ids":        []string{record.ID},
		"embeddings": [][]float64{record.Embedding},
		"metadatas":  []json.RawMessage{metalJSON},
		"documents":  []string{record.StepsText},
	}

	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("chromadb add request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("chromadb add returned status %d: %v", resp.StatusCode, errResp)
	}

	slog.Info("Inserted workflow into ChromaDB", "workflow_id", record.ID)
	return nil
}

type chromaQueryResult struct {
	Ids       [][]string          `json:"ids"`
	Documents [][]string          `json:"documents"`
	Metadatas [][]json.RawMessage `json:"metadatas"`
	Distances [][]float64         `json:"distances"`
}

type chromaMetadata struct {
	UserPrompt string `json:"user_prompt"`
	SessionID  string `json:"session_id"`
	StepsJSON  string `json:"steps_json"`
}

func (s *ChromaDBStore) Search(ctx context.Context, queryEmbedding Vector, topK int) ([]WorkflowRecord, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}

	queryURL := fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections/%s/query",
		s.baseURL, s.collection)

	body := map[string]interface{}{
		"query_embeddings": [][]float64{queryEmbedding},
		"n_results":        topK,
		"include":          []string{"metadatas", "documents", "distances"},
	}

	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chromadb query request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chromadb query returned status %d", resp.StatusCode)
	}

	var result chromaQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode chromadb query result: %w", err)
	}

	var records []WorkflowRecord
	for i := range result.Ids {
		for j, id := range result.Ids[i] {
			record := WorkflowRecord{ID: id}

			if j < len(result.Documents[i]) {
				record.StepsText = result.Documents[i][j]
			}

			if j < len(result.Metadatas[i]) {
				var meta chromaMetadata
				if err := json.Unmarshal(result.Metadatas[i][j], &meta); err == nil {
					record.UserPrompt = meta.UserPrompt
					record.SessionID = meta.SessionID
					if meta.StepsJSON != "" {
						var steps []WorkflowStep
						if json.Unmarshal([]byte(meta.StepsJSON), &steps) == nil {
							record.Steps = steps
						}
					}
				}
			}

			records = append(records, record)
		}
	}

	return records, nil
}

func (s *ChromaDBStore) Count() int {
	ctx := context.Background()
	_ = s.ensureCollection(ctx)

	countURL := fmt.Sprintf("%s/api/v2/tenants/default_tenant/databases/default_database/collections/%s",
		s.baseURL, s.collection)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, countURL, nil)
	if err != nil {
		return -1
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	var info struct {
		Count int `json:"count"`
	}
	if json.NewDecoder(resp.Body).Decode(&info) == nil {
		return info.Count
	}
	return -1
}

type VectorStore struct {
	backend   VectorStoreBackend
	embClient EmbeddingClient
}

func NewVectorStore(backend VectorStoreBackend, embClient EmbeddingClient) *VectorStore {
	return &VectorStore{
		backend:   backend,
		embClient: embClient,
	}
}

func NewFileVectorStore(storePath string, embClient EmbeddingClient) (*VectorStore, error) {
	backend, err := NewJSONFileStore(storePath)
	if err != nil {
		return nil, err
	}
	return NewVectorStore(backend, embClient), nil
}

func (s *VectorStore) Insert(ctx context.Context, record WorkflowRecord) error {
	if record.Embedding == nil {
		embedding, err := s.embClient.GetEmbedding(ctx, record.StepsText)
		if err != nil {
			return fmt.Errorf("generate embedding: %w", err)
		}
		record.Embedding = embedding
	}

	record.CreatedAt = time.Now().Unix()
	return s.backend.Insert(ctx, record)
}

func (s *VectorStore) Search(ctx context.Context, query string, topK int) ([]WorkflowRecord, error) {
	queryEmbedding, err := s.embClient.GetEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	return s.backend.Search(ctx, queryEmbedding, topK)
}

func (s *VectorStore) Count() int {
	return s.backend.Count()
}

func cosineSimilarity(a, b Vector) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func mustMarshalSteps(steps []WorkflowStep) string {
	data, err := json.Marshal(steps)
	if err != nil {
		return "[]"
	}
	return string(data)
}
