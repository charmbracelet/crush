package model

import tea "charm.land/bubbletea/v2"

// drainCmd runs a command and flattens any batch into individual messages,
// recursing into nested batches. It does not recurse into sequences.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, drainCmd(c)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}
