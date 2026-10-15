# Audit herdr, crush gốc và agent-cli: chọn tính năng cho daily workflow

> 15/10/2026. Nguồn:
> - đọc mã herdr v0.9.5 (`916fcf8`, 14/10/2026);
> - crush upstream `f49b2811`;
> - agent-cli `main` = `d4ccc579`, cộng đợt sửa 15/10 chưa commit;
> - tìm web (danh sách cuối tệp).
>
> Ký hiệu nguồn: **[đọc]** đọc mã hoặc docs, **[chạy]** đã chạy thật, **[web]** lấy từ web, chưa kiểm. Mọi số dòng code Go là ước lượng, chưa đo.
>
> Thước đo "daily" lấy từ `docs/superpowers/plans/2026-10-14-agent-cli-personal-roadmap.md`: mục 4 (daily workflow) và mục 5 (cổng WIN của Phase 1).

## 0. Kết luận

Ba bên khác loại, nên không thay thế nhau một-một.

**herdr** là runtime/multiplexer terminal cho nhiều agent CLI. [đọc]
- Server giữ PTY thật; agent chỉ là thuộc tính của pane.
- Không gọi LLM, không giữ hội thoại.
- Quy mô: Rust khoảng 289 nghìn dòng `src/`, cộng libghostty-vt vendored khoảng 356 nghìn dòng Zig.
- Giấy phép Apache-2.0.

**crush** là chính agent coding: LLM, tool, TUI. [đọc]
- agent-cli là fork của nó, tách ở `a73e1bfa`.
- Từ điểm tách, nhánh main của upstream chỉ thêm 3 commit.
- Phần mới đáng kể nằm ở các nhánh upstream chưa merge.
- Giấy phép FSL-1.1-MIT.

**agent-cli** là crush cộng các lớp mà upstream không có: [đọc][chạy]
- Home và mỗi dự án một server;
- nhiều session chạy nền;
- `run`/`send`/`ps`/`logs` cho script;
- giao việc cho Claude Code/Codex;
- chọn provider→model và ghim model theo lượt;
- draft riêng từng session.

**Khuyến nghị: giữ agent-cli làm lõi daily, code thêm bằng Go.**
- Không chuyển luồng chính sang herdr.
- Không tự viết lại phần host PTY của herdr: khoảng 32 nghìn dòng Rust cộng bộ giả lập terminal, mà MVP không cần.
- Lấy 6 ý tưởng của herdr (mục 4).
- Giữ herdr làm "vỏ ngoài" tuỳ chọn khi chạy cùng lúc từ ba CLI agent khác nhau trở lên. Muốn vậy thì vá tích hợp herdr trước (Lô 1, mục 6).

**Lý do (first principles).** Việc hằng ngày cần là trạng thái đáng tin của session, run, quyền và câu hỏi của chính agent bạn dùng.
- Server của agent-cli là nguồn sự thật của trạng thái đó: nó biết run nào chạy, quyền nào đang chờ.
- herdr phải đoán từ màn hình với mọi CLI không tự báo, kể cả Claude Code và Codex. [đọc]
- Theo docs của chính herdr, prompt lạ có thể hiện thành idle. [web]

## 1. Ba bên ở mức kiến trúc

| Mặt | herdr | crush gốc | agent-cli |
|---|---|---|---|
| Loại | Multiplexer terminal cho agent CLI | Agent coding TUI | Fork crush + lớp đa phiên |
| Đơn vị | session (một server) → workspace → tab → pane (PTY); agent là thuộc tính pane | Session hội thoại | Home → workspace (mỗi dự án một server) → session → run |
| Tiến trình nền | Server giữ PTY. Tắt client hay rớt SSH thì vẫn sống. Restart máy thì mất tiến trình, khôi phục layout rồi gõ lệnh resume | Client/server tuỳ chọn | Server riêng mỗi dự án. Run sống khi đóng TUI; trần 30 phút khi kẹt, 6 giờ khi đang chạy |
| Trạng thái agent | 3 nguồn: nhận diện tiến trình, quét màn hình theo 22 manifest TOML, agent tự báo | Không có | Biết trực tiếp (run/permission/question): BUSY/WAITING/IDLE, ▲ |
| "Đa provider" | Không gọi LLM. "Đa agent" nghĩa là host 24 loại CLI | Catalog catwalk, OAuth | Như crush + Alt+P/M/E, ghim model theo lượt |
| API cho agent | NDJSON socket khoảng 85 method; `agent.wait`, `agent.prompt --wait` (gõ phím giả) | REST+SSE nội bộ | REST+SSE + CLI `run`/`send`/`ps`/`logs` |
| Nhiều máy | `ssh` hệ thống + bridge stdio↔socket | Không | Hoãn (Phase 3) |
| Giấy phép | Apache-2.0 | FSL-1.1-MIT | FSL-1.1-MIT (fork) |

## 2. Checklist tính năng theo daily workflow

Cột "Daily" ghi tình huống trong mục 4 hoặc dòng cổng WIN của lộ trình. Ký hiệu: ✓ có, ~ một phần, ✗ không.

| # | Tính năng | Daily | herdr | crush | agent-cli (sau đợt 15/10) | Hướng | Effort |
|---|---|---|---|---|---|---|---|
| 1 | Home: recent workspace → session cũ → tiếp tục | Đầu ngày; WIN "Mở Home" | ~ (sidebar, Go-To) | ✗ | ✓ | Giữ | — |
| 2 | Workspace theo dự án; worktree là workspace riêng; gom nhóm worktree cùng repo | WIN "Hai workspace" | ✓ (gom theo git common dir) | ✗ | ✓ tách riêng; ✗ gom nhóm trong Home | Code (gom nhóm) | S |
| 3 | Nhiều session, chuyển nhanh, draft riêng | Làm song song; WIN "Hai session" | ~ (tab/pane, không có draft) | ~ | ✓ (Alt+S/N/./,, draft tự lưu sau 2 giây) | Giữ | — |
| 4 | Việc sống khi đóng TUI hoặc rớt SSH | WIN "Đóng/mở client" | ✓ | ~ | ✓ (run của `agent-cli run` thì gắn với client) | Giữ | — |
| 5 | Sống qua restart server hoặc máy | Kết thúc/đổi máy; P1.5 | ~ (layout + gõ lệnh resume) | ✗ | ~ (lịch sử còn; run dở không được đánh dấu) | Code (run journal) | M |
| 6 | Trạng thái chú ý working/blocked/idle + "xong chưa xem", gom theo workspace | Làm song song | ✓ | ✗ | ~ (thiếu "xong chưa xem" và thứ tự ưu tiên) | Code (ý herdr) | S–M |
| 7 | Thông báo nền xuyên workspace (cần quyền, đã xong) | Làm song song | ✓ (có debounce, kiểm lại trước khi báo) | ~ | ~ (chỉ trong TUI đang mở) | Code | M |
| 8 | Quyền và câu hỏi của session nền: không lọt, không treo | WIN "Hai session" | ✗ (chỉ hiện blocked) | ~ | ✓ (giữ prompt theo session, Ctrl+A/Ctrl+S, câu hỏi chờ tối đa 30 giây) | Giữ | — |
| 9 | Đa provider, chọn provider→model/effort, model không đổi giữa chừng | Daily | ✗ | ~ | ✓ | Giữ | — |
| 10 | Giao việc cho Claude Code/Codex có resume | Review; WIN "delegation" | Host TUI của chúng | ✗ | ✓ (`claude_code`, `codex`) | Giữ; Phase 2 làm agent chính | L |
| 11 | Chạy TUI của claude/codex bên trong app | Ngoài MVP | ✓ (lõi herdr) | ✗ | ✗ | Không code; dùng tmux/herdr | — |
| 12 | Lệnh điều phối `wait`/`approve`/`answer`/`events` | Rẻ, mở đường Phase 4 | ✓ | ✗ | ~ (server có grant/answer; thiếu lệnh) | Code | M |
| 13 | Job nền có owner/elapsed/output/cancel | P1.4; WIN "Job nền" | ✗ | ~ (nhánh `feat/job-reports`, `fix/background-job-budget`) | ~ | Port + panel | M |
| 14 | Worktree theo việc: new/ls/rm, chặn xoá thư mục bẩn | Phase 2 | ✓ (khoảng 500 dòng lõi git) | ✗ | ~ (`agent-cli-tmux` bằng bash) | Code bản Go | M |
| 15 | Context dài, compact, lỗi mạng không mất việc | P1.5; WIN "Context/mạng" | ✗ | ~ | ~ (chưa có fixture chứng minh) | Kiểm và sửa theo P1.5 | M |
| 16 | Nhiều máy SSH | Phase 3 | ✓ | ✗ | ✗ | Hoãn; làm thì theo cách herdr | L |
| 17 | Plugin/marketplace | Phase 5 | ✓ | ~ (MCP, skills, hooks) | ~ | Bỏ | — |
| 18 | Ghi đĩa nguyên tử + vòng sao lưu | Bền dữ liệu (máy từng reset cứng) | ✓ | ~ | ~ (`projects.json`, `drafts.json` đã nguyên tử) | Code nhỏ | S |
| 19 | Usage/cost theo run | Phase 2 | ✗ | ~ | ~ | Hoãn | M |

Đọc nhanh:
- Phần lõi daily (1, 3, 4, 8, 9, 10) agent-cli đã có và đã chạy thật.
- Khoảng trống thật nằm ở 6, 7 (biết session nào cần mình), 13, 15 (hai dòng WIN còn mở), 12 và 14.

## 3. Ba phương án

| Phương án | Được | Mất / rủi ro |
|---|---|---|
| **A. Giữ agent-cli làm lõi, code thêm** (khuyến nghị) | Trạng thái chính xác từ nguồn sự thật. Một binary Go. Đã qua audit và test. Đi tiếp theo đúng lộ trình WIN | Phải tự code 6, 7, 12, 13, 14 (vài lô M) |
| **B. Chuyển luồng chính sang herdr**: chạy claude/codex/agent-cli trong pane herdr | Có ngay: rollup chú ý, thông báo, khôi phục layout, nhiều máy | Trạng thái của Claude/Codex là đoán từ màn hình. Thêm một runtime 0.9.x đổi rất nhanh (bản 0.9.2–0.9.5 chỉ trong 2 tuần). Không giải quyết draft/quyền/model của agent-cli. Hôm nay agent-cli chạy trong herdr còn báo sai trạng thái (H2) |
| **C. Kết hợp có điều kiện**: A là lõi; khi cần nhiều CLI cùng lúc thì mở chúng trong herdr | Tận dụng herdr đúng sở trường (host nhiều CLI) | Phải vá tích hợp herdr trước (Lô 1) |

Chọn **A**, chuẩn bị đường **C** bằng Lô 1 khi bạn thực sự chạy nhiều CLI song song.

## 4. Lấy gì từ herdr

Lấy ý tưởng rồi viết lại bằng Go. Apache-2.0 cho phép chép mã, nhưng mã là Rust nên viết lại vẫn rẻ hơn.

1. **"Xong chưa xem" + gom ưu tiên.** Thứ tự: Blocked > Done-unseen > Working > Idle; workspace lấy mức cao nhất. Đánh dấu "đã xem" khi mở đúng session đó. [đọc `workspace/aggregate.rs:50-75`, `app/actions.rs:1829-1850`]
2. **Thông báo có debounce và kiểm lại.** Hẹn `delay`, tới giờ kiểm trạng thái còn như cũ mới báo; không báo khi đang nhìn đúng session đó. [đọc `app/actions.rs:1862-1960`]
3. **Wait theo ngữ nghĩa.**
   - Chờ tới khi session blocked/idle/done.
   - Ghim danh tính để một session thay thế không thoả điều kiện.
   - Từ chối gửi prompt khi đang blocked.
   - Gửi rồi mà không thấy working thì báo "stalled". [đọc `api/wait.rs`]
4. **Luồng sự kiện có số thứ tự**: vòng 512 sự kiện; người đọc chậm thì nhận `events_lost`; đồng bộ theo kiểu subscribe → snapshot → đối chiếu. [đọc `api/event_hub.rs:19-67`]
5. **Worktree**: `git worktree add/remove/list --porcelain`, không xoá branch, kiểm thư mục bẩn trước khi xoá, gom nhóm theo git common dir. [đọc `worktree.rs`]
6. **Ghi bền**: tmp + fsync + rename + fsync thư mục; giữ 3 bản sao lưu khi tệp hỏng. Khôi phục sau restart là dựng lại ý định, không dựng lại tiến trình. [đọc `persist/io.rs:49-68`, `writer.rs:78-123`]

Không lấy: host PTY và giả lập terminal, quét màn hình theo manifest, live handoff (`SCM_RIGHTS`), bàn phím prefix, plugin.

## 5. Các dự án mã nguồn mở khác làm những luồng này

| Dự án | Làm gì | Luồng đáng học | Giấy phép / trạng thái | Dùng cho agent-cli |
|---|---|---|---|---|
| herdr | Runtime terminal cho agent | Trạng thái chú ý, wait, worktree, khôi phục layout | Apache-2.0 [đọc]; khoảng 6 tháng tuổi, bản 0.9.x [web] | Ý tưởng mục 4; vỏ ngoài tuỳ chọn |
| claude-squad | Mỗi agent một tmux session + một git worktree, có TUI điều phối; chạy Claude Code, Codex, OpenCode, Amp... [web] | "Mỗi việc một worktree + một terminal" | AGPL-3.0 [web]: **không chép mã** | Thiết kế worktree helper (#14) |
| uzi | Chạy nhiều agent song song bằng worktree + tmux; mỗi agent một port dev server; gộp branch một lệnh [web] | Lô việc song song, port riêng | Chưa xác minh giấy phép. Chế độ auto duyệt mọi tool [web], rủi ro | Ý tưởng port riêng cho dev server |
| dmux | Agent song song trong tmux + worktree, fan-out nhiều lựa chọn [web] | Fan-out | MIT [web] | Tham khảo |
| ccmux | Nhảy tới agent đang cần mình, spawn vào worktree, chuyển việc giữa agent [web] | "Jump to the one that needs you" | Chưa xác minh | Ý tưởng Go-To theo trạng thái (#6) |
| Nimbalyst (trước là Crystal) | App desktop chạy nhiều Claude Code/Codex song song [web] | GUI | MIT [web] | Không hợp (MVP không GUI) |
| Vibe Kanban | Kanban điều phối agent trên trình duyệt [web] | Kanban + diff | Apache-2.0; công ty đóng 04/2026, cộng đồng duy trì [web] | Không chọn |
| opencode | Server HTTP + luồng sự kiện ở giữa; TUI, desktop, SDK đều là client; session fork được; 75+ provider qua models.dev; **đổi model chỉ áp cho session đang mở** [web] | Mô hình "model theo session", catalog models.dev | Mã nguồn mở [web] | Đối chiếu cho Phase 2 "model riêng từng session" |
| Claude Code | Có sẵn worktree (`claude -w`) từ v2.1.49 [web] | — | Đóng nguồn | Giao việc qua `claude_code` (đã có) |

Tóm lại: các công cụ "song song" đều hội tụ về cùng một khuôn: **một việc = một worktree + một tiến trình agent + một chỗ xem trạng thái**. agent-cli đã có phần tiến trình và trạng thái, còn thiếu worktree helper bằng Go (#14) và "chỗ xem" gom trạng thái (#6, #7).

## 6. Kế hoạch code (Go)

Mỗi lô giao xong là cài kèm bản sao lưu và lệnh quay lại, theo CLAUDE.md P1.

**Thứ tự (đã chỉnh 15/10 theo cổng WIN, không lệch lộ trình):** hai lỗ WIN còn lại là P1.4 (job/activity) và P1.5 (lỗi, run bị gián đoạn), nên làm trước:

1. **P1.4 = Lô 6 + Lô 2:** job có owner + trạng thái chú ý/"xong chưa xem"/thông báo. Lấy từ herdr: gom ưu tiên, done-unseen, thông báo có debounce.
2. **P1.5 = Lô 5:** run journal, đánh dấu interrupted, ghi bền. Lấy từ herdr: ghi nguyên tử, sao lưu, "khôi phục = dựng lại ý định".
3. Lô 3 → Lô 4 → (Lô 1 nếu dùng herdr) → Lô 7 làm rải rác.

Không làm: Phase 3–5 (SSH, plugin, GUI) và phần host PTY của herdr.

Lô 0 là đợt sửa 15/10, đã code, chờ commit.

### Lô 1: agent-cli làm "khách" đúng trong herdr (S–M)

Chỉ làm nếu chọn đường C. Các lỗi đánh số theo báo cáo crush↔herdr:
- H1 (**đã sửa 15/10**): server và CLI con không còn nhận `HERDR_*`, nên không báo chồng lên pane của client.
- H2: trạng thái tính theo session gốc chứ không theo tiến trình.
  - `working` nếu có session top-level đang bận; `blocked` nếu có quyền hoặc câu hỏi đang chờ.
  - Bỏ qua session con.
  - Lấy `RunComplete` làm nguồn quyết định.
- H3: gửi `resume_argv: ["agent-cli","--session",<id>]`, để herdr restart thì khôi phục được.
- H4: câu hỏi `question` thành `blocked`, kèm `message`.
- H6: báo cáo mới nhất thắng (bỏ bản cũ khi hàng đợi đầy), và ghi log lỗi trong phản hồi.
- H7: đổi nhãn `crush` thành `agent-cli`.

Tệp: `internal/herdr/*`, `internal/app/app.go`, `internal/workspace/client_workspace.go`.

Chứng minh xong:
- test hợp đồng với `herdr api schema --json`;
- chạy herdr thật một pane, xem `herdr agent list` qua các tình huống working/blocked/idle/restart.

### Lô 2: chú ý và thông báo (M)

- Trạng thái "xong chưa xem" theo session (đánh dấu đã xem khi mở session), lưu ở server.
- Gom ưu tiên Blocked > Done-unseen > Working > Idle cho panel Sessions và Home.
- Danh sách "Go-To" phẳng mọi session của mọi workspace, lọc theo trạng thái.
- Thông báo nền (desktop/OSC) khi một session chuyển sang blocked hoặc done mà không client nào đang xem nó: debounce và kiểm lại trước khi báo.

Tệp: `internal/backend` (trạng thái theo session), `internal/ui/home`, `internal/ui/model/sessions_panel.go`, `internal/ui/notification`.

Chứng minh xong:
- test chuyển trạng thái;
- chạy thật: hai workspace, một session chờ quyền ở workspace không mở → Home hiện ▲ và có thông báo.

### Lô 5: run bị gián đoạn và bền dữ liệu (M; P1.5, WIN "Context/mạng lỗi")

- Run journal (bắt đầu, kết thúc, kết quả) trong SQLite.
- Server khởi động lại thì đánh dấu run dở là `interrupted`, không hiện running giả; có lệnh hoặc nút "tiếp tục".
- Ghi nguyên tử + vòng sao lưu cho các tệp JSON còn lại.
- Fixture context dài, compact lỗi, mất stream.

Chứng minh xong: các kịch bản P1.5 chạy thật, lịch sử còn, không lặp thao tác sửa.

### Lô 6: job nền có owner (M; P1.4, WIN "Job nền")

- Port `fix/background-job-budget` (`0ac04e4f`); port `feat/job-reports` (`ca41d145`) khi upstream merge.
- Panel job: owner, bắt đầu, thời gian chạy, output, cancel.

Chứng minh xong: kịch bản WIN "Job nền".

### Lô 3: CLI điều phối (M)

- `agent-cli wait -s <id> --until blocked|idle|done [--timeout]`;
- `agent-cli approve|deny <request-id>`, `agent-cli answer`;
- `agent-cli events --json` (có số thứ tự và `events_lost`).

Dùng endpoint sẵn có (`/permissions/grant`, `/questions/answer`, SSE).

Chứng minh xong: script e2e như `test-agent-cli-run-json.sh`.

### Lô 4: worktree bằng Go (M)

- `agent-cli wt new|ls|rm` thay `agent-cli-tmux.sh`.
- Chặn xoá thư mục bẩn hoặc worktree còn session BUSY/WAITING; không xoá branch.
- Home gom worktree theo repo.

Chứng minh xong: test với repo tạm, không đụng repo thật.

### Lô 7: đồng bộ upstream (S–L, làm rải rác)

Làm ngay:
- `a48dabea` program status done/error (S, sửa tay 1 hunk ở `ui.go`);
- `fix/shutdown-records-work` (S);
- `fix/dangling-tool-calls` (M);
- `undefault` (S–M).

Chờ upstream merge rồi mới rebase (L):
- `super-yolo` (thay vá bash S1 của fork);
- `noninteractive-subagent-permission-hang` (trùng run/send/backend);
- `hooks-galore` (trùng Stop hook).

**Hoãn hoặc bỏ:** SSH nhiều máy (Phase 3), plugin, host PTY, live handoff, bàn phím prefix.

## 7. Việc cần bạn quyết

1. Hướng: A (+ chuẩn bị C), hay B?
2. Thứ tự lô: theo khuyến nghị (2 → 5 → 6 → 3 → 4), hay đưa Lô 1 lên trước nếu bạn định dùng herdr ngay.
3. Có cài herdr lên máy để thử Lô 1 không? Lệnh cài chính thức là `curl … | sh` từ herdr.dev; phải có bạn đồng ý.

## Nguồn web

- [smtg-ai/claude-squad](https://github.com/smtg-ai/claude-squad), [Claude Squad review 2026](https://vibecodinghub.org/blog/claude-squad-review)
- [devflowinc/uzi](https://github.com/devflowinc/uzi)
- [dmux](https://dmux.ai/), [epilande/ccmux](https://github.com/epilande/ccmux)
- [nimbalyst/nimbalyst](https://github.com/nimbalyst/nimbalyst), [Crystal → Nimbalyst](https://nimbalyst.com/crystal/)
- [Vibe Kanban release notes](https://www.vibekanban.com/release-notes), [Vibe Kanban sunsetting](https://first-tree.ai/compare/vs-vibe-kanban)
- [opencode models docs](https://opencode.ai/docs/models), [opencode (aiwiki)](https://aiwiki.ai/wiki/opencode)
- [herdr compare](https://herdr.dev/compare/), [A deep dive into Herdr](https://flaviocopes.com/herdr/), [herdr on Tecmint](https://tecmint.com/herdr-run-ai-coding-agents-in-linux-terminal)
- [Claude Code: run agents in parallel](https://code.claude.com/docs/en/agents), [Best Git worktree tools for AI coding 2026](https://nimbalyst.com/blog/best-git-worktree-tools-ai-coding-2026/)
