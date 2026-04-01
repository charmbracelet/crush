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

	err = service.Store(context.Background(), "project/goal", "Ship MVP")
	require.NoError(t, err)

	entry, err := service.Get(context.Background(), "project/goal")
	require.NoError(t, err)
	require.Equal(t, "project/goal", entry.Key)
	require.Equal(t, "Ship MVP", entry.Value)
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

	require.NoError(t, service.Store(context.Background(), "alpha", "first memory"))
	require.NoError(t, service.Store(context.Background(), "beta", "second memory"))
	require.NoError(t, service.Store(context.Background(), "project", "beta plan"))

	searchResults, err := service.Search(context.Background(), "beta", 10)
	require.NoError(t, err)
	require.Len(t, searchResults, 2)
	require.Equal(t, "project", searchResults[0].Key)
	require.Equal(t, "beta", searchResults[1].Key)

	listResults, err := service.List(context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, listResults, 2)
	require.Equal(t, "project", listResults[0].Key)
	require.Equal(t, "beta", listResults[1].Key)
}

func TestMemoryServiceWritesAuditLog(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	service, err := NewService(dataDir)
	require.NoError(t, err)

	require.NoError(t, service.Store(context.Background(), "k1", "v1"))
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

	err = service.Store(context.Background(), " ", "value")
	require.ErrorContains(t, err, "key is required")

	err = service.Store(context.Background(), "key", " ")
	require.ErrorContains(t, err, "value is required")

	_, err = service.Search(context.Background(), " ", 10)
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

	err = service.Store(ctx, "key", "value")
	require.True(t, errors.Is(err, context.Canceled))
}
