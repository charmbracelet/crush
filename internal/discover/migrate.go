package discover

import "charm.land/catwalk/v3/pkg/catwalk"

// legacyModelPricingFields maps catwalk v2 model cost fields onto their
// v3 pricing counterparts.
var legacyModelPricingFields = map[string]string{
	"cost_per_1m_in":         "input",
	"cost_per_1m_out":        "output",
	"cost_per_1m_in_cached":  "cache_create",
	"cost_per_1m_out_cached": "cache_hit",
}

// MigrateV2Model migrates a single model object in place and
// reports whether anything changed. Legacy fields are removed after
// being folded into their v3 counterparts so they never leak back into
// the decoded config.
func MigrateV2Model(model map[string]any) bool {
	changed := false

	if _, ok := model["pricing"]; !ok {
		pricing := map[string]any{}
		for legacy, field := range legacyModelPricingFields {
			value, ok := model[legacy]
			if !ok {
				continue
			}
			pricing[field] = value
			delete(model, legacy)
			changed = true
		}
		if len(pricing) > 0 {
			model["pricing"] = pricing
		}
	} else {
		for legacy := range legacyModelPricingFields {
			if _, ok := model[legacy]; ok {
				delete(model, legacy)
				changed = true
			}
		}
	}

	if value, ok := model["supports_attachments"]; ok {
		delete(model, "supports_attachments")
		capabilities, ok := model["capabilities"].(map[string]any)
		if !ok {
			capabilities = map[string]any{}
			model["capabilities"] = capabilities
		}
		if _, ok := capabilities["vision"]; !ok {
			capabilities["vision"] = value
		}
		changed = true
	}

	canReason, hasCanReason := model["can_reason"]
	levels, hasLevels := model["reasoning_levels"]
	defaultEffort, hasDefaultEffort := model["default_reasoning_effort"]
	for _, legacy := range []string{"can_reason", "reasoning_levels", "default_reasoning_effort"} {
		if _, ok := model[legacy]; ok {
			delete(model, legacy)
			changed = true
		}
	}
	if _, ok := model["reasoning"]; !ok && (hasCanReason || hasLevels || hasDefaultEffort) {
		reasoning := map[string]any{}
		if can, ok := canReason.(bool); ok && !can {
			reasoning["thinking"] = catwalk.ThinkingNever
		} else {
			reasoning["thinking"] = catwalk.ThinkingToggleable
			if raw, ok := levels.([]any); ok && len(raw) > 0 {
				values := make([]string, 0, len(raw))
				for _, level := range raw {
					if value, ok := level.(string); ok {
						values = append(values, value)
					}
				}
				if len(values) > 0 {
					effortLevels := make([]any, 0, len(values))
					for _, level := range catwalk.NewEffortLevels(values...) {
						effortLevels = append(effortLevels, map[string]any{
							"value":   level.Value,
							"display": level.Display,
						})
					}
					reasoning["effort_levels"] = effortLevels
					if effort, ok := defaultEffort.(string); ok && effort != "" {
						reasoning["default_effort_level"] = effort
					}
				}
			}
		}
		model["reasoning"] = reasoning
	}

	return changed
}
