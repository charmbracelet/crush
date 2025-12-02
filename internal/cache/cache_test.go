package cache

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/session"
)

// TestCacheManagerInterface ensures implementation satisfies interface
func TestCacheManagerInterface(t *testing.T) {
	factory := CacheManagerFactory{}
	cm := factory.CreateCacheManager()

	// Test that we can use the interface
	_, _, err := cm.GetSession(context.Background(), "test")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if err := cm.SetSession(context.Background(), "test", &session.Session{}, time.Minute); err != nil {
		t.Fatalf("SetSession failed: %v", err)
	}

	if _, ok := cm.GetUIComponent("test"); ok {
		t.Fatal("GetUIComponent should return false for non-existent key")
	}

	// GetSession for non-existing key = 1 miss.
	// GetUIComponent doesn't track stats when EnableUICache=false (default).
	stats := cm.Stats()
	if stats.Hits != 0 || stats.Misses != 1 {
		t.Fatalf("Unexpected stats: %+v", stats)
	}

	if err := cm.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// BenchmarkCacheManagerVsDirect compares cache vs direct access
func BenchmarkCacheManagerVsDirect(b *testing.B) {
	factory := CacheManagerFactory{}
	cm := factory.CreateCacheManager()
	defer cm.Close()

	ctx := context.Background()
	testSession := &session.Session{
		ID:    "test-session",
		Title: "Test Session",
	}

	// Warm up cache
	cm.SetSession(ctx, "bench", testSession, time.Hour)

	b.Run("CacheHit", func(b *testing.B) {
		for b.Loop() {
			_, ok, err := cm.GetSession(ctx, "bench")
			if err != nil || !ok {
				b.Fatal("Cache hit failed")
			}
		}
	})

	b.Run("DirectAccess", func(b *testing.B) {
		for b.Loop() {
			// Simulate direct access cost
			session := &session.Session{
				ID:    "test-session",
				Title: "Test Session",
			}
			_ = session // Use the value
		}
	})
}

func TestCacheConfiguration(t *testing.T) {
	cfg := DefaultConfig()
	features := DefaultFeatures()

	if cfg.SessionMaxSize != 10000 {
		t.Fatalf("Expected SessionMaxSize=10000, got %d", cfg.SessionMaxSize)
	}

	if !features.EnableSessionCache {
		t.Fatal("Expected EnableSessionCache=true")
	}

	if features.EnableUICache {
		t.Fatal("Expected EnableUICache=false for gradual rollout")
	}
}

func TestCacheFeatures(t *testing.T) {
	// Test conservative feature set
	features := DefaultFeatures()
	if !features.EnableSessionCache || !features.EnableFileCache || !features.EnableProviderCache {
		t.Fatal("Expected core features to be enabled")
	}

	if features.EnableUICache || features.EnableConfigCache {
		t.Fatal("Expected advanced features to be disabled initially")
	}

	// Test all features enabled
	allFeatures := AllFeatures()
	if !allFeatures.EnableUICache || !allFeatures.EnableConfigCache {
		t.Fatal("Expected all features to be enabled")
	}
}
