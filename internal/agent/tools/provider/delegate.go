package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/execpolicy"
	"github.com/charmbracelet/crush/internal/permission"
)

// delegationTimeout bounds a single delegation. The external CLIs
// regularly need several minutes for coding tasks, so this is far more
// generous than the bash tool timeout.
const delegationTimeout = 15 * time.Minute

// DelegationParams is the shared input of every delegation tool: a
// self-contained task and optionally the session to continue.
type DelegationParams struct {
	Prompt string `json:"prompt" description:"The complete, self-contained task to delegate, including goal, file paths, and constraints"`
	Resume string `json:"resume,omitempty" description:"Session ID returned by a previous call to this tool to continue that conversation instead of starting a new one"`
	Model  string `json:"model,omitempty" description:"Model the delegated agent runs; must be one of the models this tool's description allows (default: the first one)"`
}

// resumeIDPattern là dạng hợp lệ của resume: session/thread ID mà chính tool
// đã trả về. Model đặt resume tự do, nên giá trị giống cờ ("-...") hay có
// khoảng trắng/ký tự điều khiển phải bị chặn trước khi tới argv của CLI con.
var resumeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

// permissionRequester là phần của permission.Service mà tool uỷ quyền cần:
// mỗi lần uỷ quyền phải được duyệt trước khi CLI con chạy.
type permissionRequester interface {
	Request(ctx context.Context, opts permission.CreatePermissionRequest) (bool, error)
}

// delegationOutput is the parsed outcome of one headless CLI run.
type delegationOutput struct {
	text      string // final report from the CLI
	sessionID string // identifier for resuming the conversation
}

// delegationError marks a failure the CLI itself reported in its output
// stream. Its message surfaces verbatim and takes precedence over the
// process exit code.
type delegationError struct{ msg string }

func (e *delegationError) Error() string { return e.msg }

// strategy is the per-CLI part of a delegation tool: everything that
// differs between external agents. The shared runner owns prompt
// validation, the permission prompt, binary availability, timeout,
// execution, and the error precedence rules, so every provider behaves
// identically.
type strategy interface {
	// name is the tool name and the session label in responses.
	name() string
	// describe is the embedded tool description for the model.
	describe() string
	// label là tên CLI hiển thị trong lời xin quyền.
	label() string
	// binary is the CLI executable: tên lệnh tìm trong PATH, hoặc đường
	// dẫn người dùng đặt qua biến binaryEnv.
	binary() string
	// binaryEnv là tên biến env ghi đè binary; biến có giá trị nghĩa là
	// người dùng chủ động chọn CLI, nên CLI hỏng thì phải báo lỗi.
	binaryEnv() string
	// access mô tả mức quyền CLI con được cấp, hiện trong lời xin quyền.
	access() string
	// validate kiểm cấu hình mức quyền (env) lúc dựng tool.
	validate() error
	// args assembles the headless invocation for one run. model là ID cụ
	// thể đã qua execpolicy; rỗng thì không truyền (chỉ test dùng).
	args(prompt, resume, model string) []string
	// parse extracts the outcome from captured stdout. A failure the CLI
	// reported in its own output comes back as a *delegationError; output
	// that cannot be interpreted comes back as any other error.
	parse(stdout []byte) (delegationOutput, error)
}

// NewTools trả về các tool uỷ quyền mà agent được phép dùng (allowed) và máy
// có CLI. CLI mặc định không có trong PATH thì bỏ tool đó: model không thấy
// một tool chắc chắn hỏng, và request không tốn token mô tả nó. Biến *_BIN đã
// đặt mà không chạy được là cấu hình hỏng: trả lỗi khi dựng agent (lỗi hiện ở
// prompt đầu tiên), trừ khi tool đã bị tắt (permissions deny).
//
// Ranh giới provider (execpolicy): CLI có trên máy, đã đăng nhập hay có key
// chưa đủ. Giao việc phải được bật (AGENT_CLI_DELEGATION=on) và backend phải
// có model cụ thể trong AGENT_CLI_ALLOWED_MODELS; không thì tool không tồn
// tại, model chính không thể gọi tới.
func NewTools(workingDir string, perms permissionRequester, allowed []string, pol execpolicy.Policy) ([]fantasy.AgentTool, error) {
	var out []fantasy.AgentTool
	for _, s := range []strategy{newClaudeStrategy(), newCodexStrategy()} {
		if !slices.Contains(allowed, s.name()) {
			continue
		}
		models := pol.DelegationModels(s.name())
		if len(models) == 0 {
			slog.Debug("Delegation tool not registered", "backend", s.name(), "reason", pol.CheckDelegation(s.name(), ""))
			continue
		}
		// Mức quyền gõ sai là cấu hình hỏng như *_BIN sai: báo ngay, nêu tên
		// biến, thay vì để CLI con lỗi ở mỗi lần giao (sau khi đã duyệt).
		if err := s.validate(); err != nil {
			return nil, err
		}
		path, err := exec.LookPath(s.binary())
		if err != nil {
			if execpolicy.TrustedEnv(s.binaryEnv()) != "" {
				return nil, fmt.Errorf("%s=%q is not an executable: %w", s.binaryEnv(), s.binary(), err)
			}
			continue
		}
		// Chốt đường dẫn tuyệt đối: lúc chạy, CLI con chạy với cmd.Dir là thư
		// mục dự án nên đường dẫn tương đối sẽ trỏ sai chỗ.
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		out = append(out, newDelegationTool(workingDir, perms, resolvedStrategy{strategy: s, path: path}, models))
	}
	return out, nil
}

// resolvedStrategy là strategy đã chốt đường dẫn CLI lúc dựng tool.
type resolvedStrategy struct {
	strategy
	path string
}

func (r resolvedStrategy) binary() string { return r.path }

// newDelegationTool wires a strategy into the shared delegation flow:
// validate the prompt, ask for permission, check the binary, run it
// headlessly in workingDir, then let the structured output — not the exit
// code — decide the outcome.
//
// models là các model execpolicy cho phép, model đầu là mặc định. Model lạ
// (alias, Fable, Astra, bản cũ hay bản mới chưa liệt kê) bị từ chối trước khi
// hỏi quyền và trước khi chạy gì; danh sách rỗng thì mọi lần gọi đều bị từ chối.
func newDelegationTool(workingDir string, perms permissionRequester, s strategy, models []string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		s.name(),
		s.describe()+modelsDescription(models),
		func(ctx context.Context, params DelegationParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Prompt) == "" {
				return fantasy.ToolResponse{}, errors.New("prompt is required")
			}
			if params.Resume != "" && !resumeIDPattern.MatchString(params.Resume) {
				return fantasy.ToolResponse{}, fmt.Errorf("resume must be a session ID returned by an earlier %s call", s.name())
			}
			model := strings.TrimSpace(params.Model)
			if model == "" && len(models) > 0 {
				model = models[0]
			}
			if model == "" || !slices.Contains(models, model) {
				denied := &execpolicy.Denied{Code: execpolicy.CodeModelNotAllowed, Provider: s.name(), Model: model, Allowed: slices.Clone(models)}
				slog.Info("Delegation denied", "backend", s.name(), "model", model, "code", denied.Code)
				return fantasy.NewTextErrorResponse(denied.Error()), nil
			}
			params.Model = model
			if perms == nil {
				// Không có cổng quyền thì không chạy: CLI con không hỏi lại ai.
				return fantasy.ToolResponse{}, errors.New("permission service unavailable")
			}

			// CLI con chạy với mức quyền cao và không hỏi lại ai, nên lời
			// xin quyền này là ranh giới tin cậy duy nhất: không có session
			// để hỏi thì không chạy.
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for delegating to %s", s.label())
			}
			granted, err := perms.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  call.ID,
				ToolName:    s.name(),
				Action:      "execute",
				Path:        workingDir,
				Description: permissionDescription(s, workingDir, params),
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !granted {
				return tools.NewPermissionDeniedResponse(), nil
			}

			ctx, cancel := context.WithTimeout(ctx, delegationTimeout)
			defer cancel()

			slog.Info("Delegation approved", "backend", s.name(), "model", model)
			stdout, runErr := runCommand(ctx, workingDir, s.binary(), s.args(params.Prompt, params.Resume, model)...)
			out, parseErr := s.parse(stdout)
			switch {
			case parseErr == nil:
				return fantasy.NewTextResponse(responseText(s.name(), out)), nil
			case isCLIReported(parseErr):
				return fantasy.ToolResponse{}, parseErr
			case runErr != nil:
				return fantasy.ToolResponse{}, runErr
			default:
				return fantasy.ToolResponse{}, parseErr
			}
		},
	)
}

// permissionDescription là nội dung hộp thoại xin quyền: CLI nào, chạy ở
// đâu, với mức quyền nào, và nguyên văn việc được giao. Params để trống vì
// hộp thoại in params của tool lạ thành map khó đọc ở chế độ client/server.
func permissionDescription(s strategy, workingDir string, params DelegationParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s) will run this task headlessly in %s with %s, model %s.", s.label(), displaySafe(s.binary()), workingDir, s.access(), displaySafe(params.Model))
	if params.Resume != "" {
		fmt.Fprintf(&b, " It continues session %s.", params.Resume)
	}
	b.WriteString(" Allowing for the whole session also approves every later delegation in it without asking.")
	b.WriteString("\n\n")
	b.WriteString(displaySafe(strings.TrimSpace(params.Prompt)))
	return b.String()
}

// displaySafe thay ký tự điều khiển (trừ xuống dòng, tab) và ký tự đảo chiều
// bidi bằng U+FFFD: prompt do model viết không được dùng escape terminal hay
// đảo thứ tự hiển thị để làm hộp thoại xin quyền đọc khác với việc sẽ chạy.
func displaySafe(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var prev rune // ký tự GỐC đứng trước (trước khi thay)
	for _, orig := range s {
		r := orig
		switch {
		case r == '\n' || r == '\t':
		case unicode.IsControl(r), unicode.Is(unicode.Bidi_Control, r),
			// Ký tự vô hình (zero-width, soft hyphen, BOM, tag U+E00xx…),
			// ngăn dòng/đoạn Unicode, vùng riêng và mã chưa gán: CLI con nhận
			// được nhưng người duyệt không thấy.
			unicode.Is(unicode.Cf, r), unicode.Is(unicode.Zl, r), unicode.Is(unicode.Zp, r),
			unicode.Is(unicode.Co, r), !unicode.IsGraphic(r) && !unicode.IsSpace(r),
			isInvisibleFiller(r):
			r = '\uFFFD'
		case isHiddenMark(r):
			// Một VS15/VS16 ngay sau ký tự biểu tượng là cách vẽ emoji, giữ;
			// mọi dấu ẩn khác (hay VS lặp) có thể giấu dữ liệu.
			emojiSelector := (r == '\uFE0E' || r == '\uFE0F') && prev != '\uFFFD' &&
				unicode.In(prev, unicode.So, unicode.Sm, unicode.Sk)
			if !emojiSelector {
				r = '\uFFFD'
			}
		}
		b.WriteRune(r)
		prev = orig
	}
	return b.String()
}

// modelsDescription báo cho model chính các model được giao (phần thêm vào mô
// tả tool).
func modelsDescription(models []string) string {
	if len(models) == 0 {
		return ""
	}
	return fmt.Sprintf("\n\nModel: set `model` to one of %s (default %s). Any other model is refused.", strings.Join(models, ", "), models[0])
}

// isInvisibleFiller nhận các ký tự "lấp chỗ" hiển thị như khoảng trắng nhưng
// không phải khoảng trắng (Hangul filler, braille trống), hay dùng để giấu chữ.
func isInvisibleFiller(r rune) bool {
	switch r {
	case '\u115F', '\u1160', '\u3164', '\uFFA0', '\u2800':
		return true
	}
	return false
}

// isHiddenMark nhận dấu kết hợp vô hình (variation selector, CGJ, nguyên âm
// Khmer ẩn, Mongolian FVS) hay dùng để giấu dữ liệu sau một ký tự thấy được.
func isHiddenMark(r rune) bool {
	switch {
	case r >= 0xFE00 && r <= 0xFE0F, r >= 0xE0100 && r <= 0xE01EF,
		r == 0x034F, r >= 0x180B && r <= 0x180D, r == 0x180F, r == 0x17B4, r == 0x17B5:
		return true
	}
	return false
}

// isCLIReported reports whether err is a failure the CLI stated in its own
// output rather than a broken invocation or unparseable output.
func isCLIReported(err error) bool {
	var delErr *delegationError
	return errors.As(err, &delErr)
}

// responseText renders the report with the resume hint appended.
func responseText(name string, out delegationOutput) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(out.text))
	if out.sessionID != "" {
		fmt.Fprintf(&b, "\n\n%s session: %s (pass as resume to continue this conversation)", name, out.sessionID)
	}
	return b.String()
}
