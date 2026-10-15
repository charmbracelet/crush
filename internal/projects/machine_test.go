package projects

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/lock"
	"github.com/stretchr/testify/require"
)

// Không parallel: các test dưới đổi biến cấp gói và env.

func useMachineID(t *testing.T, id string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "machine-id")
	require.NoError(t, os.WriteFile(p, []byte(id+"\n"), 0o644))
	prev := machineIDPaths
	machineIDPaths = []string{p}
	t.Cleanup(func() { machineIDPaths = prev })
}

// Đổi hostname (DHCP, "localhost" khi chưa có mạng) không làm Home mất các
// dự án của máy này: machine-id quyết định khi cả hai phía có.
func TestOnThisMachineSurvivesHostnameChange(t *testing.T) {
	useMachineID(t, "machine-a")
	require.True(t, Project{Host: "old-hostname", MachineID: "machine-a"}.OnThisMachine())
	require.False(t, Project{Host: currentHost(), MachineID: "machine-b"}.OnThisMachine(), "another machine with the same hostname")
	// Mục cũ chưa có machine-id: vẫn so hostname như trước.
	require.True(t, Project{Host: currentHost()}.OnThisMachine())
	require.False(t, Project{Host: "other-host"}.OnThisMachine())
}

// Đăng ký lại cùng dự án sau khi hostname đổi cập nhật mục cũ (nhờ machine-id),
// không đẻ mục trùng.
func TestRegisterAfterHostnameChangeReusesEntry(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmp, "crush"))
	useMachineID(t, "machine-a")

	require.NoError(t, Register("/home/user/p1", "/home/user/p1/.crush"))
	list, err := Load()
	require.NoError(t, err)
	require.Len(t, list.Projects, 1)
	require.Equal(t, "machine-a", list.Projects[0].MachineID)

	// Giả lập hostname cũ khác trong tệp.
	list.Projects[0].Host = "old-hostname"
	require.NoError(t, Save(list))
	require.NoError(t, Register("/home/user/p1", "/home/user/p1/.crush"))
	list, err = Load()
	require.NoError(t, err)
	require.Len(t, list.Projects, 1, "the old entry is refreshed, not duplicated")
	require.Equal(t, currentHost(), list.Projects[0].Host)
}

// Khoá registry bị giữ (một agent-cli bị Ctrl+Z đúng lúc ghi): Register báo
// lỗi sau lockTimeout thay vì treo mọi lần mở sau.
func TestRegisterGivesUpOnHeldLock(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmp, "crush"))
	prev := lockTimeout
	lockTimeout = 200 * time.Millisecond
	t.Cleanup(func() { lockTimeout = prev })

	dir := filepath.Dir(projectsFilePath())
	require.NoError(t, os.MkdirAll(dir, 0o700))
	release, err := lock.File(context.Background(), filepath.Join(dir, lockFileName))
	require.NoError(t, err)
	defer release()

	start := time.Now()
	err = Register("/home/user/p1", "/home/user/p1/.crush")
	require.ErrorContains(t, err, "locked by another agent-cli process")
	require.Less(t, time.Since(start), 3*time.Second)
}
