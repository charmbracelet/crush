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

## 2026-10-15 — Đổi model hai tầng: alt+p chọn provider, alt+m chỉ hiện model của provider đang dùng
**Trạng thái:** đã làm
- **Chốt:** (commit 133eb476, 6e67f1f0) alt+p mở hộp thoại Providers, liệt kê theo `options.visible_providers` (crushrc `option visible-provider`, đúng thứ tự khai; không khai thì lấy các provider đã cấu hình), provider đang dùng đứng đầu, mỗi dòng có trạng thái `current`/`ready`/lệnh đăng nhập/`needs API key`. Chọn một provider thì mở bảng model chỉ gồm model của provider đó. alt+m mở thẳng bảng lọc theo provider của model lớn đang dùng; nếu provider không còn model nào thì mở bảng đầy đủ kèm cảnh báo. alt+e chọn mức effort (chỉ khi model có reasoning levels). alt+. / alt+, đổi session vòng tròn.
- **Vì:** ngày 15/10, khi đang dùng GLM, bảng alt+m cũ hiện model của mọi provider trong catalog (DeepSeek…). User yêu cầu tách provider và model như app Codex. Alt+Tab không dùng được vì Cinnamon chiếm phím (`switch-windows ['<Alt>Tab']`), phím không tới terminal. Đã kiểm trong tmux 15/10: alt+p hiện Z.AI (current), OpenAI (`agent-cli login openai`), Anthropic (`needs API key`); alt+m hiện "Switch Model · Z.AI" chỉ gồm GLM; alt+e hiện Low/High/Max; với hai session, alt+. đi tới rồi quay vòng, alt+, lùi lại.
- **Đã thử và bỏ:**
  - (a) `option default-providers false` để giấu provider khác: bỏ vì như vậy là bỏ luôn catalog, OpenAI/Anthropic không còn model nào để chọn kể cả khi đã đăng nhập.
  - (b) alt+tab để đổi session: hệ điều hành nuốt phím.
  - (c) Bảng lọc theo provider dùng chung bước dọn "model gần đây không tìm thấy": bỏ vì bước này xoá model gần đây của provider khác khỏi cấu hình. Chỉ bảng đầy đủ mới dọn (`TestModelsDialogScopedToProvider`).
- **Lật lại khi:** upstream có bộ chọn provider riêng; hoặc user muốn thấy mọi model trong một bảng thường xuyên hơn đổi provider.

## 2026-10-15 — Mặc định an toàn cho hộp thoại xin quyền và hộp thoại init
**Trạng thái:** đã làm
- **Chốt:** (commit 133eb476)
  - Hộp thoại xin quyền:
    - Mở ra với Deny được chọn sẵn, nên Enter ngay là từ chối.
    - Không chữ cái nào cấp quyền: cho phép một lần là `ctrl+a`, cả session là `ctrl+s` (hoặc chọn nút bằng ←/→ rồi Enter). `d`/Esc là từ chối.
    - Chỉ mũi tên trái/phải chuyển nút; h/l/Tab không chuyển.
  - Hộp thoại init (dự án chưa có AGENTS.md) chọn sẵn "Nope".
- **Vì:**
  - Sự cố: 15/10 08:12, user mở agent-cli trong `~/application`, chưa gõ gì mà init đã chạy. Kết quả: 10 lệnh bash, 0 lần từ chối, ghi `AGENTS.md` 10,6 KB, đọc tệp ngoài dự án.
  - Nguyên nhân: dựng lại cho thấy TUI vẫn hỏi quyền. Phím lạc rơi vào nút mặc định (Enter = Yep/Allow, `s` = allow session).
  - Soát độc lập E (P1): gõ chữ khi hộp thoại đang mở (vd "fix the bug") thì h/l/Tab chuyển lựa chọn sang Allow, rồi Enter là cho phép (`TestPermissions_TypingThenEnterDenies`).
  - Kiểm trên bản cài 15/10: GLM gọi tool write thì hộp thoại tô sẵn Deny; nhấn Enter ra "User denied permission" và tệp không được tạo.
- **Đã thử và bỏ:**
  - (a) Giữ Allow làm mặc định cho nhanh (hành vi upstream): bỏ vì chỉ một lần Enter lạc là lệnh chạy.
  - (b) Đổi mặc định nhưng giữ h/l/Tab: bỏ vì gõ chữ vẫn đưa lựa chọn về Allow.
  - (c) Giữ `a` = cho phép một lần, `S` = cả session (bản đầu của mục này): bỏ vì hộp thoại chỉ nuốt phím trong tối đa 1,5 giây đầu (dialog.go grace), gõ tiếp một câu có chữ `a` ("and add tests") là Allow; câu mở đầu bằng "Sửa..." có `S`.
- **Lật lại khi:** có allowlist/yolo dùng thường xuyên tới mức hộp thoại quyền hiếm khi hiện; hoặc user thấy `a` chậm hơn Enter đáng kể.

## 2026-10-15 — GLM đi qua provider dựng sẵn `zai` của catalog, không qua provider "main" tự khai
**Trạng thái:** đã làm
- **Chốt:** (commit 6e67f1f0) crushrc mẫu đọc `AGENT_CLI_PROVIDER_ID`. Nếu đặt một ID có trong catalog (vd `zai`), crushrc chỉ ghi `api_key`; endpoint, model, tên và mức effort lấy từ catalog nhúng (catwalk v0.52.49: `https://api.z.ai/api/coding/paas/v4`, glm-5.3, glm-5.3-flash…). Nếu bỏ trống, giữ provider "main" tự khai bằng `AGENT_CLI_BASE_URL`, cho model local hoặc endpoint không có trong catalog.
- **Vì:**
  - Provider "main" tự khai không có `can_reason`, nên việc đặt tiêu đề session chỉ được 40 token. GLM suy luận dùng hết số token đó, nên mọi session đều tên "Untitled Session".
  - Không có reasoning levels thì alt+e không có gì để chọn.
  - Qua `zai` (đo 15/10): session có tiêu đề thật ("Kiểm tra GLM qua provider Z.AI"), alt+e có Low/High/Max, `agent-cli run` trả PONG, việc code có tool call chạy đúng.
- **Đã thử và bỏ:**
  - (a) Chỉ thêm `--can-reason` cho "main": tiêu đề hết lỗi, nhưng vẫn không có mức effort và phải tự khai context/max tokens. Cách này giữ lại cho endpoint ngoài catalog (`AGENT_CLI_CAN_REASON`).
  - (b) Đặt `--name` cho provider catalog qua `AGENT_CLI_PROVIDER_NAME`: tên trong catalog thắng nên `--name` bị bỏ qua. Biến này chỉ áp cho "main".
  - (c) Thêm `option default-providers false`: bỏ việc gộp catalog, nên `zai` mất endpoint và danh sách model.
- **Lật lại khi:** catalog nhúng bỏ hoặc đổi `zai`. Catalog đang khoá (`option provider-auto-update false`), nên chỉ đổi khi nâng catwalk. Hoặc z.ai đổi endpoint của gói Coding.

## 2026-10-15 — Mỗi lượt chạy ghim model và bộ tool của nó; bỏ chặn đổi model/effort khi đang chạy
**Trạng thái:** đã làm
- **Chốt:** `SessionAgentCall.pinnedModel` (coordinator gán đúng model mà ProviderOptions được tính cho) và `pinnedTools` (gán lúc xếp hàng). Mỗi bước lấy model qua `stepModel`: chỉ nhận bản dựng lại của CHÍNH provider/model đó (làm mới token OAuth), không thì giữ model của lượt. Bộ tool qua `stepTools`: bản mới nhất nếu model chung chưa đổi (để nhận tool MCP mới), không thì bộ tool của lượt (tool `agent` mang agent con của đúng model). Prompt xếp hàng chỉ được gộp vào lượt đang chạy khi cùng model. TUI bỏ cổng "Agent is busy" khi đổi model/effort và khi mở lại session (khôi phục model chạy nền trong `tea.Cmd`, lần lượt, lần sau thắng).
- **Vì:** fantasy gọi `ModelProvider` ở MỌI bước và nó đọc model chung của workspace; `coordinator.run` còn gọi `updateAgentModels` ở đầu mỗi lượt gửi, nên chỉ cần gửi một prompt ở session B (hay `agent-cli run --model`) là bước kế của run ở session A đổi provider. Test chứng minh: đổi model giữa bước 1 và 2 thì bước 2 vẫn chạy model cũ (đỏ khi bỏ bản sửa).
- **Đã thử và bỏ:**
  - (a) Chặn ở TUI (bản trước): không phủ đường `run`/`send`/backend, và chặn luôn việc đổi model ở session khác khi có run nền.
  - (b) Chặn ở backend (`UpdatePreferredModel` trả bận khi có run): an toàn nhưng làm hỏng đa session (không đổi được model ở B khi A chạy).
  - (c) Model riêng từng session ở coordinator: đúng nhất nhưng phải đổi API model của UI, backend, CLI.
- **Lật lại khi:** cần model khác nhau chạy CÙNG LÚC ở nhiều session theo cấu hình riêng (không chỉ "lượt giữ model lúc gửi") thì làm (c).

## 2026-10-15 — Huỷ theo RunID; lượt của `agent-cli run` gắn với đời tiến trình `run`
**Trạng thái:** đã làm
- **Chốt:** `CancelRun(session, runID)` qua agent → coordinator (cả Stop gate) → backend → `POST .../agent/sessions/{sid}/runs/{rid}/cancel` → client. Lượt đang chạy đúng RunID thì huỷ; còn trong hàng đợi thì gỡ đúng nó (kèm RunComplete cancelled); chưa tới thì ghi dấu 2 phút, tới là huỷ ngay lúc vào. `run` (Ctrl+C, SIGTERM, SIGHUP) huỷ theo RunID cả với `-s/-C`; server cũ (404) thì lùi về cách cũ. `run` gửi kèm `bind_to_client`: client mất claim (thoát, kill -9, mất kết nối quá detach grace) thì server huỷ lượt đó. `send` cố ý không gắn.
- **Vì:** `Cancel(session)` huỷ cả lượt của TUI lẫn hàng đợi người khác, nên trước đây `run -s` bấm Ctrl+C chỉ "thôi chờ" và để prompt chạy sau không ai giám sát; `run` bị kill -9 để lại lượt chạy tiếp tới 30 phút. E2E thật (`test-agent-cli-run-json.sh`): kill -9 → `canceled`; Ctrl+C `run -s` → prompt bị gỡ, lượt `send` vẫn xong.
- **Đã thử và bỏ:** (a) "thôi chờ" cho `-s/-C` (bản trước); (b) huỷ cả session khi Ctrl+C; (c) đánh dấu "không sống khi client chết" theo session thay vì theo lượt: TUI và `send` dùng chung session.
- **Lật lại khi:** coordinator có huỷ theo lượt sẵn từ upstream, hoặc cần huỷ một lượt `send` từ xa (thêm lệnh, không đổi ngữ nghĩa này).

## 2026-10-15 — Trần mồ côi 30 phút chỉ tính khi run kẹt; thêm trần tuyệt đối 6 giờ
**Trạng thái:** đã làm
- **Chốt:** `CRUSH_SERVER_ORPHAN_RUN_TIMEOUT` (30 phút) chỉ đếm khi run mồ côi đang chờ lời xin quyền/câu hỏi (`Permissions.ActiveRequest`, `Questions.Pending`); chạy lại thì đồng hồ tính lại. Run vẫn tiến triển sống tới `CRUSH_SERVER_ORPHAN_MAX_RUN` (mặc định 6 giờ, `0` = không trần).
- **Vì:** đồng hồ cũ tính từ lúc client rời nên việc giao bằng `send` dài hơn 30 phút bị huỷ giữa chừng, chỉ có log Warn.
- **Đã thử và bỏ:** (a) một trần 30 phút từ lúc client rời (bản trước); (b) không trần cho run tiến triển: run lặp vô tận giữ server mãi.
- **Lật lại khi:** có việc giao hợp lệ dài hơn 6 giờ (đặt biến), hoặc có tín hiệu "run lặp" tốt hơn đồng hồ.

## 2026-10-15 — Tool đọc (`grep`, `glob`, `view`, `ls`) hỏi quyền khi chạm ngoài dự án, kể cả qua symlink
**Trạng thái:** đã làm
- **Chốt:** `internal/agent/tools/outside.go`: giải symlink cả hai phía rồi mới so; `grep` (action read) và `glob` (action list) nhận permission service và hỏi như `view`/`ls`.
- **Vì:** `grep`/`glob` không có cổng nào, nên model đọc được `~/.config/agent-cli/env` (chứa key GLM) mà không hỏi; `view`/`ls` so đường dẫn không giải symlink nên `docs -> ~/.ssh` trong một repo lạ qua mặt được. Test đỏ khi bỏ bản sửa.
- **Đã thử và bỏ:** (a) chỉ khuyên lấy key từ kho bí mật (`pass show`): không chặn được tool; (b) allowlist đường dẫn trong crushrc: người dùng phải tự liệt kê, và `permissions allow` bỏ luôn lời hỏi.
- **Lật lại khi:** upstream thêm cổng đường dẫn chung cho mọi tool đọc.

## 2026-10-15 — `agent-cli run` qua server là lượt phi tương tác; KHÔNG tách agent (plan/coder) theo lượt
**Trạng thái:** đã làm
- **Chốt:** `run` gửi `non_interactive`: lượt không có tool `question` và chờ MCP khởi động xong (có trần) như đường local. Chế độ plan của workspace vẫn áp cho `run` (ghi rõ trong `run --help`, server ghi Warn).
- **Vì:** trước đây `run` qua server dùng coordinator tương tác: model gọi `question` là run hỏng (rc 1), lượt đầu sau khi server lên có thể thiếu tool MCP.
- **Đã thử và bỏ:** cho `run` luôn chạy agent coder dù workspace đang plan: `Cancel`/`IsBusy`/hàng đợi của coordinator chỉ hỏi agent chính, nên lượt chạy trên agent khác vô hình với Ctrl+C và với bước teardown.
- **Lật lại khi:** coordinator theo dõi lượt trên mọi agent (khi đó tách agent theo lượt được).

## 2026-10-15 — Câu hỏi chờ tối đa 30 giây sau câu hỏi của session khác
**Trạng thái:** đã làm
- **Chốt:** dịch vụ câu hỏi vẫn hiện một câu một lúc; câu thứ hai chờ tối đa `askQueueTimeout` (30 giây) rồi trả `ErrQuestionBusy` cho model. `Cancel` lấy kênh ra dưới khoá (hai client huỷ cùng lúc không còn panic).
- **Vì:** câu hỏi xếp hàng sau câu hỏi của một session không ai xem thì vô hình (không sự kiện, không WAITING) và chặn run mãi.
- **Đã thử và bỏ:** nhiều câu hỏi chờ song song theo session: phải đổi API trả lời/huỷ (không có ID lô) ở server, client, TUI.
- **Lật lại khi:** cần hai session cùng hỏi người dùng thường xuyên.

## 2026-10-15 — Kiểm tra bản mới của upstream mặc định TẮT trong binary
**Trạng thái:** đã làm
- **Chốt:** chỉ bật khi `CRUSH_DISABLE_UPDATE_CHECK` là giá trị false của `strconv.ParseBool`; không đặt, rỗng, true hay giá trị lạ đều không gọi GitHub.
- **Vì:** chạy binary không qua launcher vẫn gọi GitHub của upstream mỗi lần mở và mời "nâng cấp" lên bản Crush; `0`/`false` trước đây lại bị hiểu là tắt.
- **Đã thử và bỏ:** giữ mặc định bật, chỉ tắt qua launcher (bản trước).
- **Lật lại khi:** fork có kênh phát hành riêng để so phiên bản.

## 2026-10-15 — Provider đang được chọn mà không có endpoint thì dừng lúc khởi động
**Trạng thái:** đã làm
- **Chốt:** `config/load.go`: provider tự khai không có `base_url` (vd gõ sai `AGENT_CLI_PROVIDER_ID`) mà đang là model large/small thì báo lỗi nêu tên provider; provider không ai chọn thì giữ hành vi cũ (cảnh báo rồi bỏ).
- **Vì:** trước đây provider bị bỏ lặng lẽ, agent tự chọn model khác rồi ghi đè lựa chọn xuống đĩa (luật: cấu hình hỏng thì chặn lúc khởi động).
- **Đã thử và bỏ:** chặn mọi provider thiếu endpoint: làm hỏng cấu hình không liên quan và chặn `agent-cli login`.
- **Lật lại khi:** catalog có cách khai provider tạm thời không endpoint mà vẫn hợp lệ.

## 2026-10-15 — Launcher dừng khi không dựng được socket riêng cho dự án
**Trạng thái:** đã làm
- **Chốt:** đường dẫn socket quá dài hoặc thiếu `sha1sum`/`shasum` thì wrapper thoát mã 1 kèm cách sửa (trừ `--host` tường minh, `ps`, Home, `CRUSH_CLIENT_SERVER=0`).
- **Vì:** trước đây wrapper chỉ cảnh báo rồi chạy không `--host`, rơi về server mặc định dùng chung mọi dự án: phá cách ly theo dự án mà người dùng không biết.
- **Đã thử và bỏ:** chọn thư mục ngắn hơn (`/tmp`): bị loại có chủ ý vì ai cũng ghi được và tên socket đoán được.
- **Lật lại khi:** có thư mục runtime riêng ngắn hơn trên mọi máy dùng.

## 2026-10-15 — Port sớm ba bản vá upstream cho P1.4/P1.5
**Trạng thái:** đã làm
- **Chốt:** lấy nguyên văn `fix/dangling-tool-calls` (settle tool call bị bỏ dở khi đọc session, `settle.go`), `fix/shutdown-records-work` (server nhận SIGTERM/SIGHUP thì dừng từng workspace) và ý của `fix/background-job-budget` (`0ac04e4f`: chỉ job đang chạy tính vào trần 50). Fork thêm: session con chỉ settle khi cả chuỗi session cha rảnh (`BusyWithAncestors`), và câu ghi cho model không mời chạy lại mù quáng.
- **Vì:** cổng WIN "Đóng/mở client" và "Context/mạng lỗi" (không hiện trạng thái giả, không lặp mutation); trần job tính cả job đã xong làm mọi lệnh bash hỏng khi sub-agent chạy song song.
- **Đã thử và bỏ:** tự viết bản sửa riêng: lệch upstream, merge sau tốn hơn; settle chỉ theo `IsSessionBusy` như upstream: sub-agent chạy trên agent riêng nên mở session đang chạy sub-agent sẽ ghi "Interrupted" lên tool call còn sống; settle không khoá: hai người đọc song song (TUI và `run`) ghi trùng kết quả (fork thêm khoá theo session).
- **Còn hở:** kiểm "bận" và lúc ghi chưa nguyên tử: một lượt mới bắt đầu và gọi tool trong vài mili-giây giữa hai bước có thể bị ghi "Interrupted" kèm kết quả thật (soát độc lập xếp P3, hiếm).
- **Lật lại khi:** upstream merge các nhánh đó (lấy bản của upstream, giữ hai chỗ fork nếu upstream chưa có).

## 2026-10-15 — Lượt kết thúc thế nào thì prompt xếp hàng phía sau vẫn chạy
**Trạng thái:** đã làm
- **Chốt:** lượt xong, lỗi (kể cả lỗi provider), bị CancelRun (Ctrl+C của `run`, client `run` bị giết), bị huỷ ngay lúc vào Run hay lúc đang tự tóm tắt đều chuyển tiếp cho prompt xếp hàng kế (`handOff`/`nextQueued` dưới khoá session, có bộ đếm prompt "đang chuyển" để giữ thứ tự); lượt lỗi vẫn phát `TypeAgentError` của nó. Huỷ cả session (Esc) vẫn bỏ các prompt mà cancel mark phủ. `ClearQueue` lấy khoá session.
- **Vì:** hai lượt soát độc lập 15/10: prompt phía sau kẹt tới lượt sau, người chờ RunID của nó không nhận RunComplete, session không xoá được; huỷ lúc tóm tắt bị nuốt; Stop hook đánh dấu nhầm prompt chuyển tiếp là Cancelled.
- **Đã thử và bỏ:** chỉ chuyển tiếp khi bị huỷ, giữ hàng đợi khi lỗi provider với lý do "bộ điều phối thử lại lượt lỗi": sai, `coordinator.run` gọi Run một lần (thử lại khi lỗi xác thực nằm trong fantasy).
- **Lật lại khi:** cần dừng cả hàng đợi khi provider hỏng hẳn (khi đó báo lỗi cho từng prompt thay vì chạy chúng).

## 2026-10-15 — Everforest Dark v2 là theme mặc định cho mọi bề mặt
**Trạng thái:** đã làm
- **Chốt:** `styles.DefaultThemeName = everforest-dark-v2` (`internal/ui/styles/themes.go`); tên rỗng, tên sai và mọi provider (kể cả hyper) đều ra theme này; màn ngoài TUI chính (Home, banner thoát, `login`/`logout`, help CLI, dòng in của `dirs`/`session`/`update-providers`) dùng `styles.Color*` lấy từ cùng token. `charmtone-panther` và `gruvbox-dark` giữ nguyên, chọn được trong Themes.
- **Vì:** người dùng chốt theme mới làm mặc định, không phải một tuỳ chọn thêm.
- **Đã thử và bỏ:** chỉ thêm theme như một lựa chọn (bản đầu): người dùng phải tự chọn, Home và banner vẫn màu cũ.
- **Lật lại khi:** người dùng chọn lại theme khác làm mặc định.

## 2026-10-15 — Jobs mở bằng Alt+J, dừng job luôn hỏi lại
**Trạng thái:** đã làm
- **Chốt:** hộp thoại Jobs (Alt+J, mục "Jobs" trong Ctrl+P) đọc `ListJobs`/`JobOutput`/`KillJob` trong `tea.Cmd`, làm tươi 1 giây; `Ctrl+K` hỏi "Stop job?" mặc định No, chỉ ←/→ đổi nút. Message của hộp thoại đi theo ID (`TargetedMsg`) nên vòng làm tươi sống cả khi hộp thoại xin quyền đè lên.
- **Vì:** roadmap P1.4 (DoD: panel có xem output và cancel); phím chữ hay phím lặp không được thành xác nhận dừng job.
- **Đã thử và bỏ:** bấm Ctrl+K lần hai để xác nhận: giữ phím sẽ tự lặp thành xác nhận; chỉ báo "N jobs running" trên sidebar: cần poll liên tục và đổi bố cục.
- **Lật lại khi:** cần chỉ báo job ngoài hộp thoại (thêm vào Activity của sidebar ở Phase 2).

## 2026-10-15 — Ranh giới provider/model: allowlist, giao việc mặc định tắt
**Trạng thái:** đã làm
- **Chốt:** `internal/execpolicy` là ranh giới duy nhất, đọc từ env đã export rồi tệp env của agent-cli (chỉ hai khoá, không qua cấu hình dự án). `AGENT_CLI_ALLOWED_MODELS` (provider:model) giới hạn model chính/nhỏ khi có provider chính trong danh sách; `AGENT_CLI_DELEGATION=on` mới đăng ký `claude_code`/`codex`, chỉ với model cụ thể của chúng trong danh sách, luôn truyền `--model`/`-c model=`. Điểm chặn: `buildAgentModels` (mọi lượt, sub-agent, tiêu đề), `provider.NewTools` + lúc gọi tool (trước khi hỏi quyền), hộp thoại Alt+M, tool bash (chặn gọi thẳng CLI agent).
- **Vì:** người dùng 15/10: session GLM/Qwen tự giao việc sang Claude Code vì CLI có trên máy ("discovered = executable"), và CLI con chạy model mặc định của nó (có thể là Fable). Cài/đăng nhập/có key không phải là cho phép.
- **Đã thử và bỏ:** chỉ đọc env của tiến trình: tệp env chỉ được crushrc `source` trong trình thông dịch riêng nên biến không tới Go; đưa qua crushrc thành `options.allowed_models`/`options.delegation`: cấu hình của dự án gộp vào nên repo lạ tự bật được giao việc; denylist (fable, astra): model mới lọt qua; nhận alias (opus, latest): CLI tự đổi sang model mới nhất.
- **Lật lại khi:** Claude/Codex thành agent chính trực tiếp (Phase 2): khi đó provider của session là claude_code/codex và ranh giới theo provider của session.

## 2026-10-15 — Tự tóm tắt hỏng thì làm tiếp, không dừng run
**Trạng thái:** đã làm
- **Chốt:** request tóm tắt có hạn chờ gấp 3 hạn thường (`summaryRequestTimeoutScale`, mặc định 6 phút vì `request-timeout` mặc định 2 phút). Tóm tắt vẫn hỏng (không phải bị huỷ) thì lượt KHÔNG dừng: lịch sử giữ nguyên (bản tóm tắt hỏng không thành điểm cắt, không gửi lại cho model), còn khoảng 20% context nên lượt làm tiếp được xếp ngay; tự tóm tắt tạm tắt cho session tới prompt mới của người dùng. Huỷ lúc tóm tắt vẫn dừng lượt.
- **Vì:** người dùng 15/10: stream tóm tắt im 2 phút → timeout → lỗi trả thẳng ra, run dừng giữa việc (tái hiện bằng test, đỏ trên mã cũ).
- **Đã thử và bỏ:** thử lại ngay một lần: provider im vì đầu vào lớn thì thử lại cũng im, người dùng chờ gấp đôi; bản tóm tắt cục bộ (cắt lịch sử) khi hỏng: mất ngữ cảnh mà vẫn còn chỗ để làm tiếp.
- **Lật lại khi:** có lỗi "context đầy" từ provider được bắt riêng (roadmap P1.5) để tóm tắt bằng model nhỏ/ngân sách nhỏ thay vì chờ provider báo lỗi.

## 2026-10-15 — Trả lời bị cắt vì hết ngân sách output thì tự làm tiếp (có trần)
**Trạng thái:** đã làm
- **Chốt:** lượt kết thúc với finish `max_tokens` thì tự xếp lượt làm tiếp cùng RunID với lời nhắc "làm tiếp đúng chỗ dừng, chia nhỏ việc"; tối đa `maxLengthContinues` = 3 lần cho mỗi prompt người dùng. crushrc mẫu đặt ngân sách output mặc định của provider `main` = min(32000, context/4) thay cho 8192.
- **Vì:** người dùng 15/10: phiên Qwen hết ngân sách output giữa việc rồi dừng; model có suy luận tiêu phần lớn ngân sách 8192 cho phần nghĩ.
- **Đã thử và bỏ:** làm tiếp không giới hạn: model cứ viết quá dài sẽ lặp mãi.
- **Lật lại khi:** provider cho biết trước trần output thật của model (khi đó đặt ngân sách theo model thay vì theo context).

