package dialog

import (
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
)

// JobItem wraps a BackgroundShell to implement the ListItem interface.
type JobItem struct {
	*list.Versioned
	job      proto.BackgroundJob
	t        *styles.Styles
	mode     jobsMode
	m        fuzzy.Match
	cache    map[int]string
	focused  bool
	hideInfo bool
}

var _ ListItem = (*JobItem)(nil)

// NewJobItem creates a new JobItem.
func NewJobItem(t *styles.Styles, job proto.BackgroundJob, mode jobsMode) *JobItem {
	return &JobItem{
		Versioned: list.NewVersioned(),
		job:       job,
		t:         t,
		mode:      mode,
	}
}

// Finished implements list.Item.
func (j *JobItem) Finished() bool {
	return true
}

// Filter returns the filterable value.
func (j *JobItem) Filter() string {
	return j.job.ID + " " + j.job.Command + " " + j.job.Description
}

// ID returns the unique identifier.
func (j *JobItem) ID() string {
	return j.job.ID
}

// SetFocused implements ListItem.
func (j *JobItem) SetFocused(focused bool) {
	if j.focused == focused {
		return
	}
	j.cache = nil
	j.focused = focused
	if j.Versioned != nil {
		j.Bump()
	}
}

// SetMatch implements ListItem.
func (j *JobItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(j.m, m) {
		return
	}
	j.cache = nil
	j.m = m
	if j.Versioned != nil {
		j.Bump()
	}
}

// InfoText returns the elapsed time for the job.
func (j *JobItem) InfoText() string {
	if j.job.Done {
		return "done"
	}
	elapsed := time.Since(j.job.StartedAt).Truncate(time.Second)
	if elapsed < time.Second {
		return "<1s"
	}
	return fmtDuration(elapsed)
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// SetHideInfo controls whether the elapsed time is shown.
func (j *JobItem) SetHideInfo(v bool) {
	if j.hideInfo == v {
		return
	}
	j.cache = nil
	j.hideInfo = v
	if j.Versioned != nil {
		j.Bump()
	}
}

// Render returns the string representation of the job item.
func (j *JobItem) Render(width int) string {
	info := j.InfoText()
	if j.hideInfo {
		info = ""
	}
	styles := ListItemStyles{
		ItemBlurred:     j.t.Dialog.NormalItem,
		ItemFocused:     j.t.Dialog.SelectedItem,
		InfoTextBlurred: j.t.Dialog.Sessions.InfoBlurred,
		InfoTextFocused: j.t.Dialog.Sessions.InfoFocused,
	}

	switch j.mode {
	case jobsModeKilling:
		styles.ItemBlurred = j.t.Dialog.Sessions.DeletingItemBlurred
		styles.ItemFocused = j.t.Dialog.Sessions.DeletingItemFocused
	}

	var title string
	if j.job.Description != "" {
		title = fmt.Sprintf("%s: %s", j.job.Description, j.job.Command)
	} else {
		title = fmt.Sprintf("%-3s %s", j.job.ID, j.job.Command)
	}

	return renderItem(styles, title, info, j.focused, width, j.cache, &j.m)
}
