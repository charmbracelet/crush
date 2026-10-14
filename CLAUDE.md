# CLAUDE.md — agent-cli: thông tin dự án và quy tắc làm việc

> Phần CORE là quy tắc chung (giữ nguyên theo bản nguồn). Phần PROJECT điền theo mã nguồn và tài liệu của repo này, cập nhật 14/10/2026.
> agent-cli cũng tự nạp tệp này làm ngữ cảnh khi chạy trong repo này (AGENTS.md, mục Context files).

## CORE — Quy tắc làm việc

- Giải đúng bài bằng cách đơn giản đủ dùng; giữ yêu cầu và quyết định sản phẩm đã chốt.
- Theo luật hệ thống/môi trường và quyền chủ dự án cấp. P2 khai thứ tự áp dụng: hướng dẫn repo theo phạm vi → skill repo → base được phép → mặc định template, trừ khi nguồn cấp trên quy định khác.
- Base chỉ đọc khi làm task dự án. Skill chuyên biệt ở trong repo, kiểm đích thực trước ghi; tạo/cập nhật trong scope đã giao, phần ngoài scope thì đề xuất. Nguồn bắt buộc thiếu thì báo; phần độc lập vẫn làm.
- Làm rõ mục tiêu, scope và tiêu chí xong; khai giả định, phân biệt dữ kiện/mục tiêu/đề xuất. Dùng nguồn thật, không bịa kết quả, path hoặc cổng đã hoạt động.
- Trước task đọc nguồn phù hợp trong P7. Khi đổi hành vi, kiểm caller/contract/test/docs/comment; kiểm regression và đồng bộ phần bị lệch.
- Identifier EN, mô tả/docs/comment VI, giữ keyword kỹ thuật. Theo convention repo, giữ thay đổi của người khác; ưu tiên reuse/stdlib/platform.
- Chọn Haiku cho việc cơ học rõ, Sonnet cho việc thường ngày, Opus cho suy luận khó; verify/edge case dùng high theo gợi ý đã khai ở P2. Môi trường quyết cấu hình thực tế; effort không thay spec, quyền hay bằng chứng.
- Chạy kiểm tra theo rủi ro và cổng repo; bộ lọc kiểm cả cho qua và từ chối. Không nới cổng/test để che lỗi hoặc ghi secret vào tài liệu.
- Review, commit/MR/merge/deploy theo scope và cổng hiện hành. Bàn giao thay đổi, lệnh/kết quả, giới hạn và việc chưa kiểm chứng; không coi tự review là soát độc lập.

## PROJECT

### P1. Mục tiêu sản phẩm và quyền thực hiện

- **Bài toán:** fork cá nhân của Crush (TUI coding agent bằng Go). User làm FE/BE/devops, cần làm nhiều session và nhiều dự án song song trong một terminal, với model local hoặc provider qua API key. ĐÃ XÁC MINH: `AGENTS.md`, roadmap.
- **Giai đoạn:** Phase 1 core MVP. Thứ tự giao hàng ở `docs/superpowers/plans/2026-10-14-core-mvp-lat1-lat2.md`: lát 1 (Home, 4 phím Alt, run sống khi đóng TUI, vá quyền bash, cài) → lát 2 (draft riêng từng session, câu hỏi xin quyền của session nền).
- **Thành công:** đạt cổng WIN ở roadmap mục 5. Nghĩa là user đi trọn daily workflow trên launcher thật với provider thật. Unit test xanh chưa đủ.
- **Ngoài phạm vi:** Phase 2–5; remote SSH picker; GUI/RAG/team; dashboard chi phí; switcher Alt+Tab, phím index, framework keybinding mới, mode quyền mới (đã DEFER).
- **Quyền agent:**
  - Mặc định THỰC HIỆN trong worktree.
  - KHÔNG commit/push khi user chưa cho phép; lời cho phép chỉ áp dụng đúng lần đó.
  - Không sửa tệp mà phiên khác đang sửa.
  - Giao sớm theo lát; mỗi lát cài kèm bản sao lưu và lệnh quay lại.

### P2. Nguồn luật và quyết định

| Nguồn | Phạm vi | Ghi chú |
|---|---|---|
| `~/.claude/CLAUDE.md` | BASE, chỉ đọc | Luật lõi của user: đòi lệnh, người chấm ≠ người làm, đo đừng đoán |
| `AGENTS.md`, `internal/ui/AGENTS.md` | REPO | Hướng dẫn dev upstream; bắt buộc đọc file UI trước khi sửa TUI |
| `docs/superpowers/plans/2026-10-14-agent-cli-personal-roadmap.md` | REPO | Thắng các plan cũ khi mâu thuẫn |
| `docs/superpowers/plans/2026-10-14-core-mvp-lat1-lat2.md` | REPO | Plan hiện hành, kèm đối chiếu file feedback UI |
| `docs/superpowers/plans/2026-10-14-multi-session-multi-project.md`, Phần K | REPO | Bảng KEEP/DISABLE/DEFER/REMOVE; các task cũ A6, A8, A9, A10 đã DEFER |
| `docs/reviews/2026-10-14-phase1-ui-feedback.md` | REPO | Danh sách đề nghị; ACCEPT/DEFER/REJECT ghi trong plan core MVP |
| `.superpowers/sdd/2026-10-14-multi-session-multi-project/` | REPO, bị `.git/info/exclude` loại | Ledger `progress.md`, brief, report, diff review, `go.sh`, `preview-install.sh`, luật implementer/reviewer |

- **Base skill** (chỉ đọc): superpowers, ponytail, code-en-doc-vi, glossary-keeper, retro, superpower. Chưa có skill chuyên biệt trong repo (`.claude/skills/` không tồn tại).
- **Model:**
  - Sonnet cho UI và task nhỏ.
  - Opus cho code an ninh, đồng thời và vòng đời run, cùng lượt soát chúng.
  - Không dùng mutant, không giao lượt soát lại, trừ code an ninh.
- **Changelog:** repo chưa có `CHANGELOG`, ADR hay `GLOSSARY.md`. Tiến độ ghi ở `docs/progress/PHASE1_STATUS.md` (chỉ agent điều phối ghi).
- **Mâu thuẫn còn mở:** cây chính có thay đổi của một phiên khác (tool provider `claude_code`/`codex`), worktree có phần multi-session; chỉ gộp ở bước cài, không sửa cây chính.

### P3. Kiến trúc và bản đồ repo

- **Stack:** Go 1.27.0 (`go.mod`), Bubble Tea v2, lipgloss, SQLite + sqlc, cobra, `charm.land/fantasy` cho provider. Module `github.com/charmbracelet/crush`. ĐÃ XÁC MINH.
- **Đường vào:** `main.go` → `internal/cmd/root.go`. Bản đồ thư mục đầy đủ ở `AGENTS.md`, mục Architecture.
- **Invariant:**
  - Tên hiển thị là agent-cli. Module path, `crush.json`, biến `CRUSH_*` và system prompt giữ chữ Crush, để merge upstream rẻ.
  - Không làm IO trong `Update`/`View` của TUI (ghi đầu `internal/ui/model/workspace_cache.go`).
  - Sửa SKILL.md builtin, template prompt hoặc mô tả tool là đổi system prompt, và sẽ làm đỏ cassette VCR của `TestCoderAgent`.
- **Worktree code hiện hành:** `/home/luannt/application/opensource/agent-cli-wt/multi-session` (HEAD `a73e1bfa`, chưa commit). Cây chính `/home/luannt/application/opensource/agent-cli` chỉ dùng để đọc docs và tích hợp.

### P4. Luồng chính

| Luồng | Trigger | Xử lý → kết quả | Lỗi/quyền quan trọng | Nguồn |
|---|---|---|---|---|
| Khởi động | Lệnh `agent-cli` (wrapper) | Wrapper đặt env, mỗi dự án một server (`--host` socket theo hash), rồi vào TUI. Home khi không có tham số là MỤC TIÊU, đang làm (H2) | Không tự dựng server chỉ để hiển thị Home | `scripts/install-agent-cli.sh`, `internal/cmd/root.go`, `internal/ui/home/` |
| Prompt → run → tool | Gửi tin trong TUI, `run`, `send` | Coordinator → session agent → tool → permission. Hook PreToolUse/Stop là opt-in | Bash tự duyệt theo tiền tố, chỉ khi không có ký tự nối lệnh/chuyển hướng | `internal/agent/`, `internal/agent/tools/safe.go`, `bash.go`, `internal/permission/` |
| Vòng đời server/workspace | Client rời hoặc gắn lại | Còn run thì hoãn teardown, kiểm lại mỗi 5s, trần mồ côi 30 phút; init không thay coordinator khi còn run | Ctrl+C trên `run` chỉ huỷ session do chính nó tạo | `internal/backend/backend.go`, `agent.go`, `internal/cmd/run.go` |
| CLI cho script | `run --json`, `send --wait`, `ps --all`, `logs` | JSON một object trên stdout; rc 0 = xong, 1 = lỗi chạy, 2 = sai cú pháp | `send --wait` + Ctrl+C chỉ thôi chờ, không huỷ run | `internal/cmd/{run,send,ps,logs}.go` |
| Cấu hình provider | Nạp crushrc/crush.json + env của launcher | Biến `AGENT_CLI_*` dùng chung; crush.json chỉ expand `api_key`/`base_url` | Không ghi secret vào docs hay log | `agent-cli.crushrc.example`, `agent-cli.env.example`, `internal/config/load.go`, `docs/agent-cli-local-setup.md` |

### P5. Lệnh và môi trường kiểm tra

| Mục đích | Thư mục | Lệnh | Thực thi | Kết quả / giới hạn |
|---|---|---|---|---|
| Mọi lệnh Go (bắt buộc) | worktree | `$SDD/go.sh <go args>`, với `SDD=.superpowers/sdd/2026-10-14-multi-session-multi-project` | ĐÃ CHẠY | Toàn máy tối đa 2 lệnh Go cùng lúc; chờ RAM trống ≥3000MB; rc=75 thì dừng, không lách. Đã thử phá: chặn được |
| Test một package | worktree | `$SDD/go.sh test ./internal/<pkg> -count=1` | ĐÃ CHẠY | Không chạy `./...`; `-race` chỉ 1 lần ở cuối |
| Cassette agent | worktree (phải là cây git) | `$SDD/go.sh test ./internal/agent -run TestCoderAgent -count=1` | ĐÃ CHẠY | Ngoài cây git, `load.go` giới hạn độ sâu của ls nên test đỏ giả |
| E2E run/send | worktree | `bash scripts/test-agent-cli-run-json.sh` | Worker báo ĐÃ CHẠY | Dựng binary và server thật với fake LLM; chạy 1 lần ở cuối task |
| Build + cài preview | bất kỳ | `DRY=1 bash $SDD/preview-install.sh`, rồi chạy lại không có `DRY` | CHƯA CHẠY | Gộp 2 cây → build → sao lưu `~/.local/lib/agent-cli/backup-<ts>` → cài; in lệnh quay lại |
| Lint | worktree | `task lint` | CHƯA CHẠY | |

- **Máy:** luannt-pc, 12 lõi, 15,9GB RAM, 8GB swap, có earlyoom. Tối đa 3 agent cùng lúc.

### P6. Cổng, phát hành và vận hành

- **CI:** `.github/workflows/` có build, lint, security… của upstream; CHƯA XÁC MINH là có chạy cho fork này.
- **Cổng local:** không commit/push khi chưa được phép. Code an ninh và vòng đời qua một lượt soát độc lập, người soát không có quyền Edit.
- **Phát hành = cài local:**
  - Binary ở `~/.local/lib/agent-cli/crush`, wrapper ở `~/.local/bin/agent-cli`.
  - Smoke sau khi cài: `agent-cli --version`, rồi một prompt thật (không in secret).
  - Quay lại: lệnh mà `preview-install.sh` in ra.
- **Log:**
  - Chế độ server: `~/.cache/agent-cli/server-<host>/crush.log`.
  - Chế độ local: `<data dir>/logs/crush.log`.
  - Đọc bằng `agent-cli logs`.
- **Môi trường:** chỉ LOCAL | `scripts/install-agent-cli.sh` | cấu hình ở `~/.config/agent-cli/`, dữ liệu ở `~/.local/share/agent-cli/`, cache ở `~/.cache/agent-cli/` | ĐÃ XÁC MINH 15:16.

### P7. Định tuyến docs, skill và mã mẫu

| Loại task | Đọc trước | Mã mẫu / convention | Ghi chú |
|---|---|---|---|
| Sửa TUI | `internal/ui/AGENTS.md` | `internal/ui/model/workspace_cache.go` (probe qua `tea.Cmd`) | `ui.go` dài khoảng 5.700 dòng: sửa có mục tiêu, đọc lại ngay trước khi sửa |
| Phím tắt | Roadmap P1.3a | `internal/ui/model/keys.go` | Core gồm Alt+W/S/N/M; Ctrl+P là đường phụ |
| Quyền / bash | `internal/agent/tools/*.md` | `safe.go`, `bash_autoapprove_test.go` (permission luôn từ chối) | Mọi thay đổi tự duyệt phải có test chứng minh vẫn hỏi quyền |
| Hook | `docs/hooks/README.md` | `internal/agent/stop_hook.go` | Không sửa dòng `description:` của SKILL.md builtin |
| Cấu hình / launcher | `docs/agent-cli-local-setup.md` | `scripts/install-agent-cli.sh` + `scripts/test-agent-cli-wrapper.sh` | Không đụng `~/.config/agent-cli` thật của user |
| Giao việc cho agent | `$SDD/implementer-rules.md`, `reviewer-rules.md` | brief dạng `task-*-brief.md` | Brief ngắn: mục tiêu, ràng buộc, tiêu chí xong |

### P8. Điểm chưa xác minh

| Điểm | Tác động | Cách kiểm | Bị chặn |
|---|---|---|---|
| Chưa gọi provider thật trên bản mới | Cổng WIN | Chạy 1 prompt sau khi cài preview | Lát 1 |
| Hook Stop vòng 1b bị reboot cắt giữa chừng | Hành vi giữ RunComplete khi có prompt xếp hàng chưa được soát lại | `$SDD/go.sh test ./internal/agent -run StopHook` ĐÃ CHẠY 14/10 → ok. Hook là opt-in | Không |
| Commit `d73a6e42` (revert luật bash) thuộc upstream hay fork | Khi merge upstream có thể mất bản vá S1 | `git fetch upstream` rồi `merge-base --is-ancestor` | Không |
| Chi phí 2 tool `claude_code`/`codex` trên mọi request | Thêm token cho mỗi request | Đo độ dài request có và không có 2 tool | Không |
| Phím Alt thật trong terminal/tmux; Esc/Enter trong picker | Acceptance P1.3a | Chạy tmux với socket riêng, `send-keys M-s` | Lát 1 |

## Bảo trì

- Cập nhật PROJECT khi hành vi/lệnh/ràng buộc đổi; đồng bộ caller và docs/comment liên quan trong phạm vi được giao.
- Chỉ cập nhật mục bị lệch: kiến trúc/đường vào → P3; luồng → P4; lệnh/phụ thuộc chạy → P5; môi trường/vận hành → P6; nguồn luật/changelog → P2; docs/skill/mã mẫu → P7. Sửa nội bộ không đổi các thông tin này thì giữ CLAUDE.md.
- Giữ CORE theo bản nguồn đã áp dụng (`agent-environment/project-local/template/CLAUDE.template.md`), chỉ đổi khi chủ dự án yêu cầu. Dẫn nguồn thay chép dài; không dùng CLAUDE.md làm backlog hoặc nhật ký.
