package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/session"
)

// CacheManagerFactory creates appropriate cache manager based on configuration
type CacheManagerFactory struct{}

// CreateCacheManager creates a cache manager with safe defaults
func (f CacheManagerFactory) CreateCacheManager() CacheManager {
	cfg := DefaultConfig()
	features := DefaultFeatures()

	return NewSafeCacheManager(cfg, features)
}

// NewSafeCacheManager creates a cache manager that uses existing csync.Map foundation
func NewSafeCacheManager(cfg CacheConfig, features Features) CacheManager {
	return &safeCacheManager{
		config:        cfg,
		features:      features,
		sessionCache:  csync.NewMap[string, *SessionCacheEntry](),
		fileCache:     csync.NewMap[string, *FileCacheEntry](),
		providerCache: csync.NewMap[string, *ProviderCacheEntry](),
		uiCache:       csync.NewMap[string, uiCacheEntry](),
		configCache:   csync.NewMap[string, *config.ProviderConfig](),
		cleanupDone:   make(chan struct{}),
		hits:          0,
		misses:        0,
	}
}

// safeCacheManager implements CacheManager using proven csync.Map foundation
type safeCacheManager struct {
	config   CacheConfig
	features Features

	// Use existing csync.Map for reliability
	sessionCache  *csync.Map[string, *SessionCacheEntry]
	fileCache     *csync.Map[string, *FileCacheEntry]
	providerCache *csync.Map[string, *ProviderCacheEntry]
	uiCache       *csync.Map[string, uiCacheEntry]
	configCache   *csync.Map[string, *config.ProviderConfig]

	// Stats tracking
	hits   uint64
	misses uint64

	// Cleanup
	cleanupDone chan struct{}
	closeOnce   sync.Once
}

// Session caching methods
func (cm *safeCacheManager) GetSession(ctx context.Context, id string) (*session.Session, bool, error) {
	if !cm.features.EnableSessionCache {
		return nil, false, nil
	}

	entry, ok := cm.sessionCache.Get(id)
	if !ok {
		atomic.AddUint64(&cm.misses, 1)
		return nil, false, nil
	}

	// Check TTL
	if time.Now().After(entry.ExpiresAt) {
		cm.sessionCache.Del(id)
		atomic.AddUint64(&cm.misses, 1)
		return nil, false, nil
	}

	atomic.AddUint64(&cm.hits, 1)
	return entry.Session, true, nil
}

func (cm *safeCacheManager) SetSession(ctx context.Context, id string, session *session.Session, ttl time.Duration) error {
	if !cm.features.EnableSessionCache {
		return nil
	}

	entry := &SessionCacheEntry{
		Session:     session,
		UpdatedAt:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 0,
		ExpiresAt:   time.Now().Add(ttl),
	}

	cm.sessionCache.Set(id, entry)
	return nil
}

func (cm *safeCacheManager) DeleteSession(ctx context.Context, id string) error {
	if !cm.features.EnableSessionCache {
		return nil
	}

	cm.sessionCache.Del(id)
	return nil
}

// File caching methods
func (cm *safeCacheManager) GetFile(ctx context.Context, path string) (*FileCacheEntry, bool, error) {
	if !cm.features.EnableFileCache {
		return nil, false, nil
	}

	entry, ok := cm.fileCache.Get(path)
	if !ok {
		atomic.AddUint64(&cm.misses, 1)
		return nil, false, nil
	}

	// Check TTL
	if time.Now().After(entry.ExpiresAt) {
		cm.fileCache.Del(path)
		atomic.AddUint64(&cm.misses, 1)
		return nil, false, nil
	}

	atomic.AddUint64(&cm.hits, 1)
	return entry, true, nil
}

func (cm *safeCacheManager) SetFile(ctx context.Context, path, content string, ttl time.Duration) error {
	if !cm.features.EnableFileCache {
		return nil
	}

	entry := &FileCacheEntry{
		Content:   content,
		UpdatedAt: time.Now(),
		Version:   0,
		ETag:      "",
		Size:      len(content),
		Checksum:  "",
		ExpiresAt: time.Now().Add(ttl),
	}

	cm.fileCache.Set(path, entry)
	return nil
}

func (cm *safeCacheManager) DeleteFile(ctx context.Context, path string) error {
	if !cm.features.EnableFileCache {
		return nil
	}

	cm.fileCache.Del(path)
	return nil
}

// UI caching methods
func (cm *safeCacheManager) GetUIComponent(key string) (any, bool) {
	if !cm.features.EnableUICache {
		return nil, false
	}

	entry, ok := cm.uiCache.Get(key)
	if !ok {
		atomic.AddUint64(&cm.misses, 1)
		return nil, false
	}

	// Check TTL
	if time.Now().After(entry.ExpiresAt) {
		cm.uiCache.Del(key)
		atomic.AddUint64(&cm.misses, 1)
		return nil, false
	}

	atomic.AddUint64(&cm.hits, 1)
	return entry.Value, true
}

func (cm *safeCacheManager) SetUIComponent(key string, value any, ttl time.Duration) {
	if !cm.features.EnableUICache {
		return
	}

	entry := uiCacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}

	cm.uiCache.Set(key, entry)
}

// Configuration caching methods
func (cm *safeCacheManager) GetConfig(key string) (*config.ProviderConfig, bool) {
	if !cm.features.EnableConfigCache {
		return nil, false
	}

	value, ok := cm.configCache.Get(key)
	if !ok {
		atomic.AddUint64(&cm.misses, 1)
		return nil, false
	}

	atomic.AddUint64(&cm.hits, 1)
	return value, true
}

func (cm *safeCacheManager) SetConfig(key string, providerConfig *config.ProviderConfig) {
	if !cm.features.EnableConfigCache {
		return
	}

	cm.configCache.Set(key, providerConfig)
}

// Provider caching methods
func (cm *safeCacheManager) GetProvider(ctx context.Context, sessionID string) (*config.ProviderConfig, bool, error) {
	if !cm.features.EnableProviderCache {
		return nil, false, nil
	}

	entry, ok := cm.providerCache.Get(sessionID)
	if !ok {
		atomic.AddUint64(&cm.misses, 1)
		return nil, false, nil
	}

	// Check TTL
	if time.Now().After(entry.ExpiresAt) {
		cm.providerCache.Del(sessionID)
		atomic.AddUint64(&cm.misses, 1)
		return nil, false, nil
	}

	atomic.AddUint64(&cm.hits, 1)
	return entry.Provider, true, nil
}

func (cm *safeCacheManager) SetProvider(ctx context.Context, sessionID string, provider *config.ProviderConfig, ttl time.Duration) error {
	if !cm.features.EnableProviderCache {
		return nil
	}

	entry := &ProviderCacheEntry{
		Provider:  provider,
		UpdatedAt: time.Now(),
		LastUsed:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}

	cm.providerCache.Set(sessionID, entry)
	return nil
}

// Stats and monitoring
func (cm *safeCacheManager) Stats() CacheStats {
	hits := atomic.LoadUint64(&cm.hits)
	misses := atomic.LoadUint64(&cm.misses)

	stats := CacheStats{
		Hits:    hits,
		Misses:  misses,
		Items:   uint64(cm.sessionCache.Len() + cm.fileCache.Len() + cm.providerCache.Len()),
		HitRate: 0,
	}

	// Calculate hit rate
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total) * 100
	}

	return stats
}

func (cm *safeCacheManager) GetSessionStats() any {
	return map[string]interface{}{
		"items": cm.sessionCache.Len(),
		"type":  "csync.Map",
	}
}

func (cm *safeCacheManager) GetFileStats() any {
	return map[string]interface{}{
		"items": cm.fileCache.Len(),
		"type":  "csync.Map",
	}
}

func (cm *safeCacheManager) GetUIStats() any {
	return map[string]interface{}{
		"items": cm.uiCache.Len(),
		"type":  "csync.Map",
	}
}

func (cm *safeCacheManager) GetConfigStats() any {
	return map[string]interface{}{
		"items": cm.configCache.Len(),
		"type":  "csync.Map",
	}
}

func (cm *safeCacheManager) Reset() {
	cm.sessionCache.Reset(make(map[string]*SessionCacheEntry))
	cm.fileCache.Reset(make(map[string]*FileCacheEntry))
	cm.providerCache.Reset(make(map[string]*ProviderCacheEntry))
	cm.uiCache.Reset(make(map[string]uiCacheEntry))
	cm.configCache.Reset(make(map[string]*config.ProviderConfig))

	atomic.StoreUint64(&cm.hits, 0)
	atomic.StoreUint64(&cm.misses, 0)
}

func (cm *safeCacheManager) Close() error {
	cm.closeOnce.Do(func() {
		close(cm.cleanupDone)
		cm.Reset()
	})
	return nil
}
