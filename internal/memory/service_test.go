package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryServiceStoreGetDelete(t *testing.T) {
	t.Parallel()

	service, err := NewService(t.TempDir())
	require.NoError(t, err)

	err = service.Store(context.Background(), StoreParams{Key: "project/goal", Value: "Ship MVP", Scope: "project", Category: "product", Type: "goal", Tags: []string{"roadmap", "launch"}})
	require.NoError(t, err)

	entry, err := service.Get(context.Background(), "project/goal")
	require.NoError(t, err)
	require.Equal(t, "project/goal", entry.Key)
	require.Equal(t, "Ship MVP", entry.Value)
	require.Equal(t, "project", entry.Scope)
	require.Equal(t, "product", entry.Category)
	require.Equal(t, "goal", entry.Type)
	require.Equal(t, []string{"launch", "roadmap"}, entry.Tags)
	require.NotZero(t, entry.UpdatedAt)

	err = service.Delete(context.Background(), "project/goal")
	require.NoError(t, err)

	_, err = service.Get(context.Background(), "project/goal")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestMemoryServiceSearchAndList(t *testing.T) {
	t.Parallel()

	service, err := NewService(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "alpha", Value: "first memory", Scope: "project", Category: "notes", Type: "fact", Tags: []string{"alpha"}}))
	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "beta", Value: "second memory", Scope: "project", Category: "preferences", Type: "workflow", Tags: []string{"golang", "tests"}}))
	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "session-note", Value: "beta plan", Scope: "session", Category: "notes", Type: "plan", Tags: []string{"beta", "tests"}}))

	searchResults, err := service.Search(context.Background(), SearchParams{Query: "beta", Limit: 10})
	require.NoError(t, err)
	require.Len(t, searchResults, 2)
	require.Equal(t, "session-note", searchResults[0].Key)
	require.Equal(t, "beta", searchResults[1].Key)

	metadataSearch, err := service.Search(context.Background(), SearchParams{Query: "workflow", Type: "workflow", Tags: []string{"golang"}, Limit: 10})
	require.NoError(t, err)
	require.Len(t, metadataSearch, 1)
	require.Equal(t, "beta", metadataSearch[0].Key)

	projectOnly, err := service.Search(context.Background(), SearchParams{Query: "beta", Scope: "project", Limit: 10})
	require.NoError(t, err)
	require.Len(t, projectOnly, 1)
	require.Equal(t, "beta", projectOnly[0].Key)

	listResults, err := service.List(context.Background(), ListParams{Scope: "project", Limit: 2})
	require.NoError(t, err)
	require.Len(t, listResults, 2)
	require.Equal(t, "beta", listResults[0].Key)
	require.Equal(t, "alpha", listResults[1].Key)

	tagFiltered, err := service.List(context.Background(), ListParams{Category: "notes", Tags: []string{"beta"}, Limit: 10})
	require.NoError(t, err)
	require.Len(t, tagFiltered, 1)
	require.Equal(t, "session-note", tagFiltered[0].Key)
}

func TestMemoryServiceReadsLegacyStoreFormat(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	memoryDir := filepath.Join(dataDir, "memory")
	require.NoError(t, os.MkdirAll(memoryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(memoryDir, "entries.json"), []byte(`{"entries":{"legacy":{"key":"legacy","value":"old value","scope":"project","updated_at":123}}}`), 0o644))

	service, err := NewService(dataDir)
	require.NoError(t, err)

	entry, err := service.Get(context.Background(), "legacy")
	require.NoError(t, err)
	require.Equal(t, "legacy", entry.Key)
	require.Equal(t, "old value", entry.Value)
	require.Equal(t, "project", entry.Scope)
	require.Empty(t, entry.Category)
	require.Empty(t, entry.Type)
	require.Nil(t, entry.Tags)
}

func TestMemoryServiceWritesAuditLog(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	service, err := NewService(dataDir)
	require.NoError(t, err)

	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "k1", Value: "v1", Scope: "project"}))
	require.NoError(t, service.Delete(context.Background(), "k1"))

	auditFile := filepath.Join(dataDir, "memory", "audit.log")
	content, err := os.ReadFile(auditFile)
	require.NoError(t, err)
	require.Contains(t, string(content), `"action":"store"`)
	require.Contains(t, string(content), `"action":"delete"`)
	require.Contains(t, string(content), `"key":"k1"`)
}

func TestMemoryServiceValidation(t *testing.T) {
	t.Parallel()

	service, err := NewService(t.TempDir())
	require.NoError(t, err)

	err = service.Store(context.Background(), StoreParams{Key: " ", Value: "value"})
	require.ErrorContains(t, err, "key is required")

	err = service.Store(context.Background(), StoreParams{Key: "key", Value: " "})
	require.ErrorContains(t, err, "value is required")

	_, err = service.Search(context.Background(), SearchParams{Query: " ", Limit: 10})
	require.ErrorContains(t, err, "query is required")

	err = service.Delete(context.Background(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestMemoryServiceContextCancellation(t *testing.T) {
	t.Parallel()

	service, err := NewService(t.TempDir())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = service.Store(ctx, StoreParams{Key: "key", Value: "value"})
	require.True(t, errors.Is(err, context.Canceled))
}
