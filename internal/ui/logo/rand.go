package logo

import (
	"math/rand/v2"
	"sync"
)

var (
	randCaches   = make(map[int]int)
	randCachesMu sync.Mutex
)

func cachedRandN(n int) int {
	randCachesMu.Lock()
	defer randCachesMu.Unlock()

	if n, ok := randCaches[n]; ok {
		return n
	}

	r := rand.IntN(n)
	randCaches[n] = r
	return r
}
