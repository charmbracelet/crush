package csync

import (
	"encoding/json"
	"maps"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewMap(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	require.NotNil(t, m)
	require.NotNil(t, m.inner)
	require.Equal(t, 0, m.Len())
}

func TestNewMapFrom(t *testing.T) {
	t.Parallel()

	original := map[string]int{
		"key1": 1,
		"key2": 2,
	}

	m := NewMapFrom(original)
	require.NotNil(t, m)
	require.Equal(t, original, m.inner)
	require.Equal(t, 2, m.Len())

	value, ok := m.Get("key1")
	require.True(t, ok)
	require.Equal(t, 1, value)
}

func TestNewLazyMap(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		t.Helper()

		waiter := sync.Mutex{}
		waiter.Lock()
		loadCalled := false

		loadFunc := func() map[string]int {
			waiter.Lock()
			defer waiter.Unlock()
			loadCalled = true
			return map[string]int{
				"key1": 1,
				"key2": 2,
			}
		}

		m := NewLazyMap(loadFunc)
		require.NotNil(t, m)

		waiter.Unlock() // Allow the load function to proceed
		time.Sleep(100 * time.Millisecond)
		require.True(t, loadCalled)
		require.Equal(t, 2, m.Len())

		value, ok := m.Get("key1")
		require.True(t, ok)
		require.Equal(t, 1, value)
	})
}

func TestMap_Reset(t *testing.T) {
	t.Parallel()

	m := NewMapFrom(map[string]int{
		"a": 10,
	})

	m.Reset(map[string]int{
		"b": 20,
	})
	value, ok := m.Get("b")
	require.True(t, ok)
	require.Equal(t, 20, value)
	require.Equal(t, 1, m.Len())
}

func TestMap_Set(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	m.Set("key1", 42)
	value, ok := m.Get("key1")
	require.True(t, ok)
	require.Equal(t, 42, value)
	require.Equal(t, 1, m.Len())

	m.Set("key1", 100)
	value, ok = m.Get("key1")
	require.True(t, ok)
	require.Equal(t, 100, value)
	require.Equal(t, 1, m.Len())
}

func TestMap_GetOrSet(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	require.Equal(t, 42, m.GetOrSet("key1", func() int { return 42 }))
	require.Equal(t, 42, m.GetOrSet("key1", func() int { return 99999 }))
	require.Equal(t, 1, m.Len())
}

func TestMap_Get(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	value, ok := m.Get("nonexistent")
	require.False(t, ok)
	require.Equal(t, 0, value)

	m.Set("key1", 42)
	value, ok = m.Get("key1")
	require.True(t, ok)
	require.Equal(t, 42, value)
}

func TestMap_Del(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 42)
	m.Set("key2", 100)

	require.Equal(t, 2, m.Len())

	m.Del("key1")
	_, ok := m.Get("key1")
	require.False(t, ok)
	require.Equal(t, 1, m.Len())

	value, ok := m.Get("key2")
	require.True(t, ok)
	require.Equal(t, 100, value)

	m.Del("nonexistent")
	require.Equal(t, 1, m.Len())
}

func TestMap_Len(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	require.Equal(t, 0, m.Len())

	m.Set("key1", 1)
	require.Equal(t, 1, m.Len())

	m.Set("key2", 2)
	require.Equal(t, 2, m.Len())

	m.Del("key1")
	require.Equal(t, 1, m.Len())

	m.Del("key2")
	require.Equal(t, 0, m.Len())
}

func TestMap_Take(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 42)
	m.Set("key2", 100)

	require.Equal(t, 2, m.Len())

	value, ok := m.Take("key1")
	require.True(t, ok)
	require.Equal(t, 42, value)
	require.Equal(t, 1, m.Len())

	_, exists := m.Get("key1")
	require.False(t, exists)

	value, ok = m.Get("key2")
	require.True(t, ok)
	require.Equal(t, 100, value)
}

func TestMap_Take_NonexistentKey(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 42)

	value, ok := m.Take("nonexistent")
	require.False(t, ok)
	require.Equal(t, 0, value)
	require.Equal(t, 1, m.Len())

	value, ok = m.Get("key1")
	require.True(t, ok)
	require.Equal(t, 42, value)
}

func TestMap_Take_EmptyMap(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	value, ok := m.Take("key1")
	require.False(t, ok)
	require.Equal(t, 0, value)
	require.Equal(t, 0, m.Len())
}

func TestMap_Take_SameKeyTwice(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 42)

	value, ok := m.Take("key1")
	require.True(t, ok)
	require.Equal(t, 42, value)
	require.Equal(t, 0, m.Len())

	value, ok = m.Take("key1")
	require.False(t, ok)
	require.Equal(t, 0, value)
	require.Equal(t, 0, m.Len())
}

func TestMap_Seq2(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 1)
	m.Set("key2", 2)
	m.Set("key3", 3)

	collected := maps.Collect(m.Seq2())

	require.Equal(t, 3, len(collected))
	require.Equal(t, 1, collected["key1"])
	require.Equal(t, 2, collected["key2"])
	require.Equal(t, 3, collected["key3"])
}

func TestMap_Seq2_EarlyReturn(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 1)
	m.Set("key2", 2)
	m.Set("key3", 3)

	count := 0
	for range m.Seq2() {
		count++
		if count == 2 {
			break
		}
	}

	require.Equal(t, 2, count)
}

func TestMap_Seq2_EmptyMap(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	count := 0
	for range m.Seq2() {
		count++
	}

	require.Equal(t, 0, count)
}

func TestMap_Seq(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 1)
	m.Set("key2", 2)
	m.Set("key3", 3)

	collected := make([]int, 0)
	for v := range m.Seq() {
		collected = append(collected, v)
	}

	require.Equal(t, 3, len(collected))
	require.Contains(t, collected, 1)
	require.Contains(t, collected, 2)
	require.Contains(t, collected, 3)
}

func TestMap_Seq_EarlyReturn(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 1)
	m.Set("key2", 2)
	m.Set("key3", 3)

	count := 0
	for range m.Seq() {
		count++
		if count == 2 {
			break
		}
	}

	require.Equal(t, 2, count)
}

func TestMap_Seq_EmptyMap(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	count := 0
	for range m.Seq() {
		count++
	}

	require.Equal(t, 0, count)
}

func TestMap_MarshalJSON(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 1)
	m.Set("key2", 2)

	data, err := json.Marshal(m)
	require.NoError(t, err)

	result := &Map[string, int]{}
	err = json.Unmarshal(data, result)
	require.NoError(t, err)
	require.Equal(t, 2, result.Len())
	v1, _ := result.Get("key1")
	v2, _ := result.Get("key2")
	require.Equal(t, 1, v1)
	require.Equal(t, 2, v2)
}

func TestMap_MarshalJSON_EmptyMap(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	data, err := json.Marshal(m)
	require.NoError(t, err)
	require.Equal(t, "{}", string(data))
}

func TestMap_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	jsonData := `{"key1": 1, "key2": 2}`

	m := NewMap[string, int]()
	err := json.Unmarshal([]byte(jsonData), m)
	require.NoError(t, err)

	require.Equal(t, 2, m.Len())
	value, ok := m.Get("key1")
	require.True(t, ok)
	require.Equal(t, 1, value)

	value, ok = m.Get("key2")
	require.True(t, ok)
	require.Equal(t, 2, value)
}

func TestMap_UnmarshalJSON_EmptyJSON(t *testing.T) {
	t.Parallel()

	jsonData := `{}`

	m := NewMap[string, int]()
	err := json.Unmarshal([]byte(jsonData), m)
	require.NoError(t, err)
	require.Equal(t, 0, m.Len())
}

func TestMap_UnmarshalJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	jsonData := `{"key1": 1, "key2":}`

	m := NewMap[string, int]()
	err := json.Unmarshal([]byte(jsonData), m)
	require.Error(t, err)
}

func TestMap_UnmarshalJSON_OverwritesExistingData(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("existing", 999)

	jsonData := `{"key1": 1, "key2": 2}`
	err := json.Unmarshal([]byte(jsonData), m)
	require.NoError(t, err)

	require.Equal(t, 2, m.Len())
	_, ok := m.Get("existing")
	require.False(t, ok)

	value, ok := m.Get("key1")
	require.True(t, ok)
	require.Equal(t, 1, value)
}

func TestMap_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := NewMap[string, int]()
	original.Set("key1", 1)
	original.Set("key2", 2)
	original.Set("key3", 3)

	data, err := json.Marshal(original)
	require.NoError(t, err)

	restored := NewMap[string, int]()
	err = json.Unmarshal(data, restored)
	require.NoError(t, err)

	require.Equal(t, original.Len(), restored.Len())

	for k, v := range original.Seq2() {
		restoredValue, ok := restored.Get(k)
		require.True(t, ok)
		require.Equal(t, v, restoredValue)
	}
}

func TestMap_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	m := NewMap[int, int]()
	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range numOperations {
				key := id*numOperations + j
				m.Set(key, key*2)
				value, ok := m.Get(key)
				require.True(t, ok)
				require.Equal(t, key*2, value)
			}
		}(i)
	}

	wg.Wait()

	require.Equal(t, numGoroutines*numOperations, m.Len())
}

func TestMap_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	m := NewMap[int, int]()
	const numReaders = 50
	const numWriters = 50
	const numOperations = 100

	for i := range 1000 {
		m.Set(i, i)
	}

	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	for range numReaders {
		go func() {
			defer wg.Done()
			for j := range numOperations {
				key := j % 1000
				value, ok := m.Get(key)
				if ok {
					require.Equal(t, key, value)
				}
				_ = m.Len()
			}
		}()
	}

	for i := range numWriters {
		go func(id int) {
			defer wg.Done()
			for j := range numOperations {
				key := 1000 + id*numOperations + j
				m.Set(key, key)
				if j%10 == 0 {
					m.Del(key)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestMap_ConcurrentSeq2(t *testing.T) {
	t.Parallel()

	m := NewMap[int, int]()
	for i := range 100 {
		m.Set(i, i*2)
	}

	var wg sync.WaitGroup
	const numIterators = 10

	wg.Add(numIterators)
	for range numIterators {
		go func() {
			defer wg.Done()
			count := 0
			for k, v := range m.Seq2() {
				require.Equal(t, k*2, v)
				count++
			}
			require.Equal(t, 100, count)
		}()
	}

	wg.Wait()
}

func TestMap_ConcurrentSeq(t *testing.T) {
	t.Parallel()

	m := NewMap[int, int]()
	for i := range 100 {
		m.Set(i, i*2)
	}

	var wg sync.WaitGroup
	const numIterators = 10

	wg.Add(numIterators)
	for range numIterators {
		go func() {
			defer wg.Done()
			count := 0
			values := make(map[int]bool)
			for v := range m.Seq() {
				values[v] = true
				count++
			}
			require.Equal(t, 100, count)
			for i := range 100 {
				require.True(t, values[i*2])
			}
		}()
	}

	wg.Wait()
}

func TestMap_ConcurrentTake(t *testing.T) {
	t.Parallel()

	m := NewMap[int, int]()
	const numItems = 1000

	for i := range numItems {
		m.Set(i, i*2)
	}

	var wg sync.WaitGroup
	const numWorkers = 10
	taken := make([][]int, numWorkers)

	wg.Add(numWorkers)
	for i := range numWorkers {
		go func(workerID int) {
			defer wg.Done()
			taken[workerID] = make([]int, 0)
			for j := workerID; j < numItems; j += numWorkers {
				if value, ok := m.Take(j); ok {
					taken[workerID] = append(taken[workerID], value)
				}
			}
		}(i)
	}

	wg.Wait()

	require.Equal(t, 0, m.Len())

	allTaken := make(map[int]bool)
	for _, workerTaken := range taken {
		for _, value := range workerTaken {
			require.False(t, allTaken[value], "Value %d was taken multiple times", value)
			allTaken[value] = true
		}
	}

	require.Equal(t, numItems, len(allTaken))
	for i := range numItems {
		require.True(t, allTaken[i*2], "Expected value %d to be taken", i*2)
	}
}

func TestMap_TypeSafety(t *testing.T) {
	t.Parallel()

	stringIntMap := NewMap[string, int]()
	stringIntMap.Set("key", 42)
	value, ok := stringIntMap.Get("key")
	require.True(t, ok)
	require.Equal(t, 42, value)

	intStringMap := NewMap[int, string]()
	intStringMap.Set(42, "value")
	strValue, ok := intStringMap.Get(42)
	require.True(t, ok)
	require.Equal(t, "value", strValue)

	structMap := NewMap[string, struct{ Name string }]()
	structMap.Set("key", struct{ Name string }{Name: "test"})
	structValue, ok := structMap.Get("key")
	require.True(t, ok)
	require.Equal(t, "test", structValue.Name)
}

func TestMap_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	var _ json.Marshaler = &Map[string, any]{}
	var _ json.Unmarshaler = &Map[string, any]{}
}

func BenchmarkMap_Set(b *testing.B) {
	m := NewMap[int, int]()

	for i := 0; b.Loop(); i++ {
		m.Set(i, i*2)
	}
}

func BenchmarkMap_Get(b *testing.B) {
	m := NewMap[int, int]()
	for i := range 1000 {
		m.Set(i, i*2)
	}

	for i := 0; b.Loop(); i++ {
		m.Get(i % 1000)
	}
}

func BenchmarkMap_Seq2(b *testing.B) {
	m := NewMap[int, int]()
	for i := range 1000 {
		m.Set(i, i*2)
	}

	for b.Loop() {
		for range m.Seq2() {
		}
	}
}

func BenchmarkMap_Seq(b *testing.B) {
	m := NewMap[int, int]()
	for i := range 1000 {
		m.Set(i, i*2)
	}

	for b.Loop() {
		for range m.Seq() {
		}
	}
}

func BenchmarkMap_Take(b *testing.B) {
	m := NewMap[int, int]()
	for i := range 1000 {
		m.Set(i, i*2)
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		key := i % 1000
		m.Take(key)
		if i%1000 == 999 {
			b.StopTimer()
			for j := range 1000 {
				m.Set(j, j*2)
			}
			b.StartTimer()
		}
	}
}

func BenchmarkMap_ConcurrentReadWrite(b *testing.B) {
	m := NewMap[int, int]()
	for i := range 1000 {
		m.Set(i, i*2)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				m.Get(i % 1000)
			} else {
				m.Set(i+1000, i*2)
			}
			i++
		}
	})
}
