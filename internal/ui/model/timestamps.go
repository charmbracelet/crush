package model

import "github.com/charmbracelet/crush/internal/ui/chat"

// refreshAssistantInfoConfig updates existing footers after a timestamp
// setting change, preserving scrolled-back viewports and follow mode.
func (m *UI) refreshAssistantInfoConfig() {
	if m.com == nil || m.com.Workspace == nil || m.chat == nil {
		return
	}
	cfg := m.com.Config()
	if cfg == nil {
		return
	}
	format := cfg.TimestampFormat()
	if m.timestampFormat.Layout() == format.Layout() {
		return
	}
	m.timestampFormat = format
	for i := range m.chat.list.Len() {
		if item, ok := m.chat.list.ItemAt(i).(*chat.AssistantInfoItem); ok {
			item.SetConfig(cfg)
		}
	}
	if m.chat.Follow() {
		m.chat.ScrollToBottom()
	}
	m.invalidateFrames()
}
