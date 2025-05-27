package logs

import (
	"encoding/json"
	"slices"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/table"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type TableComponent interface {
	util.Model
	layout.Sizeable
	layout.Bindings
}

type tableCmp struct {
	table table.Model
}

type selectedLogMsg logging.LogMessage

func (i *tableCmp) Init() tea.Cmd {
	i.setRows()
	return nil
}

func (i *tableCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg.(type) {
	case pubsub.Event[logging.LogMessage]:
		i.setRows()
		return i, nil
	}
	prevSelectedRow := i.table.SelectedRow()
	t, cmd := i.table.Update(msg)
	cmds = append(cmds, cmd)
	i.table = t
	selectedRow := i.table.SelectedRow()
	if selectedRow != nil {
		if prevSelectedRow == nil || selectedRow[0] == prevSelectedRow[0] {
			var log logging.LogMessage
			for _, row := range logging.List() {
				if row.ID == selectedRow[0] {
					log = row
					break
				}
			}
			if log.ID != "" {
				cmds = append(cmds, util.CmdHandler(selectedLogMsg(log)))
			}
		}
	}
	return i, tea.Batch(cmds...)
}

func (i *tableCmp) View() tea.View {
	t := theme.CurrentTheme()
	defaultStyles := table.DefaultStyles()
	defaultStyles.Selected = defaultStyles.Selected.Foreground(t.Primary())
	i.table.SetStyles(defaultStyles)
	return tea.NewView(i.table.View())
}

func (i *tableCmp) GetSize() (int, int) {
	return i.table.Width(), i.table.Height()
}

func (i *tableCmp) SetSize(width int, height int) tea.Cmd {
	i.table.SetWidth(width)
	i.table.SetHeight(height)
	cloumns := i.table.Columns()
	for i, col := range cloumns {
		col.Width = (width / len(cloumns)) - 2
		cloumns[i] = col
	}
	i.table.SetColumns(cloumns)
	return nil
}

func (i *tableCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(i.table.KeyMap)
}

func (i *tableCmp) setRows() {
	rows := []table.Row{}

	logs := logging.List()
	slices.SortFunc(logs, func(a, b logging.LogMessage) int {
		if a.Time.Before(b.Time) {
			return 1
		}
		if a.Time.After(b.Time) {
			return -1
		}
		return 0
	})

	for _, log := range logs {
		bm, _ := json.Marshal(log.Attributes)

		row := table.Row{
			log.ID,
			log.Time.Format("15:04:05"),
			log.Level,
			log.Message,
			string(bm),
		}
		rows = append(rows, row)
	}
	i.table.SetRows(rows)
}

func NewLogsTable() TableComponent {
	columns := []table.Column{
		{Title: "ID", Width: 4},
		{Title: "Time", Width: 4},
		{Title: "Level", Width: 10},
		{Title: "Message", Width: 10},
		{Title: "Attributes", Width: 10},
	}

	tableModel := table.New(
		table.WithColumns(columns),
	)
	tableModel.Focus()
	return &tableCmp{
		table: tableModel,
	}
}
