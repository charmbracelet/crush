// Package execpolicy là ranh giới provider/model của agent-cli: model nào
// được chạy cho session chính (và sub-agent của nó), và có được giao việc sang
// CLI agent của provider khác (Claude Code, Codex) hay không.
//
// Cài sẵn trên máy, đã đăng nhập hay có API key KHÔNG có nghĩa là được phép:
// chỉ cấu hình tường minh mới mở. Mọi đường chạy model hoặc CLI agent đều hỏi
// gói này (coordinator khi dựng model, tool giao việc khi đăng ký và khi gọi,
// tool bash khi chặn lệnh gọi thẳng CLI agent, hộp thoại chọn model).
package execpolicy

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode"
)

const (
	// AllowedModelsEnv là danh sách "provider:model" được phép, cách nhau bằng
	// dấu phẩy hoặc khoảng trắng. Có ít nhất một provider chính trong danh
	// sách thì model chính (large/small) phải nằm trong đó: model mới của
	// provider mặc định bị chặn cho tới khi được thêm vào.
	AllowedModelsEnv = "AGENT_CLI_ALLOWED_MODELS"
	// DelegationEnv bật giao việc sang CLI agent khác provider ("on"); mặc
	// định tắt dù CLI có trên máy.
	DelegationEnv = "AGENT_CLI_DELEGATION"
)

// Backend giao việc: CLI agent bên ngoài, gọi qua tool cùng tên.
const (
	BackendClaudeCode = "claude_code"
	BackendCodex      = "codex"
)

// Mã từ chối, dùng trong lỗi và log.
const (
	CodeModelNotAllowed         = "MODEL_NOT_ALLOWED"
	CodeCrossProviderNotAllowed = "CROSS_PROVIDER_NOT_AUTHORIZED"
	CodeExternalAgentNotAllowed = "EXTERNAL_AGENT_NOT_AUTHORIZED"
)

// Policy là chính sách đã nạp. Giá trị rỗng: model chính không giới hạn,
// giao việc tắt.
type Policy struct {
	models     map[string][]string // provider -> model được phép, theo thứ tự khai
	restricted bool                // có danh sách cho provider chính
	delegation bool                // DelegationEnv bật
}

// Denied là lỗi từ chối kèm mã, để người dùng và log biết vì sao.
type Denied struct {
	Code     string
	Provider string
	Model    string
	Allowed  []string
}

func (d *Denied) Error() string {
	target := d.Provider
	if d.Model != "" {
		target += ":" + d.Model
	}
	var why string
	switch d.Code {
	case CodeModelNotAllowed:
		why = "not listed in " + AllowedModelsEnv
	case CodeCrossProviderNotAllowed:
		why = "delegating to another provider's agent is off (" + DelegationEnv + "=on enables it)"
	case CodeExternalAgentNotAllowed:
		why = "no model of this agent is listed in " + AllowedModelsEnv
	}
	msg := fmt.Sprintf("%s: %s is %s", d.Code, target, why)
	if len(d.Allowed) > 0 {
		msg += "; allowed: " + strings.Join(d.Allowed, ", ")
	}
	return msg
}

// Current trả chính sách của tiến trình, nạp một lần từ hai nguồn của người
// dùng: env đã export ở shell, rồi tệp env của agent-cli (AGENT_CLI_ENV_FILE,
// mặc định ~/.config/agent-cli/env, chính tệp crushrc `source`). Cố ý KHÔNG
// đọc cấu hình của dự án (crush.json, crushrc trong repo): một repo lạ không
// được nới ranh giới. Đổi giá trị thì mở lại server của dự án.
func Current() (Policy, error) { return current() }

var current = sync.OnceValues(func() (Policy, error) {
	if testing.Testing() {
		// Test phải kín: không đọc tệp env thật của người chạy test (chính
		// sách của máy đó sẽ chặn model giả). Test cần chính sách thì dựng
		// bằng Parse/FromEnv.
		return FromEnv(os.Getenv)
	}
	return load(os.Getenv)
})

// load là Current với getenv cho trước (test).
func load(getenv func(string) string) (Policy, error) {
	file, err := readEnvFile(envFilePath(getenv))
	if err != nil {
		return Policy{}, err
	}
	return FromEnv(func(k string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return file[k]
	})
}

// envFilePath là đường dẫn tệp env của agent-cli, như crushrc mẫu tính.
func envFilePath(getenv func(string) string) string {
	if p := getenv("AGENT_CLI_ENV_FILE"); p != "" {
		return p
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "agent-cli", "env")
	}
	return ""
}

// readEnvFile đọc ĐÚNG hai biến của chính sách từ tệp env (gán đơn giản, có
// thể có `export`, nháy và comment cuối dòng). Mọi dòng khác (khoá API...) bị
// bỏ qua, không giữ lại. Tệp không có thì rỗng; gán sau thắng gán trước như
// shell.
func readEnvFile(path string) (map[string]string, error) {
	out := map[string]string{}
	if path == "" {
		return out, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, val, ok := strings.Cut(line, "=")
		if !ok || (key != AllowedModelsEnv && key != DelegationEnv) {
			continue
		}
		out[key] = shellValue(val)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return out, nil
}

// shellValue lấy giá trị của một phép gán shell đơn giản: trong cặp nháy nếu
// có, không thì tới trước comment cuối dòng.
func shellValue(v string) string {
	v = strings.TrimSpace(v)
	if v != "" && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : end+1]
		}
		return strings.Trim(v, string(v[0]))
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// FromEnv đọc chính sách qua getenv.
func FromEnv(getenv func(string) string) (Policy, error) {
	p, err := Parse(getenv(AllowedModelsEnv))
	if err != nil {
		return Policy{}, err
	}
	switch v := strings.ToLower(strings.TrimSpace(getenv(DelegationEnv))); v {
	case "", "off", "0", "false", "no":
	case "on", "1", "true", "yes":
		p.delegation = true
	default:
		return Policy{}, fmt.Errorf("%s=%q: use on or off", DelegationEnv, v)
	}
	return p, nil
}

// Parse đọc danh sách "provider:model". Model của backend giao việc phải là
// ID cụ thể (có số phiên bản): alias như "opus", "latest", "fable" để CLI tự
// chọn model mới nhất, nên không thể là ranh giới.
func Parse(spec string) (Policy, error) {
	p := Policy{models: map[string][]string{}}
	split := func(r rune) bool { return r == ',' || unicode.IsSpace(r) }
	for _, entry := range strings.FieldsFunc(spec, split) {
		provider, model, ok := strings.Cut(entry, ":")
		provider, model = strings.ToLower(strings.TrimSpace(provider)), strings.TrimSpace(model)
		if !ok || provider == "" || model == "" {
			return Policy{}, fmt.Errorf("%s: %q is not provider:model", AllowedModelsEnv, entry)
		}
		if IsBackend(provider) && !strings.ContainsAny(model, "0123456789") {
			return Policy{}, fmt.Errorf("%s: %q is an alias, which the CLI resolves to whatever model is newest; list the exact model ID (for example claude_code:claude-opus-5-5)", AllowedModelsEnv, entry)
		}
		if !slices.Contains(p.models[provider], model) {
			p.models[provider] = append(p.models[provider], model)
		}
		if !IsBackend(provider) {
			p.restricted = true
		}
	}
	return p, nil
}

// IsBackend cho biết provider là backend giao việc (CLI agent bên ngoài).
func IsBackend(provider string) bool {
	return provider == BackendClaudeCode || provider == BackendCodex
}

// Restricted cho biết model chính bị giới hạn theo danh sách.
func (p Policy) Restricted() bool { return p.restricted }

// AllowsModel cho biết model chính provider/model được phép.
func (p Policy) AllowsModel(provider, model string) bool {
	if !p.restricted {
		return true
	}
	return slices.Contains(p.models[strings.ToLower(provider)], model)
}

// CheckModel trả *Denied khi model chính không được phép.
func (p Policy) CheckModel(provider, model string) error {
	if p.AllowsModel(provider, model) {
		return nil
	}
	return &Denied{Code: CodeModelNotAllowed, Provider: provider, Model: model, Allowed: p.mainPairs()}
}

// DelegationModels trả các model được phép của backend giao việc, theo thứ
// tự khai (model đầu là mặc định); nil khi giao việc tắt hoặc backend không
// có model nào trong danh sách.
func (p Policy) DelegationModels(backend string) []string {
	if !p.delegation {
		return nil
	}
	return slices.Clone(p.models[backend])
}

// CheckDelegation trả *Denied khi không được giao việc cho backend/model.
func (p Policy) CheckDelegation(backend, model string) error {
	allowed := p.models[backend]
	switch {
	case !p.delegation:
		return &Denied{Code: CodeCrossProviderNotAllowed, Provider: backend}
	case len(allowed) == 0:
		return &Denied{Code: CodeExternalAgentNotAllowed, Provider: backend}
	case !slices.Contains(allowed, model):
		return &Denied{Code: CodeModelNotAllowed, Provider: backend, Model: model, Allowed: slices.Clone(allowed)}
	}
	return nil
}

// mainPairs liệt kê các cặp provider:model chính được phép (để báo lỗi).
func (p Policy) mainPairs() []string {
	var out []string
	for provider, models := range p.models {
		if IsBackend(provider) {
			continue
		}
		for _, m := range models {
			out = append(out, provider+":"+m)
		}
	}
	slices.Sort(out)
	return out
}

// externalAgents là CLI agent của provider khác: gọi thẳng qua bash thì vượt
// ranh giới (không ghim model, không qua hộp thoại giao việc). agent-cli và
// crush là chính công cụ này nên không nằm ở đây.
var externalAgents = map[string]bool{
	"claude": true, "codex": true, "gemini": true, "qwen": true, "opencode": true,
	"aider": true, "cursor-agent": true, "amp": true, "goose": true, "kiro": true,
}

// externalAgentPackages là gói npm của các CLI đó (chạy qua npx, bunx...).
var externalAgentPackages = []string{"claude-code", "@openai/codex", "@google/gemini-cli", "@qwen-code/"}

// launchers chạy lệnh khác làm tiến trình con; lệnh thật nằm sau các cờ,
// gán biến và tham số của chúng. Giá trị là các cờ nhận giá trị ở arg kế
// (dạng "-u NAME"; dạng "--unset=NAME" tự nằm gọn trong một arg).
var launchers = map[string][]string{
	"env":     {"-u", "--unset", "-C", "--chdir", "-S", "--split-string"},
	"nohup":   nil,
	"timeout": {"-k", "--kill-after", "-s", "--signal"},
	"nice":    {"-n", "--adjustment"},
	"stdbuf":  {"-i", "-o", "-e"},
	"sudo":    {"-u", "-g", "-C", "-D", "-h", "-p", "-r", "-t", "-U", "-T"},
	"command": nil,
	"exec":    {"-a"},
	"time":    {"-f", "-o"},
	"setsid":  nil,
}

// ExternalAgentCommand nhận lệnh bash gọi CLI agent bên ngoài (kể cả qua đường
// dẫn, env/nohup/timeout/sudo, hay npx/bunx với gói của CLI đó) và trả tên của
// nó. Là lớp chặn thứ hai: ranh giới chính là tool giao việc chỉ có khi được
// phép. Không bắt được mọi đường (vd `bash -c "..."`); các lệnh đó vẫn phải
// qua hộp thoại xin quyền của bash.
func ExternalAgentCommand(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		name := filepath.Base(args[i])
		if externalAgents[name] {
			return name, true
		}
		switch name {
		case "npx", "bunx", "pnpx", "dlx", "pnpm", "yarn", "bun":
			for _, a := range args[i+1:] {
				for _, pkg := range externalAgentPackages {
					if strings.Contains(a, pkg) {
						return a, true
					}
				}
			}
			return "", false
		}
		valueFlags, ok := launchers[name]
		if !ok {
			return "", false
		}
		// Bỏ qua cờ (và giá trị của cờ nhận giá trị), gán biến (A=b) và tham
		// số số (timeout 30) của launcher để tới lệnh thật.
	skip:
		for i+1 < len(args) {
			next := args[i+1]
			switch {
			case slices.Contains(valueFlags, next):
				i += 2
			case strings.HasPrefix(next, "-") || strings.Contains(next, "=") || isNumberish(next):
				i++
			default:
				break skip
			}
		}
	}
	return "", false
}

// isNumberish nhận tham số kiểu số/thời lượng của launcher ("30", "5s", "1.5m").
func isNumberish(s string) bool {
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false
	}
	return strings.Trim(s, "0123456789.smhd") == ""
}
