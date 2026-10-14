# Việc song song cô lập (worktree + model + log riêng) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Người thi hành:** Opus effort high, **được quyền phản biện plan**.
> **Cấm:** viết số chưa chạy lệnh · nở phạm vi · commit/push khi user chưa cho phép.

**Goal:** Mỗi việc song song có thư mục riêng (git worktree), model/provider riêng, DB và log riêng, sub-agent riêng; các việc không đụng file của nhau.

**Architecture:** Đơn vị cô lập là **một thư mục**. Lý do: tool (`bash`, `edit`…) chạy trong thư mục của workspace, và model là thiết lập của workspace (`coordinator`, do phiên khác đang sửa). `agent-cli-tmux new` tạo git worktree cho mỗi việc, ghi model riêng vào `.crush/crush.json` của worktree (scope workspace), rồi mở một window tmux. Plan 1 (Ruling R9) đã cho mỗi thư mục một server riêng, nên log và DB tách sẵn. TUI ghi model vào scope workspace, nên đổi model ở việc này không đổi model của việc khác. `agent-cli logs` đọc đúng log của server dự án.

**Tech Stack:** bash, git worktree, tmux, Go 1.27.

**Spec:** yêu cầu R9 của user (14/10/2026), xem plan 1 `2026-10-14-multi-session-multi-project.md` mục "Yêu cầu" và Ruling R9. Plan này chạy **sau** plan 1, vì nó dùng `scripts/agent-cli-tmux.sh` (B1) và wrapper chọn host theo dự án (B0).

## Sự thật nền (đọc code 14/10, chưa chạy)

- Model chọn trong TUI luôn ghi `config.ScopeGlobal`: 5 chỗ `UpdatePreferredModel(config.ScopeGlobal, …)` trong `internal/ui/model/ui.go`. Hậu quả: đổi model ở một dự án làm đổi model mặc định của mọi dự án.
- `config.ScopeWorkspace` ghi vào `.crush/crush.json` của thư mục dự án (`internal/config/scope.go`). Thư mục `.crush/` có `.gitignore` chứa `*`, nên không lọt vào git.
- Log được thiết lập một lần cho cả process (`internal/log/log.go`, `sync.Once`):
  - Chế độ local ghi vào `<data dir>/logs/crush.log`.
  - Chế độ server ghi vào `GlobalCacheDir()/server-<safeHostName(host)>/crush.log` (`internal/cmd/server.go:50`).
  - `agent-cli logs` (`internal/cmd/logs.go:58`) luôn đọc đường dẫn của chế độ local, nên ở chế độ server nó đọc nhầm file.
- **Không làm:** model riêng cho từng session **trong cùng một thư mục**. Việc đó cần đổi `coordinator.go`, nơi phiên khác đang sửa; dùng worktree thay thế. Log lọc theo session cũng không làm, vì phần lớn dòng log không có `session_id`.

## Global Constraints

- Như plan 1: không sửa `internal/agent/coordinator.go`, `internal/config/config.go`, `internal/agent/tools/provider/`. Lệnh Go chạy `GOMAXPROCS=2 -p 2`. Không commit.
- Script không bao giờ xoá branch và không bao giờ xoá worktree đang có thay đổi chưa commit.
- Test script dùng repo git tạm và tmux socket riêng (`-L`), không đụng repo hay tmux thật của user.
- Tên việc khớp `^[a-z0-9][a-z0-9-]{0,40}$`.

## Review Focus

1. `new` với tên đã có worktree → chỉ mở lại window, không tạo branch hay worktree mới (test `new-twice`).
2. `rm` khi worktree có file sửa dở → từ chối với rc=3, worktree còn nguyên (test `rm-dirty`).
3. `--model` thiếu dấu `/` → rc=2, không tạo gì (test `bad-model`).
4. Chọn model trong TUI ở worktree A không đổi model của worktree B (test `TestModelSelectionWritesWorkspaceScope`).
5. `agent-cli logs` ở chế độ server đọc log của server dự án (test `TestLogFilePathClientServer`).

## Cách chứng minh là xong

```bash
bash scripts/test-agent-cli-tmux.sh        # in PASS, rc=0
GOMAXPROCS=2 go test -p 2 ./internal/ui/model ./internal/cmd -count=1
```

Kiểm tay: chạy `agent-cli-tmux new fix-a --model <provider>/<id>` và `agent-cli-tmux new fix-b --model glm/<id>`. Kỳ vọng: 2 window, mỗi window hiện đúng model của nó; sửa file ở fix-a thì `git -C <wt fix-b> status` sạch; `agent-cli logs` ở mỗi window cho log khác nhau.

---

### Task C1: `agent-cli-tmux new | rm | ls` — mỗi việc một worktree

**Files:** Modify `scripts/agent-cli-tmux.sh` (B1 đã tạo), `scripts/test-agent-cli-tmux.sh`, `docs/agent-cli-local-setup.md`.

**Interfaces:**

- **`agent-cli-tmux new <name> [--model <provider>/<model>] [--repo <dir>] [--base <ref>]`**
  - `repo` = `git rev-parse --show-toplevel` của `--repo` hoặc `$PWD`. Không phải repo git → rc=2.
  - Worktree ở `<cha của repo>/<tên repo>-agents/<name>`, trên branch `agent/<name>` tạo từ `--base` (mặc định `HEAD`).
  - Worktree đã tồn tại thì chỉ mở window. Branch đã có mà chưa có worktree thì `git worktree add <path> agent/<name>`.
  - `--model`: ghi hoặc gộp `{"models":{"large":{"provider":P,"model":M}}}` vào `<wt>/.crush/crush.json` bằng `jq`, tạo `<wt>/.crush/.gitignore` chứa `*` nếu chưa có. Tách P và M ở dấu `/` đầu tiên; không có `/` thì rc=2.
  - Sau đó mở window như `agent-cli-tmux <dir>`. Window đặt tên `<name>`.
- **`agent-cli-tmux rm <name> [--repo <dir>]`**
  - `git -C <wt> status --porcelain` khác rỗng → in lý do, rc=3, không xoá gì.
  - Sạch thì `git worktree remove <wt>`, kill window `<name>` nếu có, và in `branch agent/<name> kept`.
- **`agent-cli-tmux ls [--repo <dir>]`**: mỗi worktree dưới `<tên repo>-agents/` một dòng `<name>  <branch>  clean|dirty`.

- [ ] **Step 1:** Thêm vào `scripts/test-agent-cli-tmux.sh` (repo git tạm có 1 commit, stub `agent-cli`, socket `-L agentcli-test`) các ca:
  - `new-basic`: có worktree và branch, stub nhận `-c <wt>`.
  - `new-model`: `.crush/crush.json` có đúng provider và model.
  - `new-twice`: không có branch hay window thừa.
  - `bad-name`: rc=2.
  - `bad-model`: rc=2, không có worktree.
  - `not-git`: rc=2.
  - `rm-dirty`: rc=3, worktree còn.
  - `rm-clean`: worktree mất, branch còn.
  - `ls`: có dòng `<name>  agent/<name>  clean`.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy lại, kỳ vọng `PASS` và rc=0. Cập nhật docs (cách dùng, nơi đặt worktree, cách dọn).

### Task C2: Chọn model trong TUI ghi theo dự án

**Files:** Modify `internal/ui/model/ui.go` (5 chỗ `UpdatePreferredModel(config.ScopeGlobal, …)`); Test `internal/ui/model/session_busy_test.go` (`countingWorkspace` ghi lại `scope`), `docs/agent-cli-local-setup.md`.

**Interfaces:** Cả 5 chỗ đổi sang `config.ScopeWorkspace`. Dự án chưa từng chọn model thì vẫn dùng model mặc định toàn cục. Không đổi gì ở `config`.

- [ ] **Step 1:** Viết `TestModelSelectionWritesWorkspaceScope`: `handleSelectModel` với một model hợp lệ phải gọi `UpdatePreferredModel` với `config.ScopeWorkspace` cho large (và small nếu có). Chạy, kỳ vọng FAIL.
- [ ] **Step 2:** Đổi 5 chỗ. Chạy `GOMAXPROCS=2 go test -p 2 ./internal/ui/model -count=1`, kỳ vọng PASS toàn bộ. Ghi docs: model chọn trong TUI giờ thuộc về dự án; muốn đổi mặc định toàn cục thì sửa `~/.config/agent-cli/crush.json`.

### Task C3: `agent-cli logs` đọc đúng log ở chế độ server

**Files:** Modify `internal/cmd/logs.go`; Test `internal/cmd/logs_test.go` (tạo mới nếu chưa có).

**Interfaces:**
- Produces `func logFilePath(dataDir string, clientServer bool, host string) string`. Khi `clientServer` là false: trả `<dataDir>/logs/crush.log`. Khi true: trả `GlobalCacheDir()/server-<safeHostName(host)>/crush.log`, cùng công thức với `internal/cmd/server.go:50`; dùng chung một hàm, không chép công thức.
- `logs` gọi hàm này với `useClientServer()` và `clientHost`.

- [ ] **Step 1:** Viết `TestLogFilePathLocal` và `TestLogFilePathClientServer`, trong đó hai host khác nhau cho hai đường dẫn khác nhau. Chạy, kỳ vọng FAIL.
- [ ] **Step 2:** Cài đặt, `server.go` dùng chung hàm này. Chạy `GOMAXPROCS=2 go test -p 2 ./internal/cmd -count=1`, kỳ vọng PASS.

---

## Thứ tự

Plan 1 (A1 → … → B2) → C1 → C2 → C3. C2 sửa `ui.go` nên chạy sau toàn bộ Phần A của plan 1.
