// Package cache provides centralized caching with modern libraries and strategies.
// It supports different cache types optimized for specific use cases:
// - Otter v2: High-performance caching with TTL and eviction
// - Theine: Zero-GC caching for UI components
// - LRU: Size-bounded caching for configuration data
package cache

import (
	"context"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
)

// CacheStats provides metrics for cache performance monitoring
type CacheStats struct {
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	Items       uint64
	MemoryBytes uint64
	HitRate     float64
}

// CacheManager provides unified interface for different cache types
type CacheManager interface {
	// Session caching - uses Otter v2 for high performance
	GetSession(ctx context.Context, id string) (*session.Session, bool, error)
	SetSession(ctx context.Context, id string, session *session.Session, ttl time.Duration) error
	DeleteSession(ctx context.Context, id string) error

	// File content caching - uses Otter v2 with cost-based eviction
	GetFile(ctx context.Context, path string) (*FileCacheEntry, bool, error)
	SetFile(ctx context.Context, path, content string, ttl time.Duration) error
	DeleteFile(ctx context.Context, path string) error

	// UI component caching - uses Theine for zero-GC
	GetUIComponent(key string) (any, bool)
	SetUIComponent(key string, value any, ttl time.Duration)

	// Configuration caching - uses LRU for size-bounded
	GetConfig(key string) (*config.ProviderConfig, bool)
	SetConfig(key string, providerConfig *config.ProviderConfig)

	// Provider caching - uses Otter v2 for session data
	GetProvider(ctx context.Context, sessionID string) (*config.ProviderConfig, bool, error)
	SetProvider(ctx context.Context, sessionID string, provider *config.ProviderConfig, ttl time.Duration) error

	// Cache management
	Stats() CacheStats
	Reset()
	Close() error

	// Debug and monitoring
	GetSessionStats() any
	GetFileStats() any
	GetUIStats() any
	GetConfigStats() any
}

// FileCacheEntry represents cached file content with metadata
type FileCacheEntry struct {
	Content   string
	UpdatedAt time.Time
	Version   int64
	ETag      string
	Size      int
	Checksum  string
	ExpiresAt time.Time
}

// SessionCacheEntry represents cached session with metadata
type SessionCacheEntry struct {
	Session     *session.Session
	UpdatedAt   time.Time
	LastUsed    time.Time
	AccessCount int64
	ExpiresAt   time.Time
}

// ProviderCacheEntry represents cached provider configuration
type ProviderCacheEntry struct {
	Provider  *config.ProviderConfig
	UpdatedAt time.Time
	LastUsed  time.Time
	ExpiresAt time.Time
}

// uiCacheEntry represents UI cache entry with TTL
type uiCacheEntry struct {
	Value     any
	ExpiresAt time.Time
}

// CacheConfig holds configuration for cache manager
type CacheConfig struct {
	// Session cache settings
	SessionMaxSize    int
	SessionDefaultTTL time.Duration

	// File cache settings
	FileMaxSize    int
	FileDefaultTTL time.Duration
	FileMaxCost    uint32

	// UI cache settings
	UILimit      int
	UIDefaultTTL time.Duration

	// Config cache settings
	ConfigMaxSize int

	// Global settings
	EnableMetrics   bool
	EnableStats     bool
	CleanupInterval time.Duration
}

// DefaultConfig returns sensible defaults for cache configuration
func DefaultConfig() CacheConfig {
	return CacheConfig{
		SessionMaxSize:    10000,
		SessionDefaultTTL: 30 * time.Minute,
		FileMaxSize:       5000,
		FileDefaultTTL:    5 * time.Minute,
		FileMaxCost:       1024 * 1024, // 1MB max cost per entry
		UILimit:           1000,
		UIDefaultTTL:      time.Minute,
		ConfigMaxSize:     1000,
		EnableMetrics:     true,
		EnableStats:       true,
		CleanupInterval:   10 * time.Minute,
	}
}

// Feature flags for gradual rollout
type Features struct {
	EnableSessionCache  bool
	EnableFileCache     bool
	EnableUICache       bool
	EnableConfigCache   bool
	EnableProviderCache bool
}

// DefaultFeatures returns conservative feature flags for gradual rollout
func DefaultFeatures() Features {
	return Features{
		EnableSessionCache:  true,  // Highest impact, lowest risk
		EnableFileCache:     true,  // Medium impact, medium risk
		EnableUICache:       false, // Lower impact, higher complexity
		EnableConfigCache:   false, // Lower impact, needs validation
		EnableProviderCache: true,  // Medium impact, medium risk
	}
}

// AllFeatures enables all caching features
func AllFeatures() Features {
	return Features{
		EnableSessionCache:  true,
		EnableFileCache:     true,
		EnableUICache:       true,
		EnableConfigCache:   true,
		EnableProviderCache: true,
	}
}
