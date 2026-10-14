package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
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
	// args assembles the headless invocation for one run.
	args(prompt, resume string) []string
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
func NewTools(workingDir string, perms permissionRequester, allowed []string) ([]fantasy.AgentTool, error) {
	var out []fantasy.AgentTool
	for _, s := range []strategy{newClaudeStrategy(), newCodexStrategy()} {
		if !slices.Contains(allowed, s.name()) {
			continue
		}
		path, err := exec.LookPath(s.binary())
		if err != nil {
			if os.Getenv(s.binaryEnv()) != "" {
				return nil, fmt.Errorf("%s=%q is not an executable: %w", s.binaryEnv(), s.binary(), err)
			}
			continue
		}
		// Chốt đường dẫn tuyệt đối: lúc chạy, CLI con chạy với cmd.Dir là thư
		// mục dự án nên đường dẫn tương đối sẽ trỏ sai chỗ.
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		out = append(out, newDelegationTool(workingDir, perms, resolvedStrategy{strategy: s, path: path}))
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
func newDelegationTool(workingDir string, perms permissionRequester, s strategy) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		s.name(),
		s.describe(),
		func(ctx context.Context, params DelegationParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Prompt) == "" {
				return fantasy.ToolResponse{}, errors.New("prompt is required")
			}
			if params.Resume != "" && !resumeIDPattern.MatchString(params.Resume) {
				return fantasy.ToolResponse{}, fmt.Errorf("resume must be a session ID returned by an earlier %s call", s.name())
			}
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

			stdout, runErr := runCommand(ctx, workingDir, s.binary(), s.args(params.Prompt, params.Resume)...)
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
	fmt.Fprintf(&b, "%s will run this task headlessly in %s with %s.", s.label(), workingDir, s.access())
	if params.Resume != "" {
		fmt.Fprintf(&b, " It continues session %s.", params.Resume)
	}
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(params.Prompt))
	return b.String()
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
