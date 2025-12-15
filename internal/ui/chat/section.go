package chat

import (
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/google/uuid"
)

// SectionItem represents a section separator showing model info and response time.
type SectionItem struct {
	id                  string
	msg                 message.Message
	lastUserMessageTime time.Time
	modelName           string
	sty                 *styles.Styles
}

// NewSectionItem creates a new section item showing assistant response metadata.
func NewSectionItem(msg message.Message, lastUserMessageTime time.Time, modelName string, sty *styles.Styles) *SectionItem {
	return &SectionItem{
		id:                  uuid.NewString(),
		msg:                 msg,
		lastUserMessageTime: lastUserMessageTime,
		modelName:           modelName,
		sty:                 sty,
	}
}

// ID implements Identifiable.
func (m *SectionItem) ID() string {
	return m.id
}

// Render implements list.Item.
func (m *SectionItem) Render(width int) string {
	finishData := m.msg.FinishPart()
	if finishData == nil {
		return ""
	}

	finishTime := time.Unix(finishData.Time, 0)
	duration := finishTime.Sub(m.lastUserMessageTime)

	icon := m.sty.Chat.Message.SectionIcon.Render(styles.ModelIcon)
	modelFormatted := m.sty.Chat.Message.SectionModel.Render(m.modelName)
	durationFormatted := m.sty.Chat.Message.SectionDuration.Render(duration.String())

	text := fmt.Sprintf("%s %s %s", icon, modelFormatted, durationFormatted)

	section := common.Section(m.sty, text, width-2)
	return m.sty.Chat.Message.SectionHeader.Render(section)
}
