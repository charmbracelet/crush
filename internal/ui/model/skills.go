package model

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
)

type skillStatusItem struct {
	icon  string
	name  string
	title string
	// description is reserved for future use (e.g. showing error details).
	description string
}

var builtinSkillsCache struct {
	once   sync.Once
	skills []*skills.Skill
}

func cachedBuiltinSkills() []*skills.Skill {
	builtinSkillsCache.once.Do(func() {
		builtinSkillsCache.skills = skills.DiscoverBuiltin()
	})
	return builtinSkillsCache.skills
}

// skillsInfo renders the skill discovery status section showing loaded and
// invalid skills.
func (m *UI) skillsInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.Resource.Heading.Render("Skills")
	if isSection {
		title = common.Section(t, title, width)
	}

	items := m.skillStatusItems()
	if len(items) == 0 {
		list := t.Resource.AdditionalText.Render("None")
		return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
	}

	list := skillsList(t, items, width, maxItems)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

func (m *UI) skillStatusItems() []skillStatusItem {
	t := m.com.Styles
	var items []skillStatusItem
	stateNames := make(map[string]struct{}, len(m.skillStates))

	disabledSet := make(map[string]bool)
	if m.com != nil && m.com.Workspace != nil {
		if cfg := m.com.Config(); cfg != nil {
			for _, name := range cfg.Options.DisabledSkills {
				disabledSet[name] = true
			}
		}
		// Repository overrides mirror the toggle dialog: repo-disabled
		// skills hide, and config-disabled skills re-enabled for this
		// repository show again.
		for name, disabled := range m.skillsDisabledOverrides {
			if disabled {
				disabledSet[name] = true
			}
		}
		for name := range m.skillsEnabledOverrides {
			delete(disabledSet, name)
		}
	}

	states := slices.Clone(m.skillStates)
	slices.SortStableFunc(states, func(a, b *skills.SkillState) int {
		return strings.Compare(a.Path, b.Path)
	})
	for _, state := range states {
		name := state.Name
		if name == "" {
			name = filepath.Base(filepath.Dir(state.Path))
		}
		if disabledSet[name] {
			continue
		}
		if _, exists := stateNames[name]; exists {
			continue
		}
		stateNames[name] = struct{}{}
		icon := t.Resource.OnlineIcon.String()
		if state.State == skills.StateError {
			icon = t.Resource.ErrorIcon.String()
		}
		items = append(items, skillStatusItem{
			icon:  icon,
			name:  name,
			title: t.Resource.Name.Render(name),
		})
	}

	builtin := cachedBuiltinSkills()
	slices.SortStableFunc(builtin, func(a, b *skills.Skill) int {
		return strings.Compare(a.Name, b.Name)
	})
	for _, skill := range builtin {
		if _, ok := stateNames[skill.Name]; ok {
			continue
		}
		if disabledSet[skill.Name] {
			continue
		}
		items = append(items, skillStatusItem{
			icon:  t.Resource.OnlineIcon.String(),
			name:  skill.Name,
			title: t.Resource.Name.Render(skill.Name),
		})
	}

	slices.SortStableFunc(items, func(a, b skillStatusItem) int {
		return strings.Compare(a.name, b.name)
	})

	return items
}

func skillsList(t *styles.Styles, items []skillStatusItem, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}

	if len(items) > maxItems {
		visibleItems := items[:maxItems-1]
		remaining := len(items) - (maxItems - 1)
		items = append(visibleItems, skillStatusItem{
			name:  "more",
			title: t.Resource.AdditionalText.Render(fmt.Sprintf("…and %d more", remaining)),
		})
	}

	renderedItems := make([]string, 0, len(items))
	for _, item := range items {
		renderedItems = append(renderedItems, common.Status(t, common.StatusOpts{
			Icon:        item.icon,
			Title:       item.title,
			Description: item.description,
		}, width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedItems...)
}

// skillsTogglesList builds the toggle-skills dialog items from the
// discovered skills, the config's disabled list, and the repository
// overrides mirrored in the UI.
func (m *UI) skillsTogglesList() []dialog.SkillToggleItem {
	cfg := m.com.Config()
	items := make([]dialog.SkillToggleItem, 0)

	seen := make(map[string]struct{})
	add := func(name string) {
		if name == "" {
			return
		}
		if _, dup := seen[name]; dup {
			return
		}
		seen[name] = struct{}{}
		_, configDisabled := disabledSet(cfg)[name]
		_, enabledOverride := m.skillsEnabledOverrides[name]
		_, disabledOverride := m.skillsDisabledOverrides[name]
		items = append(items, dialog.SkillToggleItem{
			Name:            name,
			ConfigDisabled:  configDisabled,
			EnabledOverride: enabledOverride,
			Disabled:        disabledOverride,
		})
	}

	for _, state := range m.skillStates {
		name := state.Name
		if name == "" {
			name = filepath.Base(filepath.Dir(state.Path))
		}
		add(name)
	}
	for _, skill := range cachedBuiltinSkills() {
		add(skill.Name)
	}
	for name := range disabledSet(cfg) {
		add(name)
	}
	// Override names must always appear, even when the underlying skill
	// is gone from config or disk, so it can still be re-enabled.
	for name := range m.skillsDisabledOverrides {
		add(name)
	}
	for name := range m.skillsEnabledOverrides {
		add(name)
	}

	slices.SortFunc(items, func(a, b dialog.SkillToggleItem) int {
		return strings.Compare(a.Name, b.Name)
	})
	return items
}

// disabledSet returns the config's disabled-skills set.
func disabledSet(cfg *config.Config) map[string]struct{} {
	set := make(map[string]struct{})
	if cfg == nil {
		return set
	}
	for _, name := range cfg.Options.DisabledSkills {
		set[name] = struct{}{}
	}
	return set
}

// refreshSkillOverrides fetches the repository-scoped skill toggle sets
// and emits them as a message so the sidebar and dialog update without
// blocking on workspace calls during render.
func (m *UI) refreshSkillOverrides() tea.Cmd {
	return func() tea.Msg {
		disabled, err := m.com.Workspace.SkillsDisabled(context.Background())
		if err != nil {
			return util.NewErrorMsg(err)
		}
		enabled, err := m.com.Workspace.SkillsEnabled(context.Background())
		if err != nil {
			return util.NewErrorMsg(err)
		}
		disabledSet := make(map[string]bool, len(disabled))
		for _, name := range disabled {
			disabledSet[name] = true
		}
		enabledSet := make(map[string]bool, len(enabled))
		for _, name := range enabled {
			enabledSet[name] = true
		}
		return skillsOverridesLoadedMsg{disabled: disabledSet, enabled: enabledSet}
	}
}

// openSkillsTogglesDialog opens the repository-scoped skills toggles
// dialog. The dialog starts with the overrides the UI has mirrored; a
// fresh fetch refreshes both afterwards.
func (m *UI) openSkillsTogglesDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.SkillsTogglesID) {
		m.dialog.BringToFront(dialog.SkillsTogglesID)
		return m.refreshSkillOverrides()
	}
	m.dialog.OpenDialog(dialog.NewSkillsToggles(m.com, m.skillsTogglesList()))
	return m.refreshSkillOverrides()
}

// applySkillToggle persists a skills toggle. Local toggles write a
// repository-scoped override; global toggles edit the config's
// options.disabled_skills list. Mirrors applyMCPToggle.
func (m *UI) applySkillToggle(msg dialog.ActionToggleSkill) tea.Cmd {
	name := msg.Name
	disable := msg.Disabled
	status := "enabled"
	if disable {
		status = "disabled"
	}
	return func() tea.Msg {
		if msg.Global {
			if err := m.com.Workspace.SkillSetConfigDisabled(context.TODO(), name, disable); err != nil {
				return util.NewErrorMsg(err)
			}
			return util.NewInfoMsg(fmt.Sprintf("Skill %q %s globally", name, status))
		}
		if err := m.com.Workspace.SkillSetDisabled(context.TODO(), name, disable); err != nil {
			return util.NewErrorMsg(err)
		}
		return util.NewInfoMsg(fmt.Sprintf("Skill %q %s for this repository", name, status))
	}
}
