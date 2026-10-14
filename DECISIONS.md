# DECISIONS — sổ quyết định (APPEND-ONLY, không sửa dòng cũ)
Nạp tự động vào mọi phiên Claude. Mục đích: không bàn lại cái đã chốt, không thử lại cái đã hỏng.

## 2026-10-14 — Tool claude_code/codex: xin quyền mỗi lần giao, mức quyền CLI con qua env
**Trạng thái:** đã làm
- **Chốt:** (commit 9186c66b; có test và đã chạy thật với CLI thật) Mỗi lần giao việc cho Claude Code/Codex đều qua `permissions.Request`: đây là ranh giới tin cậy duy nhất vì CLI con chạy không hỏi lại ai. Mức quyền CLI con mặc định giữ toàn quyền (claude `--dangerously-skip-permissions`, codex `sandbox_mode="danger-full-access"`), đổi được bằng env của tiến trình: `AGENT_CLI_CLAUDE_PERMISSION_MODE`, `AGENT_CLI_CODEX_SANDBOX`; `AGENT_CLI_CLAUDE_BIN`, `AGENT_CLI_CODEX_BIN` cho CLI không nằm trong PATH. Chỉ đăng ký tool khi agent được phép dùng (không bị `permissions deny`) và máy có CLI; `*_BIN` đặt mà sai thì dựng agent lỗi (fail-fast, lỗi hiện ngay ở prompt đầu, nêu tên biến). `resume` chỉ nhận dạng ID và đi thành một token (`--resume=<id>`).
- **Vì:** bản trước chạy CLI con toàn quyền mà không hỏi (P0, hai người soát độc lập cùng tìm ra). Mặc định toàn quyền là quyết định sản phẩm đã chốt từ trước và cần để CLI con chạy được test/build. Đo ngày 14/10: trước sửa, tool có mặt trong mọi request kể cả khi máy không có CLI; sau sửa, thiếu CLI thì tool vắng (chạy thật), `*_BIN` sai thì `run` rc 1 kèm tên biến.
- **Đã thử và bỏ:** (a) đặt cấu hình thành `option` của crushrc: bỏ vì phải sửa schema/options của upstream, merge upstream đắt hơn; (b) đọc biến từ `~/.config/agent-cli/env`: không được, tệp đó chỉ được crushrc `source` trong trình thông dịch riêng, biến không tới tiến trình; (c) mặc định an toàn (`acceptEdits`/`workspace-write`): bỏ vì Claude Code không chạy được bash/test và Codex không ghi được Go build cache ngoài workspace, mất phần lớn giá trị giao việc; (d) `*_BIN` sai thì chỉ cảnh báo và bỏ tool: bỏ vì luật "cấu hình hỏng thì chặn", tool biến mất âm thầm khó phát hiện; (e) truyền sandbox bằng `-s` cho cả resume: `codex exec resume` không nhận `-s` (codex-cli 0.159.2 báo `unexpected argument '-s'`), nên dùng `-c sandbox_mode=...`; (f) `--resume <id>` hai token: model đặt `resume:"--dangerously-skip-permissions"` là chèn được cờ, nên dùng một token.
- **Lật lại khi:** cần đổi mức quyền theo từng dự án mà không khởi động lại server (biến env là của tiến trình server, phải chờ server cũ tắt mới nạp lại) thì chuyển sang option của crushrc; hoặc upstream có cơ chế quyền chung cho tool gọi agent ngoài.

## 2026-10-14 — Kiểm thật giao việc ở mức toàn quyền từ phiên Claude Code
**Trạng thái:** bác bỏ — bộ phân loại auto-mode của Claude Code chặn ("Create Unsafe Agents")
- **Chốt:** Phiên Claude chỉ kiểm thật tool giao việc ở mức hạn chế: `dontAsk`/`acceptEdits` cho Claude, `read-only`/`workspace-write` cho Codex; `scripts/test-agent-cli-delegation.sh` mặc định như vậy. Mức toàn quyền mặc định chỉ có unit test (so cờ); người dùng tự chạy thật ở terminal của mình.
- **Vì:** ngày 14/10, lệnh `claude -p ... --dangerously-skip-permissions` qua được, nhưng ngay sau `codex exec -s danger-full-access` thì lệnh kế tiếp bị chặn với lý do "Create Unsafe Agents". Luật là không lách.
- **Đã thử và bỏ:** chạy CLI con toàn quyền trực tiếp bị chặn; chạy gián tiếp qua agent-cli là lách đúng điều bị chặn nên không làm.
- **Lật lại khi:** người dùng cấp quyền rõ ràng (luật permission trong settings) cho phiên Claude chạy agent toàn quyền trong thư mục nháp.

## 2026-10-14 — Ca kiểm "không ghi được tệp" của Claude Code dùng plan mode
**Trạng thái:** bác bỏ — plan mode ghi tệp kế hoạch vào `~/.claude/plans` thật của người dùng
- **Chốt:** Ca kiểm mức quyền hạn chế của Claude dùng `--permission-mode dontAsk`: chặn cả tool Write lẫn lệnh shell dự phòng, không sinh tệp nào ngoài thư mục test.
- **Vì:** mỗi lần chạy ca `plan` để lại `~/.claude/plans/create-a-file-named-*.md` (2 tệp ngày 14/10, đã xoá). `dontAsk` đo được: không tạo tệp, số tệp trong `~/.claude/plans` không đổi.
- **Đã thử và bỏ:** `plan` có tác dụng phụ ngoài thư mục test; `default`/`manual` chưa thử làm ca chặn ghi, không cần vì `dontAsk` đã đủ.
- **Lật lại khi:** Claude Code đổi ngữ nghĩa `dontAsk` (vd bắt đầu tự duyệt). Kiểm lại bằng ca `sandbox` của script.
