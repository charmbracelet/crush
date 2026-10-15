package dialog

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
)

// jobsConfigWorkspace chỉ cấp cấu hình mà bảng lệnh đọc khi dựng danh sách.
type jobsConfigWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w jobsConfigWorkspace) Config() *config.Config { return w.cfg }

// Bảng lệnh có mục "Jobs" ghi đúng phím alt+j, mở đúng hộp thoại, và tìm được
// bằng các tên gọi khác của nó (Activity trong roadmap).
func TestCommandsListJobs(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}}}
	c := &Commands{
		com:        &common.Common{Styles: &s, Workspace: jobsConfigWorkspace{cfg: cfg}},
		hasSession: true,
	}

	var jobs *CommandItem
	for _, item := range c.defaultCommands() {
		if item.ID() == "jobs" {
			jobs = item
		}
	}
	require.NotNil(t, jobs, "the palette has a Jobs entry")
	require.Equal(t, "Jobs", jobs.title)
	require.Equal(t, "alt+j", jobs.Shortcut(), "the hint matches the key binding")
	require.Equal(t, ActionOpenDialog{JobsID}, jobs.Action())
	for _, name := range []string{"Jobs", "activity", "background", "processes"} {
		require.Contains(t, jobs.Filter(), name)
	}
}
