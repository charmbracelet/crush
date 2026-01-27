package toolchain

import (
	"fmt"
	"strings"
	"time"
)

// SummarizerConfig holds configuration options for the chain summarizer.
// This provides fine-grained control over summarization behavior beyond
// what the shared Config provides.
type SummarizerConfig struct {
	// MinChainLength is the minimum number of tool calls before summarization kicks in.
	// Chains shorter than this will not be summarized.
	MinChainLength int
	// CollapseThreshold is the number of tool calls above which chains are collapsed by default.
	CollapseThreshold int
	// IncludeDuration controls whether timing information is included in summaries.
	IncludeDuration bool
	// IncludeErrors controls whether error details are included in summaries.
	IncludeErrors bool
	// MaxToolsInSummary limits how many individual tools are listed in the summary.
	MaxToolsInSummary int
}

// DefaultSummarizerConfig returns sensible default configuration.
func DefaultSummarizerConfig() SummarizerConfig {
	return SummarizerConfig{
		MinChainLength:    2,
		CollapseThreshold: 5,
		IncludeDuration:   true,
		IncludeErrors:     true,
		MaxToolsInSummary: 5,
	}
}

// SummarizerConfigFromConfig creates a SummarizerConfig from the shared Config.
func SummarizerConfigFromConfig(cfg Config) SummarizerConfig {
	return SummarizerConfig{
		MinChainLength:    cfg.MinCalls,
		CollapseThreshold: 5, // Not in Config, use default
		IncludeDuration:   cfg.IncludeTimings,
		IncludeErrors:     true, // Default to true
		MaxToolsInSummary: 5,    // Default
	}
}

// Summarizer generates human-readable summaries of tool chains.
type Summarizer struct {
	config SummarizerConfig
}

// NewSummarizer creates a new summarizer with the given configuration.
func NewSummarizer(config SummarizerConfig) *Summarizer {
	return &Summarizer{config: config}
}

// NewSummarizerFromConfig creates a summarizer from the shared Config type.
func NewSummarizerFromConfig(cfg Config) *Summarizer {
	return NewSummarizer(SummarizerConfigFromConfig(cfg))
}

// NewDefaultSummarizer creates a summarizer with default configuration.
func NewDefaultSummarizer() *Summarizer {
	return NewSummarizer(DefaultSummarizerConfig())
}

// Summarize generates a summary for the given chain.
// Returns nil if the chain is too short to summarize.
func (s *Summarizer) Summarize(chain *Chain) *Summary {
	if chain == nil || chain.IsEmpty() {
		return nil
	}

	if chain.Len() < s.config.MinChainLength {
		return nil
	}

	text := s.generateSummaryText(chain)
	summary := NewSummary(chain, text)
	summary.Collapsed = chain.Len() >= s.config.CollapseThreshold

	return summary
}

// ShouldSummarize returns true if the chain meets criteria for summarization.
func (s *Summarizer) ShouldSummarize(chain *Chain) bool {
	if chain == nil || chain.IsEmpty() {
		return false
	}
	return chain.Len() >= s.config.MinChainLength
}

// ShouldCollapse returns true if the chain should be displayed collapsed.
func (s *Summarizer) ShouldCollapse(chain *Chain) bool {
	if chain == nil {
		return false
	}
	return chain.Len() >= s.config.CollapseThreshold
}

// generateSummaryText creates the human-readable summary text.
func (s *Summarizer) generateSummaryText(chain *Chain) string {
	var sb strings.Builder

	// Count tool usage
	toolCounts := s.countTools(chain)
	totalCalls := chain.Len()
	errorCount := chain.ErrorCount()

	// Build the main summary line
	if totalCalls == 1 {
		sb.WriteString(fmt.Sprintf("Ran %s", chain.Calls[0].Name))
	} else {
		sb.WriteString(fmt.Sprintf("Ran %d tool calls", totalCalls))
	}

	// Add tool breakdown if multiple tools were used
	if len(toolCounts) > 1 {
		sb.WriteString(": ")
		sb.WriteString(s.formatToolCounts(toolCounts))
	} else if len(toolCounts) == 1 && totalCalls > 1 {
		// Single tool used multiple times
		for name := range toolCounts {
			sb.WriteString(fmt.Sprintf(" (%s)", name))
		}
	}

	// Add duration if configured and available
	if s.config.IncludeDuration && chain.Duration() > 0 {
		sb.WriteString(fmt.Sprintf(" in %s", formatDuration(chain.Duration())))
	}

	// Add error information if configured
	if s.config.IncludeErrors && errorCount > 0 {
		if errorCount == 1 {
			sb.WriteString(" (1 error)")
		} else {
			sb.WriteString(fmt.Sprintf(" (%d errors)", errorCount))
		}
	}

	return sb.String()
}

// countTools returns a map of tool names to their call counts.
func (s *Summarizer) countTools(chain *Chain) map[string]int {
	counts := make(map[string]int)
	for _, call := range chain.Calls {
		counts[call.Name]++
	}
	return counts
}

// formatToolCounts formats the tool usage counts for display.
func (s *Summarizer) formatToolCounts(counts map[string]int) string {
	// Sort tools by frequency (most used first)
	type toolCount struct {
		name  string
		count int
	}
	var sorted []toolCount
	for name, count := range counts {
		sorted = append(sorted, toolCount{name, count})
	}
	// Simple bubble sort since we expect small lists
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Build the output string
	var parts []string
	for i, tc := range sorted {
		if i >= s.config.MaxToolsInSummary {
			remaining := len(sorted) - s.config.MaxToolsInSummary
			parts = append(parts, fmt.Sprintf("+%d more", remaining))
			break
		}
		if tc.count > 1 {
			parts = append(parts, fmt.Sprintf("%s x%d", tc.name, tc.count))
		} else {
			parts = append(parts, tc.name)
		}
	}
	return strings.Join(parts, ", ")
}

// formatDuration formats a duration for human-readable display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		secs := d.Seconds()
		if secs == float64(int(secs)) {
			return fmt.Sprintf("%ds", int(secs))
		}
		return fmt.Sprintf("%.1fs", secs)
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	if secs == 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm%ds", mins, secs)
}

// PatternSummary generates a summary based on detected patterns in the chain.
// This provides more context-aware summaries for common tool usage patterns.
func (s *Summarizer) PatternSummary(chain *Chain) string {
	if chain == nil || chain.IsEmpty() {
		return ""
	}

	pattern := s.detectPattern(chain)
	switch pattern {
	case patternFileSearch:
		return s.summarizeFileSearch(chain)
	case patternCodeEdit:
		return s.summarizeCodeEdit(chain)
	case patternBashSequence:
		return s.summarizeBashSequence(chain)
	case patternReadModifyWrite:
		return s.summarizeReadModifyWrite(chain)
	default:
		return s.generateSummaryText(chain)
	}
}

// chainPattern represents detected patterns in tool chains.
type chainPattern int

const (
	patternUnknown chainPattern = iota
	patternFileSearch
	patternCodeEdit
	patternBashSequence
	patternReadModifyWrite
)

// detectPattern analyzes the chain to identify common usage patterns.
func (s *Summarizer) detectPattern(chain *Chain) chainPattern {
	if chain.Len() == 0 {
		return patternUnknown
	}

	toolNames := chain.ToolNames()

	// Check for file search pattern (glob + grep combinations)
	hasGlob := contains(toolNames, "Glob")
	hasGrep := contains(toolNames, "Grep")
	if hasGlob || hasGrep {
		// If mostly search tools, it's a file search pattern
		searchCount := 0
		for _, call := range chain.Calls {
			if call.Name == "Glob" || call.Name == "Grep" {
				searchCount++
			}
		}
		if float64(searchCount)/float64(chain.Len()) >= 0.5 {
			return patternFileSearch
		}
	}

	// Check for code edit pattern (Read + Edit combinations)
	hasRead := contains(toolNames, "Read")
	hasEdit := contains(toolNames, "Edit")
	hasWrite := contains(toolNames, "Write")
	if hasRead && (hasEdit || hasWrite) {
		return patternReadModifyWrite
	}

	// Check for bash sequence
	bashCount := 0
	for _, call := range chain.Calls {
		if call.Name == "Bash" {
			bashCount++
		}
	}
	if bashCount > 0 && float64(bashCount)/float64(chain.Len()) >= 0.7 {
		return patternBashSequence
	}

	// Check for code edit with just Edit
	if hasEdit {
		return patternCodeEdit
	}

	return patternUnknown
}

// summarizeFileSearch generates a summary for file search patterns.
func (s *Summarizer) summarizeFileSearch(chain *Chain) string {
	globCount := 0
	grepCount := 0
	for _, call := range chain.Calls {
		switch call.Name {
		case "Glob":
			globCount++
		case "Grep":
			grepCount++
		}
	}

	var parts []string
	if globCount > 0 {
		if globCount == 1 {
			parts = append(parts, "1 file search")
		} else {
			parts = append(parts, fmt.Sprintf("%d file searches", globCount))
		}
	}
	if grepCount > 0 {
		if grepCount == 1 {
			parts = append(parts, "1 content search")
		} else {
			parts = append(parts, fmt.Sprintf("%d content searches", grepCount))
		}
	}

	summary := "Searched codebase"
	if len(parts) > 0 {
		summary += " (" + strings.Join(parts, ", ") + ")"
	}

	if s.config.IncludeDuration && chain.Duration() > 0 {
		summary += fmt.Sprintf(" in %s", formatDuration(chain.Duration()))
	}

	return summary
}

// summarizeCodeEdit generates a summary for code editing patterns.
func (s *Summarizer) summarizeCodeEdit(chain *Chain) string {
	editCount := 0
	for _, call := range chain.Calls {
		if call.Name == "Edit" {
			editCount++
		}
	}

	var summary string
	if editCount == 1 {
		summary = "Made 1 code edit"
	} else {
		summary = fmt.Sprintf("Made %d code edits", editCount)
	}

	if s.config.IncludeDuration && chain.Duration() > 0 {
		summary += fmt.Sprintf(" in %s", formatDuration(chain.Duration()))
	}

	if s.config.IncludeErrors && chain.HasErrors() {
		summary += fmt.Sprintf(" (%d errors)", chain.ErrorCount())
	}

	return summary
}

// summarizeBashSequence generates a summary for bash command sequences.
func (s *Summarizer) summarizeBashSequence(chain *Chain) string {
	bashCount := 0
	for _, call := range chain.Calls {
		if call.Name == "Bash" {
			bashCount++
		}
	}

	var summary string
	if bashCount == 1 {
		summary = "Ran 1 command"
	} else {
		summary = fmt.Sprintf("Ran %d commands", bashCount)
	}

	if s.config.IncludeDuration && chain.Duration() > 0 {
		summary += fmt.Sprintf(" in %s", formatDuration(chain.Duration()))
	}

	if s.config.IncludeErrors && chain.HasErrors() {
		summary += fmt.Sprintf(" (%d errors)", chain.ErrorCount())
	}

	return summary
}

// summarizeReadModifyWrite generates a summary for read-modify-write patterns.
func (s *Summarizer) summarizeReadModifyWrite(chain *Chain) string {
	readCount := 0
	editCount := 0
	writeCount := 0
	for _, call := range chain.Calls {
		switch call.Name {
		case "Read":
			readCount++
		case "Edit":
			editCount++
		case "Write":
			writeCount++
		}
	}

	modifyCount := editCount + writeCount
	var summary string
	if readCount > 0 && modifyCount > 0 {
		if readCount == 1 && modifyCount == 1 {
			summary = "Read and modified 1 file"
		} else if readCount == modifyCount {
			summary = fmt.Sprintf("Read and modified %d files", readCount)
		} else {
			summary = fmt.Sprintf("Read %d files, made %d modifications", readCount, modifyCount)
		}
	} else if readCount > 0 {
		if readCount == 1 {
			summary = "Read 1 file"
		} else {
			summary = fmt.Sprintf("Read %d files", readCount)
		}
	} else {
		if modifyCount == 1 {
			summary = "Modified 1 file"
		} else {
			summary = fmt.Sprintf("Modified %d files", modifyCount)
		}
	}

	if s.config.IncludeDuration && chain.Duration() > 0 {
		summary += fmt.Sprintf(" in %s", formatDuration(chain.Duration()))
	}

	if s.config.IncludeErrors && chain.HasErrors() {
		summary += fmt.Sprintf(" (%d errors)", chain.ErrorCount())
	}

	return summary
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
