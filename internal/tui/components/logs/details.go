package logs

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

type DetailComponent interface {
	util.Model
	layout.Sizeable
}

type detailCmp struct {
	width, height int
	currentLog    logging.LogMessage
	viewport      viewport.Model
}

func (i *detailCmp) Init() tea.Cmd {
	messages := logging.List()
	if len(messages) == 0 {
		return nil
	}
	i.currentLog = messages[0]
	return nil
}

func (i *detailCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case selectedLogMsg:
		if msg.ID != i.currentLog.ID {
			i.currentLog = logging.LogMessage(msg)
			i.updateContent()
		}
	}

	return i, nil
}

func (i *detailCmp) updateContent() {
	var content strings.Builder
	t := styles.CurrentTheme()

	if i.currentLog.ID == "" {
		content.WriteString(t.S().Muted.Render("No log selected"))
		i.viewport.SetContent(content.String())
		return
	}

	// Level badge with background color
	levelStyle := getLevelStyle(i.currentLog.Level)
	levelBadge := levelStyle.Padding(0, 1).Render(strings.ToUpper(i.currentLog.Level))

	// Timestamp with relative time
	timeStr := i.currentLog.Time.Format("2006-01-05 15:04:05 UTC")
	relativeTime := getRelativeTime(i.currentLog.Time)
	timeStyle := t.S().Muted

	// Header line
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		timeStr,
		" ",
		timeStyle.Render(relativeTime),
	)

	content.WriteString(levelBadge)
	content.WriteString("\n\n")
	content.WriteString(header)
	content.WriteString("\n\n")

	// Message section
	messageHeaderStyle := t.S().Base.Foreground(t.Blue).Bold(true)
	content.WriteString(messageHeaderStyle.Render("Message"))
	content.WriteString("\n")
	content.WriteString(i.currentLog.Message)
	content.WriteString("\n\n")

	// Attributes section
	if len(i.currentLog.Attributes) > 0 {
		attrHeaderStyle := t.S().Base.Foreground(t.Blue).Bold(true)
		content.WriteString(attrHeaderStyle.Render("Attributes"))
		content.WriteString("\n")

		for _, attr := range i.currentLog.Attributes {
			keyStyle := t.S().Base.Foreground(t.Accent)
			valueStyle := t.S().Text
			attrLine := fmt.Sprintf("%s: %s",
				keyStyle.Render(attr.Key),
				valueStyle.Render(attr.Value),
			)
			content.WriteString(attrLine)
			content.WriteString("\n")
		}
	}

	i.viewport.SetContent(content.String())
}

func getLevelStyle(level string) lipgloss.Style {
	t := styles.CurrentTheme()
	style := t.S().Base.Bold(true)

	switch strings.ToLower(level) {
	case "info":
		return style.Foreground(t.White).Background(t.Info)
	case "warn", "warning":
		return style.Foreground(t.White).Background(t.Warning)
	case "error", "err":
		return style.Foreground(t.White).Background(t.Error)
	case "debug":
		return style.Foreground(t.White).Background(t.Success)
	case "fatal":
		return style.Foreground(t.White).Background(t.Error)
	default:
		return style.Foreground(t.FgBase)
	}
}

func getRelativeTime(logTime time.Time) string {
	now := time.Now()
	diff := now.Sub(logTime)

	if diff < time.Minute {
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	} else if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	} else if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	} else if diff < 30*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	} else if diff < 365*24*time.Hour {
		return fmt.Sprintf("%dmo ago", int(diff.Hours()/(24*30)))
	} else {
		return fmt.Sprintf("%dy ago", int(diff.Hours()/(24*365)))
	}
}

func (i *detailCmp) View() tea.View {
	t := styles.CurrentTheme()
	style := t.S().Base.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(i.width - 2).   // Adjust width for border
		Height(i.height - 2). // Adjust height for border
		Padding(1)
	return tea.NewView(style.Render(i.viewport.View()))
}

func (i *detailCmp) GetSize() (int, int) {
	return i.width, i.height
}

func (i *detailCmp) SetSize(width int, height int) tea.Cmd {
	i.width = width
	i.height = height
	i.viewport.SetWidth(i.width - 4)
	i.viewport.SetHeight(i.height - 4)
	i.updateContent()
	return nil
}

func NewLogsDetails() DetailComponent {
	return &detailCmp{
		viewport: viewport.New(),
	}
}
