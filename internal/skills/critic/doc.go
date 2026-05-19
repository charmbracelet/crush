// Package critic implements the self-critic skill for Crush.
//
// It provides a middleware decorator that wraps the primary agent's
// SessionAgent, emitting checkpoints after significant actions (plans, edits,
// tool calls, messages). A secondary critic agent reviews each checkpoint and
// returns structured feedback: approve, revise, or halt.
//
// The package is designed as a skill under internal/skills/critic/ so that
// projects can override prompts and thresholds via .crush/skills/critic/.
package critic
