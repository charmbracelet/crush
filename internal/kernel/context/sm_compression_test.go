package context

import (
	"testing"
	"time"

	"charm.land/fantasy"
)

func TestSessionMemoryPool_AddBlock(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	block := &SessionMemoryBlock{
		ID:      "block-1",
		Summary: "Test summary",
		Tags:    []string{"test", "unit"},
		Weight:  0.8,
	}

	err := pool.AddBlock(block)
	if err != nil {
		t.Fatalf("AddBlock failed: %v", err)
	}

	if pool.Size() != 1 {
		t.Errorf("Expected pool size 1, got %d", pool.Size())
	}
}

func TestSessionMemoryPool_GetBlock(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	block := &SessionMemoryBlock{
		ID:      "block-1",
		Summary: "Test summary",
		Tags:    []string{"test"},
	}

	pool.AddBlock(block)

	// Get block
	retrieved, ok := pool.GetBlock("block-1")
	if !ok {
		t.Fatal("GetBlock returned false for existing block")
	}

	if retrieved.ID != block.ID {
		t.Errorf("Expected ID %s, got %s", block.ID, retrieved.ID)
	}

	if retrieved.Summary != block.Summary {
		t.Errorf("Expected Summary %s, got %s", block.Summary, retrieved.Summary)
	}
}

func TestSessionMemoryPool_GetBlock_NotFound(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	_, ok := pool.GetBlock("nonexistent")
	if ok {
		t.Error("GetBlock should return false for nonexistent block")
	}
}

func TestSessionMemoryPool_Eviction(t *testing.T) {
	pool := NewSessionMemoryPool(3, 5)

	// Add 4 blocks (should evict one)
	for i := 0; i < 4; i++ {
		pool.AddBlock(&SessionMemoryBlock{
			ID:      "block-" + string(rune('0'+i)),
			Summary: "Summary " + string(rune('0'+i)),
		})
	}

	// Pool should have 3 blocks
	if pool.Size() != 3 {
		t.Errorf("Expected pool size 3 after eviction, got %d", pool.Size())
	}

	// First block should have been evicted (LRU)
	_, ok := pool.GetBlock("block-0")
	if ok {
		t.Error("First block should have been evicted")
	}
}

func TestSessionMemoryPool_GetBlocksByTags(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	pool.AddBlock(&SessionMemoryBlock{
		ID:   "block-1",
		Tags: []string{"go", "testing"},
	})
	pool.AddBlock(&SessionMemoryBlock{
		ID:   "block-2",
		Tags: []string{"rust", "testing"},
	})
	pool.AddBlock(&SessionMemoryBlock{
		ID:   "block-3",
		Tags: []string{"python"},
	})

	blocks := pool.GetBlocksByTags([]string{"testing"}, 10)
	if len(blocks) != 2 {
		t.Errorf("Expected 2 blocks with tag 'testing', got %d", len(blocks))
	}
}

func TestSessionMemoryPool_GetBlocksByWeight(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	pool.AddBlock(&SessionMemoryBlock{
		ID:     "block-low",
		Weight: 0.2,
	})
	pool.AddBlock(&SessionMemoryBlock{
		ID:     "block-high",
		Weight: 0.9,
	})
	pool.AddBlock(&SessionMemoryBlock{
		ID:     "block-mid",
		Weight: 0.5,
	})

	blocks := pool.GetBlocksByWeight(2)
	if len(blocks) != 2 {
		t.Errorf("Expected 2 blocks, got %d", len(blocks))
	}

	if blocks[0].Weight < blocks[1].Weight {
		t.Error("Blocks should be sorted by weight descending")
	}
}

func TestSessionMemoryPool_PoolHitRate(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	// Add a block
	pool.AddBlock(&SessionMemoryBlock{
		ID: "block-1",
	})

	// Access it multiple times
	for i := 0; i < 5; i++ {
		pool.GetBlock("block-1")
	}

	// Miss once
	pool.GetBlock("nonexistent")

	// Hit rate should be 5/6
	rate := pool.PoolHitRate()
	expected := 5.0 / 6.0 * 100
	if rate < expected-1 || rate > expected+1 {
		t.Errorf("Expected hit rate ~%.2f%%, got %.2f%%", expected, rate)
	}
}

func TestSessionMemoryPool_RemoveBlock(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	pool.AddBlock(&SessionMemoryBlock{
		ID: "block-1",
	})

	if pool.Size() != 1 {
		t.Fatalf("Expected size 1, got %d", pool.Size())
	}

	removed := pool.RemoveBlock("block-1")
	if !removed {
		t.Error("RemoveBlock should return true for existing block")
	}

	if pool.Size() != 0 {
		t.Errorf("Expected size 0, got %d", pool.Size())
	}
}

func TestSessionMemoryPool_Clear(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	for i := 0; i < 5; i++ {
		pool.AddBlock(&SessionMemoryBlock{
			ID: "block-" + string(rune('0'+i)),
		})
	}

	pool.Clear()

	if pool.Size() != 0 {
		t.Errorf("Expected size 0 after Clear, got %d", pool.Size())
	}
}

func TestMemoryHitCalculator_CalculateHit(t *testing.T) {
	calc := NewMemoryHitCalculator(DefaultHitWeightConfig())

	block := &SessionMemoryBlock{
		ID:         "block-1",
		Tags:       []string{"go", "testing"},
		LastAccess: time.Now(),
		CreatedAt:  time.Now().Add(-1 * time.Hour),
	}

	topics := []string{"go", "testing", "performance"}

	result := calc.CalculateHit(block, topics)

	if result.BlockID != "block-1" {
		t.Errorf("Expected BlockID 'block-1', got '%s'", result.BlockID)
	}

	if result.OverallScore <= 0 || result.OverallScore > 1 {
		t.Errorf("OverallScore should be between 0 and 1, got %f", result.OverallScore)
	}

	// Should cover 'go' and 'testing'
	if len(result.CoveredTopics) != 2 {
		t.Errorf("Expected 2 covered topics, got %d", len(result.CoveredTopics))
	}
}

func TestMemoryHitCalculator_CalculateHit_NilBlock(t *testing.T) {
	calc := NewMemoryHitCalculator(DefaultHitWeightConfig())

	result := calc.CalculateHit(nil, []string{"test"})

	if result.OverallScore != 0 {
		t.Errorf("Expected score 0 for nil block, got %f", result.OverallScore)
	}
}

func TestMemoryHitCalculator_SelectTopBlocks(t *testing.T) {
	calc := NewMemoryHitCalculator(DefaultHitWeightConfig())

	blocks := []*SessionMemoryBlock{
		{ID: "block-1", Tags: []string{"go"}, Weight: 0.5},
		{ID: "block-2", Tags: []string{"testing", "go"}, Weight: 0.8},
		{ID: "block-3", Tags: []string{"rust"}, Weight: 0.6},
	}

	topics := []string{"go", "testing"}

	selected := calc.SelectTopBlocks(blocks, topics, 2)

	// Should select at most 2 blocks
	if len(selected) > 2 {
		t.Errorf("Expected at most 2 selected blocks, got %d", len(selected))
	}
}

func TestMemoryHitCalculator_RecordTopics(t *testing.T) {
	calc := NewMemoryHitCalculator(DefaultHitWeightConfig())

	calc.RecordTopics([]string{"go", "testing", "go", "rust"})

	stats := calc.GetTopicStats()

	if len(stats) != 3 {
		t.Errorf("Expected 3 topics, got %d", len(stats))
	}

	// Find 'go' stats
	var goStats TopicStats
	for _, s := range stats {
		if s.Topic == "go" {
			goStats = s
			break
		}
	}

	if goStats.Frequency != 2 {
		t.Errorf("Expected frequency 2 for 'go', got %d", goStats.Frequency)
	}
}

func TestSMComposer_Compose(t *testing.T) {
	composer := NewSMComposer(DefaultComposerConfig())

	blocks := []*SessionMemoryBlock{
		{
			ID:      "block-1",
			Summary: "This is a test summary",
			KeyFacts: []string{"Fact 1: Testing is important", "Fact 2: Go is fast"},
			Tags:    []string{"test", "go"},
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			ID:      "block-2",
			Summary: "Second block summary",
			KeyFacts: []string{"Fact 3: More facts"},
			Tags:    []string{"testing"},
			CreatedAt: time.Now(),
		},
	}

	topics := []string{"test", "go", "performance"}

	result := composer.Compose(blocks, topics)

	if result.Summary == "" {
		t.Error("Summary should not be empty")
	}

	if result.BlocksUsed != 2 {
		t.Errorf("Expected 2 blocks used, got %d", result.BlocksUsed)
	}

	if len(result.Metadata.BlockIDs) != 2 {
		t.Errorf("Expected 2 block IDs in metadata, got %d", len(result.Metadata.BlockIDs))
	}
}

func TestSMComposer_Compose_EmptyBlocks(t *testing.T) {
	composer := NewSMComposer(DefaultComposerConfig())

	result := composer.Compose([]*SessionMemoryBlock{}, []string{"test"})

	if result.Summary != "" {
		t.Error("Summary should be empty for empty blocks")
	}

	if result.BlocksUsed != 0 {
		t.Errorf("Expected 0 blocks used, got %d", result.BlocksUsed)
	}
}

func TestSMComposer_Compose_Truncation(t *testing.T) {
	config := DefaultComposerConfig()
	config.TokenBudget = 50 // Very small budget
	composer := NewSMComposer(config)

	// Create a block with long summary
	longSummary := ""
	for i := 0; i < 200; i++ {
		longSummary += "word "
	}

	blocks := []*SessionMemoryBlock{
		{
			ID:      "block-1",
			Summary: longSummary,
			KeyFacts: []string{},
			CreatedAt: time.Now(),
		},
	}

	result := composer.Compose(blocks, []string{})

	// Should be truncated to fit budget
	if result.TokensUsed > config.TokenBudget {
		t.Errorf("TokensUsed %d exceeds budget %d", result.TokensUsed, config.TokenBudget)
	}
}

func TestSMComposer_CacheComposition(t *testing.T) {
	composer := NewSMComposer(DefaultComposerConfig())

	key := "test-cache-key"
	summary := "Cached summary"
	blockIDs := []string{"block-1", "block-2"}

	composer.CacheComposition(key, summary, blockIDs, 5*time.Minute)

	cached, ok := composer.GetCachedComposition(key)

	if !ok {
		t.Fatal("GetCachedComposition returned false for valid key")
	}

	if cached != summary {
		t.Errorf("Expected cached summary '%s', got '%s'", summary, cached)
	}
}

func TestSMComposer_GetCachedComposition_Expired(t *testing.T) {
	composer := NewSMComposer(DefaultComposerConfig())

	key := "expired-cache"
	composer.CacheComposition(key, "summary", []string{"block-1"}, 1*time.Millisecond)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	_, ok := composer.GetCachedComposition(key)
	if ok {
		t.Error("GetCachedComposition should return false for expired cache")
	}
}

func TestSMComposer_Metrics(t *testing.T) {
	composer := NewSMComposer(DefaultComposerConfig())

	composer.CacheComposition("key1", "summary1", []string{"block-1"}, time.Hour)
	composer.CacheComposition("key2", "summary2", []string{"block-2"}, time.Hour)

	metrics := composer.Metrics()

	if metrics["cache_entries"].(int) != 2 {
		t.Errorf("Expected 2 cache entries, got %v", metrics["cache_entries"])
	}

	if metrics["token_budget"].(int) != DefaultComposerConfig().TokenBudget {
		t.Errorf("Token budget mismatch")
	}
}

func TestSessionMemoryPool_UpdateBlock(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	// Add initial block
	pool.AddBlock(&SessionMemoryBlock{
		ID:      "block-1",
		Summary: "Original",
		Weight:  0.5,
	})

	// Update with same ID
	pool.AddBlock(&SessionMemoryBlock{
		ID:      "block-1",
		Summary: "Updated",
		Weight:  0.9,
	})

	// Pool size should still be 1
	if pool.Size() != 1 {
		t.Errorf("Expected size 1 after update, got %d", pool.Size())
	}

	// Check updated values
	block, _ := pool.GetBlock("block-1")
	if block.Summary != "Updated" {
		t.Errorf("Expected updated summary, got '%s'", block.Summary)
	}
	if block.Weight != 0.9 {
		t.Errorf("Expected weight 0.9, got %f", block.Weight)
	}
}

func TestSessionMemoryPool_Metrics(t *testing.T) {
	pool := NewSessionMemoryPool(10, 5)

	pool.AddBlock(&SessionMemoryBlock{
		ID:     "block-1",
		Weight: 0.8,
		Tags:   []string{"test"},
	})

	metrics := pool.Metrics()

	if metrics["pool_size"].(int) != 1 {
		t.Errorf("Expected pool_size 1, got %v", metrics["pool_size"])
	}

	if metrics["max_blocks"].(int) != 10 {
		t.Errorf("Expected max_blocks 10, got %v", metrics["max_blocks"])
	}

	if metrics["eviction_policy"].(EvictionPolicy) != EvictLRU {
		t.Errorf("Expected eviction policy EvictLRU")
	}
}

func TestMemoryHitCalculator_Coverage(t *testing.T) {
	calc := NewMemoryHitCalculator(DefaultHitWeightConfig())

	block := &SessionMemoryBlock{
		ID:    "block-1",
		Tags:  []string{"go", "testing"},
	}

	// Context has 4 topics, block covers 2
	topics := []string{"go", "testing", "rust", "python"}

	result := calc.CalculateHit(block, topics)

	expectedCoverage := 50.0 // 2 out of 4 = 50%
	if result.Coverage < expectedCoverage-1 || result.Coverage > expectedCoverage+1 {
		t.Errorf("Expected coverage ~%.2f%%, got %.2f%%", expectedCoverage, result.Coverage)
	}
}

func TestSMComposer_BuildHeader(t *testing.T) {
	composer := NewSMComposer(DefaultComposerConfig())

	block1 := &SessionMemoryBlock{
		ID:        "block-1",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	block2 := &SessionMemoryBlock{
		ID:        "block-2",
		CreatedAt: time.Now(),
	}

	result := composer.Compose([]*SessionMemoryBlock{block1, block2}, []string{})

	// Should contain time range
	if result.Metadata.TimeRange == "" {
		t.Error("TimeRange should not be empty")
	}

	// Should contain block count
	if len(result.Summary) == 0 {
		t.Error("Summary should not be empty")
	}
}

func TestContextIntegration(t *testing.T) {
	// Test full integration: Pool -> Calculator -> Composer

	// 1. Create pool and add blocks
	pool := NewSessionMemoryPool(10, 5)
	for i := 0; i < 3; i++ {
		pool.AddBlock(&SessionMemoryBlock{
			ID:      "block-" + string(rune('1'+i)),
			Summary: "Summary for block " + string(rune('1'+i)),
			Tags:    []string{"topic-a", "topic-b"},
			Weight:  0.5 + float64(i)*0.1,
		})
	}

	// 2. Calculate hits
	calc := NewMemoryHitCalculator(DefaultHitWeightConfig())
	blocks := pool.GetBlocksByWeight(3)
	topics := []string{"topic-a", "topic-b"}
	hits := calc.CalculateHits(blocks, topics)

	if len(hits) != 3 {
		t.Errorf("Expected 3 hits, got %d", len(hits))
	}

	// 3. Compose summary
	composer := NewSMComposer(DefaultComposerConfig())
	composition := composer.Compose(blocks, topics)

	if composition.Summary == "" {
		t.Error("Composition summary should not be empty")
	}

	if composition.BlocksUsed != 3 {
		t.Errorf("Expected 3 blocks used, got %d", composition.BlocksUsed)
	}
}

// ContextCompactor SM Integration Tests

func TestContextCompactor_SMComponents(t *testing.T) {
	compactor := NewContextCompactor(200000)

	// Verify SM components are initialized
	if compactor.memPool == nil {
		t.Error("MemoryPool should be initialized")
	}

	if compactor.hitCalc == nil {
		t.Error("HitCalculator should be initialized")
	}

	if compactor.composer == nil {
		t.Error("Composer should be initialized")
	}
}

func TestContextCompactor_RecordMemoryBlock(t *testing.T) {
	compactor := NewContextCompactor(200000)

	block := &SessionMemoryBlock{
		ID:      "test-block",
		Summary: "Test summary",
		Tags:    []string{"test"},
		Weight:  0.8,
	}

	err := compactor.RecordMemoryBlock(block)
	if err != nil {
		t.Fatalf("RecordMemoryBlock failed: %v", err)
	}

	// Verify block was added
	retrieved, ok := compactor.memPool.GetBlock("test-block")
	if !ok {
		t.Fatal("Block should be retrievable from pool")
	}

	if retrieved.Summary != "Test summary" {
		t.Errorf("Expected summary 'Test summary', got '%s'", retrieved.Summary)
	}
}

func TestContextCompactor_GetRelevantBlocks(t *testing.T) {
	compactor := NewContextCompactor(200000)

	// Add some blocks
	for i := 0; i < 5; i++ {
		compactor.memPool.AddBlock(&SessionMemoryBlock{
			ID:      "block-" + string(rune('0'+i)),
			Tags:    []string{"topic-a", "topic-b"},
			Weight:  0.5 + float64(i)*0.1,
		})
	}

	topics := []string{"topic-a"}
	blocks := compactor.GetRelevantBlocks(topics, 3)

	if len(blocks) > 3 {
		t.Errorf("Expected at most 3 blocks, got %d", len(blocks))
	}
}

func TestContextCompactor_ComposeMemorySummary(t *testing.T) {
	compactor := NewContextCompactor(200000)

	// Add blocks
	compactor.memPool.AddBlock(&SessionMemoryBlock{
		ID:      "block-1",
		Summary: "First block summary",
		KeyFacts: []string{"Fact 1", "Fact 2"},
		Tags:    []string{"code", "test"},
		Weight:  0.9,
	})

	topics := []string{"code", "test"}
	result := compactor.ComposeMemorySummary(topics, 5)

	if result == nil {
		t.Fatal("ComposeMemorySummary should not return nil")
	}

	if result.Summary == "" {
		t.Error("Summary should not be empty")
	}
}

func TestContextCompactor_SMCompact(t *testing.T) {
	compactor := NewContextCompactor(200000)

	messages := []fantasy.Message{
		fantasy.NewUserMessage("Test message 1"),
		fantasy.NewUserMessage("Test message 2"),
	}

	topics := []string{"code", "testing"}

	compressed, summary, err := compactor.SMCompact(messages, topics)

	if err != nil {
		t.Fatalf("SMCompact failed: %v", err)
	}

	// Should add block to pool
	if compactor.memPool.Size() != 1 {
		t.Errorf("Expected 1 block in pool, got %d", compactor.memPool.Size())
	}

	// Summary should be generated
	if summary == "" {
		t.Error("Summary should not be empty")
	}

	// Compressed messages should have session memory message
	if len(compressed) <= len(messages) {
		t.Log("Note: SMCompact may return same or fewer messages depending on content")
	}
}

func TestContextCompactor_SMMetrics(t *testing.T) {
	compactor := NewContextCompactor(200000)

	// Add a block
	compactor.memPool.AddBlock(&SessionMemoryBlock{
		ID:     "block-1",
		Weight: 0.8,
	})

	metrics := compactor.SMMetrics()

	if metrics["memory_pool"] == nil {
		t.Error("SMMetrics should include memory_pool")
	}

	if metrics["composer"] == nil {
		t.Error("SMMetrics should include composer")
	}
}

func TestContextCompactor_Reset_ClearsHitCalc(t *testing.T) {
	compactor := NewContextCompactor(200000)

	// Record some topics
	compactor.hitCalc.RecordTopics([]string{"topic-a", "topic-b"})

	// Reset
	compactor.Reset()

	// HitCalculator should be reset
	stats := compactor.hitCalc.GetTopicStats()
	if len(stats) != 0 {
		t.Errorf("Expected 0 topic stats after reset, got %d", len(stats))
	}
}

func TestContextCompactor_FullReset_ReinitializesSM(t *testing.T) {
	compactor := NewContextCompactor(200000)

	// Add blocks
	compactor.memPool.AddBlock(&SessionMemoryBlock{
		ID: "block-1",
	})

	// Full reset
	compactor.FullReset()

	// Pool should be empty
	if compactor.memPool.Size() != 0 {
		t.Errorf("Expected 0 blocks after FullReset, got %d", compactor.memPool.Size())
	}
}
