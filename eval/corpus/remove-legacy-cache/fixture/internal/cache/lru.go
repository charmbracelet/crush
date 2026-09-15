package cache

import (
	"container/list"
	"sync"
)

// LRU is a bounded Cache variant evicting the least recently used
// entry once Cap is exceeded.
type LRU struct {
	cap   int
	mu    sync.Mutex
	ll    *list.List
	items map[string]*list.Element
}

type lruEntry struct {
	key   string
	value string
}

// NewLRU returns an LRU holding at most cap entries.
func NewLRU(capacity int) *LRU {
	return &LRU{
		cap:   capacity,
		ll:    list.New(),
		items: make(map[string]*list.Element),
	}
}

// Get returns the value for key and marks it most recently used.
func (l *LRU) Get(key string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if el, ok := l.items[key]; ok {
		l.ll.MoveToFront(el)
		return el.Value.(lruEntry).value, true
	}
	return "", false
}

// Set stores value under key, evicting the oldest entry when full.
func (l *LRU) Set(key, value string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if el, ok := l.items[key]; ok {
		l.ll.MoveToFront(el)
		el.Value = lruEntry{key, value}
		return
	}
	l.items[key] = l.ll.PushFront(lruEntry{key, value})
	if l.ll.Len() > l.cap {
		back := l.ll.Back()
		if back != nil {
			l.ll.Remove(back)
			delete(l.items, back.Value.(lruEntry).key)
		}
	}
}

// Delete removes key.
func (l *LRU) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if el, ok := l.items[key]; ok {
		l.ll.Remove(el)
		delete(l.items, key)
	}
}
