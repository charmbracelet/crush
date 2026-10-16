package execpolicy

import (
	"regexp"
	"strings"
)

var (
	interpreterName = regexp.MustCompile(`^(node|nodejs|bun|deno|python[0-9.]*|ruby|perl|php)$`)
	processLaunch   = regexp.MustCompile(`\b(exec(File)?(Sync)?|spawn(Sync)?|system|shell_exec|passthru|popen|Popen|run|call|check_call|check_output)\s*(\(|['"])`)
	scriptLiterals  = regexp.MustCompile("(?s)'(?:\\\\.|[^'\\\\])*'|\"(?:\\\\.|[^\"\\\\])*\"|`(?:\\\\.|[^`\\\\])*`")
)

// interpreterCommand chỉ nhận diện lệnh/script agent và tên lệnh literal trong
// code inline. shortcut: code mã hoá, tên tính động và file script tuỳ ý không
// kiểm được ở lớp argv; muốn chặn cả process con phải dùng sandbox của OS.
func interpreterCommand(args []string, depth int) (string, bool) {
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-e", "-c", "--eval", "--print", "-p", "-r":
			if i+1 < len(args) {
				return inlineAgentCommand(args[i+1], depth)
			}
			return "", false
		case "--":
			return externalAgentCommand(args[i+1:], depth+1)
		}
		for _, flag := range []string{"--eval=", "--print="} {
			if source, ok := strings.CutPrefix(arg, flag); ok {
				return inlineAgentCommand(source, depth)
			}
		}
		if !strings.HasPrefix(arg, "-") {
			return externalAgentCommand(args[i:], depth+1)
		}
	}
	return "", false
}

// inlineAgentCommand không chặn lời nhắc tên CLI trong print/log thông thường;
// chỉ xét literal khi code có lời gọi API tạo process được nhận diện.
func inlineAgentCommand(source string, depth int) (string, bool) {
	if !processLaunch.MatchString(source) {
		return "", false
	}
	for _, literal := range scriptLiterals.FindAllString(source, -1) {
		if name, ok := commandString(literal[1:len(literal)-1], depth); ok {
			return name, true
		}
	}
	return "", false
}
