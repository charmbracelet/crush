package critic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
)

// FeedbackCache deduplicates critic rounds for identical diffs and diagnostics.
type FeedbackCache struct {
	cache *lru.Cache[string, *CriticFeedback]
	hits  atomic.Uint64
	misses atomic.Uint64
}

// NewFeedbackCache creates an LRU cache with the given size.
func NewFeedbackCache(size int) (*FeedbackCache, error) {
	c, err := lru.New[string, *CriticFeedback](size)
	if err != nil {
		return nil, err
	}
	return &FeedbackCache{cache: c}, nil
}

// Stats returns the number of cache hits and misses.
func (fc *FeedbackCache) Stats() (hits, misses int) {
	return int(fc.hits.Load()), int(fc.misses.Load())
}

// Key computes a deterministic hash for a checkpoint.
func (fc *FeedbackCache) Key(cp Checkpoint) string {
	h := sha256.New()
	h.Write([]byte(cp.Type))
	h.Write([]byte(cp.UserPrompt))
	h.Write([]byte(cp.PrimaryPlan))
	h.Write([]byte(cp.PrimaryDiff))

	if b, err := json.Marshal(cp.ToolCalls); err == nil {
		h.Write(b)
	}
	if b, err := json.Marshal(cp.LSPDiagnostics); err == nil {
		h.Write(b)
	}

	h.Write([]byte{byte(cp.Iteration >> 24), byte(cp.Iteration >> 16), byte(cp.Iteration >> 8), byte(cp.Iteration)})
	return hex.EncodeToString(h.Sum(nil))
}

// Get looks up cached feedback for a checkpoint.
func (fc *FeedbackCache) Get(cp Checkpoint) (*CriticFeedback, bool) {
	if fc.cache == nil {
		fc.misses.Add(1)
		return nil, false
	}
	fb, ok := fc.cache.Get(fc.Key(cp))
	if ok {
		fc.hits.Add(1)
	} else {
		fc.misses.Add(1)
	}
	return fb, ok
}

// Put stores feedback for a checkpoint.
func (fc *FeedbackCache) Put(cp Checkpoint, fb *CriticFeedback) {
	if fc.cache == nil {
		return
	}
	fc.cache.Add(fc.Key(cp), fb)
}
