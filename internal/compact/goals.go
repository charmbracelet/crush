package compact

import (
	"regexp"
	"strings"
)

var (
	scopeChangeRe = regexp.MustCompile(`(?i)\b(instead|actually|change of plan|forget that|new task|switch to|now I want|pivot|let'?s do|stop .+ and)\b`)
	taskRe        = regexp.MustCompile(`(?i)\b(fix|implement|add|create|build|refactor|debug|investigate|update|remove|delete|migrate|deploy|test|write|set up)\b`)
	noiseShortRe  = regexp.MustCompile(`(?i)^(ok|yes|no|sure|yeah|yep|go|hi|hey|thx|thanks|y|n|k)\s*[.!?]*$`)
	nonGoalRe     = regexp.MustCompile(`^\s*[\[│├└─╭╰]|` + "```" + `|^\s*(=[A-Z]+\(|function |const |let |var |import |export |class )|^(https?:|file:|\/[A-Za-z])`)
	bulletRe      = regexp.MustCompile(`^\s*(?:[-*+]|\d+\.)\s+`)
)

const (
	maxGoalChars = 200
	maxGoals     = 8
	leadingChars = 200
)

// ExtractGoals extracts session goals from user messages.
func ExtractGoals(blocks []NormalizedBlock) []string {
	var goals []string
	var latestScopeChange []string

	for _, b := range blocks {
		if b.Kind != BlockUser {
			continue
		}
		lines := filterGoalLines(NonEmptyLines(b.Text))
		if len(lines) == 0 {
			continue
		}

		if len(goals) == 0 {
			goals = append(goals, lines[:min(6, len(lines))]...)
			continue
		}

		leading := b.Text
		if len(leading) > leadingChars {
			leading = leading[:leadingChars]
		}
		if scopeChangeRe.MatchString(leading) {
			latestScopeChange = make([]string, 0, 3)
			for _, l := range lines[:min(3, len(lines))] {
				latestScopeChange = append(latestScopeChange, Clip(l, maxGoalChars))
			}
		} else if taskRe.MatchString(leading) && len(lines[0]) > 15 {
			latestScopeChange = make([]string, 0, 2)
			for _, l := range lines[:min(2, len(lines))] {
				latestScopeChange = append(latestScopeChange, Clip(l, maxGoalChars))
			}
		}
	}

	if len(latestScopeChange) > 0 {
		goals = append(goals, "[Scope change]")
		goals = append(goals, latestScopeChange...)
	}

	if len(goals) > maxGoals {
		goals = goals[:maxGoals]
	}
	return goals
}

func filterGoalLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = stripLeadingBullet(l)
		if !isSubstantiveGoal(l) {
			continue
		}
		out = append(out, l)
	}
	return out
}

func isSubstantiveGoal(text string) bool {
	t := strings.TrimSpace(text)
	if len(t) <= 5 || len(t) > maxGoalChars {
		return false
	}
	if noiseShortRe.MatchString(t) {
		return false
	}
	if nonGoalRe.MatchString(t) {
		return false
	}
	return true
}

func stripLeadingBullet(line string) string {
	return strings.TrimSpace(bulletRe.ReplaceAllString(line, ""))
}
