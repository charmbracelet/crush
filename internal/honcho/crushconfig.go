package honcho

import "github.com/charmbracelet/crush/internal/config"

// Feature is the name a skill puts in its `requires:` field to be
// hidden unless Honcho memory is switched on.
const Feature = "honcho"

// FromCrushConfig maps Crush's `honcho` config block onto the
// overrides Resolve consumes.
//
// A nil block yields the zero value, which defers entirely to the
// shared cross-harness config file and the environment.
func FromCrushConfig(h *config.Honcho) Config {
	if h == nil {
		return Config{}
	}
	out := Config{
		Enabled:         h.Enabled,
		APIKey:          h.APIKey,
		BaseURL:         h.BaseURL,
		Workspace:       h.Workspace,
		PeerName:        h.PeerName,
		AgentPeer:       h.AgentPeer,
		RecallMode:      RecallMode(h.RecallMode),
		ObservationMode: ObservationMode(h.ObservationMode),
		SessionStrategy: SessionStrategy(h.SessionStrategy),
		AgentObserveMe:  h.AgentObserveMe,
		MaxConclusions:  h.MaxConclusions,
		ContextTokens:   h.ContextTokens,
	}
	// Tool capture defaults to on, so an omitted field means "use the
	// default" and only an explicit false turns it off.
	out.CaptureTools = h.CaptureTools == nil || *h.CaptureTools
	return out
}

// Features returns the feature names to expose to skill discovery, so
// memory-specific skills stay out of the system prompt when memory is
// off.
func Features(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	if !Resolve(FromCrushConfig(cfg.Honcho)).Enabled {
		return nil
	}
	return []string{Feature}
}
