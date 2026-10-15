package shell

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Lớp bảo vệ tiến trình điều khiển: chặn CỨNG trong trình thực thi shell (như
// mọi BlockFunc, chạy trước cả YOLO và quyền đã cấp cho session) các lệnh dừng
// chính tiến trình agent-cli đang giữ phiên, hay tắt/khởi động lại cả máy.
//
// Sự cố 15/10: agent coi client/server agent-cli là "mồ côi" rồi chạy
// `/bin/kill -9 <client> <server>`, giết đúng tiến trình đang chạy việc. Cùng
// UID / cùng tên / bị coi là mồ côi không đủ để được giết; mục tiêu là chính
// cây tiến trình agent-cli thì chặn, không hỏi.

// processLaunchers là các lệnh bọc; lệnh thật nằm sau cờ của chúng. Trình
// thực thi shell đã tách sẵn argv từng lệnh, nên chỉ cần bóc vài lớp phổ biến.
var processLaunchers = map[string]bool{
	"env": true, "nohup": true, "timeout": true, "nice": true,
	"stdbuf": true, "setsid": true, "ionice": true, "command": true,
}

// shutdownCommands tắt/khởi động lại cả máy: box chạy dịch vụ thật nên chặn
// cứng, không bao giờ tự duyệt.
var shutdownCommands = map[string]bool{
	"shutdown": true, "reboot": true, "halt": true, "poweroff": true, "telinit": true,
}

// killByPID là các lệnh dừng tiến trình theo PID.
var killByPID = map[string]bool{"kill": true}

// killByName là các lệnh dừng tiến trình theo tên/pattern.
var killByName = map[string]bool{"pkill": true, "killall": true}

// baseName chuẩn hoá argv[0] về tên lệnh (bỏ đường dẫn, bỏ đuôi Windows).
func baseName(arg string) string {
	name := strings.ToLower(filepath.Base(arg))
	return strings.TrimSuffix(name, ".exe")
}

// peelLaunchers bỏ các lớp bọc (env FOO=1, timeout 5, nice -n 9...) để tới
// lệnh thật.
func peelLaunchers(args []string) []string {
	for len(args) > 0 && processLaunchers[baseName(args[0])] {
		args = args[1:]
		for len(args) > 0 {
			a := args[0]
			// Bỏ cờ, gán biến (A=b) và tham số số (timeout 5) của lớp bọc.
			if strings.HasPrefix(a, "-") || strings.Contains(a, "=") || isAllDigits(a) {
				args = args[1:]
				continue
			}
			break
		}
	}
	return args
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// killPIDs trả các PID (dương) mà một lệnh `kill` nhắm tới. Bỏ cờ tín hiệu
// (-9, -TERM, -s KILL); PID âm (nhóm tiến trình) trả trị tuyệt đối vì giết
// nhóm cũng giết tiến trình trong đó.
func killPIDs(args []string) []int {
	var pids []int
	sawSignal := false // đã gặp đối số tín hiệu (-9, -s KILL) chưa
	for i := 1; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-s" || a == "--signal" || a == "-n":
			i++ // tên/số tín hiệu ở arg kế
			sawSignal = true
		case a == "--":
			// phần còn lại là PID
		case strings.HasPrefix(a, "-"):
			// "-N" đầu tiên (chưa có tín hiệu, chưa có PID) là TÍN HIỆU như
			// `kill -9`; "-N" sau đó là PID ÂM (nhóm tiến trình) như
			// `kill -9 -946242`.
			if !sawSignal && len(pids) == 0 {
				sawSignal = true
				continue
			}
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "-")); err == nil && n > 0 {
				pids = append(pids, n)
			}
		case strings.HasPrefix(a, "%"):
			// job spec của shell, không phải PID hệ thống
		default:
			if n, err := strconv.Atoi(a); err == nil && n > 0 {
				pids = append(pids, n)
			}
		}
	}
	return pids
}

// killNames trả các pattern/tên mà pkill/killall nhắm tới (bỏ cờ).
func killNames(args []string) []string {
	var names []string
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "-s" || a == "--signal" || a == "-u" || a == "--user" || a == "-U" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		names = append(names, a)
	}
	return names
}

// ProcessGuard quyết định một PID hay tên tiến trình có được agent-cli bảo vệ
// (không được tự giết). Tách ra để test thay bằng đồ giả.
type ProcessGuard struct {
	// ProtectedPID cho biết pid thuộc cây tiến trình agent-cli (chính nó,
	// tổ tiên, hoặc tiến trình khác chạy cùng binary).
	ProtectedPID func(pid int) bool
	// ProtectedName cho biết tên/pattern của pkill/killall khớp tiến trình
	// agent-cli.
	ProtectedName func(name string) bool
}

// Blocked báo lệnh args có bị chặn không, và lý do để ghi log.
func (g ProcessGuard) Blocked(args []string) (reason string, blocked bool) {
	args = peelLaunchers(args)
	if len(args) == 0 {
		return "", false
	}
	cmd := baseName(args[0])
	switch {
	case shutdownCommands[cmd]:
		return "shutting the machine down or rebooting is never allowed", true
	case killByPID[cmd]:
		for _, pid := range killPIDs(args) {
			if g.ProtectedPID != nil && g.ProtectedPID(pid) {
				return "refusing to kill agent-cli's own process (pid " + strconv.Itoa(pid) + "): it is running this session", true
			}
		}
	case killByName[cmd]:
		for _, name := range killNames(args) {
			if g.ProtectedName != nil && g.ProtectedName(name) {
				return "refusing to " + cmd + " a process matching agent-cli (" + name + "): it would kill this session's own server or client", true
			}
		}
	}
	return "", false
}

// selfExe là đường dẫn binary agent-cli đã giải symlink; selfTree là các PID
// trong cây tiến trình của chính nó (nó + tổ tiên tới init). Tính một lần.
var (
	selfExe  = resolveSelfExe()
	selfTree = selfProcessTree()
)

func resolveSelfExe() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		return r
	}
	return exe
}

// selfProcessTree trả PID của chính tiến trình và các tổ tiên (client/server/
// launcher/shell cha đều cần được bảo vệ). Leo theo /proc; thiếu /proc thì chỉ
// có pid và ppid.
func selfProcessTree() map[int]bool {
	tree := map[int]bool{os.Getpid(): true, os.Getppid(): true}
	pid := os.Getppid()
	for range 32 {
		ppid := parentPID(pid)
		if ppid <= 1 || tree[ppid] {
			break
		}
		tree[ppid] = true
		pid = ppid
	}
	return tree
}

// parentPID đọc PPid từ /proc/<pid>/status (Linux); 0 nếu không đọc được.
func parentPID(pid int) int {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "PPid:"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
				return n
			}
		}
	}
	return 0
}

// pidRunsSelfExe cho biết tiến trình pid đang chạy cùng binary với agent-cli
// (client, server, launcher đều là cùng một binary crush).
func pidRunsSelfExe(pid int) bool {
	if selfExe == "" {
		return false
	}
	target, err := filepath.EvalSymlinks("/proc/" + strconv.Itoa(pid) + "/exe")
	return err == nil && target == selfExe
}

// defaultProcessGuard là bộ bảo vệ dùng lúc chạy thật.
var defaultProcessGuard = ProcessGuard{
	ProtectedPID: func(pid int) bool {
		return selfTree[pid] || pidRunsSelfExe(pid)
	},
	ProtectedName: func(name string) bool {
		n := baseName(name)
		if n == "crush" || n == "agent-cli" {
			return true
		}
		// pkill/killall theo tên khớp chính tên binary của mình.
		return selfExe != "" && n == baseName(selfExe)
	},
}

// ProtectControlProcess là BlockFunc chặn lệnh giết tiến trình agent-cli hay
// tắt máy. onBlock (có thể nil) nhận lý do để ghi audit log.
func ProtectControlProcess(onBlock func(reason string, args []string)) BlockFunc {
	return func(args []string) bool {
		reason, blocked := defaultProcessGuard.Blocked(args)
		if blocked && onBlock != nil {
			onBlock(reason, args)
		}
		return blocked
	}
}
