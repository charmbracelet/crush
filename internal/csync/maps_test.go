package csync

import (
	"encoding/json"
	"maps"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMap(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	assert.NotNil(t, m)
	assert.NotNil(t, m.inner)
	assert.Equal(t, 0, m.Len())
}

func TestNewMapFrom(t *testing.T) {
	t.Parallel()

	original := map[string]int{
		"key1": 1,
		"key2": 2,
	}

	m := NewMapFrom(original)
	assert.NotNil(t, m)
	assert.Equal(t, original, m.inner)
	assert.Equal(t, 2, m.Len())

	value, ok := m.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 1, value)
}

func TestMap_Set(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	m.Set("key1", 42)
	value, ok := m.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 42, value)
	assert.Equal(t, 1, m.Len())

	m.Set("key1", 100)
	value, ok = m.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 100, value)
	assert.Equal(t, 1, m.Len())
}

func TestMap_Get(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	value, ok := m.Get("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, 0, value)

	m.Set("key1", 42)
	value, ok = m.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 42, value)
}

func TestMap_Del(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 42)
	m.Set("key2", 100)

	assert.Equal(t, 2, m.Len())

	m.Del("key1")
	_, ok := m.Get("key1")
	assert.False(t, ok)
	assert.Equal(t, 1, m.Len())

	value, ok := m.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, 100, value)

	m.Del("nonexistent")
	assert.Equal(t, 1, m.Len())
}

func TestMap_Len(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	assert.Equal(t, 0, m.Len())

	m.Set("key1", 1)
	assert.Equal(t, 1, m.Len())

	m.Set("key2", 2)
	assert.Equal(t, 2, m.Len())

	m.Del("key1")
	assert.Equal(t, 1, m.Len())

	m.Del("key2")
	assert.Equal(t, 0, m.Len())
}

func TestMap_Seq2(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 1)
	m.Set("key2", 2)
	m.Set("key3", 3)

	collected := maps.Collect(m.Seq2())

	assert.Equal(t, 3, len(collected))
	assert.Equal(t, 1, collected["key1"])
	assert.Equal(t, 2, collected["key2"])
	assert.Equal(t, 3, collected["key3"])
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

	assert.Equal(t, 2, count)
}

func TestMap_Seq2_EmptyMap(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	count := 0
	for range m.Seq2() {
		count++
	}

	assert.Equal(t, 0, count)
}

func TestMap_MarshalJSON(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("key1", 1)
	m.Set("key2", 2)

	data, err := json.Marshal(m)
	assert.NoError(t, err)

	var result map[string]int
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, 1, result["key1"])
	assert.Equal(t, 2, result["key2"])
}

func TestMap_MarshalJSON_EmptyMap(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()

	data, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Equal(t, "{}", string(data))
}

func TestMap_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	jsonData := `{"key1": 1, "key2": 2}`

	m := NewMap[string, int]()
	err := json.Unmarshal([]byte(jsonData), m)
	assert.NoError(t, err)

	assert.Equal(t, 2, m.Len())
	value, ok := m.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 1, value)

	value, ok = m.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, 2, value)
}

func TestMap_UnmarshalJSON_EmptyJSON(t *testing.T) {
	t.Parallel()

	jsonData := `{}`

	m := NewMap[string, int]()
	err := json.Unmarshal([]byte(jsonData), m)
	assert.NoError(t, err)
	assert.Equal(t, 0, m.Len())
}

func TestMap_UnmarshalJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	jsonData := `{"key1": 1, "key2":}`

	m := NewMap[string, int]()
	err := json.Unmarshal([]byte(jsonData), m)
	assert.Error(t, err)
}

func TestMap_UnmarshalJSON_OverwritesExistingData(t *testing.T) {
	t.Parallel()

	m := NewMap[string, int]()
	m.Set("existing", 999)

	jsonData := `{"key1": 1, "key2": 2}`
	err := json.Unmarshal([]byte(jsonData), m)
	assert.NoError(t, err)

	assert.Equal(t, 2, m.Len())
	_, ok := m.Get("existing")
	assert.False(t, ok)

	value, ok := m.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 1, value)
}

func TestMap_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := NewMap[string, int]()
	original.Set("key1", 1)
	original.Set("key2", 2)
	original.Set("key3", 3)

	data, err := json.Marshal(original)
	assert.NoError(t, err)

	restored := NewMap[string, int]()
	err = json.Unmarshal(data, restored)
	assert.NoError(t, err)

	assert.Equal(t, original.Len(), restored.Len())

	for k, v := range original.Seq2() {
		restoredValue, ok := restored.Get(k)
		assert.True(t, ok)
		assert.Equal(t, v, restoredValue)
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
				assert.True(t, ok)
				assert.Equal(t, key*2, value)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, numGoroutines*numOperations, m.Len())
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
					assert.Equal(t, key, value)
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
				assert.Equal(t, k*2, v)
				count++
			}
			assert.Equal(t, 100, count)
		}()
	}

	wg.Wait()
}

func TestMap_TypeSafety(t *testing.T) {
	t.Parallel()

	stringIntMap := NewMap[string, int]()
	stringIntMap.Set("key", 42)
	value, ok := stringIntMap.Get("key")
	assert.True(t, ok)
	assert.Equal(t, 42, value)

	intStringMap := NewMap[int, string]()
	intStringMap.Set(42, "value")
	strValue, ok := intStringMap.Get(42)
	assert.True(t, ok)
	assert.Equal(t, "value", strValue)

	structMap := NewMap[string, struct{ Name string }]()
	structMap.Set("key", struct{ Name string }{Name: "test"})
	structValue, ok := structMap.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "test", structValue.Name)
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
