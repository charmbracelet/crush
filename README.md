# agent-cli

Trợ lý lập trình AI chạy trong terminal, bản fork cá nhân của [Crush](https://github.com/charmbracelet/crush) (Charm). Nối với model local (server tương thích OpenAI, không cần key) hoặc provider qua API key, cấu hình bằng các biến `AGENT_CLI_*` dùng chung.

Fork giữ nguyên giấy phép FSL-1.1-MIT của upstream, xem [LICENSE.md](LICENSE.md). Bản gốc chưa sửa: [docs/UPSTREAM-README.md](docs/UPSTREAM-README.md), hữu ích để tra cú pháp cấu hình.

## Cài và dùng

```bash
bash scripts/install-agent-cli.sh          # build vào ~/.local, tạo crushrc + env mẫu nếu chưa có
export PATH="$HOME/.local/bin:$PATH"

cd /path/to/project
agent-cli                                  # giao diện TUI
agent-cli run "Đọc repo và tóm tắt kiến trúc, chưa sửa code."
agent-cli --continue                       # nối phiên gần nhất
```

`agent-cli run` tự duyệt mọi lời xin quyền (bash, sửa tệp, giao việc cho Claude Code/Codex) của session nó chạy, kể cả session có sẵn qua `-s`/`-C`; muốn được hỏi thì dùng TUI hoặc `agent-cli send`.

Hướng dẫn đầy đủ (cấu hình model, kiểm chứng, cập nhật): [docs/agent-cli-local-setup.md](docs/agent-cli-local-setup.md).

## Cấu hình

| Việc | Ở đâu |
|---|---|
| Provider, model, key | biến `AGENT_CLI_*` trong `~/.config/agent-cli/env` (mode 600, mẫu: `agent-cli.env.example`) |
| Cấu hình chính, quyền công cụ | `~/.config/agent-cli/crushrc` (mẫu: `agent-cli.crushrc.example`) |
| Màu giao diện | `~/.config/agent-cli/themes/*.json`, chọn bằng `options.tui.active_theme` |
| Dữ liệu phiên, log | `~/.local/share/agent-cli`, `.crush/` trong từng project |

Tên tệp và biến môi trường bên trong (`crush.json`, `CRUSH_*`, `.crush/`) giữ nguyên để dễ gộp thay đổi từ upstream. Launcher `agent-cli` đã đặt sẵn các biến đó, tắt metrics, tắt cập nhật catalog provider và tắt kiểm tra phiên bản upstream.

## Khác upstream

- Tên và giao diện: wordmark AGENT-CLI, banner thoát, thông báo, trợ giúp lệnh.
- Không kiểm tra bản mới của upstream (`CRUSH_DISABLE_UPDATE_CHECK`).
- Nhiều session/dự án trong một TUI (Home, Alt+W/S/N/M), draft riêng từng session, run tiếp tục khi đóng TUI; `run --json`, `send --wait`, `ps`, `logs` cho script.
- Tool `claude_code`, `codex`: giao việc cho Claude Code / Codex đã đăng nhập trên máy, mỗi lần giao đều hỏi quyền trừ khi bạn chủ động cho qua (`run`, `--yolo`, "Allow for Session"; xem [docs](docs/agent-cli-local-setup.md#giao-việc-cho-claude-code--codex-tool-claude_code-codex)).
- Chưa đổi: tên module Go, system prompt (còn tự xưng "Crush"), skill và công cụ nội bộ `crush-config`, `crush_info`.

## Phát triển

```bash
go build .
go test ./...        # máy dùng chung: đặt GOMAXPROCS=2 và go test -p 2
```

Quy ước code và kiến trúc: [AGENTS.md](AGENTS.md).
