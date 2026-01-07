package attachments

import (
	"fmt"
	"path/filepath"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/x/ansi"
)

const maxFilename = 15

type Keymap struct {
	DeleteMode,
	DeleteAll,
	Escape key.Binding
}

func New(normalStyle, deletingStyle, imageStyle, textStyle lipgloss.Style, keyMap Keymap) *Attachments {
	return &Attachments{
		keyMap:        keyMap,
		normalStyle:   normalStyle,
		textStyle:     textStyle,
		imageStyle:    imageStyle,
		deletingStyle: deletingStyle,
	}
}

type Attachments struct {
	normalStyle, textStyle, imageStyle, deletingStyle lipgloss.Style
	keyMap                                            Keymap
	list                                              []message.Attachment
	deleting                                          bool
}

func (m *Attachments) List() []message.Attachment { return m.list }
func (m *Attachments) Reset()                     { m.list = nil }

func (m *Attachments) Update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case message.Attachment:
		m.list = append(m.list, msg)
		return true
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.DeleteMode):
			if len(m.list) > 0 {
				m.deleting = true
			}
			return true
		case m.deleting && key.Matches(msg, m.keyMap.Escape):
			m.deleting = false
			return true
		case m.deleting && key.Matches(msg, m.keyMap.DeleteAll):
			m.deleting = false
			m.list = nil
			return true
		case m.deleting:
			// Handle digit keys for individual attachment deletion.
			r := msg.Code
			if r >= '0' && r <= '9' {
				num := int(r - '0')
				if num < len(m.list) {
					m.list = slices.Delete(m.list, num, num+1)
				}
				m.deleting = false
			}
			return true
		}
	}
	return false
}

func (m *Attachments) Render() string {
	var chips []string

	for i, att := range m.list {
		filename := filepath.Base(att.FileName)
		// Truncate if needed.
		if ansi.StringWidth(filename) > maxFilename {
			filename = ansi.Truncate(filename, maxFilename, "…")
		}

		if m.deleting {
			chips = append(
				chips,
				m.deletingStyle.Render(fmt.Sprintf("%d", i)),
				m.normalStyle.Render(filename),
			)
		} else {
			chips = append(
				chips,
				m.icon(att).String(),
				m.normalStyle.Render(filename),
			)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, chips...)
}

func (m *Attachments) icon(a message.Attachment) lipgloss.Style {
	if a.IsImage() {
		return m.imageStyle
	}
	return m.textStyle
}
