package config

import "slices"

// SetSkillDisabledConfig toggles a skill's entry in the given scope's
// options.disabled_skills list. Skills are opt-out — an enabled skill is
// simply absent from the list — so disabling appends and enabling
// removes. The dialog only offers discovered skills, so unknown names
// cannot arrive from the UI.
func (s *ConfigStore) SetSkillDisabledConfig(scope Scope, name string, disabled bool) error {
	list := make([]string, 0, len(s.Config().Options.DisabledSkills)+1)
	for _, existing := range s.Config().Options.DisabledSkills {
		if existing == name {
			continue
		}
		list = append(list, existing)
	}
	if disabled {
		list = append(list, name)
	}
	slices.Sort(list)
	return s.SetConfigFields(scope, map[string]any{
		"options.disabled_skills": list,
	})
}
