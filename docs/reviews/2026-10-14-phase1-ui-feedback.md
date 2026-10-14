# Feedback UI và thu gọn Phase 1 — đề xuất để agent cân nhắc

Ngày: 14/10/2026. Đối tượng: agent-cli cá nhân dùng FE/BE/devops.

Đây là danh sách đề nghị và case study, không phải lệnh triển khai tất cả. Người thi hành đọc AGENTS.md, phản biện bằng code/runtime và chọn phần có ích cho Phase 1. Không tự commit/push, không ghi đè phiên khác, không mở rộng sang Phase sau.

## Cơ sở và cách đọc sao

Đã đọc Home render/model, panel session, key bindings và roadmap trong checkout/worktree multi-session. Chưa nhìn bản UI mới chạy thực; chưa đo accessibility/performance, chưa tự chạy lại toàn bộ test. Code đang thay đổi nên kiểm tra lại snapshot trước quyết định.

- ★★★★★: bảo vệ hoặc mở khóa daily workflow; nên ưu tiên nếu gap còn tồn tại.
- ★★★★☆: cải thiện đáng kể độ rõ/dễ dùng; làm sau các gap chức năng.
- ★★★☆☆: hữu ích có điều kiện; chỉ làm khi case thực chứng minh nhu cầu.
- Dưới 3 sao không đưa vào danh sách.

Sao là đánh giá sản phẩm chủ quan theo nhu cầu người dùng, không phải tỷ lệ hoàn thành, benchmark hoặc kết quả audit định lượng.

Tham chiếu chính: ../superpowers/plans/2026-10-14-agent-cli-personal-roadmap.md. Yêu cầu Alt+W/S/N/M mới nhất thắng đề xuất switcher/index cũ.

## A. Đề xuất sửa view

| ID | Sao | Đề xuất | Case study / vấn đề | Tiêu chí quyết định và nghiệm thu |
|---|---|---|---|---|
| V01 | ★★★★★ | Thống nhất Alt+W Workspace, Alt+S Session, Alt+N New, Alt+M Model | Người dùng học bốn mnemonic; Alt+W cũ đổi session làm chọn nhầm | Rà binding/handler/help; terminal/tmux thực nhận phím; Ctrl+P Commands giữ fallback |
| V02 | ★★★★★ | Header rõ workspace, host, branch, session, model | Đang xem hai repo hoặc local/server, giao lệnh vào nhầm nơi | Luôn nhìn được context; nguồn đúng, không suy từ title; tối đa hai dòng là đề xuất, điều chỉnh khi màn hẹp |
| V03 | ★★★★★ | Workspace picker tìm tên/path, active/pinned/recent, mở cwd/path | Bắt đầu ngày cần quay lại repo nhanh; hai folder trùng tên | Tên + path phân biệt được; chọn không tự gửi prompt/tạo run; show-all và tìm toàn registry |
| V04 | ★★★★★ | Session picker theo tên, bỏ index khỏi trải nghiệm core | Session thứ tự đổi khiến số nhớ hôm qua sai hôm nay | Gõ tìm, ↑↓, Enter; trạng thái/current marker rõ; ID chỉ là chi tiết |
| V05 | ★★★★★ | Navigation giữ draft/attachments, dialog không gửi/cancel | Đang gõ ở A, mở picker, Enter/Esc rồi về A | Nội dung còn đúng owner; Esc chỉ đóng modal; highlight chưa đổi session; chạy nền tiếp tục |
| V06 | ★★★★★ | Waiting hiện đúng phiên, không cướp focus | A xin quyền khi đang viết ở B | Badge và lý do waiting; mở A để duyệt; không duyệt nhầm; thông báo không chèn editor |
| V07 | ★★★★☆ | Activity riêng cho agent/subagent/job | Trộn mọi đối tượng vào session list làm khó hiểu | Owner/trạng thái/time/output rõ; chỉ hiển thị dữ liệu quan sát được; chức năng ownership P1 là bắt buộc, bố cục chi tiết có thể giản lược |
| V08 | ★★★★☆ | Trạng thái có chữ và icon nhất quán | Icon busy không phân biệt waiting, failed, unknown | Nhãn Running/Waiting/Done/Failed/Unknown đúng nguồn; không chỉ dựa màu; offline không suy idle |
| V09 | ★★★★☆ | Footer ngắn theo context; Ctrl+P luôn Commands | Home đang đề xuất Ctrl+P pin, màn chat lại Commands; help dài bị cắt | Footer core hoặc picker help; pin/remove qua action phụ; không đổi nghĩa Ctrl+P theo màn |
| V10 | ★★★★☆ | Giữ selection/scroll khi refresh | Runtime cập nhật giữa lúc ↑↓ khiến Enter chọn dòng khác | Anchor bằng identity; stale response không ghi đè workspace/session mới |
| V11 | ★★★★☆ | Loading/empty/error có bước tiếp rõ | Server timeout, repo bị di chuyển hoặc provider quota | Lỗi cụ thể, không giả status; retry/back rõ; danh sách rỗng có New/Open folder |
| V12 | ★★★★☆ | Màn hẹp ưu tiên tên/trạng thái thay path dài | SSH terminal nhỏ, đường dẫn che trạng thái | Path chuyển dòng phụ/chi tiết; không mất item chọn; thử nhiều kích thước thực, không cần redesign responsive framework |
| V13 | ★★★☆☆ | Theme tương phản dễ đọc, một default | Chữ muted quá mờ hoặc màu waiting khó nhìn | Chỉ chỉnh sau xem runtime sáng/tối; không xây theme engine; chữ trạng thái vẫn đủ nghĩa |
| V14 | ★★★☆☆ | Chi tiết đầy đủ cho mục bị truncate | Tên job/session dài có phần phân biệt nằm cuối | Detail pane hoặc Commands xem full; chỉ thêm nếu tooltip/detail gốc chưa đáp ứng |

## B. Đề xuất hoãn, tắt, gỡ wiring hoặc xóa

Không hiểu “clean” là comment nguyên khối mã. Phân biệt:
- DEFER: ngoài Phase 1, bỏ khỏi dependency bắt buộc.
- DISABLE/HIDE: cấu hình hoặc UI entry; giữ nền upstream.
- REMOVE WIRING: custom binding/handler đã bị thay thế; kiểm reference/test.
- DELETE: chỉ khi chứng minh code chết hoặc trùng; Git giữ lịch sử.
- COMMENT: chỉ option trong mẫu hỗ trợ comment; JSON không comment thì bỏ option và giải thích trong docs.

| ID | Sao | Đề nghị | Case study / lý do | Guard trước khi làm |
|---|---|---|---|---|
| C01 | ★★★★★ | Gỡ wiring navigation custom index/Alt-tab đã bị thay thế | Alt+W bị switcher chiếm, trái yêu cầu Workspace | Map references + test; giữ data tracking hữu ích; không xóa code phiên khác chưa phối hợp |
| C02 | ★★★★★ | Bỏ dependency draft/Home chờ switcher/keybinding/accept-edits nếu không cần | Core bị chặn bởi task ngoài MVP | Kiểm dependency code thật; defer task và chỉnh thứ tự, không bỏ dependency kỹ thuật cần thiết |
| C03 | ★★★★☆ | Defer custom keybinding framework mới | Bốn phím core chưa chạy nhưng thêm reflection/config riêng | Giữ compatibility đã cần; không rollback phần đã ổn chỉ vì tên task; không quảng bá thêm shortcut |
| C04 | ★★★★☆ | Defer mở rộng accept-edits/mode mới | Permission/routing core chưa xong mà thêm state/mode | Giữ permission gốc; không nới quyền; phần đã làm review an toàn trước integrate |
| C05 | ★★★★☆ | Không cấu hình provider/MCP chưa dùng | Setup phức tạp và network phụ không phục vụ daily | Không xóa adapter/MCP core; chỉ tắt entry cá nhân không cần; giữ template chọn dùng |
| C06 | ★★★★☆ | Tắt metrics/catalog auto-update theo cấu hình cá nhân | Muốn endpoint/data-routing rõ và ít hoạt động nền | Kiểm setting thực có hiệu lực; không tuyên bố chặn mọi network; model calls vẫn cần network |
| C07 | ★★★☆☆ | Tắt âm thanh/notification phụ nếu gây phiền | Nhiều phiên tạo toast/sound liên tục | Badge waiting/done vẫn hoạt động; preference riêng, không mất cảnh báo cần duyệt |
| C08 | ★★★☆☆ | Giảm custom logo/theme/animation | Tốn công mà không giải quyết resume/provider/job | Giữ upstream default đọc được; không xóa rendering utilities đang dùng |
| C09 | ★★★★☆ | Chuẩn hóa temp prefix và cleanup test thuộc sở hữu | /tmp/aclijson.* sót, khó biết nguồn | mktemp riêng user/run, trap/test cleanup; không dọn /tmp của Claude hoặc run còn sống; retain-on-failure tùy chọn |
| C10 | ★★★★☆ | Gộp logic log path/identity bị trùng nếu tìm được | logs command và server ghi khác nơi | Chứng minh hai callsites cùng semantics; reuse helper hiện có; không tạo abstraction mới không cần |

Không đề xuất xóa LSP, compact, DB/session storage, permission, client/server, Bash nền, output limits hay recovery. Đây là nền dùng trực tiếp/gián tiếp trong Phase 1.

## C. Layout tham khảo, không ép pixel

```text
workspace · LOCAL/REMOTE · branch
session title · provider/model · status

SESSIONS                 CHAT
current marker + name    nội dung phiên hiện tại
status + name

ACTIVITY
agent/job · status · elapsed → output/details/cancel

Alt+W Workspace · Alt+S Session · Alt+N New · Alt+M Model
```

Home không cần nhồi Activity mọi phiên. Chat view ưu tiên hội thoại; panel phụ có thể thu gọn. Không đổi font/theme để che thiếu chức năng. Với remote chưa triển khai, không hiển thị nhãn REMOTE giả.

## D. Cách agent quyết định

1. Kiểm HEAD/status/worktree và task ownership. Xác minh case còn tồn tại.
2. Đọc roadmap acceptance; không coi sao là authorization mở phạm vi.
3. Chọn ACCEPT / ALREADY_SATISFIED / DEFER / REJECT cho từng đề xuất có liên quan lô đang làm. Ghi lý do và code/test reference ngắn.
4. Ưu tiên V01–V06 và C01–C02 khi còn gap; không cần làm mọi mục mới giao hàng.
5. Thay đổi behavior rủi ro phải có test; sửa view đơn giản không cần test mirror implementation.
6. Chạy launcher thực, không chỉ gọi render fixture. Quan sát terminal/tmux và resize; không claim accessibility/performance chưa đo.
7. Không commit/push trong nhiệm vụ feedback này; integration theo quy trình repo và authorization riêng.

## E. Case study nghiệm thu

| Tình huống | Kỳ vọng |
|---|---|
| Hai workspace cùng basename | Path/host phân biệt rõ trước mở |
| Session/job title dài, terminal hẹp | Selection và trạng thái còn đọc được; có xem chi tiết |
| A running, B đang gõ | Refresh/toast không cướp focus hoặc mất draft |
| A waiting permission, B mở picker | Enter/Esc không duyệt hoặc cancel nhầm A |
| Server offline/timeout | Unknown/offline rõ; UI còn dùng được |
| Provider quota/context failure | Lý do và đường tiếp; không chạy lại mutation |
| Alt bị terminal giữ | Ctrl+P mở được cùng hành động |
| Chọn running session | Attach/view, không tạo run thứ hai |
| Test bị interrupt | Chỉ dọn artifact sở hữu; log lỗi còn theo policy |

Prompt ngắn:

> Đọc feedback này cùng roadmap Phase 1. Đây là đề nghị để phản biện, không phải làm tất cả. Đối chiếu case với code hiện tại; ưu tiên mục 5 sao còn thiếu, chọn ACCEPT/DEFER/REJECT có lý do. Tái sử dụng lõi, không comment hàng loạt, không nở phạm vi hoặc đè phiên khác. Hoàn thiện lô core nhỏ, chạy launcher thật và ghi evidence.
