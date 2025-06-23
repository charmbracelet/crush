package logs

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/v2/table"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

type TableComponent interface {
	util.Model
	layout.Sizeable
}

type tableCmp struct {
	table table.Model
	logs  []logging.LogMessage
}

type selectedLogMsg logging.LogMessage

func (i *tableCmp) Init() tea.Cmd {
	i.logs = logging.List()
	i.setRows()
	return nil
}

func (i *tableCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case pubsub.Event[logging.LogMessage]:
		return i, func() tea.Msg {
			if msg.Type == pubsub.CreatedEvent {
				rows := i.table.Rows()
				for _, row := range rows {
					if row[1] == msg.Payload.ID {
						return nil // If the log already exists, do not add it again
					}
				}
				i.logs = append(i.logs, msg.Payload)
				i.table.SetRows(
					append(
						[]table.Row{
							logToRow(msg.Payload),
						},
						i.table.Rows()...,
					),
				)
			}
			return selectedLogMsg(msg.Payload)
		}
	}
	t, cmd := i.table.Update(msg)
	cmds = append(cmds, cmd)
	i.table = t

	cmds = append(cmds, func() tea.Msg {
		for _, log := range logging.List() {
			if log.ID == i.table.SelectedRow()[1] {
				// If the selected row matches the log ID, return the selected log message
				return selectedLogMsg(log)
			}
		}
		return nil
	})
	return i, tea.Batch(cmds...)
}

func (i *tableCmp) View() string {
	t := styles.CurrentTheme()
	defaultStyles := table.DefaultStyles()

	// Header styling
	defaultStyles.Header = defaultStyles.Header.
		Foreground(t.Primary).
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(t.Border)

	// Selected row styling
	defaultStyles.Selected = defaultStyles.Selected.
		Foreground(t.FgSelected).
		Background(t.Primary).
		Bold(false)

	// Cell styling
	defaultStyles.Cell = defaultStyles.Cell.
		Foreground(t.FgBase)

	i.table.SetStyles(defaultStyles)
	return i.table.View()
}

func (i *tableCmp) GetSize() (int, int) {
	return i.table.Width(), i.table.Height()
}

func (i *tableCmp) SetSize(width int, height int) tea.Cmd {
	i.table.SetWidth(width)
	i.table.SetHeight(height)

	columnWidth := (width - 10) / 4
	i.table.SetColumns([]table.Column{
		{
			Title: "Level",
			Width: 10,
		},
		{
			Title: "ID",
			Width: columnWidth,
		},
		{
			Title: "Time",
			Width: columnWidth,
		},
		{
			Title: "Message",
			Width: columnWidth,
		},
		{
			Title: "Attributes",
			Width: columnWidth,
		},
	})
	return nil
}

func (i *tableCmp) setRows() {
	rows := []table.Row{}

	slices.SortFunc(i.logs, func(a, b logging.LogMessage) int {
		if a.Time.Before(b.Time) {
			return -1
		}
		if a.Time.After(b.Time) {
			return 1
		}
		return 0
	})

	for _, log := range i.logs {
		rows = append(rows, logToRow(log))
	}
	i.table.SetRows(rows)
}

func logToRow(log logging.LogMessage) table.Row {
	// Format attributes as JSON string
	var attrStr string
	if len(log.Attributes) > 0 {
		var parts []string
		for _, attr := range log.Attributes {
			parts = append(parts, fmt.Sprintf(`{"Key":"%s","Value":"%s"}`, attr.Key, attr.Value))
		}
		attrStr = "[" + strings.Join(parts, ",") + "]"
	}

	// Format time with relative time
	timeStr := log.Time.Format("2006-01-05 15:04:05 UTC")
	relativeTime := getRelativeTime(log.Time)
	fullTimeStr := timeStr + " " + relativeTime

	return table.Row{
		strings.ToUpper(log.Level),
		log.ID,
		fullTimeStr,
		log.Message,
		attrStr,
	}
}

func NewLogsTable() TableComponent {
	columns := []table.Column{
		{Title: "Level"},
		{Title: "ID"},
		{Title: "Time"},
		{Title: "Message"},
		{Title: "Attributes"},
	}

	tableModel := table.New(
		table.WithColumns(columns),
	)
	tableModel.Focus()
	return &tableCmp{
		table: tableModel,
	}
}
