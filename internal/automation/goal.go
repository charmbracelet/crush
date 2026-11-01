package automation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// GoalStatus is the lifecycle state of a goal.
type GoalStatus string

const (
	// GoalActive means the goal loop is running.
	GoalActive GoalStatus = "active"
	// GoalAchieved means the objective was completed and verified.
	GoalAchieved GoalStatus = "achieved"
	// GoalPaused means the loop stopped before completion.
	GoalPaused GoalStatus = "paused"
	// GoalIdle means a goal exists but its loop is not running, for
	// example after resuming a session.
	GoalIdle GoalStatus = "idle"
)

const (
	// DefaultGoalBudget is the default number of model turns a goal may
	// consume.
	DefaultGoalBudget = 100
	// MaxGoalBudget is the hard ceiling for a goal budget.
	MaxGoalBudget = 1000
	// MaxGoalVerificationFailures is the number of rejected completion
	// claims after which a goal pauses instead of retrying forever.
	MaxGoalVerificationFailures = 3
)

// Goal is a standing objective pursued across turns.
type Goal struct {
	Objective string     `json:"objective"`
	Status    GoalStatus `json:"status"`
	Turn      int        `json:"turn"`
	Budget    int        `json:"budget"`
	StartedAt time.Time  `json:"started_at"`
	// FinishedAt is set when the goal is achieved or paused.
	FinishedAt time.Time `json:"finished_at,omitzero"`
	// PauseReason explains why a paused goal stopped.
	PauseReason string `json:"pause_reason,omitempty"`
	// LastVerification carries the most recent rejection reason.
	LastVerification string `json:"last_verification,omitempty"`
	// VerificationFailures counts consecutive rejected completion claims.
	VerificationFailures int `json:"verification_failures,omitempty"`
}

// NewGoal creates an active goal with the given budget. A budget outside the
// allowed range is clamped to it.
func NewGoal(objective string, budget int, now time.Time) *Goal {
	if budget <= 0 {
		budget = DefaultGoalBudget
	}
	if budget > MaxGoalBudget {
		budget = MaxGoalBudget
	}
	return &Goal{
		Objective: strings.TrimSpace(objective),
		Status:    GoalActive,
		Budget:    budget,
		StartedAt: now,
	}
}

// Active reports whether the goal loop is running.
func (g *Goal) Active() bool {
	return g != nil && g.Status == GoalActive
}

// Elapsed returns how long the goal has been running.
func (g *Goal) Elapsed(now time.Time) time.Duration {
	if g == nil {
		return 0
	}
	end := now
	if !g.FinishedAt.IsZero() {
		end = g.FinishedAt
	}
	if end.Before(g.StartedAt) {
		return 0
	}
	return end.Sub(g.StartedAt)
}

// AddTurns advances the turn counter and reports whether budget remains.
func (g *Goal) AddTurns(n int) bool {
	if n < 1 {
		n = 1
	}
	g.Turn += n
	return g.Turn < g.Budget
}

// Pause stops the goal with a reason.
func (g *Goal) Pause(reason string, now time.Time) {
	g.Status = GoalPaused
	g.PauseReason = reason
	g.FinishedAt = now
}

// Achieve marks the goal complete.
func (g *Goal) Achieve(now time.Time) {
	g.Status = GoalAchieved
	g.PauseReason = ""
	g.FinishedAt = now
}

// Resume restarts an idle or paused goal without resetting its turn count.
func (g *Goal) Resume() {
	g.Status = GoalActive
	g.FinishedAt = time.Time{}
	g.PauseReason = ""
}

// StatusLine renders the one-line status shown by /goal status.
func (g *Goal) StatusLine(now time.Time) string {
	if g == nil {
		return "No goal set"
	}
	elapsed := FormatDuration(g.Elapsed(now))
	switch g.Status {
	case GoalActive:
		line := fmt.Sprintf("Goal active · turn %d/%d · %s", g.Turn, g.Budget, elapsed)
		if g.LastVerification != "" {
			line += " · check: " + truncateStatus(g.LastVerification)
		}
		return line
	case GoalAchieved:
		return fmt.Sprintf("Goal achieved · %s · %d turns", elapsed, g.Turn)
	case GoalPaused:
		if g.PauseReason == "" {
			return "Goal paused"
		}
		return "Goal paused · " + g.PauseReason
	case GoalIdle:
		return fmt.Sprintf("Goal idle · turn %d/%d · %s", g.Turn, g.Budget, elapsed)
	default:
		return "Goal " + string(g.Status)
	}
}

// FormatDuration renders a compact human duration such as 2m or 1h5m.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func truncateStatus(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 60 {
		return s
	}
	return s[:59] + "…"
}

var goalCompleteRE = regexp.MustCompile(`(?is)<goal_complete>(.*?)(?:</goal_complete>|$)`)

// ParseGoalComplete extracts the evidence from a completion marker. The
// closing tag is optional so a truncated final message still counts.
func ParseGoalComplete(text string) (string, bool) {
	m := goalCompleteRE.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	evidence := strings.TrimSpace(m[1])
	if evidence == "" {
		return "", false
	}
	return evidence, true
}

// GoalPrompt builds the instruction sent for the first or a continuation
// turn of a goal.
func GoalPrompt(objective string, turn, budget int, rejection string) string {
	var b strings.Builder
	b.WriteString("You are pursuing a standing goal across multiple turns.\n\n")
	b.WriteString("<goal>\n")
	b.WriteString(strings.TrimSpace(objective))
	b.WriteString("\n</goal>\n\n")
	if turn <= 0 {
		b.WriteString("Start working toward the goal now.\n")
	} else {
		fmt.Fprintf(&b, "Continue working toward the goal. This is turn %d of a %d-turn budget.\n", turn+1, budget)
	}
	if rejection != "" {
		b.WriteString("\nThe independent completion check rejected the previous completion claim:\n")
		b.WriteString(rejection)
		b.WriteString("\nClose that specific gap before claiming completion again.\n")
	}
	b.WriteString("\nWork from evidence: read the relevant files, run commands, and run the tests. Do not claim completion unless the objective is genuinely met.\n")
	b.WriteString("When, and only when, every part of the objective is satisfied, end your final message with:\n")
	b.WriteString("<goal_complete>evidence: files changed, commands run, tests passing</goal_complete>\n")
	b.WriteString("Do not use the marker for partial progress.\n")
	return b.String()
}

// VerificationPrompt builds the independent completion check.
func VerificationPrompt(objective, evidence string) string {
	var b strings.Builder
	b.WriteString("You are an independent completion checker. Decide whether the evidence below proves the objective is fully met.\n\n")
	b.WriteString("<objective>\n")
	b.WriteString(strings.TrimSpace(objective))
	b.WriteString("\n</objective>\n\n")
	b.WriteString("<evidence>\n")
	b.WriteString(strings.TrimSpace(evidence))
	b.WriteString("\n</evidence>\n\n")
	b.WriteString("Inspect the conversation and the evidence. Reject the claim when evidence is missing, partial, or does not prove the objective. Do not do new work.\n")
	b.WriteString("Reply with exactly one JSON object and nothing else:\n")
	b.WriteString(`{"verified": true, "reason": "why"}`)
	return b.String()
}

// Verdict is the result of an independent completion check.
type Verdict struct {
	Verified bool   `json:"verified"`
	Reason   string `json:"reason"`
}

// ParseVerdict extracts the JSON verdict from a checker reply. A reply that
// is not valid JSON is treated as a rejection with the raw text as reason.
func ParseVerdict(text string) Verdict {
	text = strings.TrimSpace(text)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		var v Verdict
		if err := json.Unmarshal([]byte(text[start:end+1]), &v); err == nil {
			if v.Reason == "" {
				v.Reason = "the checker returned no reason"
			}
			return v
		}
	}
	reason := strings.TrimSpace(text)
	if reason == "" {
		reason = "the checker returned an empty verdict"
	}
	if len(reason) > 300 {
		reason = reason[:300] + "…"
	}
	return Verdict{Verified: false, Reason: reason}
}
