package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/x/etag"
)

// fetchFunc fetches a document, returning it with the raw body it was
// decoded from, or catwalk.ErrNotModified while etag still matches.
type fetchFunc[T any] func(ctx context.Context, etag string) (T, []byte, error)

// syncer serves a remote document from cache, refreshed by Revalidate. Only
// Get's cold start, with nothing else to show, holds the lock across a fetch.
type syncer[T any] struct {
	name     string
	valid    func(T) bool
	embedded func() T

	mu         sync.Mutex
	fetch      fetchFunc[T]
	cache      cache[T]
	autoupdate bool
	ready      bool // Init has run.
	loaded     bool
	value      T
	etag       string // ETag of the cached document.
	err        error
	stale      bool // value came from the cache and awaits Revalidate.
	news       bool // A fetched document awaits Revalidate's report.
	gen        int  // Bumped on every replacement of value.
}

func newCatwalkSync() *syncer[[]catwalk.Provider] {
	return &syncer[[]catwalk.Provider]{
		name:     "Catwalk providers",
		valid:    func(p []catwalk.Provider) bool { return len(p) > 0 },
		embedded: embeddedCatwalk,
	}
}

func newHyperSync() *syncer[catwalk.Provider] {
	return &syncer[catwalk.Provider]{
		name:     "Hyper provider",
		valid:    func(p catwalk.Provider) bool { return p.ID != "" && len(p.Models) > 0 },
		embedded: embeddedHyper,
	}
}

func (s *syncer[T]) Init(fetch fetchFunc[T], path string, autoupdate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetch, s.cache, s.autoupdate, s.ready = fetch, newCache[T](path), autoupdate, true
}

// SetFetch replaces the fetcher, e.g. to pick up new credentials.
func (s *syncer[T]) SetFetch(fetch fetchFunc[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetch = fetch
}

// Initialized reports whether Init has run.
func (s *syncer[T]) Initialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready
}

// generation identifies the current document; it changes whenever it does.
func (s *syncer[T]) generation() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gen
}

// Get returns the document. The first call loads it: from the cache when
// usable, otherwise embedded when auto-update is off, otherwise fetched.
func (s *syncer[T]) Get(ctx context.Context) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		panic("called Get before Init")
	}
	if s.loaded {
		return s.value, s.err
	}
	s.loaded = true
	if s.loadLocal() || !s.autoupdate {
		s.stale = s.autoupdate
		return s.value, s.err
	}
	v, raw, err := s.fetchFrom(ctx, s.fetch, s.embedded(), "")
	s.err = err
	if raw != nil {
		s.apply(v, raw, true)
	} else {
		s.value = v
	}
	return s.value, s.err
}

// loadLocal sets value from a usable cache, reporting true, or else to the
// embedded copy. Caller must hold mu.
func (s *syncer[T]) loadLocal() bool {
	cached, etag, err := s.cache.Get()
	if err == nil && s.valid(cached) {
		slog.Info("Using cached " + s.name)
		s.value, s.etag = cached, etag
		return true
	}
	slog.Info("Using embedded " + s.name)
	s.value = s.embedded()
	return false
}

// apply keeps a fetched document and its raw body, in memory and on disk, and
// whether Revalidate should report it. Caller must hold mu.
func (s *syncer[T]) apply(v T, raw []byte, news bool) {
	s.value, s.etag, s.news = v, etag.Of(raw), s.news || news
	s.gen++
	if err := s.cache.StoreRaw(raw); err != nil {
		s.err = err
	}
}

// Revalidate refreshes a cached document, and reports whether a fetched
// document, including a cold start's, arrived since the last call. Each one
// is reported once.
func (s *syncer[T]) Revalidate(ctx context.Context) (bool, error) {
	var err error
	s.mu.Lock()
	if s.ready && s.stale {
		s.stale = false
		fetch, fallback, tag, gen := s.fetch, s.value, s.etag, s.gen
		s.mu.Unlock()
		var v T
		var raw []byte
		v, raw, err = s.fetchFrom(ctx, fetch, fallback, tag)
		s.mu.Lock()
		if raw != nil && s.gen == gen { // A Refetch meanwhile wins.
			s.apply(v, raw, true)
		}
	}
	defer s.mu.Unlock()
	news := s.news
	s.news = false
	return news, err
}

// Refetch fetches the document now and keeps the result. Used after signing
// in, when the document may differ per account.
func (s *syncer[T]) Refetch(ctx context.Context) (T, error) {
	s.mu.Lock()
	if !s.ready {
		panic("called Refetch before Init")
	}
	s.loaded, s.stale, s.err = true, false, nil
	if !s.autoupdate {
		defer s.mu.Unlock()
		s.loadLocal()
		s.gen++
		return s.value, nil
	}
	fetch, fallback, tag := s.fetch, s.value, s.etag
	if !s.valid(fallback) {
		fallback, tag = s.embedded(), ""
	}
	s.mu.Unlock()

	v, raw, err := s.fetchFrom(ctx, fetch, fallback, tag)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
	if raw != nil {
		s.apply(v, raw, false) // The caller acts on the result itself.
	} else {
		s.value = v
		s.gen++
	}
	return s.value, s.err
}

// fetchFrom fetches the document, returning its raw body when it is fresh and
// usable, or fallback when unchanged, unreachable or unusable. Being offline
// is routine, so it is not an error.
func (s *syncer[T]) fetchFrom(ctx context.Context, fetch fetchFunc[T], fallback T, tag string) (T, []byte, error) {
	slog.Info("Fetching " + s.name)
	v, raw, err := fetch(ctx, tag)
	switch {
	case errors.Is(err, catwalk.ErrNotModified):
		slog.Info(s.name + " not modified")
		return fallback, nil, nil
	case err != nil:
		slog.Warn("Could not fetch "+s.name, "error", err)
		return fallback, nil, nil
	case !s.valid(v):
		return fallback, nil, fmt.Errorf("empty %s from server", s.name)
	}
	return v, raw, nil
}

// getWithETag sends req with an If-None-Match for tag and decodes a JSON
// body, returning the raw bytes too so a cache can keep the server's ETag.
func getWithETag[T any](client *http.Client, req *http.Request, tag string) (T, []byte, error) {
	var v T
	etag.Request(req, tag)
	resp, err := client.Do(req)
	if err != nil {
		return v, nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		return v, nil, catwalk.ErrNotModified
	case http.StatusUnauthorized:
		return v, nil, errUnauthorized
	default:
		return v, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return v, nil, fmt.Errorf("failed to read response: %w", err)
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return v, nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return v, body, nil
}
