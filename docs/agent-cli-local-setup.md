# Cài agent-cli trên máy cá nhân hoặc server nhà

Fork từ Crush; giữ nguyên LICENSE.md và thông báo upstream.

## Cài đặt

Cần Git và Go hỗ trợ phiên bản trong go.mod (Go có thể tải toolchain tương ứng).

```bash
git clone https://github.com/luannt2002/agent-cli.git
cd agent-cli
bash scripts/install-agent-cli.sh
export PATH="$HOME/.local/bin:$PATH"
```

Script build tối đa hai luồng, rồi chép hai tệp mẫu nếu chưa có (không bao giờ ghi đè; riêng trường hợp đã có `crush.json` khai provider, xem "Chuyển từ `crush.json` cũ"):
`~/.config/agent-cli/crushrc` (cấu hình chính) và `~/.config/agent-cli/env` (mode 600,
chứa endpoint, model, key). Sau khi cài, mở `~/.config/agent-cli/env` và điền giá trị thật
theo mục dưới. Tệp env nằm ngoài repo; không commit key hoặc OAuth token. Sao lưu cấu hình
thật riêng bằng kênh an toàn.

## Cấu hình model

Provider và model được cấu hình bằng biến dùng chung `AGENT_CLI_*` trong
`~/.config/agent-cli/env`. `crushrc` (bản mẫu: `agent-cli.crushrc.example`) `source` tệp này
(đổi đường dẫn bằng `AGENT_CLI_ENV_FILE`; bỏ qua nếu tệp không có) rồi khai một provider tên
`main` cùng model lớn/nhỏ. Mẫu env: `agent-cli.env.example`, chỉ chứa placeholder.

| Biến | Ý nghĩa | Mặc định |
|---|---|---|
| `AGENT_CLI_PROVIDER_TYPE` | loại provider: `openai-compat`, `openai`, `anthropic`, `ollama`... | `openai-compat` |
| `AGENT_CLI_BASE_URL` | URL gốc của API (bắt buộc); dạng URL theo loại: `openai-compat`, `openai`, `ollama` tới `/v1`; `anthropic` là URL gốc KHÔNG có `/v1` | không có |
| `AGENT_CLI_API_KEY` | API key; để rỗng cho model local | rỗng |
| `AGENT_CLI_MODEL` | model chính (bắt buộc) | không có |
| `AGENT_CLI_SMALL_MODEL` | model nhỏ (đặt tiêu đề...) | `AGENT_CLI_MODEL` |
| `AGENT_CLI_CONTEXT_WINDOW` | context window khai cho model (áp cho cả model chính và nhỏ) | 128000 |
| `AGENT_CLI_MAX_TOKENS` | số token đầu ra tối đa (áp cho cả model chính và nhỏ) | 8192 |

Giá trị có ký tự đặc biệt (`<`, khoảng trắng) phải đặt trong dấu nháy kép.

Thiếu biến bắt buộc thì nạp cấu hình thất bại, và thông báo lỗi nêu đúng biến thiếu ở cả hai
chế độ: chế độ client/server (mặc định) nối phần cuối `stderr.log` của server vào câu
`Failed to initialize crush server: ...`, ví dụ `... exit status 1: AGENT_CLI_BASE_URL: đặt
AGENT_CLI_BASE_URL trong ...`.

Dạng `AGENT_CLI_BASE_URL` theo loại provider:

- `openai-compat`, `openai`, `ollama`: phần tới `/v1`, KHÔNG gồm `/chat/completions` (client
  tự thêm; thêm đuôi này vào sẽ ra URL sai).
- `anthropic`: URL gốc, KHÔNG có `/v1` (client tự thêm `/v1/messages`).

Crushrc mẫu tắt metrics, tự cập nhật catalog và provider mặc định, và ẩn các tool chạm mạng
`sourcegraph`, `fetch`, `agentic_fetch`, `download` (tên phải đúng tên tool; tên sai bị bỏ qua
không báo lỗi). Mẫu KHÔNG `permissions allow` tool đọc nào: trong thư mục dự án `view`, `ls`,
`glob`, `grep` vốn chạy không hỏi; `view`/`ls` chỉ hỏi khi đọc ngoài dự án (ví dụ tệp env chứa
key), và allow sẽ bỏ luôn lời hỏi đó. Bản crushrc cài từ mẫu cũ còn dòng
`permissions allow view ls glob grep rg`: nên xoá dòng đó.

### Cách 1: model local (không cần key)

Chạy một server tương thích OpenAI trên máy (llama.cpp, vLLM, Ollama...), rồi:

```bash
AGENT_CLI_PROVIDER_TYPE=openai-compat
AGENT_CLI_BASE_URL="http://127.0.0.1:<port>/v1"
AGENT_CLI_API_KEY=
AGENT_CLI_MODEL="<model-id>"
```

`<model-id>` là tên model server đó báo ra. Trên server nhà, `127.0.0.1` chỉ là chính server
nhà: model phải chạy ở đó, hoặc trỏ tới địa chỉ thực truy cập được. Không mở cổng model ra
công khai chỉ để thử.

### Cách 2: provider qua API key

```bash
AGENT_CLI_PROVIDER_TYPE=openai-compat
AGENT_CLI_BASE_URL="https://<your-endpoint>/v1"
AGENT_CLI_API_KEY="<your-api-key>"
AGENT_CLI_MODEL="<model-id>"
```

Key không vào repo và không dán vào chat. Nên lấy key từ kho bí mật, vì tệp env được shell
nạp nên `$(lệnh)` chạy được:

```bash
AGENT_CLI_API_KEY="$(pass show <entry>)"
```

Tránh để key dạng văn bản thường trong tệp env: tool `view` (cấu hình mẫu cho chạy tự động,
không hỏi) có thể đọc tệp ngoài thư mục dự án, kể cả `~/.config/agent-cli/env`, nên model có thể đọc key đó. Mode
600 chỉ chặn người dùng khác, không chặn agent chạy dưới tài khoản của bạn.

Kiểm cấu hình đã nạp mà không gọi model: `agent-cli models` liệt kê `main/<model-id>`.

### Sửa env khi agent-cli đang chạy

Mỗi dự án có một server nền; cấu hình (crushrc, env) được nạp lại mỗi khi một workspace mới
mở. Sửa `~/.config/agent-cli/env` (hay crushrc) KHÔNG tác dụng với workspace đang mở: còn một
client (TUI hay `run`) nào của dự án đó đang sống thì nó vẫn dùng giá trị cũ (đã đo: `touch
crushrc` cũng không kích hoạt nạp lại). Để áp giá trị mới: thoát hết client của dự án đó, rồi
mở lại; hoặc dừng thẳng server của dự án: `pgrep -af 'crush server --host'` liệt kê các server
(tên socket `agent-cli-<8 hex>` xem ở mục "Một server cho mỗi dự án" bên dưới), rồi `kill <pid>`;
việc đang chạy ở dự án đó sẽ mất.

### Chuyển từ `crush.json` cũ

`~/.config/agent-cli/crush.json` vẫn được đọc và gộp chung với `crushrc`, nhưng cùng thư mục thì
`crushrc` thắng. Vì vậy nếu `crush.json` (kể cả khi nó là symlink; installer theo symlink để
đọc, không sửa) đang khai provider, installer KHÔNG tạo `crushrc`/`env` (sẽ thay provider đang
chạy bằng placeholder): nó chỉ đặt `crushrc.example` và `env.example` cạnh `crush.json` (không
ghi đè bản có sẵn) và in hướng dẫn. Cũng vậy khi thiếu `jq` hoặc `crush.json` hỏng (không xác
định được thì coi như có provider). Để chuyển: xoá `providers` và `models` khỏi `crush.json`
(giữ theme...), đổi tên `crushrc.example` thành `crushrc`, `env.example` thành `env`
(`chmod 600`), điền giá trị rồi chạy lại. `crush.json` không khai provider thì installer tạo
`crushrc` + `env` như bình thường.

## Sử dụng

```bash
agent-cli                      # Home: chọn workspace rồi session (xem mục Home)
agent-cli -c /path/to/project  # vào thẳng workspace, bỏ qua Home
agent-cli run "Đọc repo và tóm tắt kiến trúc, chưa sửa code."
agent-cli --continue
agent-cli login openai
```

Trong TUI và `send`: sửa file, chạy shell (trừ lệnh chỉ đọc an toàn), đọc ngoài thư mục
dự án và giao việc cho Claude Code/Codex đều hỏi quyền; không dùng `--yolo` mặc định.
**`agent-cli run` thì tự duyệt MỌI lời xin quyền của session nó chạy** (session mới, hoặc
session có sẵn qua `-s`/`-C`; gồm bash, edit, claude_code, codex...): ràng buộc duy nhất là lời
trong prompt. Cần được hỏi thì dùng TUI
hoặc `send`. Cấu hình project có thể ghi đè cấu hình global.
Metrics, cập nhật catalog provider và kiểm tra phiên bản upstream (`CRUSH_DISABLE_UPDATE_CHECK`) bị tắt trong launcher; điều này không chặn
mọi network request (model, web/MCP và kiểm tra cập nhật app là các đường riêng).

Model chọn trong TUI (kể cả bật/tắt thinking, mức reasoning) giờ thuộc về dự án:
được ghi vào `<project>/.crush/crush.json`, nên đổi model ở worktree này không
đổi model ở worktree khác. Mở lại một session cũ cũng ghi model của session đó
vào file dự án, trừ khi đang có run ở bất kỳ session nào: model là chung cho cả
workspace và run đọc lại nó ở mỗi bước, nên đổi lúc đó là đổi provider giữa chừng run. Thứ tự gộp config, từ thấp đến cao: `~/.config/agent-cli/crush.json`
< `~/.local/share/agent-cli/crush.json` < `crush.json` / `.crush.json` trong repo
< `<project>/.crush/crush.json` (cao nhất). Hệ quả:

- Dự án chưa từng chọn model thì theo model mặc định toàn cục. Đã chọn một lần
  thì không còn theo mặc định nữa.
- Muốn dự án theo mặc định trở lại: xoá `models` (và `recent_models`) khỏi
  `<project>/.crush/crush.json`.
- Muốn đổi mặc định toàn cục: các bản TUI cũ ghi lựa chọn vào
  `~/.local/share/agent-cli/crush.json` (nằm TRÊN file trong `~/.config`), nên
  phải xoá `models` khỏi file đó rồi đặt `models` ở `~/.config/agent-cli/crush.json`;
  chỉ sửa file trong `~/.config` thì không có tác dụng nếu file data còn `models`.

## Home: chọn workspace và session

`agent-cli` không tham số (chỉ `-d/--debug` là được) trên terminal mở **Home**, kiểu "Open Recent":

- **Active**: workspace có server đang sống, kèm số session `running` / `waiting` (chờ duyệt hoặc trả lời).
  Server có socket mà không trả lời trong 3 giây hiện `server not responding (unknown)`, không bao giờ
  bị coi là rảnh. Home chỉ hỏi server đang chạy (quét socket như `agent-cli ps`), không khởi động server nào.
- **Pinned**, rồi **Recent**: 5 workspace mở gần nhất. 5 chỉ là giới hạn hiển thị: sổ
  (`~/.local/share/agent-cli/projects.json`) giữ hết; gõ để tìm theo tên/đường dẫn trong toàn bộ sổ,
  hoặc chọn `→ Show all / search`.
- `→ Open current folder`, `→ Open other folder…` (nhập đường dẫn thư mục đã có; chỉ đăng ký vào sổ,
  không `git init`/clone/tạo worktree). Thư mục con của repo quy về gốc repo.
- Phím: gõ để tìm · `↑↓` chọn · `enter` mở · `ctrl+p` ghim/bỏ ghim · `ctrl+x` bỏ khỏi danh sách một
  workspace có thư mục đã mất (chỉ sửa sổ, không xoá dữ liệu) · `esc` xoá ô tìm, `esc` lần nữa thoát.

Chọn workspace -> danh sách session (tối đa 50, mới nhất trước) và `→ New session`. Server đang sống
thì lấy session và trạng thái thật qua server; không có server thì đọc thẳng DB của workspace ở chế độ
chỉ đọc và trạng thái là `unknown`. Workspace từng mở từ thư mục con có DB riêng (`sub/.crush`) thì
session ở mọi DB đều hiện và mở đúng thư mục chứa nó. `→ New session` mở màn hình trống, chưa tạo
session tới khi bạn gửi prompt; chọn session đang chạy chỉ mở để xem/tiếp, không gửi lại gì. `esc` về
danh sách workspace.

Home chạy TUI của workspace như process con trên cùng terminal (`agent-cli -c <dir> [--session <id>]`,
qua chính launcher nên mỗi dự án vẫn có server riêng). Trong TUI, `Alt+W` thoát về Home và không gửi
lệnh huỷ nào; với client/server (mặc định của launcher) server giữ run đang chạy khi client cuối rời,
tối đa `CRUSH_SERVER_ORPHAN_RUN_TIMEOUT` (mặc định 30 phút). **In-process
(`CRUSH_CLIENT_SERVER=0`): `Alt+W` đóng app nên huỷ run đang chạy**, đợi run xong rồi hãy bấm.
`agent-cli -c dir` mở thẳng rồi bấm `Alt+W` cũng về Home (process tự thay bằng Home).

Chỉ thư mục trên máy này (workspace của máy khác trong sổ bị ẩn). Làm việc trên server: SSH vào
server rồi chạy `agent-cli` ở đó.

## Một server cho mỗi dự án

Launcher bật sẵn `CRUSH_CLIENT_SERVER=1`: mọi `agent-cli` (TUI, `run`, script) chạy
như client và nối vào một server nền. Launcher tự thêm
`--host unix:///run/user/$UID/agent-cli-<8 hex đầu của sha1(thư mục gốc)>.sock`
khi bạn không tự truyền `--host`/`-H`. Thư mục gốc là `git rev-parse --show-toplevel`
của `-c/--cwd` (hoặc thư mục hiện tại); ngoài repo git thì lấy đường dẫn thật
(`pwd -P`). Kết quả: mọi process cùng một dự án dùng chung một server nên trạng thái
session (bận, chờ duyệt) đúng ở mọi nơi; dự án khác thì server khác, log và DB tách riêng.
Server tự tắt sau một khoảng ngắn không còn client (theo plan khoảng 60 giây; chưa đo). Không có `/run/user/$UID` thì
socket nằm ở `$XDG_RUNTIME_DIR`, rồi tới `$HOME/.cache/agent-cli` (mode 700; cố ý không
dùng `/tmp` vì ai cũng ghi được). Đặt `AGENT_CLI_RUNTIME_DIR` để chọn thư mục khác.

- Log server: `~/.cache/agent-cli/server-<host>/crush.log` (`<host>` là địa chỉ socket
  đã thay ký tự lạ bằng `_`). Mỗi dự án một thư mục, không trộn log.
- Tắt: `CRUSH_CLIENT_SERVER=0 agent-cli` chạy in-process như trước và launcher không
  thêm `--host`. Giới hạn của chế độ này: danh sách session vẫn đủ (đọc từ DB) nhưng
  trạng thái bận/chờ duyệt chỉ đúng với session do chính process đó chạy.
- Tự chọn server: `agent-cli --host unix:///đường/dẫn.sock ...` được giữ nguyên.
- Home (`agent-cli` không tham số trên terminal) không có `--host`: chưa chọn workspace nên chưa có
  server; Home gọi lại launcher với `-c <dir>` cho từng workspace.
- Đã biết: client mất stream mà không thoát sạch thì server giữ workspace (và run) thêm
  một khoảng detach grace, mặc định 10 giây (`backend.DefaultDetachGrace`, đổi bằng
  `CRUSH_SERVER_DETACH_GRACE`); thoát sạch thì bỏ qua grace.
- Hết grace mà workspace còn run thì server không dọn: kiểm lại mỗi 5 giây, dọn khi run
  xong (kiểm bằng `scripts/test-agent-cli-run-json.sh`: client bị SIGKILL giữa run, run vẫn
  xong). Giới hạn `CRUSH_SERVER_ORPHAN_RUN_TIMEOUT` (giây, mặc định 1800; `0` = dọn ngay như
  cũ): run kẹt chờ duyệt/trả lời mà không ai mở session thì hết hạn này bị huỷ.
- Ctrl+C với `run` trên session do chính nó tạo thì huỷ run đó; với `run -s/-C` hay
  `send --wait` thì chỉ thôi chờ và báo `stopped waiting; the run keeps going on session <id>`,
  run vẫn chạy trên server.
- `run -s/-C` xếp hàng sau lượt khác (vd. lượt TUI) trong cùng session: câu hỏi và yêu cầu
  duyệt quyền của lượt đó để nguyên cho TUI (không tự duyệt, không huỷ session); `run` in một
  dòng `... from another turn is pending on session <id>; this run keeps waiting behind it`
  ra stderr rồi chờ tiếp. Nhận biết theo nội dung prompt trong hàng đợi: prompt trùng y hệt
  của người khác có thể bị đọc nhầm.
- Server đóng luồng sự kiện trước khi run xong (server chết hoặc khởi động lại): `run` và
  `send --wait` báo `event stream closed before the run completed`, mã 1, `--json` có `error`.
- Dừng server (SIGTERM, `kill <pid>`) trong lúc nó đang giữ run cho client đã rời thì run bị
  cắt giữa lượt và session không ghi được dấu kết thúc.
- Chỉ nhận `-c dir`, `-cdir`, `--cwd dir`, `--cwd=dir` đứng riêng; cờ gộp kiểu `-dc dir`
  không được hiểu (launcher quay về thư mục hiện tại).
- Kiểm launcher: `bash scripts/test-agent-cli-wrapper.sh` (cài vào thư mục tạm với binary
  giả, in `PASS`).

## Nhiều dự án trong một cửa sổ terminal

```bash
agent-cli-tmux ~/code/api ~/code/web ~/code/infra
```

Mở (hoặc nối vào) tmux session `agents`, mỗi thư mục một window đặt theo basename, chạy
`CRUSH_CLIENT_SERVER=1 agent-cli -c <đường dẫn tuyệt đối>`. Chạy lại với thư mục đã có thì
chỉ chọn window đó, không đẻ thêm (nhận diện theo đường dẫn, không theo tên). Thư mục không
tồn tại: in lỗi ra stderr, thoát mã 2 và không tạo gì. Đang ở trong tmux thì `switch-client`,
ngoài tmux thì `attach`.

- `prefix+w` liệt kê các window (dự án) để chọn; `prefix+n`/`prefix+p` chuyển nhanh.
- Đổi tên session: `AGENT_CLI_TMUX_SESSION=tên`. Không attach: `AGENT_CLI_TMUX_NO_ATTACH=1`.
- Installer tạo symlink `~/.local/bin/agent-cli-tmux` trỏ về `scripts/agent-cli-tmux.sh` trong repo.
- Mỗi window là một client nối vào server của dự án đó (xem mục trên). Thoát agent-cli thì window đóng.
- Kiểm: `bash scripts/test-agent-cli-tmux.sh` (tmux server riêng `-L agentcli-test` và
  agent-cli giả, in `PASS`; không đụng tmux thật của bạn).

### Mỗi việc một worktree: `new` / `rm` / `ls`

Chạy song song nhiều việc trên cùng một repo mà không đạp file nhau: mỗi việc có worktree
riêng (pwd riêng, DB/log `.crush/` riêng, model riêng) và một window tmux riêng.

```bash
cd ~/code/api
agent-cli-tmux new fix-login --model main/<model-id>   # worktree + branch agent/fix-login + window
agent-cli-tmux ls                                   # fix-login  agent/fix-login  clean|dirty
agent-cli-tmux rm fix-login --force                 # dọn khi xong (xoá cả .crush/ của việc đó)
```

- `new <name> [--model <provider>/<model>] [--repo <dir>] [--base <ref>]`: `<repo>` là
  `git rev-parse --show-toplevel` của `--repo` hoặc thư mục hiện tại (không phải repo git:
  thoát mã 2). Worktree nằm ở `<cha của repo>/<tên repo>-agents/<name>`, trên branch
  `agent/<name>` tạo từ `--base` (mặc định `HEAD`). Worktree đã có thì chỉ mở window; branch
  đã có mà chưa có worktree thì gắn lại branch đó. `<name>` khớp `^[a-z0-9][a-z0-9-]{0,40}$`,
  sai: mã 2.
- `--model` ghi (gộp bằng `jq`, giữ phần cấu hình có sẵn) `models.large` vào
  `<worktree>/.crush/crush.json`; tách provider ở dấu `/` đầu tiên (`openrouter/anthropic/x`
  là provider `openrouter`, model `anthropic/x`). Thiếu `/`: mã 2, không tạo gì. `.crush/`
  được tạo kèm `.gitignore` chứa `*` nên không làm worktree dơ.
- `--model` trên worktree đã có vẫn ghi lại model; agent đang chạy trong window đó phải
  khởi động lại mới nhận.
- `rm <name> [--repo dir] [--force]`: worktree còn thay đổi chưa commit (kể cả file chưa theo dõi, bất kể
  `status.showUntrackedFiles`) hoặc không đọc được trạng thái git thì in lý do, mã 3, không
  xoá gì. Worktree còn session BUSY hoặc WAITING (theo `agent-cli ps --json --all`, so path đã chuẩn hoá `pwd -P`; ps lỗi, có stderr, JSON
  hỏng, hoặc hàng BUSY có path không phân giải được đều bị coi là không an toàn và từ chối) cũng bị từ
  chối, mã 3: dưới `CRUSH_CLIENT_SERVER=1` agent chạy ở server riêng nên đóng window KHÔNG
  dừng nó ngay (xem "Đã biết" và U1 ở mục trên về detach grace); thiếu `agent-cli` trên
  PATH thì bỏ qua kiểm này và cảnh báo. Sạch thì đóng window `<name>`, đợi tiến trình pane
  thoát, gỡ worktree (git kiểm lại, kể cả file agent vừa ghi) và in `branch agent/<name> kept`. Script KHÔNG bao giờ xoá branch; bỏ hẳn thì tự chạy
  `git branch -d agent/<name>` (hoặc `-D` nếu chưa merge).
- `rm` từ chối (mã 3) khi worktree có file bị gitignore, gồm `.crush/` (DB session và log của
  agent việc đó) và các file như `.env`, vì `git worktree remove` xoá chúng vĩnh viễn; stderr
  liệt kê từng mục. Chắc chắn bỏ thì thêm `--force` (chỉ nới kiểm này; kiểm thay đổi chưa
  commit và session BUSY/WAITING vẫn giữ). Worktree từng chạy agent-cli hầu như luôn có
  `.crush/`, nên thường phải `--force`; cần giữ lịch sử thì chép `.crush/` ra trước.
- `--base` được phân giải tại nơi chạy lệnh (chạy từ worktree con thì lấy HEAD của worktree
  đó). `<name>` là symlink trong `<tên repo>-agents/` thì `new`/`rm` từ chối (mã 2).
- `new`/`rm`/`ls` luôn quy về worktree chính của repo, nên chạy từ trong một worktree con
  vẫn đặt/tìm ở cùng `<tên repo chính>-agents/`. `ls` báo `dirty` khi không đọc được trạng thái, `broken` khi dữ liệu git của worktree mất (vẫn liệt kê tiếp các worktree khác).
- Thư mục thật tên `new`/`rm`/`ls` thì viết `./new` để không bị hiểu là lệnh con.

## Giao việc vào session đang mở: `agent-cli send`

Gửi prompt vào một session của dự án đã có server (ví dụ TUI đang mở ở window tmux khác);
prompt hiện trong TUI đó như tin nhắn bình thường. Dành cho script hoặc agent điều phối
nhiều session/dự án.

```bash
agent-cli send -c ~/code/api --session 3f2a "Thêm test cho handler login"      # trả ngay, in session_id
agent-cli send -c ~/code/web --new --title review --wait "Review commit cuối"  # đợi run xong, in text cuối
agent-cli send --session 3f2a --wait --timeout 10m --json "Chạy lại test"     # một object JSON
```

- Bắt buộc đúng một trong `--session <id>` (ID đầy đủ hoặc tiền tố hash như `run -s`) hoặc
  `--new [--title t]`. Không thấy session: mã 1. Thiếu prompt, thiếu/thừa cờ, hoặc không có
  `CRUSH_CLIENT_SERVER=1` (launcher đã đặt sẵn): mã 2 kèm hướng dẫn.
- Luôn đi qua server của dự án (`-c dir` chọn dự án). Server chưa chạy thì `send` tự dựng như
  `run`; khi đó không có TUI nào để duyệt quyền, nên nhắm vào dự án đã mở.
  `send` chỉ khởi tạo agent khi server chưa có; server đã sẵn sàng thì giữ nguyên (không thay
  coordinator đang chạy của TUI).
- Không `--wait`: về ngay khi server nhận prompt; stdout là `session_id` (hoặc `{"session_id":"..."}`).
- `--wait`: đợi đúng run của lần gửi này (khớp session và run ID, run khác trên cùng session bị
  bỏ qua), rồi in text cuối. `--json` in cùng dạng với `run --json`: `session_id`, `text`,
  `error`, `prompt_tokens`, `completion_tokens`, `cost` (token/cost là tổng của session).
  `--timeout` (mặc định 0 = không giới hạn) hoặc Ctrl-C ngắt việc đợi; `send` không gửi lệnh huỷ run
  (nhưng liên quan U1 ở trên: chưa kiểm run có sống tiếp khi client cuối thoát).
  Lỗi run: mã 1, JSON vẫn có `error`.
- Run cần duyệt quyền hoặc trả lời câu hỏi mà không có TUI nào đang xem session đó: `--wait`
  dừng với lỗi (mã 1, JSON có `error`) nêu tool và session. Run vẫn treo ở server tới khi có ai
  mở session đó trong TUI (server gửi lại prompt đang chờ khi đó). Prompt của sub-agent chưa
  được nhận diện (mang ID session con), `--wait` vẫn có thể đợi mãi; dùng `--timeout`.
- `send --session S --wait` xếp hàng sau lượt khác (vd. lượt TUI) trên S: câu hỏi/yêu cầu duyệt
  quyền của lượt đó không làm `send` thất bại; `send` in `... from another turn is pending on
  session <id>; this run keeps waiting behind it` ra stderr rồi chờ tiếp, giống `run -s`.
- `--title` chỉ đi với `--new`, `--timeout` chỉ đi với `--wait`: sai thì mã 2.
- Chỉ nhận prompt từ đối số, không đọc stdin.
- Chưa kiểm với server và model thật (test chỉ dùng client giả).

## Giao việc cho Claude Code / Codex: tool `claude_code`, `codex`

Model chính có thể giao một việc trọn vẹn cho Claude Code hoặc Codex đã đăng nhập sẵn
trên máy (dùng gói thuê bao của bạn, không cần API key). Cách hoạt động:

- Tool chỉ có mặt khi tìm thấy CLI (`claude`, `codex` trong `PATH`, hoặc đường dẫn ở
  `AGENT_CLI_CLAUDE_BIN` / `AGENT_CLI_CODEX_BIN`) và không bị `permissions deny`. Không có CLI
  thì model không thấy tool và request không tốn token mô tả nó. Biến `*_BIN` đặt mà trỏ sai
  (tool không bị deny) thì mọi prompt báo lỗi nêu đúng tên biến cho tới khi sửa và khởi động
  lại server.
- Mỗi lần giao đều hỏi quyền, hộp thoại hiện nguyên văn việc giao, thư mục và mức quyền.
  Ngoại lệ, đều do bạn chủ động bật: "Allow for Session" (mọi lần giao sau của CLI đó trong
  session), `agent-cli run` (tự duyệt mọi quyền của session nó chạy), `--yolo`,
  `permissions allow claude_code`, hook PreToolUse trả allow.
- `resume` chỉ nhận ID dạng chữ/số/`._:-` (đúng ID tool đã trả về); giá trị giống cờ bị từ chối
  trước khi hỏi quyền, để model không chèn được cờ vào lệnh gọi CLI con.
- Sau khi được duyệt, CLI con chạy không hỏi lại ai: mặc định Claude Code
  `--dangerously-skip-permissions`, Codex sandbox `danger-full-access`. Muốn chặt hơn:
  `AGENT_CLI_CLAUDE_PERMISSION_MODE=acceptEdits` (hoặc `plan`),
  `AGENT_CLI_CODEX_SANDBOX=workspace-write` (hoặc `read-only`).
- Kết quả trả về kèm session/thread ID; model truyền lại vào `resume` để nói tiếp đúng
  cuộc hội thoại đó.

Các biến trên phải có trong môi trường của tiến trình agent-cli (export ở shell, ví dụ
`~/.bashrc`), KHÔNG đặt trong `~/.config/agent-cli/env`: tệp đó chỉ được crushrc `source`
trong trình thông dịch riêng. Khác tệp env, chúng là env của TIẾN TRÌNH server, lấy từ client
đã dựng server đó: mở workspace mới không nạp lại. Đổi xong phải để server cũ tắt (khoảng 60
giây sau khi client cuối thoát, lâu hơn nếu còn run chạy nền; xem `agent-cli ps`) hoặc dừng
nó như mục "Sửa env khi agent-cli đang chạy", rồi mở lại từ shell đã có biến mới.
Codex đi kèm app ChatGPT desktop thường không nằm trong `PATH` (ví dụ
`/usr/lib/chatgpt/resources/codex`): đặt `AGENT_CLI_CODEX_BIN` hoặc tạo symlink vào `~/.local/bin`.
Không muốn dùng: `permissions deny claude_code codex` trong crushrc.

## Mode Shift+Tab và "accept edits"

Shift+Tab xoay vòng: code → plan → code + YOLO → code. Các bước này đổi plan/YOLO cho cả workspace nên bị chặn khi
có bất kỳ session nào đang chạy: Shift+Tab hiện cảnh báo và giữ nguyên mode.

**Accept edits hiện chỉ bật được qua API** (chế độ client/server), không có trong vòng Shift+Tab:
`POST /v1/workspaces/{id}/permissions/accept-edits?session_id=<id>`, body `{"accept_edits": true}` (`false` để tắt;
`GET` cùng đường dẫn để đọc). Khi nó đang bật, ô nhập TUI hiện `Accepting edits` và Shift+Tab tắt nó: lúc rảnh thì tắt rồi
sang plan, lúc có session đang chạy thì chỉ tắt (về code). Shift+Tab không bật lại.

- Chỉ tự duyệt `edit`, `multiedit`, `write` của **đúng session đó**; mọi tool khác (bash, fetch, MCP,
  `lsp_rename`...) vẫn hỏi.
- Không lan sang session khác, kể cả session con do sub-agent tạo (`<message>$$<tool call>`): sub-agent sửa file vẫn hỏi.
- Chỉ tự duyệt file nằm **trong thư mục dự án** (thư mục làm việc của workspace). Đường dẫn được giải symlink
  (`filepath.Abs` + `EvalSymlinks`); file chưa tồn tại thì xét theo thư mục cha gần nhất đang có. File ngoài dự án,
  symlink trỏ ra ngoài (kể cả symlink treo) hoặc đường dẫn không giải được đều vẫn hỏi.
- Đường dẫn tuyệt đối có thành phần `..` **luôn hỏi**, kể cả khi gộp `..` xong vẫn nằm trong dự án. Lý do: tool ghi
  đúng đường dẫn thô, kernel đi theo symlink trước rồi mới xử lý `..`, nên `<dự án>/link/../x` (với `link` trỏ ra
  ngoài) ghi ra cạnh đích của `link`, dù gộp chữ thì ra `<dự án>/x`. Đường dẫn tương đối thì tool tự gộp `..` với thư
  mục dự án trước (`filepath.Join`) và ghi đúng đường đã gộp, nên được xét theo đường đã gộp (`../x` vẫn hỏi).
- **Luôn hỏi**, kể cả trong dự án: file config của agent-cli (`crush.json`, `.crush.json`, `crushrc`, `.crushrc`, ở
  thư mục nào cũng vậy, kể cả `.crush/crush.json`), vì config khai báo hook và MCP server, tức là chạy được lệnh;
  mọi thứ dưới `.git` (hook, `core.hooksPath`, `core.fsmonitor` cũng chạy lệnh); và mọi file/thư mục tên `HEAD`
  (`x/HEAD` + `x/config` dựng thành một repo bare lồng, lệnh git chỉ-đọc được tự duyệt sẽ đọc config đó). So tên không
  phân biệt hoa thường (macOS không phân biệt), xét trên đường dẫn đã giải symlink.
- Trạng thái nằm trong bộ nhớ server, mất khi server tắt, bị xoá khi xoá session.

Lỗ đã biết (lý do chưa đưa vào Shift+Tab):

- Tên được bảo vệ chỉ xét trên đường dẫn đã giải symlink: nếu một tên được bảo vệ là symlink trỏ tới file thường
  trong dự án (`crushrc` → `x.txt`), ghi `x.txt` vẫn được tự duyệt dù nó đổi config.
- Hardlink: file thường là hardlink của `.git/config` hay của một hook mang tên khác, nên ghi qua nó được tự duyệt.
- Script hook nằm ngoài `.git` (ví dụ `.husky/`, hoặc thư mục `core.hooksPath` trỏ tới) không được bảo vệ: ghi chúng
  được tự duyệt và chúng chạy ở lần `git commit`/`git push` sau.

### Rủi ro còn lại của tự duyệt bash

Lệnh chỉ-đọc trong danh sách an toàn chạy không hỏi, kể cả khi không bật accept edits. Còn các rủi ro sau:

- Git chạy chương trình theo config của repo: `core.fsmonitor`, `remote.<tên>.uploadpack`, `alias.*`,
  `diff.external`... Cần một `.git/config` (hoặc repo bare lồng) đã bị cài sẵn: tool sửa file luôn hỏi khi ghi vào
  `.git` hay `HEAD`, nhưng file đã có sẵn trên đĩa thì không ai kiểm.
- `git ls-remote` và `git remote show <tên>` kết nối mạng tới remote.
- `printenv` và `echo $X` in biến môi trường, kể cả secret (API key), vào output gửi cho model.
- `date -s ...` và `hostname <tên>` đổi giờ/tên máy nếu agent-cli chạy bằng root.

## Tổ hợp phím

Xem mọi action và phím hiện tại: `agent-cli keys` (bảng `ACTION  KEYS  HELP`) hoặc `agent-cli keys --json`.
Đổi phím bằng file `keybindings.json` nằm cạnh thư mục `themes/` (mặc định `~/.config/agent-cli/keybindings.json`,
theo `CRUSH_GLOBAL_CONFIG` nếu có). Dạng `{"<action>": ["phím", ...]}`; mảng rỗng là tắt phím đó. Ví dụ cho dev:

```json
{
  "chat.new_session": ["ctrl+n", "alt+n"],
  "chat.jump_session": ["ctrl+1", "ctrl+2", "ctrl+3"],
  "shift_tab": ["ctrl+e"],
  "suspend": []
}
```

- Tên action là đường dẫn snake_case của trường trong `KeyMap`: `Chat.NewSession` -> `chat.new_session`,
  `ShiftTab` (đổi mode) -> `shift_tab`. Action mới được thêm vào `KeyMap` tự có tên, không cần khai báo.
- Action sai tên, hoặc hai action đang bật trong cùng nhóm (`chat.*`, `editor.*`) dùng chung một phím
  (chỉ xét khi có action bị bạn đổi): vẫn áp dụng, nhưng hiện toast cảnh báo lúc khởi động và ghi log.
  JSON hỏng: dùng phím mặc định kèm một cảnh báo.
- `chat.up_down` và `chat.up_down_one_item` chỉ để hiện gợi ý ở thanh help (`agent-cli keys` ghi "display only"); gán phím cho chúng không có tác dụng. Đổi `chat.down`/`chat.up` thay thế.
- Bảng lệnh (command palette) vẫn hiện gợi ý phím mặc định sau khi đổi, vì chuỗi này viết cứng trong `dialog/commands.go`; phím thật vẫn là phím bạn đã đổi.
- File chỉ đọc lúc khởi động; sửa xong phải mở lại TUI.
- Phím chính (thanh footer ở chat): `Alt+W` workspace (`workspaces`, thoát về Home để chọn workspace/session khác, xem mục Home) · `Alt+S` sessions · `Alt+N` session mới (`chat.new_session`) · `Alt+.` / `Alt+,` sang session kế / trước, vòng theo thứ tự panel Sessions (`chat.next_session`, `chat.prev_session`) · `Alt+P` provider (`providers`) · `Alt+M` model **của provider đang dùng** (`models`) · `Alt+E` mức effort của model (`effort`, chỉ khi model có các mức); `Ctrl+P` vẫn là Commands (lệnh "Switch Model" ở đó vẫn là danh sách đầy đủ). Phím cũ `Ctrl+S` / `Ctrl+N` / `Ctrl+L` vẫn dùng được.
- `Alt+Tab` không dùng được trong TUI: trình quản lý cửa sổ (Cinnamon, GNOME, KDE...) giữ nó để đổi cửa sổ, terminal không bao giờ nhận. Vì vậy chuyển session là `Alt+.` / `Alt+,`.
- Danh sách provider ở `Alt+P` lấy theo `option visible-provider` trong crushrc (đúng thứ tự), không khai thì là các provider đã cấu hình. Mỗi dòng ghi trạng thái: `current`, `ready`, `agent-cli login openai` (ChatGPT chưa đăng nhập), `needs API key`. Trong bảng model, `Alt+P` quay lại bảng provider.
- Hộp thoại xin quyền mặc định chọn **Deny**: Enter lỡ tay là từ chối; cho phép thì bấm `a` (hoặc chọn Allow bằng ←/→ rồi Enter); Allow for Session là `Shift+S`. Chữ thường `h`/`l`/`s` và Tab không đổi lựa chọn, nên chữ đang gõ dở rơi vào hộp thoại không thành cho phép (riêng `a` vẫn là Allow). Hộp thoại "khởi tạo dự án" mặc định **Nope**; muốn khởi tạo thì bấm `y` hoặc dùng `Ctrl+P` → Initialize Project.
- `chat.jump_session` **không có phím mặc định** (chọn session theo tên, không theo số), nên panel Sessions không
  đánh số dòng. Muốn dùng thì gán trong `keybindings.json` như ví dụ trên; mô tả dưới đây áp khi đã gán.
- `chat.jump_session` (ví dụ gán `alt+1`..`alt+9`) nhảy thẳng tới session đánh số 1..9 trong panel Sessions (đúng thứ tự panel đang hiện), không đụng chữ đang gõ ở editor; số vượt quá danh sách hoặc trúng session đang xem thì không làm gì, session ghim ngoài top 9 không có số. Phím thứ i trong mảng ứng với session thứ i: đổi phím thì liệt kê theo thứ tự (ví dụ trên chỉ nhảy được tới session 1-3, nên panel chỉ đánh số 1-3, các dòng khác hiện `·`).
- Phím `alt+...` cần terminal gửi Alt/Meta đúng (kiểm bằng `agent-cli keys` rồi thử trong TUI).

### Chữ đang gõ không mất khi đổi session

Đổi session (`Alt+S`, `Alt+N`, ...) giữ riêng chữ và file đính kèm chưa gửi của từng session; quay lại là có lại. Thoát TUI (kể cả `Alt+W` về Home) lưu **chữ** của từng session vào `<data dir>/drafts.json` (0600, chỉ bản nháp không rỗng) và nạp lại lần mở sau. **File đính kèm không được lưu xuống đĩa** (mất khi thoát TUI); thoát bằng kill/crash cũng không lưu.

## Cập nhật và kiểm chứng

```bash
git pull --ff-only
bash scripts/install-agent-cli.sh
agent-cli --version
git diff --check
```

Installer sao lưu binary và wrapper đang cài vào `~/.local/lib/agent-cli/backup-<thời điểm>/`
(giữ 3 bản mới nhất), build ra tệp tạm rồi mới thay (build lỗi thì bản cũ còn nguyên), và in
lệnh `cp -p` để quay lại.

Đã kiểm chứng: nạp `crushrc` + env với biến giả (provider `main`, model lớn/nhỏ, key rỗng
và key có giá trị, thiếu biến thì dừng kèm thông báo). Chưa kiểm chứng với endpoint mới (tool
calling và streaming cần thử thật trên model bạn chọn), server nhà, phiên dài/compact, API key
thật của provider, OAuth native (`agent-cli login openai`). Login subscription không đồng nghĩa API key.
