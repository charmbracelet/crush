package dialog

import (
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// SkillsTogglesID is the identifier for the skills toggles dialog.
const SkillsTogglesID = "skills_toggles"

// SkillToggleItem describes one skill in the toggles dialog.
type SkillToggleItem struct {
	Name string
	// Disabled is the repository-scoped override: when true the skill is
	// hidden from the agent in this repository.
	Disabled bool
	// ConfigDisabled is the skill's disabled state in the config, before
	// any repository-scoped override. The Global scope reads and writes
	// this flag.
	ConfigDisabled bool
	// EnabledOverride is the repository-scoped enabled override: a
	// config-disabled skill the user enabled here. Only the Local scope
	// considers it.
	EnabledOverride bool
	// Error is non-empty when the skill failed discovery (malformed
	// frontmatter, unreadable file, ...). Errored skills show a red ×
	// and the error on the right edge instead of the enabled check.
	Error string
}

// localDisabled returns the effective local state: a config-disabled
// skill stays disabled locally unless the repository enabled override
// turned it on.
func (i SkillToggleItem) localDisabled() bool {
	return i.Disabled || (i.ConfigDisabled && !i.EnabledOverride)
}

// ActionToggleSkill is sent when the user toggles a skill. Local toggles
// persist a repository-scoped override; global toggles edit the config's
// options.disabled_skills list.
type ActionToggleSkill struct {
	Name     string
	Disabled bool
	Global   bool
}

// SkillToggles lets the user enable and disable skills, either for the
// current repository (Local, the default) or in the config (Global).
type SkillToggles struct {
	toggleDialog
	items []SkillToggleItem
}

var _ Dialog = (*SkillToggles)(nil)

// NewSkillsToggles creates a new skills toggles dialog.
func NewSkillsToggles(com *common.Common, items []SkillToggleItem) *SkillToggles {
	m := &SkillToggles{items: items}
	m.toggleDialog = newToggleDialog(com, m, "Toggle Skills", "No skills configured.")
	return m
}

// ID implements Dialog.
func (m *SkillToggles) ID() string {
	return SkillsTogglesID
}

// Items returns the current items.
func (m *SkillToggles) Items() []SkillToggleItem {
	return m.items
}

// SetItems replaces the items, keeping the cursor clamped, so an open
// dialog updates after a toggle or an overrides refresh.
func (m *SkillToggles) SetItems(items []SkillToggleItem) {
	m.items = items
	m.cursor = min(m.cursor, max(0, len(items)-1))
}

func (m *SkillToggles) toggleRows(MCPToggleScope) []toggleRow {
	t := m.com.Styles
	rows := make([]toggleRow, len(m.items))
	for i, item := range m.items {
		// Enabled skills get a check, disabled ones an empty column; errored skills a red × and "error".
		r := toggleRow{mark: " ", plainMark: " ", name: item.Name}
		switch {
		case item.Error != "":
			r.mark, r.plainMark = t.Tool.IconError.Render(), styles.ToolError
			r.status, r.statusStyle = "error", t.Tool.IconError.UnsetString()
		case !m.itemDisabled(item):
			r.mark, r.plainMark = t.Tool.IconSuccess.Render(), styles.CheckIcon
		}
		rows[i] = r
	}
	return rows
}

func (m *SkillToggles) toggleAt(i int, scope MCPToggleScope) Action {
	item := m.items[i]
	currentlyDisabled := item.localDisabled()
	if scope == MCPToggleScopeGlobal {
		currentlyDisabled = item.ConfigDisabled
	}
	newState := !currentlyDisabled
	if scope == MCPToggleScopeGlobal {
		m.items[i].ConfigDisabled = newState
	} else {
		m.items[i].Disabled = newState
		if item.ConfigDisabled && !newState {
			m.items[i].EnabledOverride = true
		}
	}
	return ActionToggleSkill{
		Name:     item.Name,
		Disabled: newState,
		Global:   scope == MCPToggleScopeGlobal,
	}
}

// itemDisabled reports whether the item shows as disabled given the
// active scope: Local reads the repository overrides, Global reads the
// config's raw disabled flag.
func (m *SkillToggles) itemDisabled(item SkillToggleItem) bool {
	if m.scope == MCPToggleScopeGlobal {
		return item.ConfigDisabled
	}
	return item.localDisabled()
}
