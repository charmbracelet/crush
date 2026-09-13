package message

// Verification states recorded under the "verification" key on
// ToolResult.Metadata. The gate's scan sorts results on these: failed
// and pending proceed to verification, unverified is recorded but never
// re-entered.
const (
	VerificationPassed     = "passed"
	VerificationFailed     = "failed"
	VerificationPending    = "pending"
	VerificationUnverified = "unverified"
)

// VerificationCheck is one entry in the "verification" metadata list —
// a mutation can carry several checks (diagnostics delta, targeted test,
// configured verify command), so the value is a list, not a scalar.
type VerificationCheck struct {
	Check  string `json:"check"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
	// Command is the shell command a gate-run check executes; empty for
	// decorator-run checks.
	Command string `json:"command,omitempty"`
	// Timeout bounds a gate-run check, in seconds.
	Timeout int `json:"timeout,omitempty"`
}
