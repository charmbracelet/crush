package proto

// SkillDiscoveryState mirrors skills.DiscoveryState across the wire.
// Values must stay in sync with internal/skills.DiscoveryState; do not
// reorder without a coordinated server/client bump.
type SkillDiscoveryState int

const (
	// SkillStateNormal indicates the skill was parsed and validated
	// successfully.
	SkillStateNormal SkillDiscoveryState = iota
	// SkillStateError indicates discovery encountered a scan/parse/validate
	// error.
	SkillStateError
)

// SkillState is the wire representation of skills.SkillState.
type SkillState struct {
	Name  string              `json:"name"`
	Path  string              `json:"path"`
	State SkillDiscoveryState `json:"state"`
	Error string              `json:"error,omitempty"`
}

// SkillsEvent is the wire representation of skills.Event.
type SkillsEvent struct {
	States []SkillState `json:"states"`
}
