package compact

import (
	"regexp"
	"strings"
)

var commitRe = regexp.MustCompile(`\b([0-9a-f]{7,40})\s*[:\-]\s*(.+)`)

const maxCommits = 8

// ExtractCommits extracts git commit references from bash output blocks.
func ExtractCommits(blocks []NormalizedBlock) []string {
	var commits []string
	seen := make(map[string]bool)

	for _, b := range blocks {
		if b.Kind != BlockBash {
			continue
		}
		text := b.Output
		if text == "" {
			text = b.Command
		}
		for line := range strings.SplitSeq(text, "\n") {
			m := commitRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			hash := m[1]
			msg := strings.TrimSpace(m[2])
			entry := hash[:min(7, len(hash))] + ": " + Clip(msg, 100)
			if !seen[entry] {
				seen[entry] = true
				commits = append(commits, entry)
			}
		}
	}

	if len(commits) > maxCommits {
		commits = commits[len(commits)-maxCommits:]
	}
	return commits
}
