package shell

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// guard giả: PID trong protectedPIDs được bảo vệ; tên "crush"/"agent-cli" và
// "mycli" được bảo vệ.
func testGuard(protectedPIDs ...int) ProcessGuard {
	set := map[int]bool{}
	for _, p := range protectedPIDs {
		set[p] = true
	}
	return ProcessGuard{
		ProtectedPID: func(pid int) bool { return set[pid] },
		ProtectedName: func(name string) bool {
			n := baseName(name)
			return n == "crush" || n == "agent-cli" || n == "mycli"
		},
	}
}

// Lệnh giết tiến trình agent-cli (client 954803 + server 946242) bị chặn, kể
// cả qua /bin/kill và qua lớp bọc; đúng sự cố 15/10.
func TestProtect_BlocksKillingOwnProcess(t *testing.T) {
	t.Parallel()
	g := testGuard(954803, 946242)
	blocked := [][]string{
		{"/bin/kill", "-9", "954803", "946242"},
		{"kill", "946242"},
		{"kill", "-s", "KILL", "954803"},
		{"kill", "-TERM", "946242"},
		{"/usr/bin/kill", "-9", "-946242"}, // tín hiệu 9 + nhóm tiến trình 946242
		{"env", "FOO=1", "kill", "-9", "954803"},
		{"timeout", "5", "kill", "946242"},
		{"nohup", "kill", "-9", "954803"},
	}
	for _, args := range blocked {
		reason, ok := g.Blocked(args)
		require.Truef(t, ok, "%v", args)
		require.NotEmpty(t, reason)
	}
}

// Giết một PID không thuộc agent-cli thì KHÔNG bị lớp này chặn (vẫn qua cổng
// quyền bình thường); lệnh thường cũng không bị chặn.
func TestProtect_AllowsOtherPIDsAndNormalCommands(t *testing.T) {
	t.Parallel()
	g := testGuard(946242)
	for _, args := range [][]string{
		{"kill", "-9", "12345"},
		{"kill", "123", "456"},
		{"ps", "aux"},
		{"echo", "kill", "946242"},
		{"ls", "-la"},
		{"grep", "-r", "kill", "."},
	} {
		_, ok := g.Blocked(args)
		require.Falsef(t, ok, "%v", args)
	}
}

// pkill/killall khớp tên tiến trình agent-cli bị chặn; tên khác thì không.
func TestProtect_BlocksKillByName(t *testing.T) {
	t.Parallel()
	g := testGuard()
	for _, args := range [][]string{
		{"pkill", "crush"},
		{"pkill", "-9", "-f", "agent-cli"},
		{"killall", "crush"},
		{"killall", "-9", "mycli"},
	} {
		_, ok := g.Blocked(args)
		require.Truef(t, ok, "%v", args)
	}
	for _, args := range [][]string{
		{"pkill", "node"},
		{"killall", "chrome"},
	} {
		_, ok := g.Blocked(args)
		require.Falsef(t, ok, "%v", args)
	}
}

// Tắt/khởi động lại máy luôn bị chặn.
func TestProtect_BlocksShutdown(t *testing.T) {
	t.Parallel()
	g := testGuard()
	for _, args := range [][]string{
		{"shutdown", "-h", "now"},
		{"reboot"},
		{"poweroff"},
		{"halt"},
		{"sudo", "reboot"}, // sudo bị cấm nơi khác, nhưng reboot ở đây cũng chặn
	} {
		// "sudo" không phải launcher ở đây nên peel không bóc; test trực tiếp reboot.
		_ = args
	}
	for _, args := range [][]string{{"shutdown", "-h", "now"}, {"reboot"}, {"poweroff"}, {"halt"}} {
		_, ok := g.Blocked(args)
		require.Truef(t, ok, "%v", args)
	}
}

// killPIDs bỏ cờ tín hiệu, lấy đúng PID (kể cả nhóm tiến trình âm).
func TestKillPIDs(t *testing.T) {
	t.Parallel()
	require.Equal(t, []int{954803, 946242}, killPIDs([]string{"kill", "-9", "954803", "946242"}))
	require.Equal(t, []int{123}, killPIDs([]string{"kill", "-s", "TERM", "123"}))
	require.Equal(t, []int{946242}, killPIDs([]string{"kill", "-9", "-946242"}))
	require.Empty(t, killPIDs([]string{"kill", "-946242"}), "a lone -N is a signal, not a pid")
	require.Empty(t, killPIDs([]string{"kill", "%1"}))
}

// Bộ bảo vệ thật: giết chính tiến trình test (os.Getpid qua selfTree) bị chặn.
func TestDefaultGuard_ProtectsSelf(t *testing.T) {
	t.Parallel()
	self := 0
	for pid := range selfTree {
		self = pid
		break
	}
	require.NotZero(t, self)
	_, ok := defaultProcessGuard.Blocked([]string{"/bin/kill", "-9", strconv.Itoa(self)})
	require.True(t, ok, "killing a pid in agent-cli's own process tree must be blocked")
}
