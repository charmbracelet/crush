# Phase 1 — vấn đề còn mở (audit 14/10, khoảng 16:55)

Nguồn:
- Ledger `.superpowers/sdd/2026-10-14-multi-session-multi-project/progress.md`. Mọi mục ở đây đều đã có người soát hoặc worker ghi lại, kèm file:line.
- Kết quả soát lát 1 ("INSTALL: GO").
- Lần chạy thật sau khi cài.

Chưa gồm phần audit độc lập của lát 2 (A7, V-lite, H3), sẽ bổ sung sau.

## A. Phải xong trước cổng WIN (ảnh hưởng dùng hằng ngày)

| # | Vấn đề | Tác động | Nguồn | Hướng sửa |
|---|---|---|---|---|
| A1 | Lệnh `!` lỗi kết nối (chế độ client/server là mặc định) không nhả slot — **đã sửa (w)** | Cả workspace đọc "bận": đổi model bị chặn tới khi chạy một lệnh `!` khác | Ledger, A5 re-review | Trả `shellResultMsg{ExitCode:1, Output: err}` khi lỗi, 1 dòng + test |
| A2 | `run -s S` xếp hàng sau lượt TUI trong S; khi lượt TUI hỏi câu hỏi, `run` tưởng là của mình và huỷ cả session S | Mất lượt đang chạy của TUI | Ledger, D1 (đường Ctrl+C đã sửa ở H0, đường câu hỏi chưa) | **Đã sửa (w)**: chỉ xử lý khi prompt của mình đã rời hàng đợi. Trước đó `run -s` còn TỰ DUYỆT quyền của lượt TUI. Giới hạn: khớp theo chữ prompt |
| A3 | Session nền xin quyền cướp focus hoặc bị duyệt nhầm (đã đo trước sửa: Enter ở B duyệt quyền của A) | Duyệt nhầm quyền | V06 | **A7 đã sửa (w)**, chờ soát |
| A4 | Job bash nền chưa có owner/elapsed/output trong UI | Kịch bản WIN "job nền" | Roadmap P1.4 | H4 (lát 3) |
| A5 | Lỗi mạng/context: chưa kiểm có chạy lặp thao tác ghi không | Kịch bản WIN "context/mạng lỗi" | Roadmap P1.5 | P1.5: chỉ kiểm, đỏ thì sửa |

## B. Nên sửa (an toàn / thấy được lỗi)

| # | Vấn đề | Nguồn | Hướng sửa |
|---|---|---|---|
| B1 | Một số lệnh git vẫn tự duyệt: `git grep -O`, `ls-remote --upload-pack`, `--ext-diff`/`--textconv` (chạy được chương trình) | S1b report | **Đã sửa (w)**. Còn lại: `git diff`/`status` thường vẫn chạy `diff.external` từ config của repo, cờ không chặn được (rủi ro khi mở repo lạ) |
| B2 | write/edit/multiedit tạo thư mục cha TRƯỚC khi hỏi quyền: bị từ chối vẫn để lại thư mục rỗng | Re-review A10 (O2), có sẵn từ upstream | **Đã sửa (w)** |
| B3 | `ps` bỏ qua lỗi từng session mà không báo | Ledger B2 | **Đã sửa (w)** |
| B4 | Shift+Tab lúc rảnh vẫn gọi HTTP đồng bộ không timeout (`PermissionSkipRequests`, `toggleYoloMode`) | Re-review A10 (O3), có sẵn | Thêm timeout ngắn |
| B6 | Hàng đợi quyền ở backend dùng chung mọi session: A đang chờ duyệt thì yêu cầu của B bị giữ (không mất) tới khi A được trả lời | A7 report | Hàng đợi theo từng session trong `internal/permission`; đo trước khi sửa |
| B7 | (Nghi, chưa tái hiện) `InitAgent` lúc rảnh bị lỗi làm coordinator = nil, mà `SendMessage` đọc coordinator không khoá → có thể crash nil-pointer | FINALFIX-A report | Đọc coordinator dưới `runMu`, hoặc giữ coordinator cũ khi init lỗi; viết test tái hiện trước khi sửa |
| B5 | `InitAgent` giữ `runMu` trong lúc dựng coordinator: mọi request backend chờ trong thời gian đó | Soát lát 1 | Chưa đo; chỉ đo khi thấy chậm |

## C. Nợ cấu hình (tệp của phiên khác: chỉ báo, không tự sửa)

| # | Vấn đề | Nguồn |
|---|---|---|
| C1 | Model đã lưu không còn trong catalog thì `load.go:166-178` ghi fallback vào ScopeGlobal | Ledger C2 |
| C2 | Mảng `recent_models` bị nối dồn qua mỗi lần khởi động, file phình dần | Ledger C2 |
| C3 | Tool `claude_code`/`codex` (cây chính) có trong mọi request kể cả khi máy không có CLI. Chi phí token chưa đo | Thử gộp P1.6 |

## D. Giới hạn chấp nhận cho Phase 1 (đã ghi docs hoặc ledger)

- Draft: file đính kèm không lưu xuống đĩa; mất draft nếu kill/crash; Windows không có khoá liên tiến trình.
- Ctrl+C trên `run -s`/`-C` chỉ ngừng chờ, không huỷ run (muốn huỷ đúng từng run cần `coordinator.go`).
- Run mồ côi giữ server tối đa 30 phút (`CRUSH_SERVER_ORPHAN_RUN_TIMEOUT`).
- Chế độ in-process (không client/server): Alt+W huỷ run.
- Home chỉ nhận server theo tên socket của launcher.
- Hook Stop: prompt xếp hàng nhận kết quả trước khi hook kết luận. Hook là opt-in; user không cấu hình.
- Accept-edits không có trong Shift+Tab (DEFER). Tên được bảo vệ chỉ xét trên đường dẫn đã resolve.

## E. Chưa kiểm trên máy user

- Phím Alt+W/S/N/M trên terminal thật của user.
- Hai session và hai dự án song song trong TUI.
- Delegation Claude/Codex với tài khoản thật.

## Kế hoạch đóng

1. Lát 2: A7 + V-lite → một lượt soát/audit độc lập → cài `0.1.0-lat2`.
2. Đợt sửa ngắn: A1, A2, B1, B2, B3, B4, giao 1 agent.
3. Lát 3: H4 + P1.5 → cài `0.1.0-lat3`.
4. User đi trọn daily workflow → xác nhận WIN.
