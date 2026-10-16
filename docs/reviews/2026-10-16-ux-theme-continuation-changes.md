# Báo cáo thay đổi 2026-10-16 — UX, theme, continuation, layout

> Mục đích: ghi lại trọn bộ logic của đợt sửa để review và để không quên. Nhánh
> review tách từ `main` (`e0e752fe`). Mọi khẳng định "đã kiểm" đều kèm lệnh đã chạy.
> Cây làm việc lúc này trộn hai phiên: phiên UX/continuation (phần A) và phiên
> Codex đang làm B0/B1 (phần B, kèm theo để không mất, CHƯA hoàn tất).

## A. Phiên này — đã có test đỏ→xanh và thử đột biến

### A1. Sửa vòng continuation và tóm tắt (`internal/agent/agent.go`)
Bốn lỗi làm mất việc/mất ngữ cảnh âm thầm trên model local (qwen/vLLM hay cắt output):

1. **Tóm tắt rỗng không còn xoá sạch lịch sử.** `summarize()` trước đây luôn
   `AddFinish(EndTurn)` rồi gán `SummaryMessageID`, kể cả khi model (tiêu hết
   ngân sách cho phần nghĩ) trả chuỗi rỗng — toàn bộ ngữ cảnh bị thay bằng rỗng.
   Nay: `strings.TrimSpace(content)==""` → coi là tóm tắt hỏng (giữ lịch sử, làm tiếp).
2. **Prompt "interrupted…" không còn lồng nhiều lớp.** Thêm trường
   `SessionAgentCall.originalPrompt`: mỗi lớp làm tiếp bọc đúng YÊU CẦU GỐC một
   lần, thay vì bọc lại chuỗi đã bọc. Cũng sửa ca: sau lượt bị cắt, "yêu cầu gốc"
   không còn bị thay bằng lời nhắc "reply was cut off".
3. **Lượt làm tiếp không nhân bản attachment.** `next.Attachments = nil` ở cả hai
   nhánh (tóm tắt, bị cắt): lịch sử đã có tệp đính kèm, gửi lại mỗi lần làm phình context.
4. **Lượt làm tiếp chạy TRƯỚC prompt người dùng gõ thêm.** Đổi `append(existing, next)`
   thành chèn đầu hàng đợi: phần còn lại của việc đang chạy ưu tiên hơn prompt mới.
5. **Trần "3 lần cắt" đếm lại khi có tiến triển.** Lượt nào đã chạy xong ≥1 tool call
   (`len(completedToolCalls) > 0`) thì `lengthContinues = 0`. Việc dài không còn dừng sớm.

Test: `internal/agent/continuation_test.go` (`scriptModel` dựng kịch bản cut/tool/done).
Lệnh: `taskset -c 0,2,3,5-8,10,11,13-15 go test ./internal/agent -run 'Continuation|Summary|Repeated|LengthContinue' -count=1`.
Đột biến: hoàn tác từng sửa thì đúng test tương ứng đỏ lại (M1–M4 + reset).

### A2. Theme Deep Charcoal (`internal/ui/styles/everforest_dark_v2.go`, `_test.go`)
Áp palette "nền gần đen, màu desaturated, dùng theo semantic" người dùng đưa:
- Nền `#141619`/`#0F1113`/`#1B1E22`/`#23272D`; viền `#30343B`; chữ `#D0D2D1`/`#929997`/`#7D8585`.
- Một màu nhấn duy nhất teal `#82AEB1`; `info` `#829CB5` chỉ còn cho chẩn đoán LSP.
- Logo "AGENT CLI" thành `fg.primary` (bỏ gradient). Viền panel To-Do dùng token `border`.
- **Sửa một lỗi tương phản thật:** nút inactive trước 4.43:1, nay > 4.5 (WCAG AA).
- Tương phản thấp nhất đã đo: `fg.subtle`/`bg.base` = 4.80; `error`/`bg.base` = 5.75.
  Lưu ý: `fg.subtle` trên nền `raised`/`overlay` = 4.43/3.98 (chữ hint/phím tắt hơi mờ).

Test: `go test ./internal/ui/styles` — palette ghim trong test là spec độc lập, không suy từ code.
Đột biến: đảo `error` về màu cũ → 2 test đỏ.

### A3. F2 đổi tên session (`keys.go`, `ui.go`, `dialog/sessions.go`)
F2 ở màn chat mở hộp thoại Sessions thẳng ở chế độ đổi tên cho session HIỆN TẠI,
dùng lại `RenameSession` nguyên tử có sẵn. Không session thì không làm gì.
Test: `rename_session_key_test.go`, `sessions_start_rename_test.go`. Gỡ phím → test đỏ.

### A4. Output tool dùng hết chiều rộng khung chat (`internal/ui/chat/*`)
Trước: mọi thứ bị bẻ ở 120 cột (`cappedMessageWidth`) nên để trống nửa màn hình rộng.
Nay: thêm `toolMessageWidth` (dùng hết khung) cho output bash/diff/code/kết quả tìm;
đoạn văn của model GIỮ trần 120 cột cho dễ đọc. Bỏ cờ `hasCappedWidth` (nay mọi tool
đều dùng hết). Test: `internal/ui/chat/width_test.go`. Đột biến: cho tool về cap 120 → đỏ.

### A5. Verification gate (`docs/hooks/examples/verify-gate.sh`)
Dùng lại cơ chế Stop hook có sẵn (không viết Go mới). Hook chạy `AGENT_CLI_VERIFY_CMD`
(test/lint); fail → in `{"decision":"block","reason":"VERDICT: NOT READY\n…"}` nên agent
không được báo "xong" khi test đỏ. Không cấu hình → bỏ qua. Đã test tay 4 ca (không
cấu hình / pass / fail / không có jq) đều đúng. Bật: xem đầu file script.

### A6. Docs (`docs/agent-cli-local-setup.md`)
Sửa mặc định `AGENT_CLI_MAX_TOKENS` ghi sai (8192 → min(32000, context/4); 8192 là
nguyên nhân hay gặp của việc bị cắt giữa tool call). Cập nhật mô tả trần continuation.

## B. Phiên Codex song song — kèm theo để KHÔNG MẤT, CHƯA hoàn tất
Các tệp dưới đây do một phiên khác (Codex) làm, nằm sẵn trong cây lúc phiên này bắt
đầu. Commit lại để bảo toàn, KHÔNG phải việc đã nghiệm thu của phiên này:
- **Đặt tên session từ prompt (B0):** `internal/session/title.go` (`DerivedTitle`),
  `internal/session/derived_title_test.go`, `internal/agent/derived_title_test.go`,
  `internal/cmd/run_title_test.go`, wiring ở `internal/app/app.go`,
  `internal/app/resolve_session_test.go`, `internal/cmd/run.go`.
- **Chặn bypass qua interpreter (B1):** `internal/execpolicy/interpreter.go` (mới),
  `internal/execpolicy/policy.go`, `internal/execpolicy/policy_test.go`,
  `internal/agent/tools/bash_policy_test.go`.
- **Đổi tên theme thành `luannt-theme-everforest-dark`** (alias `everforest-dark-v2`):
  `internal/ui/styles/themes.go`, `internal/config/config.go`, `DECISIONS.md`.
  (Phần RENAME là của Codex; phần PALETTE màu ở A2 là của phiên này — hai phần nằm
  chung tệp `everforest_dark_v2*.go`.)
- **Plan tổng:** `docs/superpowers/plans/2026-10-16-master-status-and-batches.md`.

## Kiểm tra toàn cục (đã chạy 2026-10-16)
- `taskset -c 0,2,3,5-8,10,11,13-15 go build ./...` → rc=0.
- `taskset -c 0,2,3,5-8,10,11,13-15 go test ./... -count=1 -p 4` → 66 package ok, 0 FAIL.
- `go test -race` trên `internal/agent`, `internal/ui/{model,dialog,styles,chat}` → 0 data race.
- `go vet` các package đã sửa: sạch (gofmt kêu generic method ở `app.go`/`session.go` là
  giới hạn parse của gofmt với tệp của Codex/upstream, không phải lỗi format của đợt này).

## Chưa làm (cần người dùng quyết)
- **Kill-guard:** lọt `sudo kill`, `xargs kill`, `pkill -f "crush server"`, `killall -r`.
  Bộ lọc an toàn chặn làm trong phiên này; để lại.
- **Khoá model:** đặt `AGENT_CLI_ALLOWED_MODELS="main:qwen3.8"` trong `~/.config/agent-cli/env`
  (chỉ cấu hình). Hiện máy chưa bật nên chưa chặn provider nào.
- **Tiêu đề LLM đè tên người dùng vừa tự đổi:** chưa sửa.
- **Review agent / checkpoint / `/doctor`:** đề nghị KHÔNG làm — `git` và review bằng mắt rẻ hơn.
- Theme và F2 CHƯA thử bằng mắt trong terminal thật; binary CHƯA cài lại.
