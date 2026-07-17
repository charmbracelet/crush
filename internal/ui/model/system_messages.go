package model

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/chat"
)

// modelChangedMsg is emitted once the agent model finishes updating after a
// selection. It carries the info-toast text and triggers a system-message
// recompute so context-window advisories reflect the new model.
type modelChangedMsg struct {
	info string
}

// computeSystemMessages derives the ephemeral Crush advisories that should be
// shown for the current state. It returns nil when warnings are disabled or
// no condition applies. The result is a pure function of the selected model
// and permission mode, minus any the user dismissed by sending a message, so
// the messages self-clear when that state resolves.
func (m *UI) computeSystemMessages() []chat.MessageItem {
	var items []chat.MessageItem
	for _, kind := range m.activeSystemMessageKinds() {
		if m.suppressedSystemMessages[kind] {
			continue
		}
		switch kind {
		case chat.SystemMessageContextWarning:
			if model := m.selectedLargeModel(); model != nil {
				items = append(items, m.newContextWarning(model.CatwalkCfg.Name, model.CatwalkCfg.ContextWindow))
			}
		case chat.SystemMessageSuperYolo:
			items = append(items, m.newSuperYoloWarning())
		}
	}
	return items
}

// activeSystemMessageKinds returns the advisory kinds whose triggering
// condition currently holds, ignoring dismissal. Returns nil when warnings
// are disabled in config.
func (m *UI) activeSystemMessageKinds() []chat.SystemMessageKind {
	if cfg := m.com.Config(); cfg != nil && cfg.Options != nil && cfg.Options.DisableSystemWarnings {
		return nil
	}

	var kinds []chat.SystemMessageKind

	// Small-context-model warning.
	if model := m.selectedLargeModel(); model != nil {
		cw := model.CatwalkCfg.ContextWindow
		if cw > 0 && cw < config.MinRecommendedContextWindow {
			kinds = append(kinds, chat.SystemMessageContextWarning)
		}
	}

	// Super yolo (sysadmin) mode warning.
	if m.com.Workspace.PermissionMode() == permission.PermissionModeSysadmin {
		kinds = append(kinds, chat.SystemMessageSuperYolo)
	}

	return kinds
}

// refreshSystemMessages recomputes the active advisories and pushes them into
// the chat after the transcript.
func (m *UI) refreshSystemMessages() {
	m.chat.SetSystemMessages(m.computeSystemMessages()...)
}

// retriggerSystemMessage clears the dismissal for a single kind and refreshes,
// so its triggering event (model switch, mode toggle) re-surfaces it.
func (m *UI) retriggerSystemMessage(kind chat.SystemMessageKind) {
	delete(m.suppressedSystemMessages, kind)
	m.refreshSystemMessages()
}

// dismissActiveSystemMessages marks every currently-active advisory as
// dismissed, then refreshes so they clear from the chat. Called when the user
// sends a message: continuing counts as acknowledging the advisory.
func (m *UI) dismissActiveSystemMessages() {
	kinds := m.activeSystemMessageKinds()
	if len(kinds) == 0 {
		return
	}
	if m.suppressedSystemMessages == nil {
		m.suppressedSystemMessages = make(map[chat.SystemMessageKind]bool)
	}
	for _, kind := range kinds {
		m.suppressedSystemMessages[kind] = true
	}
	m.refreshSystemMessages()
}

// resetSystemMessageSuppression clears all dismissals so advisories surface
// afresh. Called when starting or switching sessions.
func (m *UI) resetSystemMessageSuppression() {
	m.suppressedSystemMessages = make(map[chat.SystemMessageKind]bool)
}

// keybindLabel returns the primary key for a binding, pulled from the keymap
// so advisory copy never drifts from the real binding.
func keybindLabel(b key.Binding) string {
	if keys := b.Keys(); len(keys) > 0 {
		return keys[0]
	}
	return b.Help().Key
}

// systemMessagesView renders the active advisories as a standalone block for
// the landing view, where there is no chat transcript to anchor to. It
// returns an empty string when there are none.
func (m *UI) systemMessagesView(width int) string {
	items := m.computeSystemMessages()
	if len(items) == 0 {
		return ""
	}
	rendered := make([]string, len(items))
	for i, item := range items {
		rendered[i] = item.RawRender(width)
	}
	return strings.Join(rendered, "\n\n")
}

// newContextWarning builds the small-context-model advisory. Every run is
// colored explicitly so the body survives width reflow (see the item's render
// contract).
func (m *UI) newContextWarning(modelName string, contextWindow int64) chat.MessageItem {
	sty := m.com.Styles
	base := sty.Messages.SystemBody.Render
	accent := sty.Messages.SystemAccent.Render
	if modelName == "" {
		modelName = "this model"
	}
	body := base("The model you're using, ") + accent(modelName) +
		base(", has a ") + accent(formatContextWindow(contextWindow)) +
		base(" context window. That's a pretty small window. We recommend a window of at least ") +
		accent(formatContextWindow(config.MinRecommendedContextWindow)) +
		base(" in order for Crush to properly function.") +
		"\n\n" +
		base("Press ") + accent(keybindLabel(m.keyMap.Models)) +
		base(" to change models, or feel free to continue if you understand the limitations!")
	return chat.NewSystemMessageItem(sty, chat.SystemMessageContextWarning, "Model Context Warning", body)
}

// newSuperYoloWarning builds the super yolo (sysadmin) mode advisory. Every run
// is colored explicitly so the body survives width reflow.
func (m *UI) newSuperYoloWarning() chat.MessageItem {
	sty := m.com.Styles
	base := sty.Messages.SystemBody.Render
	accent := sty.Messages.SystemAccent.Render
	body := base("Super yolo mode is on. Crush will run every command automatically, including destructive ones, without asking. Only use this when you fully trust the task at hand.") +
		"\n\n" +
		base("Toggle it off any time from the command palette (") +
		accent(keybindLabel(m.keyMap.Commands)) +
		base(").")
	return chat.NewSystemMessageItem(sty, chat.SystemMessageSuperYolo, "Super Yolo Mode", body)
}

// formatContextWindow renders a context window size in a compact form such as
// "5K" or "200K".
func formatContextWindow(tokens int64) string {
	if tokens >= 1000 {
		if tokens%1000 == 0 {
			return fmt.Sprintf("%dK", tokens/1000)
		}
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}
