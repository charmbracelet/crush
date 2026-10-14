# agent-cli — tính năng thật sự cần cho cách làm việc của Luân (14/10/2026)

**Đầu vào:** bảng so sánh Codex / Claude Desktop do user dán (viết từ README), được đối chiếu lại với code thật ở `.superpowers/sdd/feature-gap-verified.md` (22 dòng, mỗi dòng có file:dòng). Kết quả: có 4, có một phần 6, chưa có 12. Bảng dán sai ở 3 chỗ:
- Plan mode **đã có**: Shift+Tab chuyển sang agent `plan`, agent này bị gỡ hết tool sửa và bash.
- Theme **đầy đủ hơn** bảng nói: có trình sửa ngay trong TUI và file JSON có kế thừa.
- Hook **chỉ có `PreToolUse`**.

**Tiêu chí chọn:** cách làm việc của user, rút từ CLAUDE.md và các yêu cầu trong phiên:
- Nhiều session, provider, dự án và workflow chạy song song.
- Agent và script điều khiển được bằng dòng lệnh.
- Người chấm khác người làm.
- Cổng kiểm do máy ép (hook `Stop` chặn khi lint đỏ), không phải lời dặn.
- Routine phải máy móc, kiểm được, lặp vô tận.
- Không commit khi chưa được phép.
- Model local (Qwen) cộng GLM.

## Đã có kế hoạch (plan 1, plan 2)

| Nhu cầu | Ở đâu |
|---|---|
| Nhiều session song song, panel trạng thái, nhảy session bằng phím, prompt của session nền | Plan 1 Phần A |
| Nhiều dự án trong một cửa sổ, mỗi dự án một server, `agent-cli ps` | Plan 1 Phần B |
| Mỗi việc một worktree, model/provider và log riêng | Plan 2 (C1–C3) |

## Cần thật, đang thiếu — đề xuất làm (plan 3)

| # | Tính năng | Vì sao user cần | Bằng chứng thiếu | Chi phí / rủi ro |
|---|---|---|---|---|
| P1 | **Hook `PostToolUse` và `Stop`**, tương thích payload Claude Code | Cổng do máy ép là luật lõi của user. `Stop` chặn agent báo "xong" khi lint hoặc test đỏ; `PostToolUse` ghi lại các file đã đụng. Script hook có sẵn trong `~/.claude/hooks` dùng lại được | Chỉ có `EventPreToolUse` (`internal/hooks/hooks.go:15`; `docs/hooks/README.md:179`: "there's currently just one") | Vừa. `PostToolUse` nằm trong `hooked_tool.go`. `Stop` cần nối vào cuối vòng chạy của agent; nếu phải đụng `coordinator.go` thì chờ phiên kia xong |
| P2 | **`agent-cli run --json`** (trả `session_id`, `text`, `error`, `usage`, `exit code`) | Hợp đồng dòng lệnh cho workflow: script hoặc agent điều phối giao việc, lấy session ID, rồi tiếp tục bằng `-s`, mỗi lần chạy một model (`-m`). Đây là nền của "multi workflow" | `run.go` không có cờ `--json` (chỉ có `quiet`, `verbose`, `model`, `session`, `continue`…) | Nhỏ, chỉ `internal/cmd/run.go` |
| P3 | **`agent-cli send`**: gửi prompt vào một session đang mở của dự án khác, qua server của dự án đó, có `--wait` và `--json` | Một agent hoặc script điều phối nhiều dự án đang chạy (đang mở trong tmux) mà không phải mở process mới; kết quả vẫn hiện trong TUI của dự án đó | Client đã có `SendMessage` và `SubscribeEvents` (`internal/client/proto.go:507,114`), nhưng chưa có lệnh CLI | Vừa. Lệnh mới + chờ `RunComplete` qua luồng sự kiện |
| P4 | **Subagent tự định nghĩa** từ `.claude/agents/*.md` (frontmatter name, description, tools, model), ví dụ một agent soát **không có quyền sửa** | Luật "người chấm ≠ người làm". User đã có 6 agent trong `~/.claude/agents` | `Config.Agents` là `json:"-"`, cố định 3 agent (`config.go:861,1103`); tool `agent` chỉ nhận `{prompt}` | Lớn. Cần sửa `config.go` và `coordinator.go`, mà phiên khác đang sửa → **chỉ làm sau khi phần đó của phiên kia xong** |
| P5 | **Đọc `.claude/commands/*.md`** làm slash command | Dùng lại lệnh có sẵn của Claude Code | `commands.go:121-136` chỉ đọc thư mục của crush | Nhỏ |

## Dùng hằng ngày — cấu hình, không cần code

- **Luật chung của user không áp vào agent-cli.** Mặc định agent-cli chỉ đọc `~/.config/agent-cli/CRUSH.md` và `~/.config/AGENTS.md` (`internal/config/load.go` `setDefaults`), không đọc `~/.claude/CLAUDE.md`. Đề xuất: đặt `options.global_context_paths` trỏ thêm vào `~/.claude/CLAUDE.md`, hoặc symlink `~/.config/agent-cli/CRUSH.md`. Chờ user quyết, vì thay đổi này đổi hành vi của mọi phiên.
- **Dự án gần đây đã có sổ** (`internal/projects`, `LastAccessed`), nên bộ chuyển (plan 1 A8b) mở được dự án chưa mở.

## Không làm bây giờ (và vì sao)

- **Auto-review / classifier duyệt quyền:** với model local thì tốn và chậm. Hook `PreToolUse` có allow-list đã thay được, và P1 làm hook mạnh hơn.
- **Lịch chạy (scheduled) và goal mode:** routine của user đã chạy bằng cron và Claude Code. P2 cho phép cron gọi `agent-cli run --json` là đủ.
- **Fork hội thoại, side chat:** side chat đã thay được bằng `ctrl+n` + `alt+N` của plan 1. Fork là thứ có cũng được.
- **Comment theo dòng trong diff, theo dõi PR/CI, plugin, memory xuyên session, browser, computer use:** xa nhu cầu hiện tại. Riêng browser và computer use thì gắn được qua MCP khi cần.

## Thứ tự đề xuất

Plan 1 → plan 2 → **P2 → P1 → P3 → P5** → P4 (khi phiên kia xong phần config/coordinator).
