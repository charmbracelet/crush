# CLAUDE.md — agent-cli: thông tin dự án và quy tắc làm việc

> Phần CORE là quy tắc chung (giữ nguyên theo bản nguồn). Phần PROJECT điền theo mã nguồn và tài liệu của repo này, cập nhật 15/10/2026.
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

- **Base skill** (chỉ đọc): superpowers, ponytail, code-en-doc-vi, glossary-keeper, retro, superpower. Chưa có skill chuyên biệt trong repo (`.claude/skills/` không tồn tại).
- **Model:**
  - Sonnet cho UI và task nhỏ.
  - Opus cho code an ninh, đồng thời và vòng đời run, cùng lượt soát chúng.
  - Không dùng mutant, không giao lượt soát lại, trừ code an ninh.
- **Changelog:** repo chưa có `CHANGELOG`, ADR hay `GLOSSARY.md`; trạng thái kiểm chứng ghi ở P8. Ledger `.superpowers/sdd/...` (`go.sh`, `preview-install.sh`, luật implementer/reviewer) và `docs/progress/` của đợt Phase 1 chưa từng vào git và đã mất khỏi máy ngày 14/10; tài liệu trong `docs/` còn nhắc tới chúng là lịch sử, đừng làm theo.
- **Đã gộp (14/10):** nhánh `feat/core-mvp-phase1` và `feat/personal-rename-providers` vào `main`, kèm đợt sửa sau audit độc lập.

### P3. Kiến trúc và bản đồ repo

- **Stack:** Go 1.27.0 (`go.mod`), Bubble Tea v2, lipgloss, SQLite + sqlc, cobra, `charm.land/fantasy` cho provider. Module `github.com/charmbracelet/crush`. ĐÃ XÁC MINH.
- **Đường vào:** `main.go` → `internal/cmd/root.go`. Bản đồ thư mục đầy đủ ở `AGENTS.md`, mục Architecture.
- **Invariant:**
  - Tên hiển thị là agent-cli. Module path, `crush.json`, biến `CRUSH_*` và system prompt giữ chữ Crush, để merge upstream rẻ.
  - Không làm IO trong `Update`/`View` của TUI (ghi đầu `internal/ui/model/workspace_cache.go`).
  - Sửa SKILL.md builtin, template prompt hoặc mô tả tool là đổi system prompt, và sẽ làm đỏ cassette VCR của `TestCoderAgent`.
- **Mã:** cây chính `/home/luannt/application/opensource/agent-cli`, nhánh `main`. Worktree `agent-cli-wt/` cũ không còn.

### P4. Luồng chính

| Luồng | Trigger | Xử lý → kết quả | Lỗi/quyền quan trọng | Nguồn |
|---|---|---|---|---|
| Khởi động | Lệnh `agent-cli` (wrapper) | Wrapper đặt env, mỗi dự án một server (`--host` socket theo hash), rồi vào TUI. Home khi không có tham số là MỤC TIÊU, đang làm (H2) | Không tự dựng server chỉ để hiển thị Home | `scripts/install-agent-cli.sh`, `internal/cmd/root.go`, `internal/ui/home/` |
| Prompt → run → tool | Gửi tin trong TUI, `run`, `send` | Coordinator → session agent → tool → permission. Hook PreToolUse/Stop là opt-in | Bash tự duyệt theo tiền tố, chỉ khi không có ký tự nối lệnh/chuyển hướng | `internal/agent/`, `internal/agent/tools/safe.go`, `bash.go`, `internal/permission/` |
| Vòng đời server/workspace | Client rời hoặc gắn lại; server nhận SIGTERM/SIGHUP | Còn run thì hoãn teardown, kiểm lại mỗi 5s; trần mồ côi 30 phút khi kẹt chờ duyệt/trả lời, 6 giờ khi vẫn chạy; init không thay coordinator khi còn run. Server bị dừng thì dừng từng workspace (ghi lại việc dở); tool call mà tiến trình chết bỏ dở được ghi "Interrupted" khi mở session | Ctrl+C trên `run` huỷ đúng lượt của nó theo RunID (prompt xếp hàng phía sau vẫn chạy) | `internal/backend/backend.go`, `agent.go`, `internal/cmd/run.go` |
| CLI cho script | `run --json`, `send --wait`, `ps --all`, `logs`, `jobs` | JSON một object trên stdout; rc 0 = xong, 1 = lỗi chạy, 2 = sai cú pháp | `send --wait` + Ctrl+C chỉ thôi chờ, không huỷ run | `internal/cmd/{run,send,ps,logs,jobs}.go` |
| Giao việc Claude Code/Codex | Model chính gọi tool `claude_code`/`codex` | Mặc định TẮT (`internal/execpolicy`, đọc env đã export rồi tệp env, không qua cấu hình dự án): chỉ đăng ký khi `AGENT_CLI_DELEGATION=on`, backend có model cụ thể trong `AGENT_CLI_ALLOWED_MODELS` và có CLI; luôn truyền model; hỏi quyền mỗi lần; CLI con chạy headless, trả session/thread ID để `resume`. Danh sách có provider chính thì model chính/nhỏ ngoài danh sách bị chặn | `run` tự duyệt; mức quyền CLI con: `AGENT_CLI_CLAUDE_PERMISSION_MODE`, `AGENT_CLI_CODEX_SANDBOX` (export ở shell) | `internal/agent/tools/provider/`, `docs/agent-cli-local-setup.md` |
| Cấu hình provider | Nạp crushrc/crush.json + env của launcher | Biến `AGENT_CLI_*` dùng chung; crush.json chỉ expand `api_key`/`base_url` | Không ghi secret vào docs hay log | `agent-cli.crushrc.example`, `agent-cli.env.example`, `internal/config/load.go`, `docs/agent-cli-local-setup.md` |

### P5. Lệnh và môi trường kiểm tra

| Mục đích | Lệnh (chạy ở gốc repo) | Thực thi | Kết quả / giới hạn |
|---|---|---|---|
| Mọi lệnh Go | `taskset -c 0,2,3,5-8,10,11,13-15 go <args>` | ĐÃ CHẠY 14/10 | Bỏ nhân CPU lỗi 1, 4 (SMT 9, 12). Một lệnh Go nặng một lúc |
| Test toàn kho | `taskset -c 0,2,3,5-8,10,11,13-15 go test ./... -count=1 -p 4` | ĐÃ CHẠY 14/10 | 65 package ok, khoảng 45 giây. `go vet` chỉ báo `internal/csync/maps.go:156` (mã upstream) |
| Race | `taskset -c 0,2,3,5-8,10,11,13-15 go test -race -count=1 -p 2 ./internal/backend/... ./internal/agent/... ./internal/cmd/... ./internal/ui/... ./internal/workspace/... ./internal/projects/... ./internal/permission/... ./internal/server/...` | ĐÃ CHẠY 14/10 | 0 data race |
| Cassette agent | `taskset -c 0,2,3,5-8,10,11,13-15 go test ./internal/agent -run TestCoderAgent -count=1` | ĐÃ CHẠY 14/10 | Phải chạy trong cây git; ngoài cây git `load.go` giới hạn độ sâu ls nên đỏ giả |
| E2E run/send | `bash scripts/test-agent-cli-run-json.sh` | xem P8 | Binary + server thật với LLM giả |
| Launcher + installer | `bash scripts/test-agent-cli-wrapper.sh` | ĐÃ CHẠY 14/10 | PASS; cài vào thư mục tạm, không đụng bản cài thật |
| Giao việc thật | `bash scripts/test-agent-cli-delegation.sh` | ĐÃ CHẠY 14/10 | Claude Code + Codex THẬT, LLM chính giả; tốn hạn mức thuê bao; mặc định mức quyền an toàn |
| Lint | `task lint` | CHƯA CHẠY | |

- **Máy:** luannt-pc, Ryzen 7 9700X 8 nhân/16 luồng (nhân 1, 4 lỗi L1D), RAM 60 GiB, swap 2 GiB (đo 14/10). Tối đa 3 agent cùng lúc.
- Phiên Claude Code chạy test giao việc phải bỏ biến `CLAUDECODE`, `CLAUDE_CODE_*` (script dùng `env -i`) để `claude` con chạy như ở terminal thường.

### P6. Cổng, phát hành và vận hành

- **CI:** `.github/workflows/` có build, lint, security… của upstream; CHƯA XÁC MINH là có chạy cho fork này.
- **Cổng local:** không commit/push khi chưa được phép. Code an ninh và vòng đời qua một lượt soát độc lập, người soát không có quyền Edit.
- **Phát hành = cài local:**
  - Binary ở `~/.local/lib/agent-cli/crush`, wrapper ở `~/.local/bin/agent-cli`.
  - Smoke sau khi cài: `agent-cli --version`, rồi một prompt thật (không in secret).
  - Quay lại: lệnh `cp -p` mà `scripts/install-agent-cli.sh` in ra (sao lưu ở `~/.local/lib/agent-cli/backup-<thời điểm>/`, giữ 3 bản).
- **Log:**
  - Chế độ server: `~/.cache/agent-cli/server-<host>/crush.log`.
  - Chế độ local: `<data dir>/logs/crush.log`.
  - Đọc bằng `agent-cli logs`.
- **Môi trường:** chỉ LOCAL | `scripts/install-agent-cli.sh` | cấu hình ở `~/.config/agent-cli/`, dữ liệu ở `~/.local/share/agent-cli/`, cache ở `~/.cache/agent-cli/` | ĐÃ XÁC MINH 15:16.

### P7. Định tuyến docs, skill và mã mẫu

| Loại task | Đọc trước | Mã mẫu / convention | Ghi chú |
|---|---|---|---|
| Sửa TUI | `internal/ui/AGENTS.md` | `internal/ui/model/workspace_cache.go` (probe qua `tea.Cmd`) | `ui.go` dài khoảng 5.700 dòng: sửa có mục tiêu, đọc lại ngay trước khi sửa |
| Phím tắt | Roadmap P1.3a | `internal/ui/model/keys.go` | Core gồm Alt+W/S/N/M. Thêm 15/10 (DECISIONS.md): Alt+P chọn provider, Alt+M chỉ hiện model của provider đó, Alt+E chọn effort, Alt+. / Alt+, đổi session. Alt+J mở Jobs (job bash: xem output, dừng có hỏi lại). Alt+Tab bị hệ điều hành chiếm. Ctrl+P là đường phụ |
| Quyền / bash | `internal/agent/tools/*.md` | `safe.go`, `bash_autoapprove_test.go` (permission luôn từ chối) | Mọi thay đổi tự duyệt phải có test chứng minh vẫn hỏi quyền |
| Hook | `docs/hooks/README.md` | `internal/agent/stop_hook.go` | Không sửa dòng `description:` của SKILL.md builtin |
| Cấu hình / launcher | `docs/agent-cli-local-setup.md` | `scripts/install-agent-cli.sh` + `scripts/test-agent-cli-wrapper.sh` | Không đụng `~/.config/agent-cli` thật của user |
| Giao việc cho agent | — | — | Brief ngắn: mục tiêu, ràng buộc, tệp được sửa, lệnh chứng minh xong |
| Tool giao việc Claude Code/Codex | `internal/agent/tools/provider/doc.go` | `delegate.go` (cổng quyền chung), `delegate_test.go` | Cờ CLI phải thử với CLI thật: unit test chỉ so với chính mã |

### P8. Điểm chưa xác minh

| Điểm | Tác động | Cách kiểm | Trạng thái |
|---|---|---|---|
| LLM chính thật (GLM/API) trên bản mới | Cổng WIN | Điền `~/.config/agent-cli/env`, chạy 1 prompt có tool call | ĐÃ 15/10: GLM qua provider `zai` của catalog; `run` trả PONG, một việc code có tool call chạy đúng |
| Giao việc Claude Code/Codex với CLI thật | Lát 1 | `scripts/test-agent-cli-delegation.sh` | ĐÃ 14/10 (local + server): tạo tệp, resume, prompt bắt đầu bằng `-`, mức quyền có hiệu lực |
| Mức quyền mặc định toàn quyền của CLI con | An ninh | Chạy `claude_code`/`codex` không đặt `AGENT_CLI_*_MODE/SANDBOX` | CHƯA chạy thật: bộ phân loại an toàn của Claude Code chặn phiên Claude tự chạy agent toàn quyền; chỉ unit test |
| Job nền có owner; tool call bỏ dở sau khi server chết (P1.4, P1.5) | Cổng WIN | `scripts/test-agent-cli-run-json.sh`; server SIGKILL giữa tool call rồi mở lại | ĐÃ 15/10: `agent-cli jobs` thấy job đang chạy kèm session; mở lại thấy "Interrupted", model nhận lời dặn kiểm trạng thái. Màn Alt+J kiểm trong TUI thật (agent con), người dùng chưa dùng |
| Ranh giới provider/model (`internal/execpolicy`) | An ninh | Unit test + chạy thật với tệp env | ĐÃ 15/10: model ngoài danh sách dừng lúc khởi tạo (`MODEL_NOT_ALLOWED`); lượt `run` không còn tool `claude_code`/`codex` dù CLI có trên máy; GLM chạy với danh sách của người dùng. Giao việc bật (`AGENT_CLI_DELEGATION=on`) chưa chạy thật với CLI thật |
| Hộp thoại xin quyền của `claude_code`/`codex` trong TUI thật | An ninh | Mở TUI, model gọi tool, xem hộp thoại | Unit test đã chứng minh có hỏi và từ chối thì không chạy; chưa xem bằng mắt |
| Commit `d73a6e42` (revert luật bash) thuộc upstream hay fork | Khi merge upstream có thể mất bản vá S1 | `git fetch upstream` rồi `merge-base --is-ancestor` | ĐÃ 15/10: của upstream (revert `1fdb081d`, PR #3565), là tổ tiên của điểm fork `a73e1bfa`; upstream main vẫn dùng safe-list theo tiền tố, bản vá S1 của fork nằm trên nền đó. Nhánh upstream `super-yolo` sẽ thay S1 khi được merge (xem `docs/reviews/2026-10-15-audit-herdr-crush-agent-cli.md`) |
| Phím Alt thật trong terminal/tmux; Esc/Enter trong picker | Acceptance P1.3a | Chạy tmux với socket riêng, `send-keys M-s` | MỘT PHẦN 15/10: Alt+P/M/E, Alt+. / Alt+, (quay vòng hai chiều) đã kiểm trong tmux với bản cài; Alt+W/S/N CHƯA |

## Bảo trì

- Cập nhật PROJECT khi hành vi/lệnh/ràng buộc đổi; đồng bộ caller và docs/comment liên quan trong phạm vi được giao.
- Chỉ cập nhật mục bị lệch: kiến trúc/đường vào → P3; luồng → P4; lệnh/phụ thuộc chạy → P5; môi trường/vận hành → P6; nguồn luật/changelog → P2; docs/skill/mã mẫu → P7. Sửa nội bộ không đổi các thông tin này thì giữ CLAUDE.md.
- Giữ CORE theo bản nguồn đã áp dụng (`agent-environment/project-local/template/CLAUDE.template.md`, chỉ có trên máy tác giả), chỉ đổi khi chủ dự án yêu cầu. Dẫn nguồn thay chép dài; không dùng CLAUDE.md làm backlog hoặc nhật ký.
