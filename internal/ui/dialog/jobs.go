package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// JobsID is the identifier for the background jobs dialog.
const JobsID = "jobs"

type jobsMode uint8

const (
	jobsModeNormal jobsMode = iota
	jobsModeKilling
)

// Jobs is a background jobs list dialog.
type Jobs struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	mode jobsMode

	jobs []proto.BackgroundJob

	keyMap struct {
		Kill        key.Binding
		ConfirmKill key.Binding
		CancelKill  key.Binding
		Next        key.Binding
		Previous    key.Binding
		UpDown      key.Binding
		Select      key.Binding
		Close       key.Binding
	}
}

var _ Dialog = (*Jobs)(nil)

// NewJobs creates a new Jobs dialog over an already-fetched job list.
//
// The list is passed in rather than read here: the background shell
// registry lives in the agent process, so under CRUSH_CLIENT_SERVER=1 it is
// only reachable over HTTP, and the dialog is constructed from the Update
// goroutine. The caller fetches off-thread (see UI.openJobsDialog) and this
// constructor stays pure.
func NewJobs(com *common.Common, jobs []proto.BackgroundJob) (*Jobs, error) {
	j := &Jobs{
		com:  com,
		jobs: jobs,
		mode: jobsModeNormal,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	j.help = help

	j.list = list.NewFilterableList()
	j.list.Focus()
	j.list.SetSelected(0)

	j.input = textinput.New()
	j.input.SetVirtualCursor(false)
	j.input.Placeholder = "Type to filter"
	j.input.SetStyles(com.Styles.TextInput)
	j.input.Focus()

	j.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "kill"),
	)
	j.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	j.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	j.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑↓", "choose"),
	)
	j.keyMap.Kill = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "kill"),
	)
	j.keyMap.ConfirmKill = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "kill"),
	)
	j.keyMap.CancelKill = key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "cancel"),
	)
	j.keyMap.Close = CloseKey

	j.refresh()
	return j, nil
}

// ID implements Dialog.
func (j *Jobs) ID() string {
	return JobsID
}

// HandleMsg implements Dialog.
func (j *Jobs) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch j.mode {
		case jobsModeKilling:
			switch {
			case key.Matches(msg, j.keyMap.ConfirmKill):
				action := j.confirmKill()
				j.mode = jobsModeNormal
				j.refresh()
				return action
			case key.Matches(msg, j.keyMap.CancelKill):
				j.mode = jobsModeNormal
				j.refresh()
			}
		default:
			switch {
			case key.Matches(msg, j.keyMap.Close):
				return ActionClose{}
			case key.Matches(msg, j.keyMap.Kill):
				if _, ok := j.selectedJob(); !ok {
					return ActionCmd{Cmd: util.ReportWarn("No job selected")}
				}
				j.mode = jobsModeKilling
				j.refresh()
			case key.Matches(msg, j.keyMap.Previous):
				j.list.Focus()
				if j.list.IsSelectedFirst() {
					j.list.SelectLast()
				} else {
					j.list.SelectPrev()
				}
				j.list.ScrollToSelected()
			case key.Matches(msg, j.keyMap.Next):
				j.list.Focus()
				if j.list.IsSelectedLast() {
					j.list.SelectFirst()
				} else {
					j.list.SelectNext()
				}
				j.list.ScrollToSelected()
			case key.Matches(msg, j.keyMap.Select):
				// Killing a job is destructive and irreversible, so enter
				// asks first exactly like ctrl+x rather than terminating
				// the highlighted job outright.
				if _, ok := j.selectedJob(); !ok {
					return ActionCmd{Cmd: util.ReportWarn("No job selected")}
				}
				j.mode = jobsModeKilling
				j.refresh()
			default:
				var cmd tea.Cmd
				j.input, cmd = j.input.Update(msg)
				j.list.SetFilter(j.input.Value())
				j.list.ScrollToTop()
				j.list.SetSelected(0)
				return ActionCmd{Cmd: cmd}
			}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (j *Jobs) Cursor() *tea.Cursor {
	return InputCursor(j.com.Styles, j.input.Cursor())
}

// Draw implements Dialog.
func (j *Jobs) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := j.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	j.input.SetWidth(dialogInputTextWidth(t, j.input, innerWidth))
	listHeight, listTotalHeight, listWidth := sizeDialogList(t, j.list, innerWidth, height)

	// Hide the info column uniformly when the widest would crowd the title.
	applyInfoColumnVisibility(j.list.FilteredItems(), listWidth, sessionInfoMaxPercent)

	// Scroll to selected if outside visible range.
	start, end := j.list.VisibleItemIndices()
	if idx := j.list.Selected(); idx < start || idx > end {
		j.list.ScrollToSelected()
	}

	var cur *tea.Cursor
	rc := NewRenderContext(t, width)
	rc.Title = "Background Jobs"

	switch j.mode {
	case jobsModeKilling:
		rc.TitleStyle = t.Dialog.Sessions.DeletingTitle
		rc.TitleGradientFromColor = t.Dialog.Sessions.DeletingTitleGradientFromColor
		rc.TitleGradientToColor = t.Dialog.Sessions.DeletingTitleGradientToColor
		rc.ViewStyle = t.Dialog.Sessions.DeletingView
		rc.AddPart(t.Dialog.Sessions.DeletingMessage.Render("Kill this job?"))
	default:
		inputView := t.Dialog.InputPrompt.Render(j.input.View())
		cur = j.Cursor()
		rc.AddPart(inputView)
	}

	listView := t.Dialog.List.Height(j.list.Height()).Render(j.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, j.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &j.help, j, innerWidth)

	view := rc.Render()

	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// refresh rebuilds the list items from the jobs already held by the dialog.
//
// It performs no IO. It is called from HandleMsg — i.e. on the Update
// goroutine — whenever the mode changes, because the item styling depends on
// the mode; re-fetching the job list there would put a synchronous HTTP
// round-trip on a keypress in client/server mode.
func (j *Jobs) refresh() {
	// Remember the highlighted job by ID, not by index: the rebuild
	// re-applies the filter, which changes how many rows precede it, and
	// confirming a kill must terminate the job the user highlighted.
	var selectedID string
	if job, ok := j.selectedJob(); ok {
		selectedID = job.ID
	}

	items := make([]list.FilterableItem, 0, len(j.jobs))
	for _, job := range j.jobs {
		items = append(items, NewJobItem(j.com.Styles, job, j.mode))
	}
	j.list.SetItems(items...)
	// FilterableList.SetItems installs the *unfiltered* set, so without
	// this the rebuild triggered by arming a kill would silently widen a
	// filtered list back to every job.
	j.list.SetFilter(j.input.Value())
	j.selectJob(selectedID)
}

// selectJob highlights the row for id, or the first row when id is empty or
// no longer listed. The list's own SetSelected(0) in the constructor runs
// before any item exists and therefore leaves the selection at -1; without
// this the first ctrl+x after opening /jobs would answer "No job selected"
// with jobs plainly on screen.
func (j *Jobs) selectJob(id string) {
	items := j.list.FilteredItems()
	if len(items) == 0 {
		j.list.SetSelected(-1)
		return
	}
	if id != "" {
		for i, item := range items {
			if ji, ok := item.(*JobItem); ok && ji.job.ID == id {
				j.list.SetSelected(i)
				return
			}
		}
	}
	j.list.SetSelected(0)
}

// Jobs returns the job list the dialog is rendering, in list order. The
// dialog is a pure view over what the workspace returned, so this is what
// the user is looking at.
func (j *Jobs) Jobs() []proto.BackgroundJob {
	return j.jobs
}

// selectedJob returns the highlighted job, or ok=false when the list is
// empty or the selection is not a job item.
func (j *Jobs) selectedJob() (proto.BackgroundJob, bool) {
	if item := j.list.SelectedItem(); item != nil {
		if ji, ok := item.(*JobItem); ok {
			return ji.job, true
		}
	}
	return proto.BackgroundJob{}, false
}

func (j *Jobs) confirmKill() Action {
	job, ok := j.selectedJob()
	if !ok {
		return ActionCmd{Cmd: util.ReportWarn("No job selected")}
	}
	return ActionKillJob{ShellID: job.ID}
}

// ShortHelp implements [help.KeyMap].
func (j *Jobs) ShortHelp() []key.Binding {
	switch j.mode {
	case jobsModeKilling:
		return []key.Binding{
			j.keyMap.ConfirmKill,
			j.keyMap.CancelKill,
		}
	default:
		return []key.Binding{
			j.keyMap.UpDown,
			j.keyMap.Kill,
			j.keyMap.Select,
			j.keyMap.Close,
		}
	}
}

// FullHelp implements [help.KeyMap].
func (j *Jobs) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		j.keyMap.UpDown,
		j.keyMap.Kill,
		j.keyMap.Select,
		j.keyMap.Close,
	}

	switch j.mode {
	case jobsModeKilling:
		slice = []key.Binding{
			j.keyMap.ConfirmKill,
			j.keyMap.CancelKill,
		}
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// InitialCmd returns the initial command to focus the input.
func (j *Jobs) InitialCmd() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, j.input.Focus())
	return tea.Batch(cmds...)
}
