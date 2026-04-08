package context

import (
	"sync"
	"sync/atomic"
	"time"

	"charm.land/fantasy"
)

// SessionMemoryBlock represents a reusable memory block in the pool
type SessionMemoryBlock struct {
	ID         string
	Summary    string
	KeyFacts   []string
	Messages   []fantasy.Message
	CreatedAt  time.Time
	LastAccess time.Time
	AccessCount int64
	Weight     float64 // Importance weight for hit calculation
	Tags       []string // Topic tags for relevance matching
}

// SessionMemoryPool implements a pool of session memory blocks for fast SM compression
// This is the core component of SM Compression, enabling <10ms compression times
type SessionMemoryPool struct {
	mu sync.RWMutex

	// Pool of memory blocks
	blocks map[string]*SessionMemoryBlock

	// LRU tracking for eviction
	lruOrder []string

	// Configuration
	maxBlocks     int
	maxBlockSize  int // Max messages per block
	poolHitCount  int64
	poolMissCount int64

	// Eviction policy
	evictionPolicy EvictionPolicy
}

// EvictionPolicy defines how to evict blocks when pool is full
type EvictionPolicy int

const (
	EvictLRU EvictionPolicy = iota // Least Recently Used
	EvictLowestWeight              // Lowest weight
	EvictOldest                    // Oldest by creation time
)

// NewSessionMemoryPool creates a new session memory pool
func NewSessionMemoryPool(maxBlocks, maxBlockSize int) *SessionMemoryPool {
	return &SessionMemoryPool{
		blocks:         make(map[string]*SessionMemoryBlock),
		lruOrder:       make([]string, 0, maxBlocks),
		maxBlocks:      maxBlocks,
		maxBlockSize:   maxBlockSize,
		evictionPolicy: EvictLRU,
	}
}

// AddBlock adds a new memory block to the pool
func (p *SessionMemoryPool) AddBlock(block *SessionMemoryBlock) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if block == nil {
		return ErrNilBlock
	}

	if block.ID == "" {
		return ErrEmptyBlockID
	}

	// Check if block already exists
	if existing, ok := p.blocks[block.ID]; ok {
		// Update existing block
		existing.Summary = block.Summary
		existing.KeyFacts = block.KeyFacts
		existing.Messages = block.Messages
		existing.Weight = block.Weight
		existing.Tags = block.Tags
		existing.LastAccess = time.Now()
		atomic.AddInt64(&existing.AccessCount, 1)
		return nil
	}

	// Evict if necessary
	if len(p.blocks) >= p.maxBlocks {
		if err := p.evictOne(); err != nil {
			return err
		}
	}

	// Add new block
	block.CreatedAt = time.Now()
	block.LastAccess = block.CreatedAt
	block.AccessCount = 1
	p.blocks[block.ID] = block
	p.lruOrder = append(p.lruOrder, block.ID)

	return nil
}

// GetBlock retrieves a block by ID with LRU update
func (p *SessionMemoryPool) GetBlock(id string) (*SessionMemoryBlock, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	block, ok := p.blocks[id]
	if !ok {
		atomic.AddInt64(&p.poolMissCount, 1)
		return nil, false
	}

	// Update LRU
	p.updateLRU(id)
	block.LastAccess = time.Now()
	atomic.AddInt64(&block.AccessCount, 1)
	atomic.AddInt64(&p.poolHitCount, 1)

	return block, true
}

// GetBlocksByTags returns blocks matching any of the given tags
func (p *SessionMemoryPool) GetBlocksByTags(tags []string, limit int) []*SessionMemoryBlock {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var results []*SessionMemoryBlock
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}

	for _, block := range p.blocks {
		if len(results) >= limit {
			break
		}
		for _, blockTag := range block.Tags {
			if tagSet[blockTag] {
				results = append(results, block)
				break
			}
		}
	}

	return results
}

// GetBlocksByWeight returns top N blocks by weight
func (p *SessionMemoryPool) GetBlocksByWeight(limit int) []*SessionMemoryBlock {
	p.mu.RLock()
	defer p.mu.RUnlock()

	type weightedBlock struct {
		block *SessionMemoryBlock
		weight float64
	}

	var weighted []weightedBlock
	for _, block := range p.blocks {
		weighted = append(weighted, weightedBlock{block: block, weight: block.Weight})
	}

	// Sort by weight descending
	for i := 0; i < len(weighted); i++ {
		for j := i + 1; j < len(weighted); j++ {
			if weighted[j].weight > weighted[i].weight {
				weighted[i], weighted[j] = weighted[j], weighted[i]
			}
		}
	}

	result := make([]*SessionMemoryBlock, 0, limit)
	for i := 0; i < len(weighted) && i < limit; i++ {
		result = append(result, weighted[i].block)
	}

	return result
}

// GetRecentBlocks returns the N most recently accessed blocks
func (p *SessionMemoryPool) GetRecentBlocks(limit int) []*SessionMemoryBlock {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// lruOrder is most recent last
	start := len(p.lruOrder) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*SessionMemoryBlock, 0, limit)
	for i := start; i < len(p.lruOrder); i++ {
		if block, ok := p.blocks[p.lruOrder[i]]; ok {
			result = append(result, block)
		}
	}

	return result
}

// RemoveBlock removes a block from the pool
func (p *SessionMemoryPool) RemoveBlock(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.blocks[id]; !ok {
		return false
	}

	delete(p.blocks, id)

	// Update LRU order
	for i, lruID := range p.lruOrder {
		if lruID == id {
			p.lruOrder = append(p.lruOrder[:i], p.lruOrder[i+1:]...)
			break
		}
	}

	return true
}

// Clear removes all blocks from the pool
func (p *SessionMemoryPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.blocks = make(map[string]*SessionMemoryBlock)
	p.lruOrder = make([]string, 0, p.maxBlocks)
}

// Size returns the current number of blocks in the pool
func (p *SessionMemoryPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.blocks)
}

// TotalMessages returns the total number of messages across all blocks
func (p *SessionMemoryPool) TotalMessages() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0
	for _, block := range p.blocks {
		total += len(block.Messages)
	}
	return total
}

// PoolHitRate returns the pool hit rate as a percentage
func (p *SessionMemoryPool) PoolHitRate() float64 {
	hits := atomic.LoadInt64(&p.poolHitCount)
	misses := atomic.LoadInt64(&p.poolMissCount)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// updateLRU moves a block to the end of LRU order (most recent)
func (p *SessionMemoryPool) updateLRU(id string) {
	for i, lruID := range p.lruOrder {
		if lruID == id {
			p.lruOrder = append(p.lruOrder[:i], p.lruOrder[i+1:]...)
			p.lruOrder = append(p.lruOrder, id)
			return
		}
	}
}

// evictOne removes one block according to eviction policy
// Note: This method does NOT acquire a lock. Caller must hold the lock.
func (p *SessionMemoryPool) evictOne() error {
	if len(p.blocks) == 0 {
		return ErrPoolEmpty
	}

	var evictID string

	switch p.evictionPolicy {
	case EvictLRU:
		// Evict least recently used (first in LRU order)
		evictID = p.lruOrder[0]
	case EvictLowestWeight:
		// Find block with lowest weight
		evictID = p.findLowestWeightBlock()
	case EvictOldest:
		// Find oldest block by creation time
		evictID = p.findOldestBlock()
	default:
		evictID = p.lruOrder[0]
	}

	// Directly remove without locking (caller holds lock)
	delete(p.blocks, evictID)

	// Update LRU order
	for i, lruID := range p.lruOrder {
		if lruID == evictID {
			p.lruOrder = append(p.lruOrder[:i], p.lruOrder[i+1:]...)
			break
		}
	}

	return nil
}

// findLowestWeightBlock finds the block with the lowest weight
func (p *SessionMemoryPool) findLowestWeightBlock() string {
	var lowestID string
	var lowestWeight float64 = -1

	for id, block := range p.blocks {
		if lowestWeight < 0 || block.Weight < lowestWeight {
			lowestWeight = block.Weight
			lowestID = id
		}
	}

	return lowestID
}

// findOldestBlock finds the block with earliest creation time
func (p *SessionMemoryPool) findOldestBlock() string {
	var oldestID string
	var oldestTime time.Time

	for id, block := range p.blocks {
		if oldestTime.IsZero() || block.CreatedAt.Before(oldestTime) {
			oldestTime = block.CreatedAt
			oldestID = id
		}
	}

	return oldestID
}

// SetEvictionPolicy changes the eviction policy
func (p *SessionMemoryPool) SetEvictionPolicy(policy EvictionPolicy) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evictionPolicy = policy
}

// Metrics returns pool metrics for monitoring
func (p *SessionMemoryPool) Metrics() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	blockStats := make([]map[string]interface{}, 0, len(p.blocks))
	for id, block := range p.blocks {
		blockStats = append(blockStats, map[string]interface{}{
			"id":           id,
			"access_count": atomic.LoadInt64(&block.AccessCount),
			"weight":       block.Weight,
			"msg_count":    len(block.Messages),
			"tags":         block.Tags,
		})
	}

	return map[string]interface{}{
		"pool_size":       len(p.blocks),
		"max_blocks":      p.maxBlocks,
		"max_block_size":  p.maxBlockSize,
		"total_messages":  p.TotalMessages(),
		"pool_hit_rate":   p.PoolHitRate(),
		"pool_hits":       atomic.LoadInt64(&p.poolHitCount),
		"pool_misses":     atomic.LoadInt64(&p.poolMissCount),
		"eviction_policy": p.evictionPolicy,
		"blocks":          blockStats,
	}
}

// Pool errors
var (
	ErrNilBlock     = &PoolError{Message: "nil block provided"}
	ErrEmptyBlockID = &PoolError{Message: "empty block ID"}
	ErrPoolEmpty    = &PoolError{Message: "pool is empty"}
)

// PoolError represents a pool operation error
type PoolError struct {
	Message string
}

func (e *PoolError) Error() string {
	return e.Message
}
