# Core MVP dùng được: lát 1 → 2 → 3 rồi tới cổng WIN

> Người thi hành: được quyền phản biện plan.
> Cấm: viết số chưa chạy lệnh, nở phạm vi, commit/push khi user chưa cho phép.
> Mọi lệnh Go chạy qua `.superpowers/sdd/2026-10-14-multi-session-multi-project/go.sh`. Tối đa 3 agent cùng lúc.

## Mục tiêu

User cài và dùng được agent-cli cho daily work. Làm từng lát: xong lát nào thì cài lát đó, mỗi lát đều có lệnh quay lại bản cũ.

Thay đổi so với cách làm trước:
- Không dùng mutant.
- Không giao lượt soát lại: khi sửa xong, điều phối tự chạy lại đúng test mà người soát đã dùng để bắt lỗi.
- Soát đầy đủ chỉ dành cho an ninh và vòng đời run.

## Lát 1: dùng được hằng ngày (mục tiêu khoảng 17:30)

| Việc | Trạng thái | Còn lại |
|---|---|---|
| Home → workspace → session; Alt+W về Home; registry được ghi ở chế độ client/server (V03, C01) | H2 đang code | Xong H2, rồi một lượt soát |
| Alt+S / Alt+N / Alt+M thêm cạnh Ctrl+S / Ctrl+N / Ctrl+L; footer `Alt+W Workspace · Alt+S Sessions · Alt+N New · Alt+M Model` (V01, V09) | Chưa làm | Một task nhỏ dùng Sonnet sau H2, vì cùng sửa keys.go. Test: Esc trong picker không huỷ run, Enter trong picker không gửi draft |
| Run sống khi đóng TUI hoặc về Home; Ctrl+C chỉ huỷ run của chính nó (H0) | Vòng 1 xong (worker tự báo) | Soát gộp với H2 |
| Lỗ bash: lệnh bọc, `>`, `&`, xuống dòng phải hỏi quyền (S1 + việc vặt) | Xong (worker tự báo), có test đỏ → xanh | Điều phối chạy lại test |
| Hook Stop: agent vòng 1b bị reboot cắt ngang | Không rõ | Chạy test `StopHook`. Đỏ thì trả `stop_hook.go` và test về bản đã soát ở `snap 2797f1d`. Hook là opt-in, user không cấu hình |
| Cài: gộp 2 cây → build → test lõi → sao lưu → cài → chạy provider thật 1 lượt (không in secret) | Đã có `preview-install.sh` | Chạy |

**Nghiệm thu lát 1:** user chạy được trên launcher thật:
1. Gõ `agent-cli` để vào Home, chọn repo, rồi resume một session.
2. Hai session chạy song song, thấy trạng thái từng session.
3. Alt+W quay về Home, sau đó mở lại thì run vẫn chạy.
4. Alt+S, Alt+N, Alt+M mở đúng picker.
5. `agent-cli ps` thấy các run đang chạy.

## Lát 2: hai session chạy cùng lúc mà không lẫn (khoảng 1–1,5 giờ sau lát 1)

- **H3:** draft và attachments riêng từng session. Đổi session hoặc workspace không gửi prompt, không huỷ run (V05).
  - Task UI → Sonnet, một lượt soát.
- **V-lite (sau A7, Sonnet, khoảng 15 phút):** V08 thêm chữ trạng thái vào panel session; V10 giữ dòng đang chọn ở Home khi danh sách cập nhật.
- **A7:** session nền xin quyền hoặc hỏi user thì chỉ hiện badge ở đúng session, không cướp focus, không bị duyệt nhầm (V06).
  - Đụng quyền → Opus, một lượt soát.

## Lát 3: job nền và lỗi (khoảng 1–1,5 giờ sau lát 2)

- **H4:** job bash nền có owner (session), thời gian, output, kết quả, và huỷ được đúng job. Log ghi `workspace_id`/`session_id`/`job_id`.
- **P1.5 (chỉ kiểm, sửa khi đỏ):** mất mạng, lỗi provider hoặc tràn context thì không chạy lặp mutation, lịch sử còn nguyên, có đường làm tiếp.

## Cổng WIN (roadmap mục 5) → lát nào đóng

Cột "Lát" ghi mục tiêu theo plan, chưa phải kết quả đã kiểm.

| Kịch bản WIN | Lát | Kiểm thế nào |
|---|---|---|
| Mở Home ngoài repo: hiện Recent, không khởi động mọi workspace | 1 | `agent-cli` ở `~`, `agent-cli ps` không thấy server mới |
| Chọn workspace/session cũ: lịch sử đúng, không tạo run mới | 1 | Resume session cũ, `ps` không có run mới |
| Đọc/sửa file, chạy test/build: lệnh và kết quả thật, diff đúng | 1 | Một việc thật trên repo của user |
| Hai session: chuyển session, huỷ đúng owner | 1 | Hai session chạy, Esc chỉ huỷ session đang xem |
| Hai session: draft và quyền đúng owner | 2 | H3 + A7 |
| Hai workspace: model/cwd/log không bị trộn | 1 | Hai repo, hai model, `agent-cli logs` ở từng repo |
| Delegation Claude/Codex: ID resume và kết quả thật | 1 | Tool `claude_code`/`codex` (từ cây chính); quota thì ghi "chưa đạt phần đó" |
| Job nền: có owner, thời gian, output | 3 | H4 |
| Lỗi context/mạng: không lặp mutation, lịch sử còn | 3 | P1.5 |
| Đóng/mở client: đúng lifecycle đã công bố | 1 | H0: Alt+W rồi mở lại, run vẫn chạy và huỷ được |
| Update launcher: config/session giữ nguyên | 1 | `preview-install.sh` sao lưu; so `~/.config/agent-cli` trước và sau khi cài |

**WIN chỉ được xác nhận khi user tự đi trọn daily workflow trên bản cài.** Agent chỉ chuẩn bị bằng chứng cho từng dòng.

## Đối chiếu feedback UI (`docs/reviews/2026-10-14-phase1-ui-feedback.md`)

| ID | Quyết định | Lý do |
|---|---|---|
| V01, C01 | ACCEPT, lát 1 | Bốn phím core. Gỡ phím mặc định của index và switcher đã nằm trong H2 |
| V03 | ACCEPT, lát 1 | Đúng phạm vi H2 |
| V04 | ALREADY | Ctrl+S (sắp có Alt+S) mở dialog session có ô tìm theo tên |
| V02 | ALREADY | `header.go:178-189` đã hiện branch • cwd và model. Đã kiểm code, chưa kiểm trên màn hình thật |
| V05 | ACCEPT, lát 2 | H3 |
| V06 | ACCEPT có điều kiện, lát 2 | A7, nếu dùng thật gặp |
| V08 | ACCEPT, lát 2 (V-lite) | Panel chỉ có `●`/`▲` và màu (`sessions_panel.go:295-297`). Thêm chữ ngắn `running`/`waiting`/`failed` để không phải đoán màu |
| V10 | ACCEPT, lát 2 (V-lite), chỉ Home | Danh sách Active của Home tới sau khi probe xong (tối đa 3s). Đang bấm ↑↓ mà danh sách đổi thì Enter chọn nhầm dòng. Giữ dòng đang chọn theo path |
| V07, V11–V14 | DEFER | V07 thuộc H4 (lát 3). V11 đã có một phần: Home luôn có "Mở thư mục", và trạng thái không biết thì hiện `unknown`. V12–V14 làm sau khi đã dùng |
| C02, C03, C04 | ALREADY | Draft không phụ thuộc switcher. Keybinding framework và accept-edits đã hoãn hoặc gỡ khỏi Shift+Tab |
| C05, C06 | ALREADY | Launcher đã tắt metrics và auto-update. Không cấu hình provider hay MCP thừa |
| C09 | DEFER | L1, sau lát 2 |
| C07, C08, C10 | REJECT trong Phase 1 | Không giải quyết việc dùng hằng ngày |
