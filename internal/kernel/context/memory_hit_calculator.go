package context

import (
	"math"
	"strings"
	"sync"
)

// MemoryHitCalculator calculates the relevance and coverage of session memory blocks
// It determines which blocks should be included in SM compression for optimal recall
type MemoryHitCalculator struct {
	mu sync.RWMutex

	// Configuration
	weightConfig HitWeightConfig

	// Topic tracking
	topicFrequency map[string]int
	totalTopics    int
}

// HitWeightConfig holds weights for hit calculation
type HitWeightConfig struct {
	// Recency weight (0-1): Higher = more recent memories are more important
	RecencyWeight float64

	// Frequency weight (0-1): Higher = frequently referenced topics are more important
	FrequencyWeight float64

	// Coverage weight (0-1): Higher = memories covering more topics are better
	CoverageWeight float64

	// Relevance weight (0-1): Higher = memories matching current context are better
	RelevanceWeight float64

	// Decay factor for older memories
	DecayFactor float64
}

// DefaultHitWeightConfig returns the default weight configuration
func DefaultHitWeightConfig() HitWeightConfig {
	return HitWeightConfig{
		RecencyWeight:    0.25,
		FrequencyWeight:  0.20,
		CoverageWeight:  0.25,
		RelevanceWeight:  0.30,
		DecayFactor:      0.95, // 5% decay per time unit
	}
}

// NewMemoryHitCalculator creates a new hit calculator
func NewMemoryHitCalculator(config HitWeightConfig) *MemoryHitCalculator {
	if config.RecencyWeight == 0 && config.FrequencyWeight == 0 &&
		config.CoverageWeight == 0 && config.RelevanceWeight == 0 {
		config = DefaultHitWeightConfig()
	}
	return &MemoryHitCalculator{
		weightConfig:    config,
		topicFrequency: make(map[string]int),
	}
}

// TopicStats holds statistics about a topic
type TopicStats struct {
	Topic             string
	Frequency         int
	TotalFrequency    int
	NormalizedFreq    float64
	BlocksContaining  int
	TotalBlocks       int
}

// HitResult contains the hit calculation result for a block
type HitResult struct {
	BlockID         string
	OverallScore    float64
	RecencyScore    float64
	FrequencyScore  float64
	CoverageScore   float64
	RelevanceScore  float64
	CoveredTopics   []string
	UncoveredTopics []string
	Coverage        float64 // Percentage of topics covered (0-100)
}

// CalculateHit calculates the relevance score for a single block
func (calc *MemoryHitCalculator) CalculateHit(block *SessionMemoryBlock, contextTopics []string) *HitResult {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	if block == nil {
		return &HitResult{OverallScore: 0}
	}

	result := &HitResult{
		BlockID:       block.ID,
		CoveredTopics: make([]string, 0),
	}

	// Normalize weights
	totalWeight := calc.weightConfig.RecencyWeight +
		calc.weightConfig.FrequencyWeight +
		calc.weightConfig.CoverageWeight +
		calc.weightConfig.RelevanceWeight

	recencyW := calc.weightConfig.RecencyWeight / totalWeight
	freqW := calc.weightConfig.FrequencyWeight / totalWeight
	coverageW := calc.weightConfig.CoverageWeight / totalWeight
	relevanceW := calc.weightConfig.RelevanceWeight / totalWeight

	// 1. Recency Score (0-1, where 1 is most recent)
	result.RecencyScore = calc.calculateRecencyScore(block)

	// 2. Frequency Score (based on topic frequency)
	result.FrequencyScore = calc.calculateFrequencyScore(block)

	// 3. Coverage Score (how many topics does this block cover)
	result.CoverageScore, result.CoveredTopics, result.UncoveredTopics =
		calc.calculateCoverageScore(block, contextTopics)

	// 4. Relevance Score (tag match with current context)
	result.RelevanceScore = calc.calculateRelevanceScore(block, contextTopics)

	// Calculate overall score
	result.OverallScore = result.RecencyScore*recencyW +
		result.FrequencyScore*freqW +
		result.CoverageScore*coverageW +
		result.RelevanceScore*relevanceW

	// Coverage percentage
	if len(contextTopics) > 0 {
		result.Coverage = float64(len(result.CoveredTopics)) / float64(len(contextTopics)) * 100
	}

	return result
}

// CalculateHits calculates hit scores for multiple blocks
func (calc *MemoryHitCalculator) CalculateHits(blocks []*SessionMemoryBlock, contextTopics []string) []*HitResult {
	results := make([]*HitResult, 0, len(blocks))

	for _, block := range blocks {
		results = append(results, calc.CalculateHit(block, contextTopics))
	}

	// Sort by overall score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].OverallScore > results[i].OverallScore {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// SelectTopBlocks selects the top N blocks that maximize coverage
func (calc *MemoryHitCalculator) SelectTopBlocks(blocks []*SessionMemoryBlock, contextTopics []string, limit int) []*SessionMemoryBlock {
	if limit <= 0 {
		limit = len(blocks)
	}

	hits := calc.CalculateHits(blocks, contextTopics)

	// Greedy selection for coverage
	selected := make(map[string]bool)
	var result []*SessionMemoryBlock
	remainingTopics := make(map[string]bool)
	for _, t := range contextTopics {
		remainingTopics[t] = true
	}

	for len(result) < limit && len(remainingTopics) > 0 {
		var bestBlock *SessionMemoryBlock
		var bestScore float64
		var bestCovered []string

		for _, hit := range hits {
			if selected[hit.BlockID] {
				continue
			}

			// Calculate incremental coverage
			incrementalCoverage := 0
			for _, topic := range hit.CoveredTopics {
				if remainingTopics[topic] {
					incrementalCoverage++
				}
			}

			// Score = hit score + incremental coverage bonus
			coverageBonus := float64(incrementalCoverage) / float64(len(contextTopics)+1)
			score := hit.OverallScore + coverageBonus*0.5

			if bestBlock == nil || score > bestScore {
				bestBlock = findBlockByID(blocks, hit.BlockID)
				bestScore = score
				bestCovered = hit.CoveredTopics
			}
		}

		if bestBlock == nil {
			break
		}

		selected[bestBlock.ID] = true
		result = append(result, bestBlock)

		// Remove covered topics
		for _, topic := range bestCovered {
			delete(remainingTopics, topic)
		}
	}

	return result
}

// calculateRecencyScore calculates recency score using exponential decay
func (calc *MemoryHitCalculator) calculateRecencyScore(block *SessionMemoryBlock) float64 {
	if block == nil {
		return 0
	}

	// Age in hours (assuming LastAccess is set)
	ageHours := block.LastAccess.Sub(block.CreatedAt).Hours()
	if ageHours < 0 {
		ageHours = 0
	}

	// Exponential decay: score = e^(-decay * age)
	decay := calc.weightConfig.DecayFactor
	score := math.Exp(-decay * ageHours / 24) // Decay per day

	return math.Max(0, math.Min(1, score))
}

// calculateFrequencyScore calculates score based on topic frequency
func (calc *MemoryHitCalculator) calculateFrequencyScore(block *SessionMemoryBlock) float64 {
	if block == nil || len(block.Tags) == 0 {
		return 0.5 // Neutral score for blocks without tags
	}

	totalFreq := float64(0)
	for _, tag := range block.Tags {
		totalFreq += float64(calc.topicFrequency[tag])
	}

	if calc.totalTopics == 0 {
		return 0.5
	}

	// Normalize frequency score
	avgFreq := totalFreq / float64(len(block.Tags))
	maxPossibleFreq := float64(calc.totalTopics)

	return math.Min(1, avgFreq/maxPossibleFreq)
}

// calculateCoverageScore calculates how many context topics are covered
func (calc *MemoryHitCalculator) calculateCoverageScore(block *SessionMemoryBlock, contextTopics []string) (float64, []string, []string) {
	covered := make([]string, 0)
	uncovered := make([]string, 0)

	blockTags := make(map[string]bool)
	for _, tag := range block.Tags {
		blockTags[tag] = true
	}

	for _, topic := range contextTopics {
		if blockTags[topic] {
			covered = append(covered, topic)
		} else {
			uncovered = append(uncovered, topic)
		}
	}

	if len(contextTopics) == 0 {
		return 0, covered, uncovered
	}

	coverage := float64(len(covered)) / float64(len(contextTopics))
	return coverage, covered, uncovered
}

// calculateRelevanceScore calculates tag overlap with context
func (calc *MemoryHitCalculator) calculateRelevanceScore(block *SessionMemoryBlock, contextTopics []string) float64 {
	if block == nil || len(contextTopics) == 0 {
		return 0.5 // Neutral
	}

	blockTagSet := make(map[string]bool)
	for _, tag := range block.Tags {
		blockTagSet[strings.ToLower(tag)] = true
	}

	matchCount := 0
	for _, topic := range contextTopics {
		if blockTagSet[strings.ToLower(topic)] {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(contextTopics))
}

// RecordTopics records topic frequencies for hit calculation
func (calc *MemoryHitCalculator) RecordTopics(topics []string) {
	calc.mu.Lock()
	defer calc.mu.Unlock()

	for _, topic := range topics {
		calc.topicFrequency[topic]++
		calc.totalTopics++
	}
}

// RecordBlockAccess records a block access and updates frequencies
func (calc *MemoryHitCalculator) RecordBlockAccess(block *SessionMemoryBlock) {
	if block == nil {
		return
	}

	calc.mu.Lock()
	defer calc.mu.Unlock()

	// Update access count affects frequency weighting
	for _, tag := range block.Tags {
		calc.topicFrequency[tag]++
		calc.totalTopics++
	}
}

// GetTopicStats returns statistics about tracked topics
func (calc *MemoryHitCalculator) GetTopicStats() []TopicStats {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	stats := make([]TopicStats, 0, len(calc.topicFrequency))
	for topic, freq := range calc.topicFrequency {
		stats = append(stats, TopicStats{
			Topic:          topic,
			Frequency:     freq,
			TotalFrequency: calc.totalTopics,
			NormalizedFreq: float64(freq) / float64(calc.totalTopics+1),
		})
	}

	return stats
}

// Reset clears topic tracking
func (calc *MemoryHitCalculator) Reset() {
	calc.mu.Lock()
	defer calc.mu.Unlock()
	calc.topicFrequency = make(map[string]int)
	calc.totalTopics = 0
}

// findBlockByID finds a block by ID in a slice
func findBlockByID(blocks []*SessionMemoryBlock, id string) *SessionMemoryBlock {
	for _, block := range blocks {
		if block.ID == id {
			return block
		}
	}
	return nil
}

// Score returns the overall score (for compatibility)
func (hr *HitResult) Score() float64 {
	return hr.OverallScore
}
