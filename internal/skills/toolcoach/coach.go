package toolcoach

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// Content cache limits to prevent unbounded memory growth.
const (
	maxCachedFiles        = 50
	maxCachedBytesPerFile = 4096
)

// coachResult is returned by the heuristic engine when an anti-pattern is
// detected.
type coachResult struct {
	PatternID      string
	Severity       string
	Tip            string
	SuggestedInput string // Optional improved tool input for guided retry.
	DelayMicros    int64
}

// toolCallRecord tracks a single tool invocation for sequence analysis.
type toolCallRecord struct {
	Name      string
	Input     string
	Timestamp time.Time
}

// sessionState tracks per-session tool history, file access patterns,
// cumulative coach timing, and telemetry metrics.
type sessionState struct {
	viewedFiles      map[string]int
	editedFiles      map[string]int
	viewTimestamps   map[string]time.Time
	fileContentCache map[string]string
	toolHistory      []toolCallRecord
	totalCoachTime   time.Duration
	patternsFired    int
	metrics          *coachMetrics

	// patternOrder is a mutable copy of defaultPatterns that may be reordered
	// based on observed hit frequency within this session.
	patternOrder    []*pattern
	patternHitCount map[string]int
	totalChecks     int

	// adaptiveSeverity overrides pattern base severities based on historical
	// effectiveness. A value of "silent" suppresses the pattern entirely.
	adaptiveSeverity map[string]string

	// retriedThisTurn prevents infinite retry loops.
	retriedThisTurn bool

	// activeIndicatorID holds the message ID of the currently visible coach
	// spinner indicator, so we can deduplicate and clean up.
	activeIndicatorID string

	mu sync.RWMutex
}

// newSessionState creates a fresh state tracker for a session.
func newSessionState() *sessionState {
	order := make([]*pattern, len(defaultPatterns))
	for i := range defaultPatterns {
		order[i] = &defaultPatterns[i]
	}
	return &sessionState{
		viewedFiles:      make(map[string]int),
		editedFiles:      make(map[string]int),
		viewTimestamps:   make(map[string]time.Time),
		fileContentCache: make(map[string]string),
		toolHistory:      make([]toolCallRecord, 0, DefaultHistorySize),
		metrics:          newCoachMetrics(),
		patternOrder:     order,
		patternHitCount:  make(map[string]int),
	}
}

// recordToolCall registers a tool invocation in the session history.
func (s *sessionState) recordToolCall(name, input string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.toolHistory = append(s.toolHistory, toolCallRecord{
		Name:      name,
		Input:     input,
		Timestamp: time.Now(),
	})
	if len(s.toolHistory) > DefaultHistorySize {
		s.toolHistory = s.toolHistory[len(s.toolHistory)-DefaultHistorySize:]
	}
}

// hasViewed reports whether the given file path was previously viewed.
func (s *sessionState) hasViewed(filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.viewedFiles[filePath] > 0
}

// hasEdited reports whether the given file path was previously edited.
func (s *sessionState) hasEdited(filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.editedFiles[filePath] > 0
}

// viewCount returns how many times a file was viewed.
func (s *sessionState) viewCount(filePath string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.viewedFiles[filePath]
}

// lastViewTime returns the most recent time the file was viewed, or zero.
func (s *sessionState) lastViewTime(filePath string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.viewTimestamps[filePath]
}

// consecutiveEdits counts how many of the most recent tool calls were edits
// to the same file.
func (s *sessionState) consecutiveEdits(filePath string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for i := len(s.toolHistory) - 1; i >= 0; i-- {
		rec := s.toolHistory[i]
		if rec.Name != "edit" {
			break
		}
		p, _ := jsonpeek(rec.Input, "file_path")
		if p != filePath {
			break
		}
		count++
	}
	return count
}

// cacheFileContent stores a snippet of file content for semantic validation.
func (s *sessionState) cacheFileContent(filePath, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.fileContentCache) >= maxCachedFiles {
		// Simple eviction: clear half the cache when full.
		newCache := make(map[string]string, maxCachedFiles/2)
		i := 0
		for k, v := range s.fileContentCache {
			if i >= maxCachedFiles/2 {
				break
			}
			newCache[k] = v
			i++
		}
		s.fileContentCache = newCache
	}

	if len(content) > maxCachedBytesPerFile {
		content = content[:maxCachedBytesPerFile]
	}
	s.fileContentCache[filePath] = content
}

// hasCachedContent reports whether content for the file is in the cache.
func (s *sessionState) hasCachedContent(filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.fileContentCache[filePath]
	return ok
}

// cachedContentContains reports whether the cached content for the file
// contains the given substring.
func (s *sessionState) cachedContentContains(filePath, substr string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	content, ok := s.fileContentCache[filePath]
	if !ok {
		return false
	}
	return strings.Contains(content, substr)
}

// addCoachTime accumulates time spent coaching.
func (s *sessionState) addCoachTime(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalCoachTime += d
}

// totalTime returns the cumulative coach time for this session.
func (s *sessionState) totalTime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalCoachTime
}

// incrementPatternsFired bumps the per-turn counter.
func (s *sessionState) incrementPatternsFired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patternsFired++
}

// patternsFiredThisTurn returns how many tips have fired in the current turn.
func (s *sessionState) patternsFiredThisTurn() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.patternsFired
}

// resetTurnCounters resets per-turn counters at the start of a new agent turn.
func (s *sessionState) resetTurnCounters() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patternsFired = 0
	s.retriedThisTurn = false
}

// hasRetriedThisTurn reports whether an auto-retry was already attempted this
// turn. Used to enforce the max-1-retry cap.
func (s *sessionState) hasRetriedThisTurn() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.retriedThisTurn
}

// markRetriedThisTurn records that an auto-retry was attempted.
func (s *sessionState) markRetriedThisTurn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retriedThisTurn = true
}

// swapActiveIndicator atomically swaps the active indicator message ID and
// returns the previous one (if any). Used for per-session deduplication.
func (s *sessionState) swapActiveIndicator(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.activeIndicatorID
	s.activeIndicatorID = id
	return prev
}

// clearActiveIndicator clears the active indicator if it matches the given ID.
func (s *sessionState) clearActiveIndicator(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeIndicatorID == id {
		s.activeIndicatorID = ""
	}
}

// getActiveIndicatorID returns the currently tracked indicator message ID.
func (s *sessionState) getActiveIndicatorID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeIndicatorID
}

// exportMetrics emits the session's coach metrics via telemetry and persists
// effectiveness data to the store if one is configured.
func (s *sessionState) exportMetrics(sessionID string, store *Store) {
	if s.metrics != nil {
		s.metrics.export(sessionID)
	}
	if store == nil || sessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	counts := s.metrics.patternCounts()
	for patternID, triple := range counts {
		fired, acted, ignored := triple[0], triple[1], triple[2]
		if fired == 0 {
			continue
		}
		if err := store.RecordSessionEffectiveness(ctx, sessionID, patternID, int64(fired), int64(acted), int64(ignored)); err != nil {
			slog.Debug("Failed to persist toolcoach effectiveness", "pattern_id", patternID, "error", err)
		}
	}
}

// buildCoachSummary returns a human-readable summary of coaching observations
// for the current session turn, suitable for injection into the critic prompt.
func (s *sessionState) buildCoachSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metrics == nil || len(s.metrics.patternFireCount) == 0 {
		return ""
	}

	var sb strings.Builder
	first := true
	for patternID, fired := range s.metrics.patternFireCount {
		if fired == 0 {
			continue
		}
		acted := s.metrics.patternActedCount[patternID]
		ignored := s.metrics.patternIgnoredCount[patternID]

		if !first {
			sb.WriteString("\n")
		}
		first = false

		// Find the pattern name for readability.
		patternName := patternID
		for _, p := range s.patternOrder {
			if p.ID == patternID {
				patternName = p.Name
				break
			}
		}

		fmt.Fprintf(&sb, "- %s: fired %d time(s) this session", patternName, fired)
		if acted > 0 || ignored > 0 {
			fmt.Fprintf(&sb, " (acted: %d, ignored: %d)", acted, ignored)
		}
		sb.WriteString(".")
	}

	return sb.String()
}

// adaptSeverity computes an effective severity for a pattern based on its
// historical effectiveness. Low-action patterns are downgraded; high-action
// patterns are upgraded.
func adaptSeverity(baseSeverity string, rec EffectivenessRecord) string {
	total := rec.TotalActed + rec.TotalIgnored
	if total < 5 {
		return baseSeverity
	}
	score := float64(rec.TotalActed) / float64(total)
	switch baseSeverity {
	case "hint":
		if score < 0.2 {
			return "silent"
		}
		if score > 0.7 {
			return "warning"
		}
	case "warning":
		if score < 0.2 {
			return "hint"
		}
		if score > 0.7 {
			return "error"
		}
	case "error":
		if score < 0.2 {
			return "warning"
		}
	}
	return baseSeverity
}

// loadAdaptiveSeverity queries the store for historical effectiveness and
// populates adaptiveSeverity. It should be called once when the session state
// is created.
func (s *sessionState) loadAdaptiveSeverity(store *Store, lookbackDays int) {
	if store == nil || lookbackDays <= 0 {
		return
	}
	lookback := time.Duration(lookbackDays) * 24 * time.Hour
	severity := make(map[string]string, len(defaultPatterns))
	for i := range defaultPatterns {
		p := &defaultPatterns[i]
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		rec, err := store.GetPatternEffectiveness(ctx, p.ID, lookback)
		cancel()
		if err != nil {
			continue
		}
		sev := adaptSeverity(p.Severity, rec)
		if sev != p.Severity {
			severity[p.ID] = sev
		}
	}
	if len(severity) > 0 {
		s.mu.Lock()
		s.adaptiveSeverity = severity
		s.mu.Unlock()
	}
}

// effectiveSeverity returns the severity to use for a pattern, respecting any
// adaptive override.
func (s *sessionState) effectiveSeverity(pat *pattern) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.adaptiveSeverity != nil {
		if sev, ok := s.adaptiveSeverity[pat.ID]; ok {
			return sev
		}
	}
	return pat.Severity
}

// reorderPatterns sorts patternOrder by hit frequency (descending). This is
// called periodically to move frequently-firing patterns earlier in the check
// order, reducing the average number of Detect calls per tool invocation.
func (s *sessionState) reorderPatterns() {
	s.mu.Lock()
	defer s.mu.Unlock()

	sort.Slice(s.patternOrder, func(i, j int) bool {
		return s.patternHitCount[s.patternOrder[i].ID] > s.patternHitCount[s.patternOrder[j].ID]
	})
}

// runCoach analyzes a tool call against registered patterns and returns a
// coaching tip if an anti-pattern is detected. Execution time is kept under
// 100µs by using only lightweight heuristics.
func (s *sessionState) runCoach(
	cfg ToolcoachConfig,
	intensity CoachingIntensity,
	toolName string,
	input string,
) *coachResult {
	start := time.Now()

	// Fast-path: skip tools that have no patterns at all.
	if !hasPatterns(toolName) {
		s.recordToolCall(toolName, input)
		return nil
	}

	// Always record the tool call so later patterns have context.
	s.recordToolCall(toolName, input)

	// For view tools, update file counts now but defer timestamp update until
	// after pattern detection so repeated_view sees the previous timestamp.
	var viewFilePath string
	if toolName == "view" {
		viewFilePath, _ = jsonpeek(input, "file_path")
		if viewFilePath != "" {
			s.mu.Lock()
			s.viewedFiles[viewFilePath]++
			s.mu.Unlock()
		}
	} else {
		s.trackFileAccess(toolName, input)
	}

	// Fast-path: respect the per-turn limit.
	if s.patternsFiredThisTurn() >= cfg.MaxPatternsPerTurn {
		return nil
	}

	hasFilter := len(cfg.enabledSet) > 0

	s.mu.Lock()
	order := s.patternOrder
	s.totalChecks++
	shouldReorder := s.totalChecks%20 == 0
	s.mu.Unlock()

	if shouldReorder {
		s.reorderPatterns()
	}

	for _, pat := range order {
		if hasFilter {
			if _, ok := cfg.enabledSet[pat.ID]; !ok {
				continue
			}
		}

		// Skip if pattern does not apply to this tool.
		if len(pat.AppliesTo) > 0 {
			applies := false
			for _, n := range pat.AppliesTo {
				if n == toolName {
					applies = true
					break
				}
			}
			if !applies {
				continue
			}
		}

		// Respect adaptive severity: silent patterns are suppressed entirely.
		effectiveSev := s.effectiveSeverity(pat)
		if effectiveSev == "silent" {
			continue
		}

		// Respect progressive coaching intensity.
		if !severityAllowedByIntensity(effectiveSev, intensity) {
			continue
		}

		if pat.Detect(s, toolName, input) {
			s.incrementPatternsFired()
			s.mu.Lock()
			s.patternHitCount[pat.ID]++
			s.mu.Unlock()
			elapsed := time.Since(start)
			// Update view timestamp now that detection is done.
			if viewFilePath != "" {
				s.mu.Lock()
				s.viewTimestamps[viewFilePath] = time.Now()
				s.mu.Unlock()
			}
			result := &coachResult{
				PatternID:   pat.ID,
				Severity:    effectiveSev,
				Tip:         pat.Suggest(s, toolName, input),
				DelayMicros: elapsed.Microseconds(),
			}
			if pat.FixInput != nil {
				result.SuggestedInput = pat.FixInput(s, toolName, input)
			}
			return result
		}
	}

	// Update view timestamp on no-match path too.
	if viewFilePath != "" {
		s.mu.Lock()
		s.viewTimestamps[viewFilePath] = time.Now()
		s.mu.Unlock()
	}

	return nil
}

// trackFileAccess updates viewedFiles, editedFiles, and timestamps based on
// the tool. This is used both by runCoach and by direct callers in tests.
func (s *sessionState) trackFileAccess(toolName, input string) {
	switch toolName {
	case "view":
		p, _ := jsonpeek(input, "file_path")
		if p != "" {
			s.mu.Lock()
			s.viewedFiles[p]++
			s.viewTimestamps[p] = time.Now()
			s.mu.Unlock()
		}
	case "edit":
		p, _ := jsonpeek(input, "file_path")
		if p != "" {
			s.mu.Lock()
			s.editedFiles[p]++
			s.mu.Unlock()
		}
	}
}
