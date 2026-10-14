# Đa session + đa dự án cho agent-cli — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Người thi hành:** Opus effort high, **được quyền phản biện plan**. Thấy giả định sai thì dừng, ghi lại, sửa plan rồi mới code.
> **Cấm:** viết số chưa chạy lệnh · nở phạm vi ngoài các task dưới · commit/push khi user chưa cho phép.

**Goal:** Trong một cửa sổ terminal, thấy mọi session (và mọi dự án) kèm trạng thái, nhảy qua lại bằng một phím, cho nhiều agent chạy song song, và được báo khi session nền xong hoặc cần duyệt.

**Architecture:** Lõi đã chạy song song theo session (agent) và theo dự án (một server cho nhiều workspace). Việc còn lại nằm ở TUI: một bộ theo dõi trạng thái theo session, một panel danh sách có phím nhảy, đổi khái niệm "bận" từ cả workspace sang từng session, và giữ lại prompt của session nền. Phần đa dự án dùng tmux cộng một server chung (`CRUSH_CLIENT_SERVER=1`) và lệnh `agent-cli ps`. Không làm chuyển dự án ngay trong một TUI; lý do ở mục "Không làm".

**Tech Stack:** Go 1.27, Bubble Tea v2, lipgloss v2, testify, tmux.

**Spec:** yêu cầu của user trong phiên 14/10/2026, tóm tắt ở mục "Yêu cầu" (không có tệp spec riêng).

## Yêu cầu

- **R1** Luôn thấy danh sách session của dự án hiện tại (thanh bên và màn landing). Mỗi dòng có trạng thái: rảnh, đang chạy, chờ duyệt, xong chưa xem, lỗi chưa xem.
- **R2** Nhảy session bằng một phím (`alt+1`..`alt+9`), không phải mở hộp thoại.
- **R3** Tạo session mới và gửi prompt trong khi session khác đang chạy. Hiện tại thao tác này bị chặn.
- **R4** Khi session nền xin quyền hoặc hỏi: không bật hộp thoại đè lên session đang xem. Đánh dấu "chờ duyệt", hiện toast, và mở hộp thoại khi chuyển sang session đó.
- **R5** Khi session nền xong hoặc lỗi: hiện toast; nếu cửa sổ không được focus thì gửi thêm thông báo hệ thống.
- **R6** Mở nhiều dự án song song trong một cửa sổ terminal bằng một lệnh.
- **R7** Một lệnh xem toàn cảnh: mọi dự án × session × trạng thái.
- **R8** (bổ sung 14/10) Danh sách session phải bắt được session do **mọi nguồn** tạo ra: TUI khác, script `agent-cli run`, và sub-agent (session con của agent tool). Trạng thái phải đúng bất kể process nào đang chạy session đó.

- **R13** (bổ sung 14/10) Nhanh, không lag, không nhấp nháy khi có nhiều session (khoảng 50) và vài session cùng chạy.
- **R12** (bổ sung 14/10) Shift+Tab đổi mode gồm auto (accept edits), plan, yolo. Làm ở A10.
- **R11** (bổ sung 14/10) Tự cấu hình tổ hợp phím cho hợp với dev, làm mọi việc trong một app. Làm ở A9.
- **R10** (bổ sung 14/10) Phím kiểu alt-tab của Windows để đổi session và dự án (Shift+Tab vẫn là đổi mode). Làm ở A8 và A8b.
- **R9** (bổ sung 14/10) Mỗi việc song song có pwd riêng, model/provider riêng, danh sách sub-agent riêng, log và DB riêng; các việc không đụng file của nhau. Phần này làm ở plan 2 (`2026-10-14-isolated-parallel-sessions.md`); plan này chỉ đổi B0 và B2 cho khớp.

**Ruling R9 (thay phần "một server chung" của R8):** dùng **một server cho mỗi thư mục dự án**. Wrapper tự thêm `--host unix:///run/user/$UID/agent-cli-<8 hex đầu của sha1(thư mục gốc)>.sock` nếu user không truyền `--host`. Thư mục gốc = `git rev-parse --show-toplevel` của `-c/--cwd` (hoặc `$PWD`); không phải repo git thì lấy `realpath`. Lý do: log server ghi theo host (`server.go`: `server-<host>/crush.log`) và log là toàn cục theo process (`log.Setup` dùng `sync.Once`), nên một server chung sẽ trộn log của mọi dự án. Khi mỗi dự án có server riêng thì log và DB tách riêng, còn TUI và script cùng thư mục vẫn chung một server nên trạng thái vẫn đúng (R8). Nếu ruling sai: số process server tăng theo số dự án đang mở (mỗi server tự tắt sau 60 giây rảnh).

**Ruling R8 — một nguồn sự thật:** launcher bật mặc định `CRUSH_CLIENT_SERVER=1`; ai cần thì tắt bằng `CRUSH_CLIENT_SERVER=0`. Khi đó mọi TUI và mọi script của user đi qua **một** server, nên `ListSessions`, trạng thái bận và trạng thái chờ duyệt đều đúng cho mọi process. Session con lấy từ sự kiện message, vốn không bị server lọc. Ở chế độ in-process vẫn chạy được: danh sách vẫn đủ (đọc từ DB), nhưng trạng thái chỉ đúng cho session do chính process đó chạy; giới hạn này được ghi vào docs. Nếu ruling sai: client/server có lỗi khi dùng hằng ngày; gỡ bằng một dòng trong launcher.

## Sự thật nền (14/10)

**Đã chứng minh bằng lệnh:**
- Gateway qwen chịu 2 request song song. `agent-cli run` chạy một mình mất 7,0 giây; hai lệnh song song mất 6,4 và 5,6 giây, cả hai trả "OK". Đây là 2 process với prompt rất ngắn.
- Một server phục vụ được 2 dự án cùng lúc. Chạy song song `CRUSH_CLIENT_SERVER=1 agent-cli run -c projA` và `-c projB`: cả hai rc=0, trả "OK" (7,0 và 6,0 giây). `pgrep` thấy đúng một `crush server`. Server tự tắt khi rảnh (mặc định 60 giây, chỉnh bằng `CRUSH_SERVER_IDLE_TIMEOUT`).

**Mới đọc code, chưa chạy:**
- **Agent:** prompt chỉ xếp hàng khi chính session đó đang bận (`sessionAgent.Run`, nhánh `IsSessionBusy` trong `internal/agent/agent.go`), nên các session khác chạy song song được. Task A1 sẽ chứng minh.
- **TUI:** `isAgentBusy()` là bận của cả workspace (`dispatchBusyRefresh` gọi `AgentIsBusy`). `ctrl+n` bị chặn ở 3 chỗ trong `ui.go` với chuỗi "please wait before starting a new session".
- **`question`:** `questionService.Ask` chỉ giữ một `pending`. Nếu hai session hỏi cùng lúc, câu sau đè câu trước và session trước chờ mãi. Task A2 sẽ chứng minh.
- **`permission`:** `permission.Request` giữ `requestMu` suốt thời gian chờ, nên các yêu cầu xin quyền xếp hàng theo cả workspace. Phía TUI, `openPermissionsDialog` không lọc theo session.
- **Server:** ở chế độ client/server, prompt xin quyền và câu hỏi chỉ được gửi cho client đang xem đúng session đó (`deliverToClient` trong `internal/server/events.go`).
- **Launcher:** mặc định chạy in-process, vì không đặt `CRUSH_CLIENT_SERVER`.
- **Session con** (tạo bởi agent tool) không có trong `ListSessions` (điều kiện `parent_session_id is NULL`).
- **Server gửi lại prompt đang chờ** khi client chuyển sang session đó: `republishPendingPrompts` ([backend/events.go](../../../internal/backend/events.go)), dùng `Permissions.ActiveRequest()`, `Questions.Pending()` và `sessionMatches` (đi ngược lên session gốc).
- `coordinator.IsSessionBusy` chỉ hỏi agent chính, nên session con của sub-agent không có cờ bận riêng. Trong lúc sub-agent chạy, session cha vẫn bận.
- `Backend.GetAgentSession` ([backend/session.go](../../../internal/backend/session.go)) hiện chỉ trả `IsBusy`.

## Global Constraints

- **Không sửa** `internal/agent/coordinator.go`, `internal/config/config.go`, `internal/config/load_test.go`, `internal/agent/tools/provider/`. Phiên khác đang sửa các file này.
- Máy dùng chung: mọi lệnh Go chạy với `GOMAXPROCS=2` và `-p 2`.
- Chỉ commit/push khi user cho phép. Khi được phép: mỗi task một commit, message semantic một dòng.
- Không làm IO đồng bộ trong `Update` hoặc `View` (luật ghi ở đầu `workspace_cache.go`). Mọi probe đi qua `tea.Cmd` và trả về msg.
- Chữ trên UI viết tiếng Anh như phần còn lại. Log viết hoa chữ đầu (`task lint:log`).
- Test dùng testify `require` và `t.Parallel()`. Stub workspace thì mở rộng `countingWorkspace` trong `internal/ui/model/session_busy_test.go`.
- Khi chỉ có một session, hành vi phải giữ nguyên: toàn bộ test hiện có của `./internal/ui/model` vẫn xanh.
- Phím `alt+1`..`alt+9` chưa ai dùng trong `keys.go`. `alt+n` chỉ có trong dialog câu hỏi.
- **Hiệu năng (R13):**
  - Mỗi lần làm mới chỉ được đọc một số session có giới hạn (tối đa `sessionsPanelMax` + 1 lời gọi trạng thái) và không làm IO trong `Update`/`View`.
  - Chuỗi panel chỉ dựng lại khi state đổi, không dựng mỗi frame.
  - Cổng đo: chạy benchmark render có sẵn của `internal/ui/model` (`go test -run '^$' -bench . -benchmem -count 5`) trên baseline và sau khi sửa, so bằng `benchstat` nếu có. Mọi tăng trên 10% ở ns/op hoặc allocs/op phải giải thích hoặc sửa. Số liệu ghi từ lệnh vừa chạy, không ước lượng.

## Review Focus

1. **Session nền xin quyền khi bạn đang gõ ở session khác:** hộp thoại không được bật lên, chỉ đánh dấu và hiện toast. Test `TestBackgroundPermissionIsHeld` ở A7.
2. **Session con của một session nền xin quyền:** đi theo session cha, không bật lên ở session đang xem. Test `TestChildSessionPromptRoutesToParent` ở A7.
3. **Bấm alt+N khi editor đang có chữ:** không gửi gì, chữ giữ nguyên. Test `TestAltNumberKeepsDraft` ở A6.
4. **Có hơn 9 session:** chỉ 9 session gần nhất được đánh số; session đang xem nằm ngoài top 9 vẫn hiện, có đánh dấu. Test `TestSessionsPanelPinsCurrentBeyondNine` ở A4.
5. **Xoá một session đang hiện trong panel:** dòng biến mất, tracker quên ID đó. Test `TestSessionsPanelDropsDeleted` ở A4.

## Không làm (và vì sao)

- **Chuyển dự án ngay trong một TUI.** `m.com.Workspace` là một đối tượng xuyên suốt `ui.go` (hơn 5000 dòng: cache, LSP/MCP, config riêng từng dự án). Đổi nóng nghĩa là viết lại tầng UI. tmux cộng server chung cho trải nghiệm tương đương mà gần như không đụng lõi. Chỉ xét lại khi B1–B2 dùng ổn mà vẫn thiếu.
- **Khoá file khi hai session sửa cùng file trong một dự án.** Quy ước thay thế: việc song song đụng cùng repo thì mỗi việc dùng một `git worktree` riêng, mở bằng B1 như một "dự án".
- **Bản nháp riêng cho từng session.** Editor giữ nguyên chữ khi chuyển session, giống `ctrl+s` hiện nay.
- **Đổi model khi có session đang chạy:** vẫn chặn, vì đổi model dựng lại agent của cả workspace.

## Đường vào

| File | Vai trò |
|---|---|
| `internal/agent/dispatch_race_test.go` | Harness `concurrencyProbeModel`, `testSessionAgent`, `testEnv` (A1) |
| `internal/question/question.go` | `questionService.Ask`, `NewService` (A2) |
| `internal/ui/model/ui.go` | Vòng `Update`, xử lý sự kiện (`pubsub.Event[...]`, `notify.RunComplete`), phím |
| `internal/ui/model/workspace_cache.go` | Mẫu probe ngoài luồng: gen, in-flight, TTL |
| `internal/ui/model/sidebar.go`, `landing.go`, `skills.go` | Cách dựng section; `skillsInfo` là mẫu chữ ký |
| `internal/ui/model/session.go` | `loadSession`, `reportCurrentSession` |
| `internal/ui/model/session_busy_test.go` | Stub `countingWorkspace` |
| `internal/server/events.go`, `internal/backend/events.go` | `deliverToClient`, `republishPendingPrompts`, `sessionMatches` (A3b) |
| `internal/client/proto.go` | `ListWorkspaces`, `ListSessions`, `GetAgentSessionInfo` (B2) |

## Chỗ tôi không chắc (kèm cách kiểm)

- **U1 — Server dọn workspace khi client cuối rời đi có giết run đang chạy không?** Đoạn teardown trong `backend.go` không thấy kiểm tra busy. Cách kiểm: đặt `CRUSH_CLIENT_SERVER=1`, gửi một prompt dài trong TUI, thoát TUI, chờ 90 giây, mở lại session xem câu trả lời có hoàn tất không. Nếu run bị giết: B1 ghi rõ "đừng đóng pane khi đang chạy", và báo user để quyết có thêm task giữ workspace khi còn bận hay không.
- **U2 — đã trả lời (đọc code):** `SetCurrentSession` gọi `republishPendingPrompts`, nên hộp thoại hiện lại khi chuyển sang. Vẫn nên thử tay một lần ở hàng 3 của bảng kiểm.
- **U3 — `RunComplete` có được phát ở mọi đường kết thúc (lỗi lúc setup, huỷ, hàng đợi) không?** Commit `a299286a` cho thấy có, nhưng chưa kiểm hết. Nếu thiếu, `Reconcile` chạy mỗi 2 giây sẽ sửa lại trạng thái.
- **U4 — Phím `alt+số` có tới được app trong terminal bạn dùng không?** Terminal tích hợp của VS Code có thể giữ phím này cho việc khác. Cách kiểm: chạy `cat -v`, bấm alt+1, phải thấy `^[1`. Nếu không tới: bật `terminal.integrated.sendKeybindingsToShell` trong VS Code. `ctrl+s` vẫn dùng được trong mọi trường hợp.
- **U5 — Tải song song thật trên qwen local.** Mới đo 2 request ngắn. Cách kiểm: cho 3 session chạy task thật cùng lúc, ghi thời gian từng câu trả lời so với khi chạy một mình.
- **U6 — Số dòng trong `ui.go` sẽ lệch.** Plan định vị bằng chuỗi nhận diện, không bằng số dòng.

## Cách chứng minh là xong

```bash
export GOMAXPROCS=2
go build -p 2 ./...
go test -p 2 ./internal/agent -run 'TestRun_TwoSessionsStreamConcurrently' -count=1 -v
go test -p 2 ./internal/question ./internal/ui/model ./internal/cmd -count=1
bash scripts/test-agent-cli-tmux.sh
```

Cổng đỏ/xanh: so **theo tên test** với ảnh chụp trước khi sửa. Chạy `go test -json ... | jq -r 'select(.Action=="fail").Test'` ở cả hai bản, kiểm tra số tên test lớn hơn 0, và không có tên nào chuyển từ xanh sang đỏ.

Kiểm tay (in-process, qwen):

| # | Thao tác | Kỳ vọng |
|---|---|---|
| 1 | Ở session A gửi prompt dài; `ctrl+n`; gõ prompt B rồi enter | B chạy; panel hiện A ● và B ●; không có cảnh báo busy |
| 2 | Đang ở B thì A xin quyền chạy bash | Không bật hộp thoại; A ▲; toast có tên A |
| 3 | `alt+<số của A>` | Sang A, hộp thoại xin quyền hiện; duyệt xong thì A ● |
| 4 | A xong khi đang ở B | A ✓, có toast; quay lại A thì A ○ |
| 5 | Bấm esc ở B khi chỉ A đang chạy | Không hiện "press esc again" |
| 6 | `ctrl+l` đổi model khi A đang chạy | Vẫn bị chặn (cố ý) |
| 7 | Editor có chữ "abc", bấm `alt+1` | Sang session 1, editor vẫn là "abc", không gửi gì |
| 8 | `agent-cli-tmux ~/p1 ~/p2` rồi `agent-cli ps` | Có 2 window; `ps` liệt kê 2 dự án, session đang chạy hiện busy |

---

## Phần A — Đa session trong một dự án

### Task A1: Chứng minh hai session chạy đồng thời (chỉ thêm test)

**Files:** Test: `internal/agent/dispatch_race_test.go`

**Interfaces:** Consumes `concurrencyProbeModel{inFlight, maxSeen, entered, release}`, `testSessionAgent`, `testEnv`, `fastModel` có sẵn trong file.

- [ ] **Step 1:** Viết `TestRun_TwoSessionsStreamConcurrently`. Tạo 2 session, gọi `sa.Run` cho mỗi session trong một goroutine riêng, `entered` có cap 2, chờ nhận đủ 2 tín hiệu `entered` (timeout 5 giây), rồi `require.Equal(t, int32(2), model.maxSeen.Load())`, sau đó `close(release)` và `wg.Wait()`.
- [ ] **Step 2:** Chạy `go test -p 2 ./internal/agent -run TestRun_TwoSessionsStreamConcurrently -count=1 -v`. Kỳ vọng PASS ngay, vì đây là test mô tả hành vi đang có. Nếu FAIL thì giả định nền của cả plan sai: dừng lại và báo user.
- [ ] **Step 3:** Commit `test(agent): pin concurrent runs across sessions` (nếu được phép).

### Task A2: `question.Ask` xếp hàng thay vì đè nhau

**Files:** Modify `internal/question/question.go` (struct `questionService`, `NewService`, `Ask`); Test `internal/question/question_test.go`.

**Interfaces:** Produces: hành vi `Ask` tuần tự trên toàn service; chữ ký giữ nguyên.

- [ ] **Step 1:** Viết `TestAskFromTwoSessionsBothAnswered`.
  - Goroutine A gọi `Ask` với session "a". Chờ event `Request` đầu tiên.
  - Goroutine B gọi `Ask` với session "b".
  - Gọi `Answer(ansA)`: A phải trả về đúng `ansA`.
  - Event `Request` thứ hai phải có `SessionID == "b"`. Gọi `Answer(ansB)`: B phải trả về `ansB`.
  - Mỗi bước có timeout 2 giây.
- [ ] **Step 2:** Viết `TestAskQueuedCancelledByContext`. B xếp hàng sau A, huỷ ctx của B: B trả `context.Canceled` trong vòng 2 giây, A vẫn nhận câu trả lời bình thường.
- [ ] **Step 3:** Chạy `go test -p 2 ./internal/question -count=1`. Kỳ vọng test đầu FAIL (A treo hoặc nhận nhầm câu trả lời của B). Nếu test PASS thì giả định "đè nhau" sai: giữ test làm chốt chống hồi quy, bỏ Step 4, và ghi lại điều này trong báo cáo.
- [ ] **Step 4:** Thêm trường `askSem chan struct{}` (cap 1, tạo trong `NewService`). Lấy slot trước khi gán `pending`, dùng `select` trên `ctx.Done()`; trả slot trong `defer` sau khi reset `pending`. Cùng mẫu với `requestMu` của `permission`, nhưng huỷ được bằng ctx.
- [ ] **Step 5:** Chạy lại lệnh ở Step 3. Kỳ vọng PASS toàn package.
- [ ] **Step 6:** Commit `fix(question): serialize concurrent asks across sessions`.

### Task A3: Bộ theo dõi trạng thái session (logic thuần)

**Files:** Create `internal/ui/model/session_activity.go`, `internal/ui/model/session_activity_test.go`.

**Interfaces:** Produces:
- `type sessionActivity uint8` với các hằng `activityIdle`, `activityWorking`, `activityWaiting`, `activityDoneUnseen`, `activityFailedUnseen`.
- `type sessionActivityTracker struct{ states map[string]sessionActivity }`, zero value dùng được.
- Các phương thức: `Get(id string) sessionActivity` · `Working(id string)` · `Waiting(id string)` · `Finished(id string, failed bool, viewing string)` · `Viewed(id string)` · `Forget(id string)` · `Reconcile(probes map[string]workspace.SessionStatus)`. `workspace.SessionStatus` do A3b định nghĩa; nếu A3 chạy trước A3b thì A3 tạo luôn type này trong `internal/workspace/workspace.go` đúng như A3b mô tả.

- [ ] **Step 1:** Viết test bảng `TestSessionActivityTransitions`, đúng các hàng sau:

| trước | gọi | sau |
|---|---|---|
| idle | `Working` | working |
| working | `Finished(false, viewing=khác)` | doneUnseen |
| working | `Finished(false, viewing=chính nó)` | idle |
| working | `Finished(true, viewing=khác)` | failedUnseen |
| doneUnseen / failedUnseen | `Viewed` | idle |
| working | `Waiting` | waiting |
| waiting | `Working` | working |
| waiting | `Reconcile{Busy:false}` | idle |
| idle | `Reconcile{Busy:true}` | working |
| working / idle | `Reconcile{Busy:true, PendingPrompt:true}` | waiting |
| waiting | `Reconcile{Busy:true, PendingPrompt:false}` | working |
| doneUnseen | `Reconcile{Busy:false}` | doneUnseen |
| doneUnseen | `Reconcile{Busy:true}` | working |
| bất kỳ | `Forget` | idle (`Get` trả idle) |

  `Reconcile` chỉ đụng tới các ID có trong map truyền vào.
- [ ] **Step 2:** Chạy `go test -p 2 ./internal/ui/model -run TestSessionActivity -count=1`. Kỳ vọng FAIL (chưa có type).
- [ ] **Step 3:** Cài đặt cho đúng bảng trên.
- [ ] **Step 4:** Chạy lại, kỳ vọng PASS. Commit `feat(ui): track per-session activity`.

### Task A3b: Nguồn trạng thái chung — bận và chờ duyệt cho mọi process

**Files:** Modify:
- `internal/workspace/workspace.go`: type mới và method mới trong interface `Workspace`.
- `internal/workspace/app_workspace.go`, `internal/workspace/client_workspace.go`.
- `internal/proto/proto.go`: thêm `PendingPrompt bool \`json:"pending_prompt"\`` vào `AgentSession`.
- `internal/backend/session.go`: `GetAgentSession` điền `PendingPrompt`.
- Các stub test đang implement `Workspace` mà không embed interface (tìm bằng `go vet ./...` sau khi đổi).

Test: `internal/backend/session_test.go` (tạo mới nếu chưa có), `internal/workspace/app_workspace_status_test.go`.

**Interfaces:** Produces:
- `type SessionStatus struct { Busy bool; PendingPrompt bool }` trong package `workspace`.
- Method `AgentSessionStatus(sessionID string) SessionStatus` trong interface `Workspace`.
  - `ClientWorkspace`: một lần gọi `GetAgentSessionInfo`, map `IsBusy` và `PendingPrompt`; nếu lỗi thì trả zero value.
  - `AppWorkspace`: `Busy = coordinator.IsSessionBusy(id)`. `PendingPrompt = true` khi `Permissions.ActiveRequest()` hoặc `Questions.Pending()` có session mà gốc của nó bằng `id`. Gốc tìm bằng cách đi ngược `ParentSessionID` qua `app.Sessions.Get`, tối đa 5 cấp; cùng ngữ nghĩa với `Backend.sessionMatches`.
  - `Backend.GetAgentSession`: điền `PendingPrompt` bằng `ws.Permissions.ActiveRequest()`, `ws.Questions.Pending()` và `b.sessionMatches` có sẵn.
- `AgentIsSessionBusy` giữ nguyên để không vỡ chỗ đang gọi.

- [ ] **Step 1:** Viết các test:
  - `TestGetAgentSessionReportsPendingPermission`: có permission đang chờ ở session S thì `PendingPrompt == true`; Grant xong thì `false`.
  - `TestGetAgentSessionPendingFromChildSession`: permission của session con C (cha là S) làm `GetAgentSession(S).PendingPrompt == true`.
  - `TestAppWorkspaceSessionStatus`: cùng hai tình huống trên, qua `AppWorkspace`.
- [ ] **Step 2:** Chạy `go test -p 2 ./internal/backend ./internal/workspace -run 'PendingPermission|PendingFromChild|SessionStatus' -count=1`, kỳ vọng FAIL. Cài đặt. Chạy lại, kỳ vọng PASS; chạy thêm `go build -p 2 ./... && go vet -p 2 ./internal/...` cho sạch.
- [ ] **Step 3:** Commit `feat(workspace): report busy and pending-prompt status per session`.

### Task A4: Panel "Sessions" và làm mới ngoài luồng

**Files:**
- Create: `internal/ui/model/sessions_panel.go`, `internal/ui/model/sessions_panel_test.go`.
- Modify: `sidebar.go` (section mới nằm sau `modelInfo`, trước `filesSection`), `landing.go` (thêm cột thứ 4 cạnh LSP/MCP/Skills), `ui.go` (trường mới, xử lý msg và sự kiện), `session_busy_test.go` (`countingWorkspace` thêm `ListSessions` và `AgentSessionStatus` có bộ đếm).

**Interfaces:**
- Consumes: `sessionActivityTracker` (A3).
- Produces:
  - `const sessionsPanelMax = 9`; `var sessionListTTL = 2 * time.Second`.
  - `type sessionListMsg struct{ gen uint64; sessions []session.Session; status map[string]workspace.SessionStatus; err error }`.
  - `func (m *UI) dispatchSessionListRefresh() tea.Cmd`: chạy ngoài luồng `ListSessions`, rồi `AgentSessionStatus` (A3b) cho các ID trong `panelSessions`. Có cờ in-flight và gen như `dispatchBusyRefresh`. Session do process khác hoặc script tạo ra sẽ tự xuất hiện, vì `ListSessions` đọc DB và server chung.
  - `func (m *UI) applySessionList(msg sessionListMsg) []tea.Cmd`: lưu `m.sessionList`, gọi `m.activity.Reconcile(status)`, `Forget` các ID không còn trong danh sách.
  - `func (m *UI) panelSessions() []session.Session`: 9 session đầu theo thứ tự `ListSessions`; nếu session đang xem nằm ngoài 9 thì thêm nó làm phần tử thứ 10.
  - `func (m *UI) sessionsInfo(width, maxItems int, isSection bool) string`: cùng chữ ký và khung với `skillsInfo`.
  - Định dạng một dòng: `"<n> <glyph> <title>"`. `n` là 1..9, phần tử ghim dùng `·`. Glyph: idle `○`, working `●`, waiting `▲`, doneUnseen `✓`, failedUnseen `✗`. Màu lấy từ token có sẵn (fgMoreSubtle, busy, warning, success, error), không thêm token mới. Dòng của session đang xem dùng style nổi bật có sẵn.
  - Khi nào làm mới: lúc khởi tạo và `loadSessionMsg`; mọi `pubsub.Event[session.Session]`; `CreatedEvent` của message thuộc session bất kỳ; `RunComplete`; và TTL kiểm ở cuối `Update` (mẫu `busyCacheTTL`).
  - Nối sự kiện vào tracker:
    - `CreatedEvent` của message ở session X có trong `sessionList` → `Working(X)`.
    - `RunComplete` → `Finished(X, Error != "", currentID)`.
    - `loadSessionMsg` cho X → `Viewed(X)`.

- [ ] **Step 1:** Viết các test sau:
  - `TestSessionsPanelListsTopNine`: 12 session, có đúng 9 dòng đánh số.
  - `TestSessionsPanelPinsCurrentBeyondNine`.
  - `TestSessionsPanelDropsDeleted`: lần làm mới thứ hai không còn ID đó, dòng biến mất và `Get` trả idle.
  - `TestSessionListRefreshIsOffThread`: `Update` với một session event không gọi `ListSessions` cho đến khi cmd chạy (bộ đếm bằng 0, rồi bằng 1).
  - `TestBackgroundRunCompleteMarksDone`: `RunComplete` của session khác làm nội dung sidebar chứa `✓`.
- [ ] **Step 2:** Chạy `go test -p 2 ./internal/ui/model -run 'TestSessionsPanel|TestSessionListRefresh|TestBackgroundRunCompleteMarksDone' -count=1`. Kỳ vọng FAIL.
- [ ] **Step 3:** Cài đặt theo Interfaces.
- [ ] **Step 4:** Chạy lại các test trên và cả `go test -p 2 ./internal/ui/model -count=1`. Kỳ vọng PASS toàn bộ. Nếu golden test đổi vì sidebar có thêm section, xem diff trước, đúng thì mới chạy `-update`.
- [ ] **Step 5:** Commit `feat(ui): sessions panel with live status`.

### Task A4b: Dòng sub-agent dưới session cha

**Files:** Modify `internal/ui/model/sessions_panel.go`, `internal/ui/model/ui.go` (nhánh `pubsub.Event[message.Message]` khi `SessionID != m.session.ID`, trước `handleChildSessionMessage`); Test `internal/ui/model/sessions_panel_test.go`.

**Interfaces:**
- Consumes: `sessionsInfo`, `panelSessions`, tracker (A3/A4); `m.com.Workspace.ParseAgentToolSessionID`; `GetSession`.
- Produces:
  - `type childRun struct{ id, parentID, title string; failed bool }` và `m.childRuns map[string]*childRun`, key là ID session con. Chỉ giữ các con đang chạy.
  - `func (m *UI) noteChildActivity(msg message.Message) tea.Cmd`. Áp cho mọi message có `SessionID` không thuộc `m.sessionList` và parse được bằng `ParseAgentToolSessionID`. Lần đầu gặp: resolve cha ngoài luồng (`GetSession(child).ParentSessionID`, đi tới session gốc tối đa 5 cấp) trả `childParentMsg`. Message assistant có `IsFinished()` và finish reason khác tool-use: xoá con khỏi map.
  - Con bị xoá hết khi `RunComplete` của session cha tới.
  - Hiển thị: dưới dòng cha in tối đa 3 dòng `"  └ ● <title con>"`; dư thì thêm `"  └ +k more"`. Dòng con không có số và không nhảy bằng alt.
  - Ở client/server, sự kiện message không bị server lọc, nên sub-agent của process khác cũng hiện.

- [ ] **Step 1:** Viết các test:
  - `TestChildRunShownUnderParent`: message của session con thuộc cha P nằm trong danh sách, sau khi `childParentMsg` tới thì nội dung sidebar có `└ ●` ngay dưới dòng của P.
  - `TestChildRunClearedOnFinish`: message assistant đã finished (end turn) của con thì dòng con biến mất.
  - `TestChildRunsCappedAtThree`: 5 con thì có 3 dòng cộng `+2 more`.
  - `TestChildRunClearedOnParentRunComplete`.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy `go test -p 2 ./internal/ui/model -count=1`, kỳ vọng PASS toàn bộ.
- [ ] **Step 3:** Commit `feat(ui): show running sub-agents under their parent session`.

### Task A5: "Bận" tính theo session hiện tại

**Files:** Modify `internal/ui/model/ui.go`, `internal/ui/model/workspace_cache.go`; Test `internal/ui/model/session_busy_test.go`.

**Interfaces:** Produces `func (m *UI) isCurrentSessionBusy() bool`, trả `m.bangCancel != nil || (m.hasSession() && m.turnInFlight(m.session.ID))`.

Phân loại các chỗ gọi (định vị bằng chuỗi hoặc ghi chú ở gần):

| Hành động | Chỗ |
|---|---|
| **Đổi sang `isCurrentSessionBusy`** | Bật spinner todo (gần "only start spinner if there is an in-progress todo") · spinner khi có message mới ("start the spinner if there is a new message") · placeholder `m.workingPlaceholder` · `m.chat.SetAnimationsAllowed(...)` · esc huỷ ("Handle cancel key when agent is busy") · tóm tắt ("before summarizing session") · `prevBusy`/`busy` trong `applyBusyState` |
| **Bỏ hẳn điều kiện chặn** | 3 chỗ "please wait before starting a new session" |
| **Giữ toàn cục (`isAgentBusy`)** | `handleSelectModel` · reasoning effort · initialize project · suspend (`ctrl+z`) · 2 chỗ "Agent is working, please wait..." (mở editor ngoài) · hyper credits · `program_status.go` |

- [ ] **Step 1:** Viết các test, mỗi test đặt stub `agentBusy=true` và session hiện tại không bận:
  - `TestNewSessionAllowedWhileOtherSessionBusy`: `ctrl+n` đưa về `uiLanding`, không có cảnh báo.
  - `TestEscDoesNotArmCancelWhenOnlyOtherSessionBusy`: `isCanceling` vẫn false.
  - `TestPlaceholderReadyWhenOnlyOtherSessionBusy`.
  - `TestModelSwitchStillBlockedWhileAnySessionBusy`: `handleSelectModel` trả cảnh báo.
- [ ] **Step 2:** Chạy, kỳ vọng 3 test đầu FAIL và test thứ 4 PASS.
- [ ] **Step 3:** Áp bảng phân loại trên.
- [ ] **Step 4:** Chạy `go test -p 2 ./internal/ui/model -count=1`, kỳ vọng PASS toàn bộ. Commit `fix(ui): scope busy state to the current session`.

### Task A6: Phím `alt+1`..`alt+9`

**Files:** Modify `internal/ui/model/keys.go` (thêm `km.Chat.JumpSession`, help "alt+1-9 jump"), `internal/ui/model/ui.go` (cả hai nhánh focus đang xử lý `Chat.NewSession`), `docs/agent-cli-local-setup.md` (thêm mục phím tắt); Test `internal/ui/model/sessions_panel_test.go`.

**Interfaces:** Consumes `panelSessions()` (A4), `loadSession` (đã có). Produces `func (m *UI) jumpToSession(n int) tea.Cmd`: `n` từ 1 đến 9. Trả nil nếu `n` vượt quá số session có đánh số, hoặc trúng session đang xem. Không đụng tới `m.textarea`. Khi có dialog đang mở thì phím đi vào dialog như hiện nay.

- [ ] **Step 1:** Viết các test:
  - `TestAltNumberLoadsNthSession`: stub ghi lại ID truyền cho `GetSession`; khi cmd chạy, ID đó phải bằng `panelSessions()[n-1].ID`.
  - `TestAltNumberKeepsDraft`: editor có "abc", sau `alt+1` vẫn là "abc" và không có `SendMessage`.
  - `TestAltNumberOutOfRangeNoop`.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy lại, kỳ vọng PASS.
- [ ] **Step 3:** Commit `feat(ui): jump between sessions with alt+1..9`.

### Task A7: Prompt và thông báo của session nền (chế độ in-process)

**Files:** Create `internal/ui/model/background_prompts.go`, `internal/ui/model/background_prompts_test.go`; Modify `ui.go` (các nhánh `PermissionRequest`, `PermissionNotification`, `question.Request`, `question.Notification`, `RunComplete`, `loadSessionMsg`).

**Interfaces:**
- Consumes: `sessionActivityTracker` (A3), `sessionList` và `panelSessions` (A4).
- Produces:
  - `type heldPrompts struct{ perms map[string]permission.PermissionRequest; questions map[string]question.Request }`, key là ID của session gốc.
  - `type promptRootMsg struct{ root string; perm *permission.PermissionRequest; q *question.Request }`.
  - `func (m *UI) resolvePromptRoot(sessionID string, perm *permission.PermissionRequest, q *question.Request) tea.Cmd`: chạy ngoài luồng, đi ngược `Workspace.GetSession(...).ParentSessionID` tối đa 5 cấp.
- Luật định tuyến:
  - Request có `SessionID == current` → mở ngay theo đường cũ, không cần resolve.
  - Ngược lại → `resolvePromptRoot`. Nếu `root == current` thì mở. Nếu không, hoặc đang ở màn landing: giữ vào `heldPrompts[root]`, gọi `activity.Waiting(root)`, toast `"<title> needs your approval (alt+<n>)"` (bỏ phần số nếu session không được đánh số), rồi `sendNotification`.
  - `loadSessionMsg` cho X: có permission đang giữ thì mở trước, sau đó tới câu hỏi; xong xoá khỏi map.
  - `PermissionNotification` hoặc `question.Notification` khớp với một request đang giữ: xoá nó và gọi `activity.Working(root)`.
  - `RunComplete` của X khác session hiện tại: toast `"✓ <title> done"` hoặc `"✗ <title> failed"`; nếu `!m.notifyWindowFocused` thì gửi thêm `sendNotification`.
  - Ở chế độ client/server, server đã lọc nên code này không có tác dụng gì.
  - **Bổ sung từ review A2 (đã chứng minh):** `handleQuestionNotification` hiện đóng bất kỳ form câu hỏi nào. Khi câu hỏi đã xếp hàng (A2), thông báo "đã trả lời" của A có thể tới sau request của B (hai broker đi hai goroutine), nên đóng nhầm form của B và B treo. Sửa: chỉ đóng form khi `n.BatchID` khớp ID của request đang mở; câu hỏi đang giữ trong `heldPrompts` cũng chỉ xoá khi ID khớp. Soát `handlePermissionNotification` theo cùng kiểu (khớp `ToolCallID`). Test: `TestLateQuestionNotificationKeepsOtherForm`: form B đang mở, notification của A tới, form B vẫn mở.

- [ ] **Step 1:** Viết các test:
  - `TestBackgroundPermissionIsHeld`: không có `PermissionsID` trong dialog, có ▲ trong sidebar.
  - `TestHeldPermissionOpensOnSwitch`.
  - `TestCurrentSessionPermissionOpensImmediately` (chống hồi quy).
  - `TestChildSessionPromptRoutesToParent`: con của session hiện tại thì mở; con của session nền thì giữ dưới ID session cha.
  - `TestBackgroundQuestionIsHeld`.
  - `TestBackgroundRunCompleteToast`: status msg chứa title.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy `go test -p 2 ./internal/ui/model -count=1`, kỳ vọng PASS toàn bộ.
- [ ] **Step 3:** Commit `feat(ui): hold background prompts and notify on completion`.

### Task A8: Bộ chuyển kiểu alt-tab (session)

**Files:** Create `internal/ui/dialog/switcher.go`, `internal/ui/dialog/switcher_test.go`, `internal/ui/model/switcher.go`; Modify `internal/ui/model/keys.go`, `internal/ui/model/ui.go`, `docs/agent-cli-local-setup.md`.

**Interfaces:**
- Consumes: `panelSessions()`, `m.activity` (A3/A4), `loadSession`.
- Produces:
  - `km.Chat.Switcher` (phím `ctrl+tab`, `alt+tab`, `alt+w`) và `km.Chat.SwitcherBack` (`ctrl+shift+tab`, `alt+shift+tab`, `alt+W`). Lý do có nhiều phím: GNOME và Windows bắt `alt+tab` trước khi tới app; `ctrl+tab` chỉ tới được trên terminal có kitty keyboard protocol; `alt+w` là phím dự phòng.
  - `m.recentSessions []string`: lịch sử session đã xem, mới nhất đứng đầu, tối đa 20, cập nhật khi `loadSessionMsg` tới.
  - `dialog.NewSwitcher(com, items []SwitcherItem, start int)` với `type SwitcherItem struct{ Kind SwitcherKind; ID, Title, Detail string; Activity string }`, trong đó `SwitcherKind` là `SwitcherSession` hoặc `SwitcherProject`.
  - Hành vi:
    - Bấm phím Switcher → mở overlay, highlight phần tử thứ 2 (session vừa xem trước đó), giống alt-tab.
    - Bấm tiếp Switcher → xuống một dòng; SwitcherBack → lên một dòng; vòng tròn.
    - `enter`/`space` hoặc `1`..`9` → chọn. `esc` → đóng.
    - Nếu `m.keyenh` báo terminal có key release: nhả `alt` thì chọn phần tử đang highlight.
    - Chọn session thì phát `ActionSelectSession` (đường cũ). Glyph trạng thái giống panel A4.

- [ ] **Step 1:** Viết các test:
  - `TestSwitcherStartsOnPrevious`: 3 session trong lịch sử, mở overlay thì highlight index 1.
  - `TestSwitcherCyclesAndWraps`.
  - `TestSwitcherBackMovesUp`.
  - `TestSwitcherEnterSelects`: phát `ActionSelectSession` với đúng ID.
  - `TestSwitcherDigitSelects`.
  - `TestSwitcherEscCancels`.
  - `TestRecentSessionsMRU`: xem A, B, A thì thứ tự là A, B; tối đa 20.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy `GOMAXPROCS=2 go test -p 2 ./internal/ui/... -count=1`, kỳ vọng PASS. Docs ghi các phím và cách kiểm terminal (`cat -v` rồi bấm `ctrl+tab`).

### Task A9: Tự cấu hình tổ hợp phím + `agent-cli keys`

**Files:** Create `internal/ui/model/keys_custom.go`, `internal/ui/model/keys_custom_test.go`, `internal/cmd/keys.go`, `internal/cmd/keys_test.go`; Modify chỗ UI dựng `DefaultKeyMap()` (trong `ui.go`), `docs/agent-cli-local-setup.md`. Không sửa `internal/config/config.go`.

**Interfaces:**
- File phím: `filepath.Join(filepath.Dir(config.GlobalConfig()), "keybindings.json")`, cùng chỗ với thư mục `themes/`. Dạng `{"<action>": ["phím", ...]}`. Mảng rỗng nghĩa là tắt phím đó.
- `func keyRegistry(km *KeyMap) map[string]*key.Binding`: duyệt struct `KeyMap` bằng reflection, phủ **mọi** trường `key.Binding` kể cả struct lồng nhau. Tên action là đường dẫn snake_case, ví dụ `Editor.SendMessage` → `editor.send_message`, `Chat.Switcher` → `chat.switcher`, `Quit` → `quit`. Thêm phím mới vào `KeyMap` thì tự có tên, không phải sửa danh sách tay.
- `func applyKeyOverrides(km *KeyMap, overrides map[string][]string) []string`: đặt lại keys, đổi phần help sang phím đầu tiên, mảng rỗng thì `SetEnabled(false)`. Trả cảnh báo cho action không tồn tại, và cho trường hợp hai action đang bật trong cùng nhóm cấp 1 (ví dụ `chat.*`) dùng chung một phím.
- `func LoadKeyMap(path string) (KeyMap, []string)`: file không có → mặc định, không cảnh báo; JSON hỏng → mặc định kèm 1 cảnh báo. UI dùng hàm này thay `DefaultKeyMap()`. Cảnh báo ghi log (`slog.Warn`, viết hoa chữ đầu) và hiện một toast lúc khởi động.
- `agent-cli keys [--json]`: in bảng `ACTION  KEYS  HELP` của keymap sau khi áp file. `--json` in mảng `{action, keys, help, enabled}`.

- [ ] **Step 1:** Viết các test:
  - `TestKeyRegistryCoversAllBindings`: số phần tử bằng số trường `key.Binding` đếm độc lập bằng reflection, và tên không trùng.
  - `TestApplyKeyOverridesSetsKeys`.
  - `TestApplyKeyOverridesEmptyDisables`.
  - `TestApplyKeyOverridesUnknownWarns`.
  - `TestApplyKeyOverridesConflictWarns`.
  - `TestLoadKeyMapMissingFile`.
  - `TestLoadKeyMapBadJSON`.
  - `TestKeysCmdJSON`: có `chat.switcher` và `chat.new_session`.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy `GOMAXPROCS=2 go test -p 2 ./internal/ui/model ./internal/cmd -count=1`, kỳ vọng PASS. Docs có ví dụ file `keybindings.json` cho dev: switcher, nhảy session, session mới, đổi mode.

### Task A10: Mode "accept edits" theo session trong vòng Shift+Tab

**Bối cảnh (đọc code):**
- Vòng Shift+Tab hiện tại là code → plan → code+YOLO (`toggleInputMode` trong `ui.go`).
- YOLO (`SetSkipRequests`) và plan (`SetMainAgent`) áp cho **cả workspace**. Vì vậy `toggleInputMode` bị chặn khi có bất kỳ session nào bận, và chặn này **giữ nguyên**.
- `permission` đã có `autoApproveSessions` theo session. Accept-edits được làm theo session dựa trên đó, nên không phải đụng `coordinator.go`.

**Files:**
- Modify: `internal/permission/permission.go`, `internal/workspace/{workspace,app_workspace,client_workspace}.go`, `internal/proto/proto.go`, `internal/server/` (endpoint) và `internal/backend/` (handler), `internal/client/proto.go`, `internal/ui/model/ui.go`, `docs/agent-cli-local-setup.md`.
- Test: `internal/permission/permission_test.go`, `internal/server/*_test.go` theo harness có sẵn, `internal/ui/model/session_busy_test.go`.

**Interfaces:**
- Permission service:
  - `SetAcceptEdits(sessionID string, on bool)` và `AcceptEdits(sessionID string) bool`.
  - Trong `Request`, khi session bật accept-edits và `ToolName` thuộc `{"edit","multiedit","write"}`, cấp quyền theo cùng đường với nhánh `autoApprove` (phát `PermissionNotification{Granted:true}`). Mọi tool khác vẫn hỏi.
- Workspace: thêm `PermissionSetAcceptEdits(sessionID string, on bool)` và `PermissionAcceptEdits(sessionID string) bool`.
  - `AppWorkspace` gọi thẳng service.
  - `ClientWorkspace` đi qua endpoint mới `POST/GET /v1/workspaces/{id}/permissions/accept-edits?session=` (theo mẫu endpoint permission sẵn có).
- UI:
  - Vòng Shift+Tab mới: code → **code+accept-edits** → plan → code+YOLO → code.
  - Trạng thái accept-edits đọc ngoài luồng cùng lượt `dispatchBusyRefresh`, lưu theo session hiện tại; đổi session thì hiện đúng trạng thái của session đó.
  - Placeholder `"Accepting edits"`.
  - Banner dùng chung cơ chế với `planModeBannerMsg`.

- [ ] **Step 1:** Viết các test:
  - `TestAcceptEditsAutoApprovesEditTools`: `edit`, `multiedit`, `write` được cấp ngay.
  - `TestAcceptEditsStillAsksForBash`.
  - `TestAcceptEditsIsPerSession`: session khác vẫn hỏi.
  - Test endpoint theo harness của `internal/server`.
  - `TestShiftTabCycleIncludesAcceptEdits`: 4 lần bấm đi đúng thứ tự và quay về code.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy `GOMAXPROCS=2 go test -p 2 ./internal/permission ./internal/server ./internal/workspace ./internal/ui/model -count=1`, kỳ vọng PASS.

### Task A8b: Bộ chuyển thêm các dự án đang mở (sau B1)

**Files:** Modify `internal/ui/model/switcher.go`, `internal/ui/dialog/switcher.go`; Test `internal/ui/model/switcher_test.go`.

**Interfaces:**
- Khi `$TMUX` có giá trị và tmux session `agents` (đổi được bằng `AGENT_CLI_TMUX_SESSION`) tồn tại: một cmd ngoài luồng chạy `tmux list-windows -t <s> -F '#{window_index}\t#{window_name}\t#{pane_current_path}\t#{window_activity}'`.
- Mỗi window ứng với **một thư mục khác thư mục hiện tại** thành một `SwitcherProject` (Detail = đường dẫn). Các dòng này nằm sau phần session, xếp theo `window_activity` giảm dần.
- Chọn một dự án → chạy `tmux select-window -t <s>:<index>`. Không có tmux → không có mục dự án nào (không lỗi).
- Thêm mục **dự án gần đây chưa mở**: lấy từ `projects.List()` (`internal/projects`), xếp theo `LastAccessed` giảm dần, bỏ các thư mục đã có window, tối đa 9 mục. Chọn một mục thì chạy `tmux new-window -t <s> -n <basename> -c <dir> agent-cli -c <dir>`, đi qua cùng runner tiêm được. Không có tmux thì hiện toast `"Open it with: agent-cli-tmux <dir>"`.
- Lệnh tmux chạy qua một `func(name string, args ...string) ([]byte, error)` tiêm được, để test không gọi tmux thật.

- [ ] **Step 1:** Viết các test:
  - `TestSwitcherListsTmuxProjects`: runner giả trả 3 window, trong đó có 1 window là thư mục hiện tại → ra 2 mục dự án, đúng thứ tự.
  - `TestSwitcherProjectSelectRunsTmux`: runner ghi lại đúng args `select-window -t agents:2`.
  - `TestSwitcherNoTmuxNoProjects`.
  - `TestSwitcherRecentProjectOpensWindow`: dự án gần đây chưa mở có trong danh sách; chọn thì runner nhận đúng `new-window ... -c <dir>`.
- [ ] **Step 2:** Chạy, kỳ vọng FAIL. Cài đặt. Chạy lại, kỳ vọng PASS.

---

## Phần B — Đa dự án trong một cửa sổ

### Task B0: Launcher dùng server chung theo mặc định

**Files:** Modify `scripts/install-agent-cli.sh` (wrapper thêm `export CRUSH_CLIENT_SERVER="${CRUSH_CLIENT_SERVER:-1}"` và chọn `--host` theo dự án như Ruling R9), `docs/agent-cli-local-setup.md` (giải thích mỗi dự án một server, log ở `~/.cache/agent-cli/server-<host>/crush.log`, cách tắt bằng `CRUSH_CLIENT_SERVER=0`, giới hạn của chế độ in-process, và U1); Create `scripts/test-agent-cli-wrapper.sh`.

**Interfaces:** Produces: mọi lệnh `agent-cli` (TUI, `run`, script) trong cùng một thư mục gốc dùng chung server ở `unix:///run/user/$UID/agent-cli-<hash8>.sock`; thư mục gốc khác thì server khác.

- [ ] **Step 1:** Viết `scripts/test-agent-cli-wrapper.sh`. Cài vào thư mục tạm (`AGENT_CLI_INSTALL_DIR`), thay binary thật bằng stub in ra `$@` và `$CRUSH_CLIENT_SERVER`, rồi kiểm:
  - Hai thư mục con của cùng một repo cho cùng một `--host`.
  - Hai repo khác nhau cho hai `--host` khác nhau.
  - User đã truyền `--host` hoặc `-H` thì wrapper giữ nguyên, không thêm.
  - `-c <dir>` và `--cwd=<dir>` đều được hiểu.
  - `CRUSH_CLIENT_SERVER=0` vẫn chạy và không thêm `--host`.
  Chạy thấy đỏ, sửa wrapper, chạy lại thấy in `PASS` và rc=0. Không ghi đè bản cài thật.
- [ ] **Step 2:** Cập nhật docs. Commit `feat(scripts): run every agent-cli process against one shared server`.

### Task B1: `agent-cli-tmux` mở nhiều dự án

**Files:** Create `scripts/agent-cli-tmux.sh`, `scripts/test-agent-cli-tmux.sh`; Modify `scripts/install-agent-cli.sh` (symlink `~/.local/bin/agent-cli-tmux`), `docs/agent-cli-local-setup.md` (cách dùng; `prefix+w` để chọn dự án).

**Interfaces:**
- Cú pháp: `agent-cli-tmux <dir>...`.
- Dùng tmux session tên `agents` (đổi được bằng `AGENT_CLI_TMUX_SESSION`).
- Mỗi thư mục là một window đặt theo basename, chạy `CRUSH_CLIENT_SERVER=1 agent-cli -c <đường dẫn tuyệt đối>`.
- Nếu thư mục đã có window thì chỉ chọn window đó, không tạo thêm.
- Thư mục không tồn tại: in lỗi ra stderr và `exit 2`.
- Cuối cùng attach, hoặc `switch-client` nếu đang ở trong tmux. Bỏ bước này khi `AGENT_CLI_TMUX_NO_ATTACH=1`.

- [ ] **Step 1:** Viết `scripts/test-agent-cli-tmux.sh`.
  - Dùng socket riêng `tmux -L agentcli-test`, không đụng tmux của user.
  - `PATH` trỏ tới một stub `agent-cli` ghi `$CRUSH_CLIENT_SERVER $*` ra file rồi `sleep 30`.
  - Chạy với 2 thư mục: phải có 2 window, stub thấy `1 -c <dir>`.
  - Chạy lại với thư mục cũ: vẫn 2 window.
  - Thư mục sai: rc=2.
  - Cuối test gọi `kill-server`.
- [ ] **Step 2:** Chạy `bash scripts/test-agent-cli-tmux.sh`, kỳ vọng FAIL (chưa có script). Cài đặt. Chạy lại, kỳ vọng in `PASS` và rc=0. `bash -n` không thay được bước này.
- [ ] **Step 3:** Commit `feat(scripts): open several projects in one tmux session`.

### Task B2: `agent-cli ps`

**Files:** Create `internal/cmd/ps.go`, `internal/cmd/ps_test.go`; đăng ký trong `init()` bằng `rootCmd.AddCommand(psCmd)`, theo mẫu `models.go`.

**Interfaces:**
- `type psSource interface { ListWorkspaces(ctx context.Context) ([]proto.Workspace, error); ListSessions(ctx context.Context, id string) ([]proto.Session, error); GetAgentSessionInfo(ctx context.Context, id, sessionID string) (*proto.AgentSession, error) }`. `*client.Client` thoả interface này.
- `type psRow struct{ Path string \`json:"path"\`; SessionID string \`json:"session_id"\`; Title string \`json:"title"\`; Busy bool \`json:"busy"\` }`.
- `func collectPS(ctx context.Context, src psSource, perWorkspace int) ([]psRow, error)`: mặc định lấy 9 session mỗi workspace.
- Status lấy từ `IsBusy`/`PendingPrompt` (A3b): `WAITING`, `BUSY`, `IDLE`.
- Output dạng bảng `PROJECT  SESSION  STATUS  TITLE`, có cờ `--json`. Dùng cờ `--host` sẵn có.
- Theo Ruling R9 có nhiều server. Không có `--host` thì `ps` quét mọi `/run/user/$UID/agent-cli-*.sock`, hỏi từng server, bỏ qua socket chết (`Health` lỗi). Có `--host` thì chỉ hỏi server đó.
- **Không được tự bật server.** Không tìm thấy server nào còn sống thì in `No agent-cli server running (open projects with agent-cli-tmux)` và rc=0.

- [ ] **Step 1:** Viết các test với `psSource` giả:
  - `TestCollectPSRowsAcrossWorkspaces`: 2 workspace × 2 session, 1 session bận, ra đúng 4 dòng và đúng giá trị `Busy`.
  - `TestPSJSONOutput`.
  - `TestPSNoServerDoesNotSpawn`: host trỏ tới socket không tồn tại, có message trên, rc=0, không có process mới.
- [ ] **Step 2:** Chạy `go test -p 2 ./internal/cmd -run 'TestCollectPS|TestPS' -count=1`, kỳ vọng FAIL. Cài đặt. Chạy lại, kỳ vọng PASS.
- [ ] **Step 3:** Commit `feat(cmd): add ps to list projects and sessions on the server`.

---

## Thứ tự và phụ thuộc

A1 → A2 → A3b → A3 → A4 → A4b → A5 → A6 → A7 → A8 → A9 → A10 → B0 → B1 → A8b → B2.

- Phần A dùng được ngay ở chế độ mặc định (in-process).
- Phần B độc lập với A về code, nhưng chỉ có ích khi A xong, vì mỗi pane tmux sẽ là một TUI có panel session.
- B2 hiện `WAITING` từ `PendingPrompt` của A3b.
- A4–A7 cùng sửa `ui.go`, nên không chạy song song.

---

## Phần H — Home, vòng đời, draft, job (spec user 14/10 chiều)

**Mindset bắt buộc:** Project = repo · Workspace = thư mục gốc đã chuẩn hoá (git toplevel hoặc realpath; thư mục con không tạo workspace mới; worktree khác = workspace khác) · Session = một việc trong workspace · Run = một lượt chạy · ownership workspace → session → run → job/subagent · window/pane chỉ là nơi hiển thị · không tuyên bố model riêng từng session (coordinator dùng model chung theo workspace).

**Kiểm kê (đọc code 14/10):**

| Yêu cầu | Hiện có | Thiếu | Task |
|---|---|---|---|
| Danh sách session + trạng thái thật | A4/A4b/A3b (running/waiting/done/failed, Known) | — | xong |
| Chuyển session khi phiên khác chạy, ctrl+n | A5 | — | review |
| alt+1..9 + phím thay thế | A6, A9 cấu hình phím, A8 switcher (ctrl+tab/alt+w) | — | A8 đang làm |
| Draft riêng từng session | Chỉ 1 textarea + history.draft | lưu/khôi phục theo session | **H3** |
| Quyền/câu hỏi session nền không chen | permission/question chưa lọc theo session ở in-process | | A7 |
| Cancel đúng session/run | A5 r0 (isCanceling); D1 nợ: run -s có thể huỷ lượt TUI | | A7 + nợ D1 |
| Cùng thư mục gốc = cùng server; socket ngắn | B0 (hash 8 hex trong /run/user) | test repo đường dẫn dài | **H1** |
| Rời client/về Home không huỷ run | ❌ teardown sau 10s detach grace giết run (đo bởi reviewer) | giữ workspace khi còn run | **H0** |
| Model theo workspace | C2 | — | xong |
| Subagent theo session cha | A4b | — | xong |
| Bash job: command, trạng thái, started_at, elapsed, exit, ownership | BackgroundShell không có SessionID/StartedAt; manager toàn process | gắn ownership + panel | **H4** |
| Usage estimated/unknown, subscription ≠ $0 | cost có; flat-rate flag có | đánh dấu | **H5** |
| Home → workspace → session | projects registry (không khoá liên process, ghi không nguyên tử) | Home TUI + routing | **H1, H2** |

### Task H0: Server giữ workspace khi còn run (vòng đời)
**Files:** internal/backend/backend.go (teardown / scheduleShutdownIfIdleLocked), test backend.
**Interfaces:** khi client cuối rời (hết detach grace) mà workspace còn session bận (`AgentCoordinator.IsBusy()`), KHÔNG teardown; đặt hẹn kiểm lại (ví dụ mỗi 5s) và teardown khi rảnh + không client. Server idle shutdown chỉ tính khi không còn workspace bận. Không đổi hành vi khi không có run.
- [ ] Test: run đang bận + client duy nhất rời → sau grace workspace còn, run hoàn tất; sau khi rảnh → teardown. Không bận → teardown như cũ. E2E (script): `agent-cli run "SLOW"` bị SIGKILL client giữa chừng → run vẫn xong (ghi file/RunComplete trong DB).

### Task H1: Sổ workspace an toàn + chuẩn hoá
**Files:** internal/projects/projects.go (+test), internal/cmd/root.go (chỗ Register).
**Interfaces:** `NormalizeWorkspace(dir) (string, error)`: realpath, rồi git toplevel nếu trong repo (worktree giữ riêng vì toplevel của worktree là chính nó), khớp cách launcher B0 tính. Ghi registry: khoá liên process (flock trên `projects.json.lock`) + ghi tmp rồi rename. `Remove(path)` chỉ xoá khỏi danh sách (không đụng DB/file). Bản ghi thêm `Host` (hostname máy, định danh = host + path, chuẩn bị cho tương lai không nhầm cùng đường dẫn trên hai máy) và `Pinned bool` + `Pin(path, bool)`; giữ tương thích đọc tệp cũ (thiếu trường → Host = máy hiện tại, Pinned = false). Lưu TOÀN BỘ lịch sử (không cắt theo giới hạn hiển thị). Test: thư mục con → cùng mục; 2 worktree → 2 mục; 2 process ghi song song (go test chạy subprocess) → không mất mục, JSON hợp lệ; path dài (>150 ký tự) qua launcher → socket vẫn ngắn (bổ sung ca trong test-agent-cli-wrapper.sh).

### Task H2: Màn hình Home (chọn workspace → session)
**Files:** mới internal/ui/home/ (model, view, test), internal/cmd/root.go (định tuyến), scripts/install-agent-cli.sh (wrapper không chèn --host khi vào Home), docs.
**Hành vi:** `agent-cli` không tham số trên TTY → Home; `--cwd/--session/--continue`, `-c` và mọi subcommand → đi thẳng như cũ. Home KHÔNG khởi động server/workspace nào để hiển thị. Bố cục kiểu "Open Recent": ĐANG HOẠT ĐỘNG (server sống, số running/waiting) · ĐÃ GHIM · GẦN ĐÂY (5 mục mới nhất theo lần truy cập, chỉ là giới hạn hiển thị) · → Mở thư mục hiện tại · → Mở thư mục khác (nhập đường dẫn) · → Xem tất cả / tìm kiếm · phím ghim/bỏ ghim. Danh sách: (1) workspace có server sống (từ quét socket như `ps`, timeout, off-thread) kèm số session running/waiting; (2) workspace gần đây từ registry (branch nếu đọc được, lần truy cập); (3) "Mở thư mục hiện tại"; (4) "Thêm đường dẫn"; tìm kiếm. Không probe được → `unknown/offline`, không suy ra idle. Path không tồn tại → báo, cho bỏ khỏi danh sách (không xoá dữ liệu). Chọn workspace → danh sách session (server sống: qua client API; không sống: mở DB của workspace đó chỉ-đọc, giới hạn 50) + "Session mới" + tìm kiếm. Chọn → chạy TUI workspace như process con (`agent-cli -c <dir> [--session <id>]`, cùng TTY); TUI thoát → về Home (không huỷ run nhờ H0 ở client/server; in-process thì ghi rõ giới hạn). Không tự gửi prompt/tạo session/chạy tác vụ. Phạm vi: chỉ thư mục LOCAL và thư mục hiện tại. Remote MVP = SSH vào máy chủ rồi chạy agent-cli tại đó (docs nói rõ); KHÔNG có SSH workspace picker trong lô này và docs không được quảng cáo nó. Chỉ hiện workspace có Host = máy hiện tại.
- [ ] Test model Home (bảng, lọc, offline/unknown, path thiếu, không spawn server), test định tuyến root (Home chỉ khi không tham số + TTY), test wrapper (không --host khi vào Home). Chạy thật bằng launcher đã cài.

### Task H3: Draft riêng từng session
**Files:** internal/ui/model/ui.go (+ session.go), test.
**Interfaces:** `m.drafts map[string]string` (key = session ID, "" = landing). Trước khi đổi session (loadSession/newSession/jump/switcher/Home) lưu `m.textarea.Value()`; sau khi load khôi phục draft của session đích (rỗng nếu chưa có). Gửi prompt xoá draft của session đó. Attachments: giữ theo session nếu cấu trúc cho phép, nếu không ghi rõ giới hạn.
- [ ] Test: gõ "a" ở s1 → sang s2 (draft rỗng) gõ "b" → về s1 thấy "a", về s2 thấy "b"; enter ở s2 gửi đúng s2; ctrl+n từ s1 → landing rỗng, s1 vẫn giữ "a".

### Task H4: Job bash có ownership + panel Jobs
**Files:** internal/shell/background.go (SessionID, StartedAt, ExitCode), nơi tool bash tạo job (lấy session từ ctx), backend/server/client API liệt kê job theo workspace, workspace interface, sidebar panel (internal/ui/model).
**Interfaces:** mỗi job: id, session_id, command, description, status (running/done/failed/killed), started_at, elapsed, exit code. Panel hiện job của session đang xem (và đếm job của session khác). Xem output/dừng qua cơ chế có sẵn (job_output/job_kill hoặc lệnh palette). Không liệt kê process ngoài agent.
- [ ] Test ownership (job của s1 không hiện ở s2; kill job của s1 không đụng s2), panel memo (R13), client/server.

### Task H5: Usage đánh dấu estimated/unknown
**Files:** internal/ui/common (ModelInfo), nơi cộng cost.
**Interfaces:** provider flat-rate/subscription → hiện "subscription" thay $0; usage từ ước lượng → "~"; không có dữ liệu → "unknown". Không cộng hai lần usage CLI ngoài.

**Thứ tự Phần H:** H0 ∥ H1 (không đụng ui.go) → H2 (Home) → H3 (draft, ui.go, sau A7/A8/A10) → H4 → H5. Kiểm recovery/compact: chỉ điều tra + báo cáo, sửa khi tái hiện được lỗi.

## Phần K — KEEP / DISABLE / DEFER / REMOVE (user 14/10 15:1x, đo hiện trạng 15:16)

| Quyết định | Gì | Tệp chính | Lý do |
|---|---|---|---|
| KEEP, hoàn thiện | Home + registry (H1, H2) | `internal/projects`, `internal/ui/home`, `internal/cmd/root.go` | Luồng 1 "vào việc" |
| KEEP | Bận/trạng thái theo session, panel session + subagent (A1–A5, A4/A4b) | `internal/ui/model/session_activity.go`, `sessions_panel.go` | Luồng 3 "song song" |
| KEEP | Vòng đời server: run sống khi client rời (H0) | `internal/backend/backend.go` | Luồng 5 "tiếp tục" |
| KEEP | Cấu hình provider chung qua biến AGENT_CLI_* (E1), model theo workspace (C2) | `agent-cli.crushrc.example`, `agent-cli.env.example` | Luồng 2 "làm việc" |
| KEEP, tối thiểu | Launcher và server theo dự án (B0), `ps`/`logs` (B2), `agent-cli-tmux` (C1), `run --json`/`send` (D1, D3), hook Stop (D2), `.claude/commands` (D4) | `scripts/`, `internal/cmd/` | Script và agent điều khiển được; không mở rộng thành orchestration |
| KEEP, không đụng | Lõi upstream: permission, compact, DB, client/server, LSP, giới hạn output | — | Nền bắt buộc |
| DISABLE (env của launcher, đã có) | metrics, tự cập nhật provider, kiểm tra update | `scripts/install-agent-cli.sh:52-54` | Chưa chứng minh mọi đường mạng đều tắt, xem P1.6-N |
| DISABLE (không cấu hình) | MCP/provider phụ, notification; theme giữ nguyên, không custom thêm | — | Ngoài core |
| DEFER (giữ code, không có phím mặc định, không quảng bá) | A6 alt+1..9, A8 switcher, A9 keybindings tuỳ chỉnh | `keys.go`, `keys_custom.go`, `dialog/switcher.go` | Roadmap P1.3a. Không phải code chết: `keybindings.json` (A9) vẫn gán phím được, nên không xoá |
| DEFER | A8b (huỷ); phần mở rộng A10. A10 chỉ còn vòng sửa an ninh, vì code đã nằm trong vòng Shift+Tab mà user yêu cầu | — | Thêm mode quyền mới là ngoài Phase 1 |
| DEFER (Phase 2+) | Chuyển session DB sang `~/.local/share`; remote SSH picker; Claude/Codex làm agent chính; model riêng từng session; automation, GUI, RAG, team; dashboard chi phí | — | Lõi upstream đặt DB theo dự án, và H2 đọc DB ở đó |
| REMOVE (đã làm) | `agent-cli.config.example.json` | E1 | Chứa endpoint/UUID cũ |
| REMOVE | Chưa có gì khác | — | Chưa có bằng chứng code chết |

Hiện trạng lưu trữ (đo 15:16):
- Cấu hình: `~/.config/agent-cli`.
- Dữ liệu và registry: `~/.local/share/agent-cli/projects.json`.
- Log và cache: `~/.cache/agent-cli` (log server: `server-<host>/crush.log`).
- Socket: `/run/user/$UID/agent-cli-<hash8>.sock`.
- Session DB: `<dự án>/.crush/crush.db`.

Kết luận: khớp đề xuất XDG, trừ DB theo dự án (DEFER).

Task nhỏ thêm:
- **L1** (sau H0/H2, vì hai task đó đang sửa các script này): đổi tiền tố thư mục tạm của `scripts/test-agent-cli-*.sh` thành `mktemp -d /tmp/agent-cli-test-<tên>.XXXXXX`; mọi script đều `trap` dọn khi EXIT/INT/TERM; giữ thư mục lại chỉ khi `KEEP_TMP=1`.
- **Log có ID:** gộp vào H4 (xem brief H4).
- **P1.6-N:**
  - Chứng minh tắt mạng: chạy binary đã cài trong network namespace riêng hoặc theo dõi kết nối ra ngoài.
  - Chỉ được phép gọi tới đúng endpoint provider đã cấu hình.
  - Nếu chưa đo thì ghi "chưa đo".
- **Tích hợp:** chỉ đăng ký tool `claude_code`/`codex` (cây chính) khi tìm thấy CLI tương ứng, sau khi phối hợp với phiên sở hữu.

Thứ tự: H2 → N1 → H3 → A7 → H4 (+log ID) → L1 → P1.5 → P1.6. H3 không phụ thuộc A8/A10; chỉ phải đi tuần tự vì cùng sửa `ui.go`.
