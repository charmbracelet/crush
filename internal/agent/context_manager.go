package agent

import (
	"fmt"
	"math"
	"sync"
	"time"

	"charm.land/fantasy"
)

type ContextLayer int

const (
	L1RollingWindow ContextLayer = iota + 1
	L2TopicSegmentation
	L3ImportantMemory
	L4LongTermMemory
	L5CompressionStrategy
	L6ActivityTracking
	L7IntelligentAging
)

type ContextEntry struct {
	Content    string
	Timestamp  time.Time
	Importance float64
	Layer      ContextLayer
	AccessCount int
	LastAccess time.Time
}

type contextManager struct {
	mu sync.RWMutex

	l1Window    []ContextEntry
	l2Topics    map[string][]ContextEntry
	l3Important []ContextEntry
	l4LongTerm  []ContextEntry

	activity map[string]int
	maxAge   map[string]time.Time

	windowSize    int
	maxLongTerm   int
	activityThreshold int
	agingFactor   float64
}

func newContextManager() *contextManager {
	return &contextManager{
		l2Topics:         make(map[string][]ContextEntry),
		activity:         make(map[string]int),
		maxAge:           make(map[string]time.Time),
		windowSize:       100,
		maxLongTerm:      50,
		activityThreshold: 5,
		agingFactor:      0.95,
	}
}

func (cm *contextManager) Add(entry ContextEntry) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	entry.Timestamp = time.Now()
	entry.LastAccess = entry.Timestamp

	switch entry.Layer {
	case L1RollingWindow:
		cm.l1Window = append(cm.l1Window, entry)
		if len(cm.l1Window) > cm.windowSize {
			evicted := cm.l1Window[0]
			cm.l1Window = cm.l1Window[1:]
			cm.promoteToL2(evicted)
		}

	case L2TopicSegmentation:
		topic := cm.extractTopic(entry.Content)
		cm.l2Topics[topic] = append(cm.l2Topics[topic], entry)
		cm.checkL2Overflow(topic)

	case L3ImportantMemory:
		cm.l3Important = append(cm.l3Important, entry)
		if len(cm.l3Important) > 20 {
			cm.demoteLeastImportant()
		}

	case L4LongTermMemory:
		cm.l4LongTerm = append(cm.l4LongTerm, entry)
		if len(cm.l4LongTerm) > cm.maxLongTerm {
			cm.l4LongTerm = cm.l4LongTerm[1:]
		}
	}

	cm.trackActivity(entry.Content)
}

func (cm *contextManager) promoteToL2(entry ContextEntry) {
	if entry.Importance >= 0.7 {
		entry.Layer = L2TopicSegmentation
		topic := cm.extractTopic(entry.Content)
		cm.l2Topics[topic] = append(cm.l2Topics[topic], entry)
		cm.checkL2Overflow(topic)
	} else if entry.Importance >= 0.5 {
		entry.Layer = L3ImportantMemory
		cm.l3Important = append(cm.l3Important, entry)
	}
}

func (cm *contextManager) checkL2Overflow(topic string) {
	const maxPerTopic = 30
	if len(cm.l2Topics[topic]) > maxPerTopic {
		oldest := cm.l2Topics[topic][0]
		cm.l2Topics[topic] = cm.l2Topics[topic][1:]
		if oldest.Importance >= 0.6 {
			cm.promoteToL3(oldest)
		}
	}
}

func (cm *contextManager) promoteToL3(entry ContextEntry) {
	entry.Layer = L3ImportantMemory
	cm.l3Important = append(cm.l3Important, entry)
}

func (cm *contextManager) demoteLeastImportant() {
	minImportance := 1.0
	minIdx := 0
	for i, e := range cm.l3Important {
		if e.Importance < minImportance {
			minImportance = e.Importance
			minIdx = i
		}
	}
	demoted := cm.l3Important[minIdx]
	cm.l3Important = append(cm.l3Important[:minIdx], cm.l3Important[minIdx+1:]...)
	if demoted.Importance >= 0.4 {
		cm.promoteToL4(demoted)
	}
}

func (cm *contextManager) promoteToL4(entry ContextEntry) {
	entry.Layer = L4LongTermMemory
	cm.l4LongTerm = append(cm.l4LongTerm, entry)
	if len(cm.l4LongTerm) > cm.maxLongTerm {
		cm.l4LongTerm = cm.l4LongTerm[1:]
	}
}

func (cm *contextManager) trackActivity(content string) {
	sig := cm.signature(content)
	cm.activity[sig]++
	if cm.activity[sig] >= cm.activityThreshold {
		cm.maxAge[sig] = time.Now().Add(30 * 24 * time.Hour)
	}
}

func (cm *contextManager) extractTopic(content string) string {
	topicLen := 50
	if len(content) < topicLen {
		return content
	}
	return content[:topicLen]
}

func (cm *contextManager) signature(content string) string {
	const maxLen = 100
	if len(content) > maxLen {
		content = content[:maxLen]
	}
	return content
}

func (cm *contextManager) GetContextForLayer(layer ContextLayer) []ContextEntry {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	switch layer {
	case L1RollingWindow:
		return cm.copyEntries(cm.l1Window)
	case L2TopicSegmentation:
		var result []ContextEntry
		for _, entries := range cm.l2Topics {
			result = append(result, entries...)
		}
		return result
	case L3ImportantMemory:
		return cm.copyEntries(cm.l3Important)
	case L4LongTermMemory:
		return cm.copyEntries(cm.l4LongTerm)
	case L6ActivityTracking:
		return cm.getActiveEntries()
	case L7IntelligentAging:
		return cm.applyAging()
	}
	return nil
}

func (cm *contextManager) copyEntries(entries []ContextEntry) []ContextEntry {
	result := make([]ContextEntry, len(entries))
	copy(result, entries)
	return result
}

func (cm *contextManager) getActiveEntries() []ContextEntry {
	now := time.Now()
	var result []ContextEntry
	for sig, count := range cm.activity {
		if count >= cm.activityThreshold {
			if maxAge, ok := cm.maxAge[sig]; ok && maxAge.After(now) {
				result = append(result, ContextEntry{
					Content:    sig,
					Importance: float64(count) / 10.0,
					AccessCount: count,
				})
			}
		}
	}
	return result
}

func (cm *contextManager) applyAging() []ContextEntry {
	now := time.Now()
	var result []ContextEntry

	allEntries := append(cm.l1Window, cm.l3Important...)
	allEntries = append(allEntries, cm.l4LongTerm...)

	for _, entry := range allEntries {
		age := now.Sub(entry.Timestamp)
		days := float64(age.Hours()) / 24.0
		decay := math.Pow(cm.agingFactor, days)
		entry.Importance *= decay
		if entry.Importance > 0.1 {
			result = append(result, entry)
		}
	}
	return result
}

func (cm *contextManager) Summarize() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	total := len(cm.l1Window) + len(cm.l3Important) + len(cm.l4LongTerm)
	for _, entries := range cm.l2Topics {
		total += len(entries)
	}

	return fmt.Sprintf("Context: %d entries (L1:%d, L2:%d, L3:%d, L4:%d)",
		total, len(cm.l1Window), len(cm.l2Topics), len(cm.l3Important), len(cm.l4LongTerm))
}

func (cm *contextManager) OnRetry(err *fantasy.ProviderError, delay time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	errMsg := "unknown"
	if err != nil {
		errMsg = err.Message
	}
	content := fmt.Sprintf("error:%s:delay:%v", errMsg, delay)
	entry := ContextEntry{
		Content:    content,
		Importance: 0.8,
		Layer:      L3ImportantMemory,
	}
	cm.l3Important = append(cm.l3Important, entry)
}
