package acp

import (
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	acp "github.com/coder/acp-go-sdk"
)

// Crush's TUI presents four modes on a single Shift+Tab ring — code, plan,
// YOLO, sysadmin — but they are not four agents. Only two agent profiles exist
// (coder and plan); YOLO and sysadmin are the coder with a wider permission
// mode. ACP's `modes` is one flat enum, so this file flattens the ring into it
// and remembers that each entry is really an (agent, permission mode) pair.
//
// Wire ids are the TUI's own words, lower-cased, with "code" standing in for
// the coder agent. "coder" is still accepted as an alias so a client that
// learned the id from an older advertisement keeps working.

// acpMode is one entry of the flattened mode ring.
type acpMode struct {
	// id is the wire value used in `availableModes`, `set_mode` and the mode
	// config option.
	id acp.SessionModeId
	// name is the human-readable label.
	name string
	// description explains what selecting the mode does.
	description string
	// agent is the Crush agent the mode runs. Switched through
	// Coordinator.SetMainAgent.
	agent string
	// perm is the permission mode applied alongside the agent. YOLO and
	// sysadmin are the coder with a wider permission mode, not separate
	// agents.
	perm permission.PermissionMode
}

// acpModes is the fixed mode ring, in TUI order. The set is deliberately
// closed: it is what config.SetupAgents creates, so advertising anything else
// would offer modes that SetMainAgent rejects.
var acpModes = []acpMode{
	{
		id:          "code",
		name:        "Code",
		description: "Write and modify code with full tool access.",
		agent:       config.AgentCoder,
		perm:        permission.PermissionModeNormal,
	},
	{
		id:          "plan",
		name:        "Plan",
		description: "Analyse and prepare an implementation plan without modifying files.",
		agent:       config.AgentPlan,
		perm:        permission.PermissionModeNormal,
	},
	{
		id:          "yolo",
		name:        "YOLO",
		description: "Code mode, prompting only for dangerous commands.",
		agent:       config.AgentCoder,
		perm:        permission.PermissionModeYolo,
	},
	{
		id:          "sysadmin",
		name:        "Sysadmin",
		description: "Code mode with every command unblocked.",
		agent:       config.AgentCoder,
		perm:        permission.PermissionModeSysadmin,
	},
}

// legacyModeAliases maps wire ids from earlier advertisements onto the current
// ring, so a client that saw the raw agent name keeps working.
var legacyModeAliases = map[acp.SessionModeId]acp.SessionModeId{
	"coder": "code",
}

// resolveModeId normalises a requested mode id and reports whether the ring
// contains it.
func resolveModeId(id acp.SessionModeId) (acpMode, bool) {
	if alias, ok := legacyModeAliases[id]; ok {
		id = alias
	}
	for _, m := range acpModes {
		if m.id == id {
			return m, true
		}
	}
	return acpMode{}, false
}

// modeForState reports the ring entry matching an agent/permission pair. More
// than one entry can share an agent (code, yolo, sysadmin are all the coder),
// so the permission mode breaks the tie. An agent outside the ring, or an
// agent/permission combination the ring does not express (plan + yolo is
// reachable in the TUI but has no ACP id), degrades to the closest entry
// rather than reporting a mode that is not in availableModes
// (session-modes.md:58-64).
func modeForState(agent string, perm permission.PermissionMode) acpMode {
	for _, m := range acpModes {
		if m.agent == agent && m.perm == perm {
			return m
		}
	}
	// Same agent, different permission: fall back to that agent's plain entry.
	for _, m := range acpModes {
		if m.agent == agent && m.perm == permission.PermissionModeNormal {
			return m
		}
	}
	return acpModes[0]
}

// availableModes renders the ring for `availableModes`.
func availableModes() []acp.SessionMode {
	modes := make([]acp.SessionMode, 0, len(acpModes))
	for _, m := range acpModes {
		desc := m.description
		modes = append(modes, acp.SessionMode{
			Id:          m.id,
			Name:        m.name,
			Description: &desc,
		})
	}
	return modes
}

// modeSelectOptions renders the ring for the mode config option.
func modeSelectOptions() []acp.SessionConfigSelectOption {
	opts := make([]acp.SessionConfigSelectOption, 0, len(acpModes))
	for _, m := range acpModes {
		desc := m.description
		opts = append(opts, acp.SessionConfigSelectOption{
			Value:       acp.SessionConfigValueId(m.id),
			Name:        m.name,
			Description: &desc,
		})
	}
	return opts
}
