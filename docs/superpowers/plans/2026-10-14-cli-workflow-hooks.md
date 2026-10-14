# Dòng lệnh cho workflow + hook Stop — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Người thi hành:** Opus effort high, **được quyền phản biện plan**.
> **Cấm:** viết số chưa chạy lệnh · nở phạm vi · commit/push khi user chưa cho phép.

**Goal:** Script và agent điều khiển được agent-cli bằng dòng lệnh để chạy nhiều workflow: `run --json`, `send` vào session đang mở của dự án khác, hook `Stop` làm cổng máy, và đọc `.claude/commands`.

**Architecture:** Chỉ thêm lệnh CLI và một decorator. Không đụng `internal/agent/coordinator.go` hay `internal/config/config.go`, vì phiên khác đang sửa hai file này.
- Hook `Stop` được làm bằng decorator `Coordinator` bọc ở `app.go`, giống cách `hookedTool` bọc tool.
- Hook `PostToolUse` bị hoãn, vì runner của hook được dựng ở `coordinator.go:874-876`.

**Tech Stack:** Go 1.27, cobra, testify.

**Spec:** `docs/superpowers/specs/2026-10-14-feature-gap-analysis.md` (P1, P2, P3, P5). Plan này chạy độc lập với plan 1 và plan 2, trừ D3 (`send`) cần wrapper chọn server theo dự án (plan 1, B0).

## Sự thật nền (đọc code 14/10)

- **`run`:**
  - Có hai đường chạy: local gọi `App.RunNonInteractive(ctx, os.Stdout, …)` (`internal/app/app.go:285`); client/server gọi `runNonInteractive(ctx, c, ws, …)` trong `internal/cmd`.
  - Cả hai đều không trả session ID ra ngoài.
  - Có sẵn cờ `-m`, `-s`, `-C`; chưa có `--json`.
- **Hook:**
  - Cấu hình hook là `map[string][]HookConfig` (`config.go:856`). Tên event được chuẩn hoá trong `load.go` `normalizeHookEvent`, hiện chỉ biết `pretooluse`.
  - `hooks.Runner.Run(ctx, eventName, sessionID, toolName, toolInputJSON)` đã nhận tên event bất kỳ.
  - Decision có `None`, `Allow`, `Deny`.
- **Client:** có `SendMessage` và `SubscribeEvents` (`internal/client/proto.go:507,114`).
- **Slash command:** chỉ đọc thư mục của crush (`internal/commands/commands.go:121-136`).

## Global Constraints

- Không sửa `coordinator.go`, `config.go`, `internal/agent/tools/provider/`. Lệnh Go chạy `GOMAXPROCS=2 -p 2`. Không commit.
- Đầu ra `--json` là **một** object JSON trên stdout, tên trường dạng snake_case. Log không bao giờ in ra stdout.
- Mã thoát: 0 là thành công; 1 là lỗi khi chạy (vẫn in JSON có `error`); 2 là gọi sai cú pháp.

## Review Focus

1. `run --json` khi model lỗi: in JSON có `error` khác rỗng và rc=1, không in stacktrace ra stdout (`TestRunJSONErrorShape`).
2. `run --json` khi chạy song song ở 2 process: mỗi process trả **đúng** session của nó, không lấy "session mới nhất" (`TestRunJSONReportsOwnSession`).
3. Hook `Stop` chặn liên tục: agent chạy lại tối đa 3 lần rồi dừng kèm cảnh báo, không lặp vô hạn (`TestStopHookLoopGuard`).
4. `send --wait` khi session không tồn tại: rc=1 và có thông báo rõ ràng, không treo (`TestSendUnknownSession`).
5. `.claude/commands` trùng tên với lệnh của crush: lệnh crush thắng, có cảnh báo (`TestClaudeCommandsDoNotShadow`).

## Cách chứng minh là xong

```bash
GOMAXPROCS=2 go test -p 2 ./internal/cmd ./internal/app ./internal/agent ./internal/hooks ./internal/commands ./internal/config -count=1
```

Kiểm tay:
- `agent-cli run --json "Trả lời: OK"` in object có `session_id`, `text` chứa "OK", và `prompt_tokens` > 0.
- `agent-cli run --json -s <id> "tiếp"` dùng lại đúng session đó.

---

### Task D1: `agent-cli run --json`

**Files:** Modify `internal/cmd/run.go`, `internal/app/app.go` (`RunNonInteractive`), hàm `runNonInteractive` của nhánh client/server trong `internal/cmd`; Test `internal/cmd/run_json_test.go`, `internal/app/app_test.go` (hoặc file test sẵn có của `RunNonInteractive`).

**Interfaces:**
- Cờ `--json` (bool).
- `type runJSONResult struct { SessionID string \`json:"session_id"\`; Text string \`json:"text"\`; Error string \`json:"error,omitempty"\`; PromptTokens int64 \`json:"prompt_tokens"\`; CompletionTokens int64 \`json:"completion_tokens"\`; Cost float64 \`json:"cost"\` }`.
- Cả hai đường chạy phải **báo lại session ID thật** đã dùng hoặc đã tạo. Ví dụ: thêm tham số `onSession func(sessionID string)` vào `RunNonInteractive`, hoặc một struct option; chọn cách ít đụng caller nhất và ghi lý do.
- Text: thu từ writer (buffer) thay cho stdout.
- Token và chi phí: đọc session theo ID sau khi chạy xong.
- Khi có `--json`: tắt spinner (như `-q`); bỏ qua `--verbose`, vì log không được ra stdout.

- [ ] **Step 1:** Viết các test:
  - `TestRunJSONSuccessShape`: dùng provider mock hoặc stub theo mẫu test có sẵn (`run_stream_test.go`).
  - `TestRunJSONErrorShape`.
  - `TestRunJSONReportsOwnSession`: gọi hai lần với hai session khác nhau, mỗi lần trả đúng ID của nó.
  - `TestRunJSONContinueSession`.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy `GOMAXPROCS=2 go test -p 2 ./internal/cmd ./internal/app -count=1`, kỳ vọng PASS.

### Task D2: Hook `Stop` (cổng máy khi agent định kết thúc)

**Files:** Create `internal/agent/stop_hook.go`, `internal/agent/stop_hook_test.go`; Modify `internal/app/app.go` (bọc `AgentCoordinator` ngay sau `agent.NewCoordinator`), `internal/config/load.go` (`normalizeHookEvent` thêm `stop`), `internal/hooks/input.go` (payload thêm `stop_hook_active`), `internal/hooks/hooks.go` (hằng `EventStop = "Stop"`), `docs/hooks/README.md`.

**Interfaces:**
- `func WithStopHooks(inner Coordinator, runner *hooks.Runner) Coordinator`: trả `inner` khi `runner == nil`. Decorator embed `Coordinator` và chỉ override `Run` và `RunAccepted`.
- Sau khi lượt chạy bên trong kết thúc **không lỗi và không bị huỷ**: gọi `runner.Run(ctx, "Stop", sessionID, "", "{}")` với payload `stop_hook_active` = đây có phải lần chạy lại do hook chặn hay không.
  - Kết quả `Deny` (exit 2 hoặc decision `block`): chạy lại cùng session với prompt là `Reason` của hook, tối đa **3** lần liên tiếp cho mỗi lời gọi gốc.
  - Quá 3 lần: dừng, ghi `slog.Warn("Stop hook kept blocking", …)`, rồi trả về kết quả cuối.
- Runner dựng từ `cfg.Hooks["Stop"]`, giống `coordinator.go:875` làm với PreToolUse.

- [ ] **Step 1:** Viết các test:
  - `TestStopHookAllowsFinish`: hook exit 0 → 1 lần chạy.
  - `TestStopHookBlockReruns`: hook chặn 1 lần → 2 lần chạy, lần 2 có prompt là reason.
  - `TestStopHookLoopGuard`: luôn chặn → đúng 4 lần chạy (1 + 3) rồi dừng.
  - `TestStopHookSkippedOnError`.
  - `TestNormalizeHookEventStop`.
  - Inner coordinator là stub đếm số lần chạy; hook là script tạm trong `t.TempDir()`.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy `GOMAXPROCS=2 go test -p 2 ./internal/agent ./internal/hooks ./internal/config ./internal/app -count=1`, kỳ vọng PASS. Docs có ví dụ hook `Stop` chạy lint và chặn khi đỏ.

### Task D3: `agent-cli send` — giao việc vào session đang mở

**Files:** Create `internal/cmd/send.go`, `internal/cmd/send_test.go`; Modify `docs/agent-cli-local-setup.md`.

**Interfaces:**
- Cú pháp: `agent-cli send [-c dir] (--session <id> | --new [--title t]) [--wait] [--json] "prompt"`.
- Luôn đi qua server của dự án (dùng `connectToServer` như `run`): bắt buộc `CRUSH_CLIENT_SERVER=1` (wrapper B0 đã đặt), nếu không thì rc=2 kèm hướng dẫn.
- `--session` nhận ID đầy đủ hoặc tiền tố như `run -s` (dùng `resolveSessionByID`). Không thấy session thì rc=1.
- Không có `--wait`: trả ngay sau khi server nhận prompt; in `session_id` (hoặc JSON `{session_id}`).
- Có `--wait`: theo dõi `SubscribeEvents` cho tới `RunComplete` của **đúng** session đó, rồi in text cuối (hoặc JSON giống `runJSONResult` của D1). Thoát theo ctx (SIGINT) hoặc `--timeout` (mặc định 0 = không giới hạn).
- Prompt hiện trong TUI đang mở dự án đó như một tin nhắn bình thường.

- [ ] **Step 1:** Viết test với client giả (interface nhỏ: `SendMessage`, `SubscribeEvents`, `ListSessions`, `CreateSession`):
  - `TestSendNoWaitPrintsSession`.
  - `TestSendWaitReturnsOnOwnRunComplete`: RunComplete của session khác bị bỏ qua.
  - `TestSendUnknownSession`.
  - `TestSendRequiresClientServer`.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy lại, kỳ vọng PASS.

### Task D4: Đọc `.claude/commands/*.md` làm slash command

**Files:** Modify `internal/commands/commands.go`; Test `internal/commands/commands_test.go`.

**Interfaces:**
- Thêm hai nguồn sau các nguồn của crush: `~/.claude/commands` và `<project>/.claude/commands`.
- Thư mục con thành tiền tố, giống cách crush đang làm. Placeholder `$ARGUMENTS` của Claude Code được hiểu như một tham số tên `ARGUMENTS`.
- Trùng tên với lệnh có sẵn: lệnh có sẵn thắng, ghi `slog.Warn` một lần.

- [ ] **Step 1:** Viết các test: `TestLoadsClaudeCommandsProjectAndUser`, `TestClaudeArgumentsPlaceholder`, `TestClaudeCommandsDoNotShadow`. Chạy, kỳ vọng FAIL. Cài đặt. Chạy `GOMAXPROCS=2 go test -p 2 ./internal/commands -count=1`, kỳ vọng PASS.

## Hoãn

- **PostToolUse:** cần sửa `coordinator.go:874-876` (dựng runner cho cả `PostToolUse`) và `hooked_tool.go` (gọi hook sau khi tool chạy). Làm sau khi phiên khác xong phần sửa `coordinator.go`.
- **Subagent tự định nghĩa (P4):** cũng hoãn, cùng lý do.

## Thứ tự

D1 → D4 (độc lập, chạy được ngay) → D2 → D3 (sau B0 của plan 1).
