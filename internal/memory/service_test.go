package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestMemoryServiceStoresAsMarkdownFiles(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	service, err := NewService(dataDir)
	require.NoError(t, err)

	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "test-entry", Value: "hello world", Scope: "project", Type: "fact"}))

	memoryDir := filepath.Join(dataDir, "memory")
	files, err := os.ReadDir(memoryDir)
	require.NoError(t, err)

	var mdFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".md") && f.Name() != indexFilename {
			mdFiles = append(mdFiles, f.Name())
		}
	}
	require.Len(t, mdFiles, 1)
	require.True(t, strings.HasSuffix(mdFiles[0], ".md"))

	content, err := os.ReadFile(filepath.Join(memoryDir, mdFiles[0]))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(content), "---\n"))
	require.Contains(t, string(content), "key: test-entry")
	require.Contains(t, string(content), "---\n\nhello world")
}

func TestMemoryServiceRebuildsIndex(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	service, err := NewService(dataDir)
	require.NoError(t, err)

	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "entry-one", Value: "first value"}))
	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "entry-two", Value: "second value"}))

	indexContent, err := service.ReadIndex()
	require.NoError(t, err)
	require.Contains(t, indexContent, "entry-one")
	require.Contains(t, indexContent, "entry-two")

	require.NoError(t, service.Delete(context.Background(), "entry-one"))

	indexContent, err = service.ReadIndex()
	require.NoError(t, err)
	require.NotContains(t, indexContent, "entry-one")
	require.Contains(t, indexContent, "entry-two")
}

func TestMemoryServiceListMemoryFiles(t *testing.T) {
	t.Parallel()

	service, err := NewService(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "k1", Value: "value one", Type: "user"}))
	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "k2", Value: "value two", Type: "feedback"}))

	infos, err := service.ListMemoryFiles()
	require.NoError(t, err)
	require.Len(t, infos, 2)
	require.Equal(t, "k2", infos[0].Key)
	require.Equal(t, "k1", infos[1].Key)
}

func TestMemoryServiceReadMemoryFileBody(t *testing.T) {
	t.Parallel()

	service, err := NewService(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, service.Store(context.Background(), StoreParams{Key: "body-test", Value: "the actual body content"}))

	infos, err := service.ListMemoryFiles()
	require.NoError(t, err)
	require.Len(t, infos, 1)

	body, err := service.ReadMemoryFileBody(infos[0].FileName)
	require.NoError(t, err)
	require.Equal(t, "the actual body content", body)

	_, err = service.ReadMemoryFileBody("../../../etc/passwd")
	require.ErrorContains(t, err, "invalid memory file path")
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

func TestMemoryServiceSanitizeFilename(t *testing.T) {
	t.Parallel()

	require.Contains(t, sanitizeFilename("hello world"), "hello_world")
	require.Contains(t, sanitizeFilename("path/to/file"), "path__to__file")
	require.Contains(t, sanitizeFilename("   "), "___")
	require.Contains(t, sanitizeFilename("colon:separated"), "colon-separated")

	require.NotEqual(t, sanitizeFilename("hello world"), sanitizeFilename("hello_world"))
	require.Contains(t, sanitizeFilename("hello world"), "_")
	require.Contains(t, sanitizeFilename("hello_world"), "_")
}

func TestMemoryServiceTruncateForDescription(t *testing.T) {
	t.Parallel()

	short := "short value"
	require.Equal(t, short, truncateForDescription(short))

	long := strings.Repeat("a", 200)
	result := truncateForDescription(long)
	require.Len(t, []rune(result), 121)
	require.True(t, strings.HasSuffix(result, "…"))
}
