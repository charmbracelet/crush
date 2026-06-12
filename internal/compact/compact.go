package compact

import (
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

// handoffPreamble is prepended to every compaction summary.
const handoffPreamble = "This summary captures work done before the most recent messages in this session. " +
	"Read it to pick up context — this is work already in progress. " +
	"Do not recap what was done, do not ask what to do next. " +
	"Continue directly where you left off."

// Input holds the parameters for a compaction.
type Input struct {
	Messages        []message.Message
	PreviousSummary string
}

// Compact produces an algorithmic, LLM-free compaction of the conversation.
// It normalizes messages, filters noise, extracts structured sections, and
// formats them with cache-friendly ordering. If a previous summary exists,
// it merges using per-section policies (sticky/volatile/union).
func Compact(input Input) string {
	blocks := Normalize(input.Messages)
	blocks = FilterNoise(blocks)

	data := buildSections(blocks)
	fresh := FormatSummary(data)

	if input.PreviousSummary != "" {
		prev := stripPreamble(input.PreviousSummary)
		fresh = MergePrevious(prev, fresh)
	}

	if fresh == "" {
		return ""
	}

	return wrapLongLines(handoffPreamble+"\n\n"+fresh, tuiSafeLineChars)
}

func buildSections(blocks []NormalizedBlock) SectionData {
	fileData := ExtractFileAndSymbols(blocks)

	briefSections := BuildBriefSections(blocks)
	sessionGoal := ExtractGoals(blocks)
	outstandingContext := ExtractOutstandingContext(blocks)
	commits := ExtractCommits(blocks)

	turns := IdentifyTurns(blocks)
	turnSummaries := make([]string, len(turns))
	for i, t := range turns {
		turnSummaries[i] = t.Summary
	}

	return SectionData{
		SessionGoal:        sessionGoal,
		OutstandingContext: outstandingContext,
		FilesAndChanges:    formatFileActivity(fileData),
		Commits:            commits,
		TypeCatalog:        formatTypeCatalog(fileData),
		TurnSummaries:      turnSummaries,
		BriefTranscript:    StringifyBrief(briefSections),
	}
}

func formatFileActivity(data UnifiedExtractResult) []string {
	act := data.FileActivity
	var lines []string
	if len(act.Modified) > 0 {
		lines = append(lines, "Modified: "+joinCapped(act.Modified, 10))
	}
	if len(act.Created) > 0 {
		lines = append(lines, "Created: "+joinCapped(act.Created, 10))
	}
	if len(act.Read) > 0 {
		lines = append(lines, "Read: "+joinCapped(act.Read, 10))
	}
	return lines
}

func formatTypeCatalog(data UnifiedExtractResult) []string {
	if len(data.TypeCatalog) == 0 {
		return nil
	}
	var lines []string
	totalSigs := 0
	for _, entry := range data.TypeCatalog {
		if totalSigs >= maxTotalSigs {
			break
		}
		lines = append(lines, entry.File+":")
		for _, sig := range entry.Signatures {
			if totalSigs >= maxTotalSigs {
				break
			}
			lines = append(lines, "  "+sig)
			totalSigs++
		}
	}
	return lines
}

const maxTotalSigs = 30

func joinCapped(items []string, limit int) string {
	if len(items) <= limit {
		return strings.Join(items, ", ")
	}
	kept := items[:limit]
	omitted := items[limit:]
	return strings.Join(kept, ", ") + ", +recall: " + strings.Join(omitted, ", ")
}

func stripPreamble(text string) string {
	if strings.HasPrefix(text, "This summary captures work done before") {
		idx := strings.IndexByte(text, '[')
		if idx > 0 {
			return strings.TrimSpace(text[idx:])
		}
		nl := strings.Index(text, "\n\n")
		if nl > 0 {
			return strings.TrimSpace(text[nl+2:])
		}
	}
	return text
}
