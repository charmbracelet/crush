package config

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

// blockingFetch serves name, after blocking until release closes when
// blocked is set.
func blockingFetch(name string, entered chan<- struct{}, release <-chan struct{}) fetchFunc[[]catwalk.Provider] {
	return func(ctx context.Context, _ string) ([]catwalk.Provider, []byte, error) {
		if entered != nil {
			entered <- struct{}{}
			<-release
		}
		v := []catwalk.Provider{{Name: name, ID: catwalk.InferenceProvider(name)}}
		raw, err := json.Marshal(v)
		return v, raw, err
	}
}

func cachedSyncer(t *testing.T, fetch fetchFunc[[]catwalk.Provider]) *syncer[[]catwalk.Provider] {
	t.Helper()
	path := t.TempDir() + "/providers.json"
	require.NoError(t, os.WriteFile(path, []byte(`[{"name":"cached","id":"cached"}]`), 0o644))
	s := newCatwalkSync()
	s.Init(fetch, path, true)
	_, err := s.Get(t.Context())
	require.NoError(t, err)
	return s
}

func TestSyncer_RevalidateDoesNotBlockReaders(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	s := cachedSyncer(t, blockingFetch("revalidated", entered, release))

	done := make(chan bool)
	go func() {
		changed, _ := s.Revalidate(t.Context())
		done <- changed
	}()
	<-entered

	got, err := s.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "cached", got[0].Name, "Get answers while a revalidation is in flight")

	s.SetFetch(blockingFetch("refetched", nil, nil))
	got, err = s.Refetch(t.Context())
	require.NoError(t, err)
	require.Equal(t, "refetched", got[0].Name, "Refetch does not wait on the revalidation")

	close(release)
	select {
	case changed := <-done:
		require.False(t, changed, "a revalidation superseded by Refetch reports nothing")
	case <-time.After(5 * time.Second):
		t.Fatal("revalidation never finished")
	}
	got, _ = s.Get(t.Context())
	require.Equal(t, "refetched", got[0].Name, "the older revalidation does not overwrite Refetch")
	onDisk, _, err := s.cache.Get()
	require.NoError(t, err)
	require.Equal(t, "refetched", onDisk[0].Name, "nor the cache the next startup reads")
}

func TestSyncer_RefetchWithoutAutoUpdateKeepsTheCache(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/providers.json"
	require.NoError(t, os.WriteFile(path, []byte(`[{"name":"cached","id":"cached"}]`), 0o644))
	s := newCatwalkSync()
	s.Init(blockingFetch("fetched", nil, nil), path, false)
	got, err := s.Refetch(t.Context())
	require.NoError(t, err)
	require.Equal(t, "cached", got[0].Name, "Refetch serves what Get would, not the embedded copy")
}
