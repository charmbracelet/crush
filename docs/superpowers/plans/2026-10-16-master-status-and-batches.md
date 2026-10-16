# Plan tổng 2026-10-16 — trạng thái và các đợt kế tiếp

> Người thi hành theo luật nhà: Opus 5 effort high, **được quyền phản biện
> plan** vì plan là đầu vào do model yếu hơn viết theo phân vai quy định.
> Bản tổng hợp này do Codex đối chiếu repository, không phải receipt thi hành.
> Không viết số chưa chạy ra · không nở phạm vi · không tự ý push.
> Chỉ viết plan trong nhiệm vụ này; chưa cho phép thực hiện B0/B1.

Plan treo vào [roadmap Phase 1–5](2026-10-14-agent-cli-personal-roadmap.md),
không thay thế roadmap hoặc các plan đang có. Mỗi đợt cần brief riêng được
người duyệt trước code, TDD cho logic, verify và soát độc lập theo luật nhà.
Chỉ đưa vào critical path những việc giúp biết việc nào cần mình, giữ công
việc khi vắng mặt và gọi đúng model. Agent-cli vẫn là công cụ chính.

## 1. Trạng thái hiện tại — kiểm ngày 16/10/2026

### Git đã kiểm trực tiếp

Đã chạy `git fetch origin` thành công, sau đó:

```text
git rev-list --left-right --count HEAD...origin/main
1       0

HEAD main:   e0e752fe — fix(agent): attach the tool-call repair hook to every stream call
origin/main: 8152a7de — fix(agent): do not auto-repair tool arguments for mutating tools

git status --short
 M internal/agent/agent.go
 M internal/cmd/run.go
?? docs/superpowers/plans/2026-10-16-master-status-and-batches.md
?? internal/session/derived_title_test.go
?? internal/session/title.go
```

Local trước origin đúng một commit tại thời điểm fetch; chưa có commit phía
origin mà local thiếu. Bốn file code đặt tên session chưa commit; file plan
này cũng chưa track. `git diff --stat` chỉ đếm hai file tracked, không bao gồm
hai file code mới. Không được hiểu đây là cây sạch hoặc B0 đã xong.

### Binary, launcher và cấu hình

- Binary: `~/.local/lib/agent-cli/crush`; launcher: `~/.local/bin/agent-cli`.
  Metadata từ `go version -m`: `vcs.revision=e0e752fea96a5bc3065b5af681327a5cc6c064f0`,
  `vcs.modified=true`, `CGO_ENABLED=0`, `GOEXPERIMENT=greenteagc`.
  Mtime binary/launcher là 16/10 09:26 giờ máy. Đây là build từ cây có thay
  đổi trên HEAD hiện tại; chưa chứng minh nó chứa đúng toàn bộ diff đang có.
- Launcher mặc định `CRUSH_CLIENT_SERVER=1`, chọn socket theo workspace;
  không đặt `AGENT_CLI_DELEGATION` hoặc `AGENT_CLI_ALLOWED_MODELS`.
  Tệp env được kiểm không có dòng gán hai biến đó; chưa kiểm env của mọi
  tiến trình server đang sống. Thay launcher không tự cập nhật server cũ.
- `~/.config/agent-cli/crush.json` symlink tới `.crush.json` trong repo:
  JSON khai GLM `glm-5.3`, `glm-5.3-flash`, context 1000000,
  default max tokens 131072; Anthropic `disable=true`, default providers tắt.
  Đây là metadata cấu hình, chưa phải thông số endpoint được xác nhận.
- **Chưa được kết luận GLM là provider duy nhất hiệu lực:** global `crushrc`
  vẫn tạo provider `main` và lấy model từ env; env hiện ghi `qwen3.8`.
  Cần kiểm merge/load order và danh sách model qua launcher trước khi chốt.
  Không sửa config cá nhân trong nhiệm vụ viết plan.

### Bằng chứng đã chạy và giới hạn

```bash
GOMAXPROCS=2 go test -p 2 ./internal/session ./internal/execpolicy ./internal/agent/tools/provider -count=1
```

Kết quả: cả ba package trả `ok`, exit 0. `git diff --check` exit 0.
Đây là unit/package tests, không chứng minh TUI hoặc provider thật hoạt động.
Đã đọc DB `.crush/crush.db` ở chế độ chỉ đọc: không tìm thấy các marker
`CODEX_VIA_TOOL_OK`, `CLAUDE_FROM_AGENT_CLI`, `CLAUDE_OK_AFTER_REFACTOR`, `PONG`
hoặc title chứa `Go trong go.mod`. DB global được kiểm có kích thước 0.
Không kết luận receipts không tồn tại ở workspace khác; chỉ chưa tìm thấy
receipt trong các DB đã kiểm. Không chạy API có phí hoặc delegation để dựng
lại chứng cứ trong nhiệm vụ tài liệu này.

## 2. Bảng 20 luồng

✅ có bằng chứng trong phạm vi ghi rõ · 🔄 một phần/cần xác minh · ⬜ chưa làm
hoặc chưa đủ bằng chứng nghiệm thu · ⏸ parked có chủ ý theo yêu cầu hiện tại.
Kết quả lịch sử luôn ghi nguồn, không tự nâng thành runtime kiểm hôm nay.

| # | Luồng | Trạng thái | Bằng chứng / phần còn thiếu |
|---|---|---|---|
| 1 | GLM qua Z.ai Coding Plan | 🔄 | JSON có hai model GLM; `DECISIONS.md` ghi PONG qua `zai` ngày 15/10. Chưa kiểm lại endpoint hôm nay; cấu hình Qwen trong crushrc cần reconcile. |
| 2 | Delegation `claude_code` | 🔄 | Tool/parser và package tests xanh. Marker `CLAUDE_FROM_AGENT_CLI`, `CLAUDE_OK_AFTER_REFACTOR` được nêu trong đầu vào plan cũ, chưa tìm thấy receipt để xác nhận runtime. |
| 3 | Delegation `codex` | 🔄 | Tool/parser và package tests xanh; `CODEX_VIA_TOOL_OK` chưa tìm thấy trong DB đã kiểm. Quota/login hiện tại chưa kiểm. |
| 4 | Strategy chung `tools/provider` | ✅ trong phạm vi code/test | Có `delegate.go`, adapter Claude/Codex và package tests vừa chạy xanh; không gán số test chưa đếm. |
| 5 | Execpolicy allowlist + delegation mặc định tắt | ✅ trong phạm vi code/test | `policy.go`, `policy_test.go`, provider policy tests; hai package xanh. Không đồng nghĩa đã chặn mọi cách chạy CLI ngoài. |
| 6 | Bypass qua interpreter | ⬜ B1 ưu tiên | Plan cũ báo node gọi Codex; chưa tái hiện hôm nay. Filter shell có xử lý một số wrapper, không được coi mọi `env`/`sh -c` đều chưa chặn. Cần regression tái hiện cụ thể. |
| 7 | Redact secret toàn tuyến | ⬜ B1 | Chưa có receipt chứng minh che nhất quán tool output, transcript, log và UI. Phải lập inventory trước khi chọn điểm chặn. |
| 8 | Đặt tên session từ prompt | 🔄 B0 | Bốn file local; `session.DerivedTitle`, `GenerateTitle`, `resolveSession`; test session xanh. TUI/runtime title mới chưa xác minh hôm nay. Có helper `DerivedSessionTitle` trùng logic cần soát trước commit. |
| 9 | Chú ý/thông báo xuyên workspace | ⬜ B2 | Có Home và package notification; thiếu acceptance liên workspace, trạng thái done-unseen, debounce và ownership. |
| 10 | Run journal + bền dữ liệu P1.5 | 🔄 B3 | Audit ghi recovery một phần; chưa có receipt restart/resume và fixture context/compact/mất stream. |
| 11 | CLI `wait/approve/answer/events` | ⬜ B4 | Kế hoạch tái dùng server/client; cần kiểm hợp đồng endpoint và CLI hiện có trước khi thêm. |
| 12 | Worktree bằng Go | ⬜ B5 | Chưa có acceptance `wt new/ls/rm`; helper tmux là đường tham chiếu. |
| 13 | Jobs panel owner/elapsed | 🔄 | Audit ghi panel đã có; ownership/elapsed và job history còn cần kiểm. Không dùng phiên bản upstream chưa xác minh làm bằng chứng. |
| 14 | Qwen local thay WorkGPT | ⏸ | Theo yêu cầu cho nghỉ chờ chọn server. Env còn `qwen3.8`, không chứng minh đó là endpoint local hay đang dùng được. |
| 15 | Claude API credits | ⏸ | Chưa kiểm entitlement/claim. Con số “200 USD/tháng” trong bản cũ chưa xác minh, không dùng làm cơ sở triển khai. |
| 16 | Login OpenAI native | ⏸ | Chưa chạy lại login; delegation Codex là hướng Phase 1. Không khẳng định nguyên nhân lỗi chỉ từ option default providers. |
| 17 | Server nhà | ⏸ Phase 3 | Chưa kiểm runtime remote; SSH MVP là SSH vào host rồi chạy tại đó. |
| 18 | herdr guest integration, Lô 1 | ⏸ có điều kiện | Chỉ mở lại khi thực tế cần nhiều CLI khác nhau chạy song song; không chuyển công cụ chính. |
| 19 | Đồng bộ upstream | 🔄 | Commit tool-call repair hiện diện trong git log; các mục port/chờ merge trong audit cần kiểm lại từng diff trước khi nhận. |
| 20 | Cổng WIN Phase 1 | ⬜ | Chưa đủ receipts daily workflow; unit tests và marker chat đơn lẻ không đóng cổng WIN. |

Nguồn đối chiếu: [setup](../../agent-cli-local-setup.md),
[audit herdr/Crush](../../reviews/2026-10-15-audit-herdr-crush-agent-cli.md),
[quyết định](../../../DECISIONS.md), [core MVP](2026-10-14-core-mvp-lat1-lat2.md).
Không có artifact audit WorkGPT độc lập đã kiểm trong nhiệm vụ này; chỉ dùng
các ý tưởng redact, usage, warmup làm đầu vào cần thẩm định.

## 3. Các đợt B0 → B5 và Phase 2

Thứ tự: B0 → B1 → B2 → B3 → B4 → B5 → Phase 2. B3 xử lý phần P1.5,
không tự đóng toàn bộ WIN. B5 lấy worktree từ Phase 2 xuống thành đợt chuẩn bị
có điều kiện: chỉ triển khai sau receipts Phase 1 hoặc khi người duyệt rõ việc
đổi thứ tự roadmap. Không âm thầm mở Phase 3–5.
Các lệnh dưới đây là **verify dự kiến**, chưa chạy trừ những lệnh ghi ở mục 1.

### B0 — Chốt phần đặt tên session đang dở

- **Mục tiêu:** soát và commit phần title sau khi được phép; push commit local
  cùng thay đổi đã duyệt qua nhánh/MR theo luật nhà.
- **Ràng buộc:** giữ nguyên thay đổi có trước; không push thẳng main; không
  gộp sửa provider/policy. Không hứa “5 phút” vì chưa đo.
- **Đường vào:** bốn file code ở mục 1; tests cmd/agent gọi `resolveSession`;
  script installer và launcher chỉ dùng kiểm bản cài.
- **Chỗ không chắc:** helper trùng, Unicode width/rune, fallback khi LLM lỗi,
  resume có đổi tên cũ không; kiểm callers và regression tests trước commit.
- **Cách chứng minh là xong:** `GOMAXPROCS=2 go test -p 2 ./internal/session ./internal/cmd ./internal/agent -count=1`,
  `git diff --check`; smoke TUI và `run` trên fixture bằng launcher thật,
  lưu session ID/title và build revision. Sau authorization và gates: commit
  semantic, push nhánh, mở MR có evidence; báo chính xác thay đổi còn lại.

### B1 — Bypass interpreter + redact secret (S + S)

- **Mục tiêu:** tái hiện và chặn đường vượt delegation policy; che secret trước
  khi dữ liệu đi vào nơi lưu/hiển thị. Hai brief nhỏ, nghiệm thu riêng.
- **Ràng buộc:** giữ hợp đồng `AGENT_CLI_*`; không coi blacklist interpreter
  là sandbox. Nếu không kiểm soát được process con tùy ý, trình người duyệt
  lựa chọn hạn chế execution hoặc sandbox và giới hạn bảo đảm cụ thể.
  Test dùng secret giả, không đưa key thật vào prompt/log/receipt.
- **Đường vào:** `internal/execpolicy/policy.go`, bash tool, shell execution,
  `internal/agent/tools/provider`; tracing output → message/DB → logger/UI.
- **Chỗ không chắc:** đường bypass thật, interpreter/wrapper nào đã được lọc,
  output chunk tách secret, lỗi/mã hóa, false positive và nơi dữ liệu đã ghi
  trước redact; kiểm bằng fixtures và inventory sink, không suy từ tên file.
- **Cách chứng minh là xong:** TDD với CLI giả ghi sentinel: policy off và
  model không được phép đều không tạo sentinel; policy on đúng model chạy
  được. Thử direct/wrapper/interpreter theo attack fixture đã chốt.
  `GOMAXPROCS=2 go test -p 2 ./internal/execpolicy ./internal/agent/tools/provider ./internal/agent/tools ./internal/shell -count=1`.
  Redact fixture phải tìm không thấy secret giả trong output, DB, log, UI và
  export; text thường giữ nguyên, chunk boundary không làm lộ. Brief redact
  phải chốt package/test cụ thể sau inventory trước khi code.

### B2 — Chú ý và thông báo xuyên workspace

- **Mục tiêu:** ưu tiên Blocked > Done-unseen > Working > Idle; Go-To phẳng,
  mở đúng workspace/session và biết việc nào đang cần người.
- **Ràng buộc:** trạng thái từ server; không spam; không notify khi đang xem
  đúng phiên; navigation không gửi prompt hoặc cancel run.
- **Đường vào:** `internal/backend`, `internal/ui/home`,
  `internal/ui/model/sessions_panel.go`, `internal/ui/notification` đã tồn tại.
- **Chỗ không chắc:** nguồn trạng thái liên server, nhiều client xem cùng
  phiên, kênh desktop/OSC; kiểm capability và fixture hai workspace.
- **Cách chứng minh là xong:** `GOMAXPROCS=2 go test -p 2 ./internal/backend ./internal/ui/home ./internal/ui/model ./internal/ui/notification -count=1`;
  receipt hai workspace: phiên nền blocked có thông báo, debounce không lặp,
  mở đúng phiên đánh dấu seen, không trộn owner/cwd.

### B3 — Run journal và bền dữ liệu, P1.5

- **Mục tiêu:** restart không để running giả; transcript và ý định tiếp tục
  còn đọc được; bổ sung fixture context dài/compact lỗi/mất stream.
- **Ràng buộc:** reuse SQLite; không khôi phục process đã chết hoặc replay
  mutation đã xong. JSON registry/config và SQLite có chiến lược backup riêng;
  không sao chép DB đang ghi tùy tiện. Chính sách ba bản backup cần được duyệt.
- **Đường vào:** `internal/db`, `internal/session`, server/coordinator teardown,
  registry/config persistence và fixtures P1.5 trong roadmap.
- **Chỗ không chắc:** liveness qua nhiều client, WAL/backup, resume boundary;
  kiểm kill/restart trong data dir tạm, không kill server làm việc thật.
- **Cách chứng minh là xong:** `GOMAXPROCS=2 go test -p 2 ./internal/db ./internal/session ./internal/server ./internal/workspace -count=1`;
  fixture server bị kill → restart → interrupted → tiếp tục có kiểm soát;
  không chạy lại mutation sentinel, transcript còn. Thử restore backup,
  compact failure và mất stream, ghi receipts để xét P1.5.

### B4 — CLI điều phối

- **Mục tiêu:** `wait`, `approve/deny`, `answer`, `events --json` phục vụ script.
- **Ràng buộc:** reuse endpoint/client hiện có; không tự approve; target owner
  tường minh, timeout/exit code rõ; stream có sequence và `events_lost`.
- **Đường vào:** `internal/cmd`, `internal/client`, `internal/server/events.go`,
  permission/question handlers và scripts E2E hiện có.
- **Chỗ không chắc:** auth/scope endpoint, stale request, reconnect/event mất;
  đối chiếu handlers và fixture trước khi chốt CLI contract.
- **Cách chứng minh là xong:** `GOMAXPROCS=2 go test -p 2 ./internal/cmd ./internal/client ./internal/server -count=1`;
  script fixture không TUI chạy hết blocked → approve/answer → done; deny,
  timeout, sai owner, mất event và reconnect có kết quả xác định.

### B5 — Worktree bằng Go

- **Mục tiêu:** `wt new/ls/rm`, Home nhóm theo git common dir; thay thao tác
  helper thủ công khi đã duyệt điều kiện Phase 1 ở đầu mục này.
- **Ràng buộc:** chặn xóa dirty/untracked hoặc session busy/waiting; không xóa
  branch; không đụng worktree thật trong test, không viết lại Git.
- **Đường vào:** `internal/cmd`, `internal/projects`, `internal/ui/home`,
  `scripts/agent-cli-tmux.sh` làm tham chiếu nếu tồn tại.
- **Chỗ không chắc:** race giữa kiểm sạch và xóa, symlink/path ngoài repo,
  mapping session-worktree; kiểm repo tạm và trạng thái thay đổi giữa hai bước.
- **Cách chứng minh là xong:** `GOMAXPROCS=2 go test -p 2 ./internal/cmd ./internal/projects ./internal/ui/home -count=1`;
  fixture repo tạm tạo/list/xóa worktree sạch; dirty, untracked, busy/waiting
  bị từ chối; branch và workspace khác còn nguyên.

### Phase 2 — Agent chính Claude/Codex, usage và warmup

- **Mục tiêu:** adapter/session driver Claude/Codex làm agent chính; usage
  theo run/session; warmup có lợi ích đo được; model riêng từng session.
- **Ràng buộc:** chỉ mở sau receipts WIN Phase 1; subscription không suy ra
  chi phí USD; usage thiếu ghi unknown, không cộng đôi parent/child.
- **Đường vào:** roadmap Phase 2, provider strategy, coordinator/session,
  message usage và driver lifecycle; giữ delegation Phase 1 hoạt động.
- **Chỗ không chắc:** login/quota, stream/cancel/resume native và lợi ích
  warmup; thử CLI fixture trước, runtime theo authorization; đo trước/sau.
- **Cách chứng minh là xong:** brief riêng chốt package/test cho adapter và
  ledger; baseline `GOMAXPROCS=2 go test -p 2 ./internal/agent/tools/provider ./internal/session -count=1`;
  E2E login/quota/error/cancel/resume, isolation hai session, usage không cộng
  đôi và warmup không tạo việc ngoài ý muốn. Baseline không thay E2E.

Bảng chân trị tối thiểu cho các brief:

| Trường hợp | Kết quả phải có |
|---|---|
| Delegation off hoặc model ngoài allowlist | CLI giả không được chạy |
| Delegation on + model hợp lệ + người duyệt quyền | Đúng CLI/model, outcome có owner |
| Output có secret giả / output thường | Secret bị che ở mọi sink / text thường giữ nguyên |
| Phiên nền blocked / mở đúng phiên | Notify đúng owner / cập nhật seen, không gửi run mới |
| Server chết giữa run | Interrupted, transcript còn, không tự replay mutation |
| CLI sai owner hoặc request cũ | Từ chối, không đổi trạng thái phiên khác |
| Worktree dirty/untracked/busy / sạch và idle | Chặn xóa / chỉ xóa đúng worktree được chọn |

## 4. Bốn quyết định cần người dùng

1. **Delegation:** bật mặc định trong launcher với allowlist cụ thể hay export
   tay? Đề xuất giữ tắt đến khi B1 chứng minh ranh giới; áp dụng phải tính
   server đang sống và không nới permission của CLI con.
2. **Qwen:** dùng server local thay WorkGPT hay tiếp tục parked vì GLM đủ?
   Chưa chọn Ollama/llama.cpp; trước tiên reconcile crushrc/env còn Qwen,
   không kết luận endpoint Studio đã mất chỉ từ việc dự định gỡ app.
3. **Claude API:** có muốn kiểm eligibility/claim credits Max 20x không?
   Chưa xác minh quyền lợi hoặc số tiền; chỉ tra cứu khi mở luồng này,
   delegation không cần chờ quyết định đó.
4. **B0 commit/push:** khi nào cho phép? Theo luật nhà, dùng nhánh/MR,
   không push thẳng `main`. Commit `e0e752fe` chưa push cũng cần soát trong
   phạm vi bàn giao. Nhiệm vụ hiện tại chưa thực hiện commit/push.

Ba việc sớm nhất: duyệt B0 sau smoke title và reconcile bằng chứng; duyệt
brief B1 để TDD bypass/redact; chốt bốn quyết định trên. Chưa đặt thời lượng.

## 5. Đã chốt KHÔNG làm trong phạm vi này

- Không xây registry lớn, gateway đa provider, RAG/knowledge, GUI/dashboard,
  marketplace hoặc i18n chỉ vì dự án tham khảo có sẵn. Các lựa chọn Phase 5
  vẫn nằm ngoài critical path, chỉ mở lại bằng user story được duyệt.
- Không host PTY, giả lập terminal, quét màn hình hoặc live handoff của herdr.
- Không chuyển luồng chính sang herdr. Lô 1 chỉ xét lại khi nhu cầu nhiều CLI
  song song được chứng minh và người dùng mở lại phạm vi.
- Không tự merge, deploy, đổi config cá nhân hoặc chạy B0–B5 từ plan này.
