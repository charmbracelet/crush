# Audit trước khi gộp vào `main` — 14/10/2026

## Phạm vi và cách làm

- **Gộp:** `feat/core-mvp-phase1` (`c330628e`, `1757a90d`) và `feat/personal-rename-providers` (`2ed75353`, `61d8acf9`, `9aaf150d`) vào `main` (base upstream `a73e1bfa`). Gộp không xung đột văn bản; build và test chạy trên bản đã gộp.
- **Soát:** ba người soát độc lập, chỉ đọc, chia vùng không chồng nhau (vòng đời run + CLI script; TUI đa session; script cài, cấu hình, tài liệu, đổi tên). Người điều phối tự soát phần an ninh và đo lại mọi khẳng định trước khi sửa. Sau đợt sửa có thêm một lượt soát độc lập cho chính các thay đổi.
- **Chạy thật:** agent-cli giao việc cho Claude Code 2.1.299 và Codex 0.159.2 thật (đăng nhập sẵn trên máy). LLM chính dùng server giả có kịch bản, vì máy chưa có key GLM/API. Đã chạy cả chế độ local lẫn client/server.

## Kết quả kiểm sau khi sửa

| Kiểm | Kết quả |
|---|---|
| `go test ./... -count=1 -p 4` | 65 package xanh, 0 đỏ (17 package không có test) |
| `go test -race` các package lõi | 31 package xanh, 0 data race |
| `go vet ./...` | chỉ còn `internal/csync/maps.go:156` (mã upstream, hai nhánh không đụng) |
| `scripts/test-agent-cli-run-json.sh` (E2E `run --json`, local + server) | 39/39 PASS |
| `scripts/test-agent-cli-wrapper.sh` (launcher + installer) | PASS |
| `scripts/test-agent-cli-delegation.sh` (Claude Code + Codex thật, server) | 10/10 PASS |
| Quét bí mật (19.192 dòng thêm, 7 thông điệp commit, 4 ref) | sạch; chỉ có định danh cá nhân (đường dẫn `/home/luannt`, tên máy, email trong metadata commit) |

## Đã sửa

| Mức | Lỗi | Sửa | Bằng chứng |
|---|---|---|---|
| P0 | Tool `claude_code`/`codex` chạy CLI con toàn quyền (`--dangerously-skip-permissions`, `danger-full-access`) mà KHÔNG hỏi quyền, dù mô tả tool nói có. Trong TUI, model chính (hoặc prompt injection trong tệp nó đọc) giao được lệnh bất kỳ, qua mặt toàn bộ hệ quyền. Hai người soát độc lập cùng tìm ra | Mỗi lần giao gọi `permissions.Request` (cần session; từ chối thì không chạy CLI); hộp thoại hiện CLI, thư mục, mức quyền và nguyên văn việc giao | `delegate_test.go`: từ chối thì CLI không chạy, đúng một lời hỏi với đủ trường |
| P1 | `codex` resume hỏng hoàn toàn: `codex exec resume` không nhận `-s` | Sandbox đi qua `-c sandbox_mode="..."` cho cả lượt mới lẫn resume | CLI thật trước sửa: `unexpected argument '-s'` (rc 2); sau sửa: resume nhớ đúng hội thoại |
| P1 | Prompt bắt đầu bằng `-` (gạch đầu dòng) bị CLI hiểu thành cờ | Thêm `--` trước tham số vị trí | CLI thật: claude `unknown option`, codex `unexpected argument`; sau sửa chạy đúng |
| P1 | Server chết đột ngột (kill, OOM, reset): luồng SSE lặp đọc lại body hỏng mỗi 2 giây mãi mãi, `run` và `send --wait` treo vô hạn, TUI không tự nối lại | Mọi lỗi đọc đều đóng kênh, để tầng trên xử lý | `TestSubscribeEventsServerDeathClosesEvents`: đỏ trước sửa, xanh sau |
| P1 | Mở session khác đổi model/provider của CẢ workspace, kể cả run đang chạy nền (run đọc lại model ở mỗi bước) | `restoreModelFromSession` bỏ qua khi có run ở bất kỳ session nào, cùng cổng với chọn model tay | `TestRestoreModelFromSessionKeepsModelWhileBusy` |
| P1 | Installer không sao lưu, không có lệnh quay lại, ghi đè thẳng (trái luật cài của CLAUDE.md) | Sao lưu binary + wrapper (giữ 3 bản), build ra tệp tạm rồi `mv`, ghi wrapper nguyên tử, in lệnh quay lại | ca mới trong `test-agent-cli-wrapper.sh` |
| P1 | CLAUDE.md (tự nạp cho mọi agent) bắt dùng `go.sh`/`preview-install.sh` không có trong git, trỏ worktree đã mất, ghi sai RAM | Viết lại P2/P3/P5–P8 theo trạng thái sau gộp, lệnh thật, số đo máy | đối chiếu `ls`, `free`, `lscpu` |
| P1 | `permissions allow view ls` trong crushrc mẫu bỏ luôn lời hỏi khi đọc NGOÀI dự án, nên agent đọc được tệp env chứa key | Bỏ dòng đó (trong dự án hai tool vốn không hỏi) | `view.go`: chỉ hỏi khi ngoài thư mục làm việc |
| P2 | Tool giao việc có mặt trong mọi request kể cả khi máy không có CLI (tốn token, model gọi tool chắc chắn hỏng) | Chỉ đăng ký khi tìm thấy CLI; đường dẫn và mức quyền CLI con cấu hình được qua `AGENT_CLI_*`; biến `*_BIN` sai thì dừng lúc dựng agent | chạy thật: thiếu codex thì tool vắng; `BIN` sai thì rc 1, lỗi nêu tên biến |
| P2 | Đổi tên session ghi đè cost/token/todos/summary bằng ảnh chụp lúc mở dialog | Đọc lại session ngay trước khi lưu, chỉ đổi tiêu đề | `TestRenameSessionKeepsFreshState` |
| P2 | Dấu "đã đọc" của tệp @-mention rò sang session khác, lách chốt "phải đọc trước khi sửa" | Xoá khi đổi session; closure gửi tin chụp sẵn session và danh sách | `TestFileReadsDoNotLeakAcrossSessions` |
| P2 | Home bỏ qua nội dung dán, không dán được đường dẫn | Chuyển `tea.PasteMsg` vào ô nhập | `TestPastePathIntoOpenOtherFolder` |
| P2 | `SendMessage` kiểm coordinator nil ngoài khoá: `InitAgent` lỗi song song gây panic và rò `runWG`/`inflightRuns`, `Shutdown()` treo | Đọc và kiểm nil dưới `runMu`, trước khi tăng bộ đếm | test hiện có thêm khẳng định bộ đếm = 0 |
| P2 | crushrc mẫu chặn `web_search`/`web_fetch` (không tồn tại), để lọt `agentic_fetch` | Sửa danh sách đúng tên tool | `allToolNames` |
| P2 | env mẫu đặt placeholder làm giá trị thật nên kiểm "bắt buộc" của crushrc không bao giờ chặn | Để các dòng bắt buộc dạng comment | chạy thật: thiếu biến thì dừng lúc khởi động, báo đúng biến |
| P2 | Thông báo bảo gõ lệnh `crush ...` không có trên PATH; tiêu đề cửa sổ `crush`; báo crash hướng về issue tracker upstream | Đổi các chuỗi đó sang `agent-cli` | grep |
| P2 | Tài liệu sai: không nói `run` tự duyệt MỌI quyền; socket dự phòng ghi `/tmp` (mã cố ý không dùng) | Sửa README, `docs/agent-cli-local-setup.md`; thêm mục giao việc Claude Code/Codex | — |
| P3 | `agent-cli logs -f` panic khi `tail` tự đóng kênh | Kiểm kênh đóng | — |

### Vòng 2: lỗi do lượt soát độc lập tìm trong chính đợt sửa

| Mức | Lỗi | Sửa | Bằng chứng |
|---|---|---|---|
| P1 | `resume` do model đặt nằm trước `--` trong lệnh gọi `claude`: `resume:"--dangerously-skip-permissions"` chèn cờ, đè mức quyền người dùng cấu hình | Dạng một token `--resume=<id>`; `resume` phải là ID (chữ/số/`._:-`), sai thì từ chối trước khi hỏi quyền | `TestDelegationRejectsFlagLikeResume`; resume với claude thật vẫn chạy |
| P2 | Test cũ của package provider bị biến `AGENT_CLI_*` của shell ghi đè: có `AGENT_CLI_CODEX_BIN` là `go test` chạy Codex THẬT toàn quyền | `TestMain` gỡ 4 biến trước khi chạy test | chạy test với biến "phá" vẫn xanh |
| P2 | `*_BIN` trỏ sai làm hỏng cả agent kể cả khi tool đã `permissions deny`; đường dẫn tương đối tính sai thư mục | `NewTools` chỉ xét tool được phép; chốt đường dẫn tuyệt đối lúc dựng | `TestNewToolsRejectsBrokenExplicitBinary`, `TestNewToolsResolvesRelativeBinary` |
| P2 | Lệnh quay lại `cp` thẳng lên binary đang chạy báo "Text file busy"; không được in khi bản mới hỏng ở `--version`; sao lưu trước khi build nên build lỗi liên tiếp đẩy mất bản cũ | `cp` ra tệp tạm rồi `mv`; in trước `--version`; sao lưu sau khi build xong | mô phỏng: bản mới `exit 3` vẫn in lệnh quay lại, quay lại được khi binary cũ đang chạy |
| P2 | Tài liệu: đoạn mô tả crushrc mẫu mâu thuẫn bản mới; không nói biến giao việc là env của tiến trình server | Viết lại | — |

## Chưa sửa — cần quyết định hoặc để đợt sau

| Mức | Vấn đề | Đề xuất |
|---|---|---|
| P2 | `run` coi run bị huỷ là thành công (rc 0, text rỗng), kể cả khi prompt xếp hàng bị Esc ở TUI gỡ đi chưa từng chạy; `send --wait` thì báo lỗi. Upstream ghim "Ctrl+C thoát sạch" bằng test | Phân biệt tự bấm Ctrl+C với bị nơi khác huỷ; cái sau trả rc 1 |
| P2 | `run` chết bất thường để lại run phía server chạy mồ côi tới 30 phút, không ai duyệt quyền; ngược lại việc giao bằng `send` dài hơn 30 phút bị huỷ vì đồng hồ tính từ lúc client rời đi | Đo theo tiến triển của run thay vì thời điểm client rời |
| P2 | Workspace mồ côi được client sau tái dùng với cờ YOLO của client trước, chỉ ghi log Debug | Nâng thành cảnh báo hiện cho client vào, hoặc từ chối khi lệch YOLO |
| P2 | `run`/`send` ở chế độ client/server dùng coordinator tương tác của workspace (có tool `question`, không chờ MCP); `send --wait` treo khi sub-agent xin quyền | Thiết kế lại đường phi tương tác ở server |
| P2 | Draft chỉ ghi đĩa khi thoát êm; đóng terminal, SSH rớt, mất điện là mất | Ghi định kỳ hoặc khi nhận SIGHUP |
| P2 | Bảng Sessions chỉ phủ 9 session tạo gần nhất: session cũ chạy nền chờ quyền không ai báo | Theo dõi theo session đang bận, không theo thời điểm tạo |
| P2 | Alt+S và mỗi lần đổi session vẫn làm IO đồng bộ trong `Update` (trái bất biến của repo) | Chuyển sang `tea.Cmd` như `workspace_cache.go` |
| P2 | Attribution mặc định thêm `Assisted-by: Crush:<model>` và `Generated with Crush` vào commit/PR do agent tạo | Nếu không muốn: `option attribution-trailer-style none` và `option attribution-generated-with false` trong crushrc |
| P2 | Kiểm tra bản mới của upstream mặc định BẬT khi chạy binary không qua launcher; giá trị `0`/`false` cũng bị coi là tắt | Mặc định tắt cho fork, dùng `ParseBool` |
| P2 | `grep`/`glob` không có cổng quyền (thiết kế upstream) nên key dạng văn bản trong tệp env vẫn đọc được | Lấy key từ kho bí mật: `AGENT_CLI_API_KEY="$(pass show ...)"` |
| P3 | Installer build khác cấu hình chính thức (`CGO_ENABLED=0`, `GOEXPERIMENT=greenteagc`); `agent-cli-tmux` là symlink vào bản checkout. Phiên bản thì đã có từ VCS (`v0.0.0-<ngày>-<hash>`), không phải `devel` | Thêm biến build khi cần |
| P3 | Chuỗi nội bộ còn chữ "crush" (`crush server`, trang OAuth, icon thông báo) | Giữ để merge upstream rẻ |
| P3 | 5 commit của hai nhánh mang email tài khoản A trong repo của tài khoản B | Chỉ là metadata; viết lại phải force-push |
| P2 | Cổng "không đổi model khi có run" mới chặn ở TUI; `run`/`send` có đổi model và `UpdateAgent` ở backend vẫn đổi model dùng chung giữa chừng run của session khác | Gốc là model chung cho cả workspace: đưa model vào tham số từng run |
| P3 | Biến mức quyền/đường dẫn của tool giao việc là env của tiến trình server (đã ghi trong docs) | Đưa vào `option` của crushrc để nạp lại theo workspace |
| P3 | Đổi tên session vẫn là đọc rồi ghi hai lượt; `session.Service.Rename` nguyên tử có sẵn nhưng chưa nối ra Workspace/server | Thêm `RenameSession` qua cả đường client/server |
| P3 | Huỷ/timeout giao việc: tiến trình con của CLI giữ stdout thì `cmd.Run` có thể treo quá hạn; CLI con thừa kế env của server (kể cả `CLAUDECODE*` nếu agent-cli được mở từ một phiên Claude) | `cmd.WaitDelay` + tín hiệu mềm; dựng `cmd.Env` tường minh |
| P3 | Giá trị sai của `AGENT_CLI_CLAUDE_PERMISSION_MODE`/`AGENT_CLI_CODEX_SANDBOX` chỉ lộ khi chạy, sau khi đã duyệt; ký tự điều khiển trong prompt hiện thẳng trong hộp thoại | Kiểm giá trị lúc dựng tool; lọc ký tự điều khiển khi hiển thị |

Các mục P3 nhỏ khác (thứ tự gửi `reportCurrentSession`, lịch sử prompt khi đổi session, Alt+W thoát không hỏi, `question` đóng kênh hai lần, tên máy trong định danh dự án...) nằm trong báo cáo chi tiết của từng người soát; chưa mục nào chặn việc dùng hằng ngày.

Sự cố trong lúc soát: một lệnh thử của người soát vòng 2 (`claude -p --permission-mode default -- "hi"`) chạy thật một lượt Claude (khoảng 0,37 USD theo giá API, không tool nào chạy).

## Chạy thật các luồng

| Luồng | Kết quả |
|---|---|
| agent-cli → Claude Code | ĐẠT: tạo đúng tệp, resume nhớ hội thoại, prompt bắt đầu bằng `-`, mức `dontAsk` chặn ghi thật (local + server) |
| agent-cli → Codex (bản đi kèm app ChatGPT desktop) | ĐẠT: như trên; `read-only` chặn ghi thật |
| LLM chính GLM | CHƯA: máy chưa có key; điền `~/.config/agent-cli/env` rồi chạy một prompt có tool call |
| Mức mặc định toàn quyền của CLI con | CHƯA chạy thật: bộ phân loại an toàn của Claude Code chặn phiên Claude tự chạy agent toàn quyền; đã có unit test cho cờ |
| Hộp thoại xin quyền giao việc trong TUI | Unit test chứng minh có hỏi và từ chối thì không chạy; chưa xem bằng mắt |
