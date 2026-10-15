# Cài agent-cli trên máy cá nhân hoặc server nhà

Fork từ Crush; giữ nguyên LICENSE.md và thông báo upstream.

## Cài đặt

Cần sẵn:

- `git` và `go` (phiên bản trong go.mod; Go có thể tải toolchain tương ứng): để lấy mã và build.
- `sha1sum` hoặc `shasum`: launcher dùng để chọn server riêng cho từng dự án (mục "Một server cho
  mỗi dự án"). Thiếu cả hai thì launcher dừng với lỗi rõ, trừ khi chạy `CRUSH_CLIENT_SERVER=0`.
- `jq`: installer dùng để đọc `crush.json` cũ (thiếu thì coi như `crush.json` đang khai provider,
  xem "Chuyển từ `crush.json` cũ"); `agent-cli-tmux new --model` và `agent-cli-tmux rm` cũng cần.
- `tmux`: chỉ cho `agent-cli-tmux`.
- Chạy các script kiểm trong `scripts/`: thêm `python3` (server LLM giả của
  `test-agent-cli-run-json.sh` và `test-agent-cli-delegation.sh`) và lệnh `script` của util-linux
  (ca terminal của `test-agent-cli-wrapper.sh`; thiếu thì ca đó bị bỏ qua).

```bash
git clone https://github.com/luannt2002/agent-cli.git
cd agent-cli
bash scripts/install-agent-cli.sh
export PATH="$HOME/.local/bin:$PATH"
```

Script build tối đa hai luồng với `CGO_ENABLED=0 GOEXPERIMENT=greenteagc` (như bản phát hành chính thức; phiên bản
hiển thị lấy từ VCS stamping của `go build`), rồi chép hai tệp mẫu nếu chưa có (không bao giờ ghi đè; riêng trường hợp đã có `crush.json` khai provider, xem "Chuyển từ `crush.json` cũ"):
`~/.config/agent-cli/crushrc` (cấu hình chính) và `~/.config/agent-cli/env` (mode 600,
chứa endpoint, model, key). Sau khi cài, mở `~/.config/agent-cli/env` và điền giá trị thật
theo mục dưới. Tệp env nằm ngoài repo; không commit key hoặc OAuth token. Sao lưu cấu hình
thật riêng bằng kênh an toàn.

Installer cài hai lệnh: `agent-cli` (wrapper, trỏ tới binary `lib/agent-cli/crush`) và `agent-cli-tmux`
(**bản chép** của `scripts/agent-cli-tmux.sh`, không phải symlink vào thư mục checkout; muốn lấy bản mới
thì chạy lại installer). Cuối lần chạy, nếu `crushrc` hoặc `env` đang dùng khác mẫu trong repo (mẫu có thể
đã thêm biến hay chú thích mới), installer in sẵn lệnh `diff -u <tệp của bạn> <mẫu>` để bạn tự xem và gộp
tay; nó không bao giờ ghi đè tệp của bạn (với env, `diff` hiện cả key: đừng chạy khi chia sẻ màn hình).
Nếu `~/.config/agent-cli` là symlink thì installer chỉ siết quyền (về 700) thư mục đích khi đích là của chính
bạn, và nói rõ đã làm; symlink hỏng thì dừng.

Biến môi trường của installer, launcher và crushrc:

| Biến | Dùng ở | Ý nghĩa | Mặc định |
|---|---|---|---|
| `AGENT_CLI_INSTALL_DIR` | installer | gốc cài: `bin/agent-cli`, `bin/agent-cli-tmux`, `lib/agent-cli/crush` nằm dưới đó. Đường dẫn tương đối được đổi sang tuyệt đối lúc cài (wrapper ghi cứng đường dẫn binary) | `~/.local` |
| `AGENT_CLI_SKIP_BUILD` | installer | đúng `1` thì bỏ `go build` và dùng binary sẵn có ở `<gốc cài>/lib/agent-cli/crush` (test launcher đặt binary giả vào đó); giá trị khác `1` vẫn build | không đặt |
| `AGENT_CLI_ENV_FILE` | crushrc | tệp env mà crushrc `source` (bỏ qua nếu không có) | `~/.config/agent-cli/env` |
| `AGENT_CLI_RUNTIME_DIR` | launcher | thư mục chứa socket server của từng dự án; phải đã tồn tại, thuộc bạn, mode 700 (launcher chỉ siết quyền thư mục dự phòng `~/.cache/agent-cli`), và đủ ngắn để socket không quá 107 byte | `/run/user/$UID`, rồi `$XDG_RUNTIME_DIR`, rồi `~/.cache/agent-cli` |

## Cấu hình model

Provider và model được cấu hình bằng biến dùng chung `AGENT_CLI_*` trong
`~/.config/agent-cli/env`. `crushrc` (bản mẫu: `agent-cli.crushrc.example`) `source` tệp này
(đổi đường dẫn bằng `AGENT_CLI_ENV_FILE`; bỏ qua nếu tệp không có) rồi khai provider chính cùng
model lớn/nhỏ: hoặc một provider tên `main` tự khai bằng `AGENT_CLI_BASE_URL` (cách 1 và 2 bên
dưới), hoặc một provider dựng sẵn trong catalog chọn bằng `AGENT_CLI_PROVIDER_ID` (cách 3). Mẫu env:
`agent-cli.env.example`, chỉ chứa placeholder.

| Biến | Ý nghĩa | Mặc định |
|---|---|---|
| `AGENT_CLI_PROVIDER_ID` | ID provider dựng sẵn trong catalog (vd `zai`, `openai`, `anthropic`); đặt thì dùng cách 3 và bỏ qua `AGENT_CLI_BASE_URL`, `AGENT_CLI_PROVIDER_TYPE`, `AGENT_CLI_PROVIDER_NAME`, `AGENT_CLI_CAN_REASON`, `AGENT_CLI_CONTEXT_WINDOW` | không có |
| `AGENT_CLI_PROVIDER_TYPE` | loại provider: `openai-compat`, `openai`, `anthropic`, `ollama`... | `openai-compat` |
| `AGENT_CLI_PROVIDER_NAME` | tên hiển thị của provider `main` | `main` |
| `AGENT_CLI_BASE_URL` | URL gốc của API (bắt buộc ở cách 1 và 2); dạng URL theo loại: `openai-compat`, `openai`, `ollama` tới `/v1`; `anthropic` là URL gốc KHÔNG có `/v1` | không có |
| `AGENT_CLI_API_KEY` | API key; để rỗng cho model local, và cho `openai`/`xai`/`copilot`/`hyper` đã đăng nhập (cách 3) | rỗng |
| `AGENT_CLI_MODEL` | model chính (bắt buộc) | không có |
| `AGENT_CLI_SMALL_MODEL` | model nhỏ (đặt tiêu đề...) | `AGENT_CLI_MODEL` |
| `AGENT_CLI_CAN_REASON` | model của provider `main` có suy luận không: `true`/`false` (nhận cả `1`/`0`, `yes`/`no`, không phân biệt hoa thường); giá trị khác làm khởi động dừng và nêu đúng tên biến này | `false` |
| `AGENT_CLI_CONTEXT_WINDOW` | context window khai cho model (áp cho cả model chính và nhỏ) | 128000 |
| `AGENT_CLI_MAX_TOKENS` | số token đầu ra tối đa (áp cho cả model chính và nhỏ) | 8192 |

Tệp env là Bash nên cách đặt nháy quan trọng. Giá trị có ký tự đặc biệt (`<`, khoảng trắng) phải nằm
trong nháy. **Key dạng chữ thường nên đặt trong nháy ĐƠN** (`AGENT_CLI_API_KEY='sk-...'`): nháy kép để
shell mở rộng `$`, dấu backtick và `\`, nên key có các ký tự đó bị đổi âm thầm (vd `"ab$cd"` ra `ab`).
Nháy kép dành cho giá trị cần mở rộng, như `"$(pass show <entry> | head -n1)"` (xem cách 2).

Thiếu biến bắt buộc thì nạp cấu hình thất bại, và thông báo lỗi nêu đúng biến thiếu ở cả hai
chế độ: chế độ client/server (mặc định) nối phần cuối `stderr.log` của server vào câu
`failed to initialize agent-cli server: ...`, ví dụ `... exit status 1: AGENT_CLI_BASE_URL: đặt
AGENT_CLI_BASE_URL trong ...`.

Dạng `AGENT_CLI_BASE_URL` theo loại provider:

- `openai-compat`, `openai`, `ollama`: phần tới `/v1`, KHÔNG gồm `/chat/completions` (client
  tự thêm; thêm đuôi này vào sẽ ra URL sai).
- `anthropic`: URL gốc, KHÔNG có `/v1` (client tự thêm `/v1/messages`).

Crushrc mẫu tắt metrics và tự cập nhật catalog (dùng catalog nhúng sẵn), KHÔNG tắt provider dựng sẵn
(cần cho `agent-cli login` và `Alt+P`; muốn tắt thì bật dòng `option default-providers false` trong mẫu),
và ẩn các tool chạm mạng
`sourcegraph`, `fetch`, `agentic_fetch`, `download` (tên phải đúng tên tool; tên sai bị bỏ qua
không báo lỗi). Mẫu KHÔNG `permissions allow` tool đọc nào: trong thư mục dự án `view`, `ls`,
`glob`, `grep` chạy không hỏi; cả bốn tool hỏi quyền khi chạm ngoài dự án (ví dụ tệp env chứa
key), kể cả qua symlink trong dự án trỏ ra ngoài (đường dẫn được giải symlink trước khi so), và
allow sẽ bỏ luôn lời hỏi đó. Bản crushrc cài từ mẫu cũ còn dòng
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
AGENT_CLI_API_KEY='<your-api-key>'
AGENT_CLI_MODEL="<model-id>"
```

Key không vào repo và không dán vào chat. Nên lấy key từ kho bí mật, vì tệp env được shell
nạp nên `$(lệnh)` chạy được:

```bash
AGENT_CLI_API_KEY="$(pass show <entry> | head -n1)"
```

`head -n1` vì `pass show` in cả các dòng ghi chú sau dòng mật khẩu; thiếu nó thì key gồm nhiều dòng.

Tránh để key dạng văn bản thường trong tệp env: tool `view` (cấu hình mẫu cho chạy tự động,
không hỏi) có thể đọc tệp ngoài thư mục dự án, kể cả `~/.config/agent-cli/env`, nên model có thể đọc key đó. Mode
600 chỉ chặn người dùng khác, không chặn agent chạy dưới tài khoản của bạn.

Kiểm cấu hình đã nạp mà không gọi model: `agent-cli models` liệt kê `main/<model-id>`.

### Cách 3: provider dựng sẵn trong catalog

```bash
AGENT_CLI_PROVIDER_ID=zai
AGENT_CLI_MODEL="glm-5.3"
AGENT_CLI_SMALL_MODEL="glm-5.3-flash"
AGENT_CLI_API_KEY='<khoá z.ai>'
```

Catalog lo endpoint, danh sách model, mức effort (`Alt+E`) và khả năng suy luận, nên không cần
`AGENT_CLI_BASE_URL` hay `AGENT_CLI_CAN_REASON`. Mỗi provider có thông tin xác thực riêng:

- Phần lớn provider (`zai`, `anthropic`, `openrouter`...) bắt buộc `AGENT_CLI_API_KEY`; thiếu thì khởi
  động dừng và nêu đúng biến.
- `openai` (tài khoản ChatGPT), `xai` (Grok), `copilot`, `hyper` còn đăng nhập được bằng tài khoản
  (`agent-cli login openai`, `login grok`, `login copilot`, `login` không đối số cho hyper). Đã đăng nhập thì để
  TRỐNG `AGENT_CLI_API_KEY`: crushrc không phát key rỗng đè lên token đã lưu (nằm ở
  `~/.local/share/agent-cli/crush.json`, được nạp SAU crushrc). Ngược lại, đã đăng nhập thì token đăng nhập
  thắng `AGENT_CLI_API_KEY` (đã có test cho thứ tự nạp của `xai`); muốn dùng key thì `agent-cli logout <nền tảng>`
  trước. Chưa đăng nhập và chưa có key thì provider chỉ là chưa được cấu hình, agent-cli vẫn khởi động được
  (cần vậy để chạy được lệnh login). Cấu hình này đã kiểm bằng test nạp cấu hình với token giả; việc đăng nhập
  thật (trình duyệt) chưa được thử trong đợt sửa này.
- `AGENT_CLI_PROVIDER_ID` gõ sai (không có trong catalog, và không có base URL để thành provider tự khai)
  làm khởi động DỪNG, lỗi nêu tên provider và model đang chọn nó (`provider "foo" is selected as the large
  model but is not a built-in provider and has no base_url`). Trước đây provider bị bỏ qua lặng lẽ, agent-cli
  chọn model của provider khác và ghi lựa chọn đó xuống đĩa. Provider không ai chọn mà thiếu base URL vẫn chỉ bị
  cảnh báo trong log như cũ.

### Attribution trong commit và PR

Mặc định upstream bảo model thêm `Assisted-by: Crush:<model>` vào commit và dòng `Generated with Crush`
vào commit/PR do agent tạo (qua mô tả của tool bash). Crushrc mẫu tắt cả hai bằng `option attribution-trailer-style none` và
`option attribution-generated-with false`. Bản crushrc cài từ mẫu cũ chưa có hai dòng này: thêm vào, rồi
thoát hết client của dự án và mở lại. Muốn bật lại: `option attribution-trailer-style assisted-by` (hoặc `co-authored-by`) và
`option attribution-generated-with true`; dòng chú thích trong mẫu ghi sẵn cách này.

### Sửa env khi agent-cli đang chạy

Mỗi dự án có một server nền; cấu hình (crushrc, env) được nạp lại mỗi khi một workspace mới
mở. Sửa `~/.config/agent-cli/env` (hay crushrc) KHÔNG tác dụng với workspace đang mở: còn một
client (TUI hay `run`) nào của dự án đó đang sống thì nó vẫn dùng giá trị cũ (đã đo: `touch
crushrc` cũng không kích hoạt nạp lại). Để áp giá trị mới: thoát hết client của dự án đó, rồi
mở lại; hoặc dừng thẳng server của dự án: `pgrep -af 'crush server --host'` liệt kê các server
(tên socket `agent-cli-<8 hex>` xem ở mục "Một server cho mỗi dự án" bên dưới), rồi `kill <pid>`;
việc đang chạy ở dự án đó sẽ mất.

Chỉ sửa tệp trên đĩa thì không nạp lại, nhưng MỌI lần ghi cấu hình của chính workspace đó (vd chọn model
trong TUI) làm nó nạp lại toàn bộ, gồm chạy lại crushrc và `source` lại env: đã đo ở tầng cấu hình (sửa
crushrc trên đĩa rồi ghi một trường cấu hình khác thì giá trị mới có hiệu lực). Đừng dựa vào đó để áp cấu hình; nó chỉ giải thích vì sao
đôi khi giá trị mới "tự" xuất hiện sau khi đổi model. Các tool `claude_code`/`codex` thì khác: biến của chúng
là env của tiến trình server (xem mục giao việc ở dưới).

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
agent-cli run "Đọc repo và tóm tắt kiến trúc, chưa sửa code."   # CẢNH BÁO: run TỰ DUYỆT mọi quyền (xem dưới)
agent-cli --continue
agent-cli login openai
```

Trong TUI và `send`: sửa file, chạy shell (trừ lệnh chỉ đọc an toàn), đọc ngoài thư mục
dự án và giao việc cho Claude Code/Codex đều hỏi quyền; không dùng `--yolo` mặc định.
**`agent-cli run` thì tự duyệt MỌI lời xin quyền của session nó chạy** (session mới, hoặc
session có sẵn qua `-s`/`-C`; gồm bash, edit, claude_code, codex...): ràng buộc duy nhất là lời
trong prompt (dòng "chưa sửa code" ở ví dụ trên chỉ là lời dặn model, không phải rào chắn). Cần được hỏi
thì dùng TUI hoặc `send` (`send` KHÔNG tự duyệt). `run` không có tool `question` (không ai trả lời
được nên model làm tiếp thay vì dừng hỏi) và chờ MCP khởi động xong trước khi chạy. Nó dùng chế độ
agent hiện hành của dự án: TUI đang bật plan mode thì `run` cũng chỉ đọc và lập kế hoạch. Mọi lệnh
gọi sai cú pháp (cờ lạ, thiếu/thừa tham số, lệnh con sai tên) trả mã 2; chạy lỗi trả mã 1.
Cấu hình project có thể ghi đè cấu hình global.
Metrics và tự cập nhật catalog provider bị tắt (launcher đặt `CRUSH_DISABLE_METRICS`,
`CRUSH_DISABLE_PROVIDER_AUTO_UPDATE`; crushrc mẫu tắt thêm hai option tương ứng). Kiểm tra bản mới của
upstream (hỏi GitHub) **mặc định TẮT ngay trong binary**, kể cả khi chạy binary không qua launcher: fork không có
feed release riêng để so sánh. Chỉ bật khi đặt tường minh `CRUSH_DISABLE_UPDATE_CHECK=false` (hoặc `0`; giá trị
hiểu theo `strconv.ParseBool`); không đặt, rỗng, `1`/`true` hay giá trị không hiểu được đều là không gọi GitHub.
Launcher luôn đặt `CRUSH_DISABLE_UPDATE_CHECK=1` nên qua launcher kiểm tra này không bao giờ chạy; chỉ chạy thẳng
binary (`~/.local/lib/agent-cli/crush`) mới bật được.
Các cờ tắt này không chặn mọi request mạng: request model tới provider đang chọn, MCP và các tool mạng mà bạn
không `permissions deny` (crushrc mẫu chặn `sourcegraph`, `fetch`, `agentic_fetch`, `download`) vẫn đi.

Model chọn trong TUI (kể cả bật/tắt thinking, mức reasoning) giờ thuộc về dự án:
được ghi vào `<project>/.crush/crush.json`, nên đổi model ở worktree này không
đổi model ở worktree khác. Mở lại một session cũ cũng ghi model của session đó
vào file dự án (chạy nền, không làm TUI đứng). Model, effort đổi được bất cứ lúc nào, kể cả khi
có run ở session này hay session khác: mỗi lượt chạy giữ model và bộ tool lúc nó bắt đầu, prompt
xếp hàng giữ model lúc gửi, nên thiết lập mới chỉ áp cho tin nhắn gửi sau, không đổi provider
giữa chừng lượt đang chạy (agent con của lượt cũng vậy). Thứ tự gộp config, từ thấp đến cao: `~/.config/agent-cli/crush.json`
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
lệnh huỷ nào; với client/server (mặc định của launcher) server giữ run đang chạy khi client cuối rời
(xem "Đã biết" bên dưới về các trần thời gian). **In-process
(`CRUSH_CLIENT_SERVER=0`): `Alt+W` đóng app nên huỷ run đang chạy**; khi đang có run, TUI hỏi
xác nhận ("Stop the running task and go to Home?") thay vì huỷ ngay. `Ctrl+P` → Home làm đúng như
`Alt+W` (dùng khi terminal không gửi phím Alt).
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
Server tự tắt sau một khoảng ngắn không còn client (theo plan khoảng 60 giây; chưa đo). Socket nằm ở
`AGENT_CLI_RUNTIME_DIR` nếu bạn đặt, mặc định `/run/user/$UID`; thư mục đó chưa có thì tới `$XDG_RUNTIME_DIR`, rồi
cuối cùng tới `$HOME/.cache/agent-cli` (launcher tạo và siết mode 700). Cố ý KHÔNG bao giờ dùng `/tmp` vì ai
cũng ghi được và tên socket đoán được. `AGENT_CLI_RUNTIME_DIR` bạn tự chọn phải đã tồn tại, thuộc bạn và mode 700
(launcher không siết quyền hộ).

- Log server: `~/.cache/agent-cli/server-<host>/crush.log` (`<host>` là địa chỉ socket
  đã thay ký tự lạ bằng `_`). Mỗi dự án một thư mục, không trộn log.
- Server bị dừng (`kill <pid>`, SIGTERM, SIGHUP) thì dừng từng workspace trước: run đang
  chạy bị huỷ và được ghi lại, không bỏ dở giữa chừng. Server chết đột ngột (SIGKILL, mất
  điện) thì tool call đang chạy dở được ghi "Interrupted" khi session được mở lại (TUI,
  `session show`, `run -s`), không hiện "đang chạy" mãi; session con của sub-agent chỉ được
  ghi khi session cha không còn run. Kết quả ghi cho model dặn kiểm tra trạng thái thật
  trước khi chạy lại, vì lệnh có thể đã chạy một phần (bản vá upstream `fix/dangling-tool-calls`
  và `fix/shutdown-records-work`, port 15/10).
- Tắt: `CRUSH_CLIENT_SERVER=0 agent-cli` chạy in-process như trước và launcher không
  thêm `--host`. Giới hạn của chế độ này: danh sách session vẫn đủ (đọc từ DB) nhưng
  trạng thái bận/chờ duyệt chỉ đúng với session do chính process đó chạy. Launcher hiểu
  giá trị đúng như `strconv.ParseBool` của Go: bật là `1 t T TRUE true True`, tắt là
  `0 f F FALSE false False`. Giá trị khác (`yes`, `on`, `2`...) Go coi là tắt, nên launcher
  cũng chạy in-process và in cảnh báo thay vì thêm `--host`.
- Không chọn được server riêng thì launcher DỪNG (mã 1, lỗi rõ) thay vì chạy tiếp không `--host`: client sẽ rơi về
  server mặc định dùng chung và mất cách ly theo dự án (session, trạng thái bận, log lẫn nhau). Hai trường hợp: socket
  vượt 107 byte (đặt `AGENT_CLI_RUNTIME_DIR` ngắn hơn) và thiếu cả `sha1sum` lẫn `shasum`. Lối thoát: cài công
  cụ còn thiếu, hoặc `CRUSH_CLIENT_SERVER=0`. Bước chọn server (và hai lỗi trên) được bỏ qua khi bạn tự truyền
  `--host`/`-H`, với `ps` và với Home.
- Tự chọn server: `agent-cli --host unix:///đường/dẫn.sock ...` được giữ nguyên.
- Home (`agent-cli` không tham số trên terminal) không có `--host`: chưa chọn workspace nên chưa có
  server; Home gọi lại launcher với `-c <dir>` cho từng workspace.
- Đã biết: client mất stream mà không thoát sạch thì server giữ workspace (và run) thêm
  một khoảng detach grace, mặc định 10 giây (`backend.DefaultDetachGrace`, đổi bằng
  `CRUSH_SERVER_DETACH_GRACE`); thoát sạch thì bỏ qua grace.
- Hết grace mà workspace còn run thì server không dọn: kiểm lại mỗi 5 giây, dọn khi run
  xong (kiểm bằng `scripts/test-agent-cli-run-json.sh`: client bị SIGKILL giữa run, run vẫn
  xong). Hai trần thời gian:
  - `CRUSH_SERVER_ORPHAN_RUN_TIMEOUT` (giây, mặc định 1800; `0` = dọn ngay như cũ) chỉ tính
    khi run **kẹt**: đang chờ duyệt quyền hoặc trả lời câu hỏi mà không còn client nào. Run
    chạy lại được thì đồng hồ tính lại từ đầu.
  - `CRUSH_SERVER_ORPHAN_MAX_RUN` (giây, mặc định 21600 = 6 giờ; `0` = không trần) cho run
    mồ côi vẫn đang tiến triển: việc giao dài bằng `send` không bị huỷ ở phút 30, nhưng một
    run lặp vô tận không giữ server mãi.
- Lượt của `agent-cli run` gắn với chính tiến trình `run`: Ctrl+C, SIGTERM hay đóng terminal
  (SIGHUP) huỷ **đúng lượt đó** theo RunID, kể cả với `run -s/-C` (lượt khác và prompt xếp
  hàng của session giữ nguyên; Ctrl+C khi prompt chưa kịp tới server thì nó bị huỷ lúc tới).
  `run` bị kill hẳn (SIGKILL, OOM) thì server huỷ lượt của nó sau detach grace, thay vì để lượt
  chạy tiếp không ai giám sát. Server bản cũ chưa có huỷ theo RunID: `run -s/-C` chỉ thôi chờ
  và báo `stopped waiting; the run keeps going on session <id> (this server cannot cancel a
  single run)`.
- `send --wait` thì Ctrl+C/SIGHUP chỉ thôi chờ và báo `stopped waiting; the run keeps going on
  session <id>`: `send` giao việc rồi đi, lượt của nó không gắn với client.
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
- Installer chép `scripts/agent-cli-tmux.sh` thành `~/.local/bin/agent-cli-tmux` (bản chép mode 755, không
  symlink: đổi nhánh hay xoá checkout không làm hỏng nó; chạy lại installer để lấy bản mới). Bản cài cũ là
  symlink thì được thay bằng bản chép, không ghi xuyên vào tệp trong repo.
- Mỗi window là một client nối vào server của dự án đó (xem mục trên). Thoát agent-cli thì window đóng.
- Kiểm: `bash scripts/test-agent-cli-tmux.sh` (tmux server riêng qua `-S <socket trong thư mục tạm>` và
  agent-cli giả, in `PASS`; không đụng tmux thật của bạn).

### Mỗi việc một worktree: `new` / `rm` / `ls`

Chạy song song nhiều việc trên cùng một repo mà không đạp file nhau: mỗi việc có worktree
riêng (pwd riêng, DB/log `.crush/` riêng, model riêng) và một window tmux riêng.

```bash
cd ~/code/api
agent-cli-tmux new fix-login --model main/<model-id>   # worktree + branch agent/fix-login + window
agent-cli-tmux ls                                   # fix-login  agent/fix-login  clean|dirty|broken
agent-cli-tmux rm fix-login                         # dọn khi xong; từ chối nếu còn thay đổi, session đang chạy hoặc file gitignore
```

`rm` không kèm `--force` từ chối khi worktree có file bị gitignore, mà worktree từng chạy agent-cli hầu như
luôn có `.crush/` (DB session và log). Chỉ khi chắc chắn bỏ cả lịch sử đó mới thêm `--force`
(`agent-cli-tmux rm fix-login --force` xoá `.crush/` VĨNH VIỄN); cần giữ thì chép `.crush/` ra trước.

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
  dừng nó ngay (xem "Đã biết" ở mục trên về detach grace và trần mồ côi); thiếu `agent-cli` trên
  PATH thì bỏ qua kiểm này và cảnh báo. Sạch thì đóng window `<name>`, đợi tiến trình pane
  thoát, gỡ worktree (git kiểm lại, kể cả file agent vừa ghi) và in `branch agent/<name> kept`. Script KHÔNG bao giờ xoá branch; bỏ hẳn thì tự chạy
  `git branch -d agent/<name>` (hoặc `-D` nếu chưa merge).
- `rm` từ chối (mã 3) khi worktree có file bị gitignore, gồm `.crush/` (DB session và log của
  agent việc đó) và các file như `.env`, vì `git worktree remove` xoá chúng vĩnh viễn; stderr
  liệt kê từng mục. Chắc chắn bỏ thì thêm `--force` (chỉ nới kiểm này; kiểm thay đổi chưa
  commit và session BUSY/WAITING vẫn giữ). Vì vậy lần dọn đầu của một worktree đã chạy agent-cli thường bị
  từ chối cho tới khi bạn quyết định bỏ hay giữ `.crush/` (xem ví dụ ở trên).
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
  `--timeout` (mặc định 0 = không giới hạn) hoặc Ctrl-C ngắt việc đợi; `send` không gửi lệnh huỷ run, và run
  sống tiếp khi client cuối thoát (đã kiểm: `scripts/test-agent-cli-run-json.sh`, client bị SIGKILL giữa run mà run
  vẫn xong; trần thời gian xem "Đã biết" ở mục Home: 30 phút khi kẹt chờ duyệt/trả lời, 6 giờ khi vẫn chạy).
  Lỗi run: mã 1, JSON vẫn có `error`.
- Run cần duyệt quyền hoặc trả lời câu hỏi mà không có TUI nào đang xem session đó: `--wait`
  dừng với lỗi (mã 1, JSON có `error`) nêu tool và session. Run vẫn treo ở server tới khi có ai
  mở session đó trong TUI (server gửi lại prompt đang chờ khi đó). Prompt của sub-agent (mang
  ID session con) được quy về session gốc nên cũng làm `--wait` dừng như trên.
- `send --session S --wait` xếp hàng sau lượt khác (vd. lượt TUI) trên S: câu hỏi/yêu cầu duyệt
  quyền của lượt đó không làm `send` thất bại; `send` in `... from another turn is pending on
  session <id>; this run keeps waiting behind it` ra stderr rồi chờ tiếp, giống `run -s`.
- `--title` chỉ đi với `--new`, `--timeout` chỉ đi với `--wait`: sai thì mã 2.
- Chỉ nhận prompt từ đối số, không đọc stdin.
- Chưa kiểm với server và model thật (test chỉ dùng client giả).

## Xem và dừng job bash: `agent-cli jobs`

Mỗi lệnh bash do agent chạy là một job, kèm session sở hữu, giờ bắt đầu, thời gian chạy và kết
quả. Job sống trong server của dự án, nên đóng TUI không làm mất danh sách; server dừng thì
danh sách mất theo (job chết cùng server, không có job "running" giả).

```bash
agent-cli jobs                # đang chạy trước, rồi job vừa xong; mới nhất trước
agent-cli jobs --json         # một mảng JSON: id, session_id, tool_call_id, command, status, exit_code, started_at, finished_at, background, output_bytes
agent-cli jobs output 01A     # output của job (stdout rồi stderr)
agent-cli jobs kill 01A       # dừng job đang chạy; job đã xong thì không làm gì
```

- `STATUS`: `RUNNING`, `SUCCEEDED`, `FAILED (mã thoát)`, `KILLED`. Cột `BG` là job chạy tiếp
  sau khi tool bash đã trả về (agent yêu cầu chạy nền, hoặc lệnh quá lâu nên tự chuyển nền).
- Chỉ liệt kê job trong thư mục của dự án này (một server có thể giữ nhiều workspace).
  Server nhớ 100 job đã xong gần nhất, giữ output của 20 job cuối và của mọi job nền còn trong
  bảng (job đã xong nằm trong bảng tối đa 8 giờ, tối đa 250 job).
- Output dài (dev server, `tail -f`) chỉ giữ 256 KiB đầu và 1 MiB cuối mỗi luồng, phần giữa
  thay bằng dòng `[... N bytes of output dropped ...]`, nên bộ nhớ server có trần.
- Trần 50 job đang chạy cùng lúc cho cả server; job đã xong không tính (bản vá upstream
  `0ac04e4f`: trước đây 50 job nền đã xong là mọi lệnh bash hỏng).
- Dừng bằng `jobs kill` vẫn để job trong bảng: agent gọi `job_output` sau đó thấy output và
  trạng thái đã dừng, không bị "not found".
- Cần server của dự án (launcher đã đặt `CRUSH_CLIENT_SERVER=1`); chế độ in-process mỗi lệnh là
  một tiến trình riêng nên không thấy job của TUI. Server cũ chưa có API job: lỗi nói rõ phải
  cài lại và khởi động lại server.

## Giao việc cho Claude Code / Codex: tool `claude_code`, `codex`

Model chính có thể giao một việc trọn vẹn cho Claude Code hoặc Codex đã đăng nhập sẵn
trên máy (dùng gói thuê bao của bạn, không cần API key). Cách hoạt động:

- Tool chỉ có mặt khi tìm thấy CLI (`claude`, `codex` trong `PATH`, hoặc đường dẫn ở
  `AGENT_CLI_CLAUDE_BIN` / `AGENT_CLI_CODEX_BIN`) và không bị `permissions deny`. Không có CLI
  thì model không thấy tool và request không tốn token mô tả nó. Biến `*_BIN` đặt mà trỏ sai,
  hay `AGENT_CLI_CLAUDE_PERMISSION_MODE` / `AGENT_CLI_CODEX_SANDBOX` gõ sai giá trị (tool không bị
  deny), thì mọi prompt báo lỗi nêu đúng tên biến và các giá trị hợp lệ cho tới khi sửa và khởi
  động lại server.
- Mỗi lần giao đều hỏi quyền, hộp thoại hiện nguyên văn việc giao, thư mục và mức quyền (bằng lời,
  ví dụ "FULL ACCESS: runs any command…", kèm cờ thật). Ký tự điều khiển và ký tự đảo chiều trong
  prompt được thay bằng `�` để hộp thoại không hiện khác việc sẽ chạy.
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
- CLI con chạy trong nhóm tiến trình riêng: huỷ hoặc hết 15 phút thì dừng cả nhóm (kể cả dev
  server, watcher nó bật), không treo theo tiến trình cháu giữ stdout. Nó không nhận các biến
  gắn với một phiên Claude Code khác (`CLAUDECODE`, `CLAUDE_CODE_ENTRYPOINT`,
  `CLAUDE_CODE_SESSION_ID`, socket nhắn tin…), nên mở agent-cli từ trong Claude Code thì CLI con
  vẫn chạy như ở terminal thường; biến cấu hình của bạn (khoá, provider) giữ nguyên.

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
- Phím chính (thanh footer ở chat): `Alt+W` workspace (`workspaces`, thoát về Home để chọn workspace/session khác, xem mục Home) · `Alt+S` sessions · `Alt+N` session mới (`chat.new_session`) · `Alt+.` / `Alt+,` sang session kế / trước, vòng trong đúng các session panel Sessions đang hiện (bấm nhanh hai lần là tiến hai bước; không còn session khác thì báo "No other session") (`chat.next_session`, `chat.prev_session`) · `Alt+P` provider (`providers`) · `Alt+M` model **của provider đang dùng** (`models`) · `Alt+E` mức effort của model (`effort`, chỉ khi model có các mức); `Ctrl+P` vẫn là Commands (lệnh "Switch Model" ở đó vẫn là danh sách đầy đủ; có thêm "Home" và "Switch Provider", gợi ý phím ghi đúng `alt+n`/`alt+s`/`alt+m`/`alt+p`/`alt+e`/`alt+w`). Phím cũ `Ctrl+S` / `Ctrl+N` / `Ctrl+L` vẫn dùng được.
- `Alt+J` mở **Jobs** (`jobs`; trong `Ctrl+P` là mục "Jobs", gõ `activity` cũng ra): các lệnh bash agent chạy trong dự án, đang chạy trước, kèm thời gian chạy, kết quả (`exit N`, `killed`, `bg` cho job chạy nền) và session sở hữu; tự làm tươi mỗi giây. Enter xem output (bám đuôi như `tail -f`, cuộn lên thì dừng bám, `End` bám lại), Esc quay lại. `Ctrl+K` dừng job: luôn hỏi lại, mặc định **No**, chỉ ←/→ đổi nút rồi Enter. Cùng dữ liệu với lệnh `agent-cli jobs`. Không nằm ở footer (footer giữ bốn phím core); có trong help đầy đủ (`Ctrl+G`).
- `Alt+Tab` không dùng được trong TUI: trình quản lý cửa sổ (Cinnamon, GNOME, KDE...) giữ nó để đổi cửa sổ, terminal không bao giờ nhận. Vì vậy chuyển session là `Alt+.` / `Alt+,`.
- Danh sách provider ở `Alt+P` lấy theo `option visible-provider` trong crushrc (đúng thứ tự), không khai thì là các provider đã cấu hình. Mỗi dòng ghi trạng thái: `current`, `ready`, `agent-cli login openai` (ChatGPT chưa đăng nhập), `needs API key`. Trong bảng model, `Alt+P` quay lại bảng provider.
- Hộp thoại xin quyền mặc định chọn **Deny**: Enter lỡ tay là từ chối. Cho phép một lần là `Ctrl+A`, cho phép cả session là `Ctrl+S` (hoặc chọn nút bằng ←/→ rồi Enter). **Không chữ cái nào cấp quyền**: `a`, `s`, `h`, `l`, Tab... gõ dở rơi vào hộp thoại không làm gì (chỉ `d`/Esc là từ chối), vì hộp thoại chỉ nuốt phím trong tối đa 1,5 giây đầu. Hộp thoại "khởi tạo dự án" mặc định **Nope**; không còn phím `y` tắt: muốn khởi tạo thì chọn Yep bằng ←/→ (hoặc Tab) rồi Enter, hoặc `Ctrl+P` → Initialize Project.
- `chat.jump_session` **không có phím mặc định** (chọn session theo tên, không theo số), nên panel Sessions không
  đánh số dòng. Muốn dùng thì gán trong `keybindings.json` như ví dụ trên; mô tả dưới đây áp khi đã gán.
- `chat.jump_session` (ví dụ gán `alt+1`..`alt+9`) nhảy thẳng tới session đánh số 1..9 trong panel Sessions (đúng thứ tự panel đang hiện), không đụng chữ đang gõ ở editor; số vượt quá danh sách hoặc trúng session đang xem thì không làm gì, session ghim ngoài top 9 không có số. Phím thứ i trong mảng ứng với session thứ i: đổi phím thì liệt kê theo thứ tự (ví dụ trên chỉ nhảy được tới session 1-3, nên panel chỉ đánh số 1-3, các dòng khác hiện `·`).
- Phím `alt+...` cần terminal gửi Alt/Meta đúng (kiểm bằng `agent-cli keys` rồi thử trong TUI).

### Chữ đang gõ không mất khi đổi session

Đổi session (`Alt+S`, `Alt+N`, ...) giữ riêng chữ và file đính kèm chưa gửi của từng session; quay lại là có lại. **Chữ** của từng session được lưu vào `<data dir>/drafts.json` (0600, chỉ bản nháp không rỗng) khoảng 2 giây sau lần gõ cuối, khi thoát TUI (kể cả `Alt+W` về Home) và khi đóng terminal/SSH rớt (SIGHUP), rồi nạp lại lần mở sau; bản nháp của session đã xoá được dọn. Đang duyệt lịch sử prompt (↑) mà đổi session thì bản nháp thật được giữ, không bị thay bằng prompt cũ. **File đính kèm không được lưu xuống đĩa** (mất khi thoát TUI); `kill -9`/mất điện thì mất phần gõ trong ~2 giây cuối.

## Theme (màu giao diện)

Theme mặc định là **`everforest-dark-v2`** (chốt 15/10): nền tối dịu cho phiên làm việc dài, chữ thường `#D4D7DD`,
đường dẫn/metadata `#A1A6B0`, gợi ý và phím tắt `#7B818C`. Chỉ có MỘT màu nhấn (xanh `#7FA6E8`), dành cho thứ
đang chọn/tương tác; xanh lá chỉ cho thành công và dòng thêm, đỏ chỉ cho lỗi và dòng xoá, vàng chỉ cho cảnh báo;
sidebar và logo trầm. Không cần cấu hình gì: TUI, Home, hộp thoại, banner khi thoát, màn `login`/`logout` và
help của CLI đều dùng bảng màu này (mọi provider, kể cả hyper).

Theme cũ `charmtone-panther` (và `gruvbox-dark`) không bị sửa và vẫn chọn được: `Ctrl+P` → **Themes** (↑/↓ xem thử,
Enter chọn, Esc quay lại), hoặc đặt trong `~/.config/agent-cli/crush.json`:

```json
{"options": {"tui": {"active_theme": "charmtone-panther"}}}
```

Bỏ trống hoặc xoá `active_theme` thì về `everforest-dark-v2`. Thứ tự ưu tiên khi nhiều nơi cùng đặt (đã thử):
`crush.json`/`crushrc` trong thư mục dự án thắng `<data dir>/crush.json` (hộp thoại Themes ghi vào đây, mặc định
`~/.local/share/agent-cli/crush.json`), và nó thắng `~/.config/agent-cli/crush.json`. Đã từng chọn theme trong TUI
thì giá trị đó còn trong `<data dir>/crush.json`: chọn lại `everforest-dark-v2` trong Themes để dùng mặc định mới.

Giới hạn đã biết: đường kẻ bảng markdown do glamour vẽ theo màu mặc định của terminal; màu cú pháp trong khối code
markdown theo theme đầu tiên của tiến trình cho tới khi mở lại (đổi theme giữa chừng thì khối code mới chưa đổi màu).
Đừng sửa `everforest-dark-v2` bằng trình soạn theme (`Ctrl+E`): bản đã sửa là theme người dùng nên mất các điều chỉnh
theo vai trò; `Ctrl+D` (revert) trả về bản dựng sẵn. Bảng màu nằm ở `internal/ui/styles/everforest_dark_v2.go`
(theme mặc định chọn ở `DefaultThemeName`, `internal/ui/styles/themes.go`); kiểm bằng `go test ./internal/ui/styles/`.

## Cập nhật và kiểm chứng

```bash
git pull --ff-only
bash scripts/install-agent-cli.sh   # cuối lần chạy in gợi ý diff -u nếu crushrc/env của bạn khác mẫu mới
agent-cli --version
agent-cli models                    # kiểm cấu hình nạp được, không gọi model
bash scripts/test-agent-cli-wrapper.sh   # tuỳ chọn: kiểm launcher + installer trong thư mục tạm, in PASS
```

(`git diff --check` chỉ dùng để soát khoảng trắng trong thay đổi CỦA BẠN trước khi commit; sau
`git pull --ff-only` cây sạch nên nó luôn rỗng và không kiểm được gì về bản vừa cập nhật.)

Installer sao lưu binary và wrapper đang cài vào `~/.local/lib/agent-cli/backup-<thời điểm>/`
(giữ 3 bản mới nhất), build ra tệp tạm rồi mới thay (build lỗi thì bản cũ còn nguyên), và in
lệnh `cp -p` để quay lại. `agent-cli-tmux` là bản chép nên cũng được thay bằng bản mới ở mỗi lần cài (không
nằm trong bản sao lưu).

Đã kiểm chứng: nạp `crushrc` + env với biến giả (provider `main`, model lớn/nhỏ, key rỗng
và key có giá trị, thiếu biến thì dừng kèm thông báo). Chưa kiểm chứng với endpoint mới (tool
calling và streaming cần thử thật trên model bạn chọn), server nhà, phiên dài/compact, API key
thật của provider, OAuth native (`agent-cli login openai`). Login subscription không đồng nghĩa API key.
