#!/usr/bin/env bash
# Build và cài CLI cá nhân; không ghi đè cấu hình thật.
set -euo pipefail
repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
install_dir="${AGENT_CLI_INSTALL_DIR:-$HOME/.local}"
# Đổi sang đường dẫn tuyệt đối ngay từ đầu: wrapper ghi cứng đường dẫn binary vào chính nó,
# nên nếu để tương đối thì wrapper chỉ chạy được khi đứng đúng thư mục cài.
mkdir -p -- "$install_dir"
install_dir="$(cd -- "$install_dir" && pwd)"
config_dir="$HOME/.config/agent-cli"
lib_dir="$install_dir/lib/agent-cli"
bin_path="$lib_dir/crush"
wrapper_path="$install_dir/bin/agent-cli"
mkdir -p "$install_dir/bin" "$lib_dir"
# Thư mục cấu hình chứa env (có key): tạo mới thì 700; đã có thì chỉ siết nếu còn mở cho nhóm/người khác.
# Nếu nó là symlink thì xét và siết thư mục ĐÍCH đã phân giải, có chủ đích: chỉ siết khi đích là của
# chính user (đích có thể là thư mục dùng chung), và nói rõ đã đổi gì. `find` không theo symlink nên
# xét symlink thì luôn thấy mode 777 và chmod sẽ đổi quyền đích một cách mù quáng.
if [[ -L "$config_dir" && ! -d "$config_dir" ]]; then
  echo "$config_dir là symlink hỏng (đích không tồn tại): tạo đích hoặc gỡ symlink rồi cài lại." >&2
  exit 1
elif [[ ! -d "$config_dir" ]]; then
  install -d -m 700 "$config_dir"
else
  real_config_dir="$(cd -P -- "$config_dir" && pwd)"
  if [[ -n "$(find "$real_config_dir" -maxdepth 0 -perm /077)" ]]; then
    if [[ ! -O "$real_config_dir" ]]; then
      echo "Cảnh báo: $real_config_dir (đích của $config_dir) không thuộc user hiện tại và còn mở cho nhóm/người khác; installer không đổi quyền. Tệp env trong đó chứa key: tự kiểm tra." >&2
    else
      chmod go-rwx "$real_config_dir"
      if [[ -L "$config_dir" ]]; then
        echo "$config_dir là symlink: đã siết quyền thư mục đích $real_config_dir về go-rwx (chứa env có key)."
      fi
    fi
  fi
fi
# Cấu hình chung: crushrc đọc biến AGENT_CLI_* từ tệp env. Không ghi đè tệp có sẵn.
# crushrc thắng crush.json cùng thư mục: nếu crush.json (theo cả symlink, không sửa) đang khai
# provider thì KHÔNG tạo crushrc, kẻo thay provider đang chạy bằng placeholder. jq thiếu hoặc
# JSON hỏng thì coi như có provider (an toàn hơn là ghi đè).
legacy_providers=
if [[ -e "$config_dir/crush.json" ]]; then
  n=$(jq '(.providers // {}) | length' "$config_dir/crush.json" 2>/dev/null) || n=1
  [[ "$n" != 0 ]] && legacy_providers=1
fi
if [[ ! -e "$config_dir/crushrc" && -n "$legacy_providers" ]]; then
  [[ -e "$config_dir/crushrc.example" ]] || install -m 644 "$repo_dir/agent-cli.crushrc.example" "$config_dir/crushrc.example"
  [[ -e "$config_dir/env.example" ]] || install -m 644 "$repo_dir/agent-cli.env.example" "$config_dir/env.example"
  echo "Có $config_dir/crush.json đang khai provider: KHÔNG tạo crushrc (nó sẽ ghi đè provider đó). Đã để crushrc.example và env.example cạnh nó; để chuyển sang biến AGENT_CLI_*, xoá 'providers' và 'models' khỏi crush.json (giữ theme), rồi đổi tên: crushrc.example -> crushrc, env.example -> env (chmod 600) và điền giá trị."
else
  if [[ ! -e "$config_dir/crushrc" ]]; then
    install -m 644 "$repo_dir/agent-cli.crushrc.example" "$config_dir/crushrc"
    echo "Đã tạo $config_dir/crushrc."
  fi
  if [[ ! -e "$config_dir/env" ]]; then
    install -m 600 "$repo_dir/agent-cli.env.example" "$config_dir/env"
    echo "Đã tạo $config_dir/env (mode 600); điền AGENT_CLI_BASE_URL, AGENT_CLI_MODEL, AGENT_CLI_API_KEY trước khi gọi AI."
  fi
  if [[ -e "$config_dir/crush.json" || -L "$config_dir/crush.json" ]]; then
    echo "Có $config_dir/crush.json cũ: giữ nguyên (không khai provider). Cấu hình model nay nằm ở crushrc + env."
  fi
fi
# AGENT_CLI_SKIP_BUILD=1: dùng binary có sẵn (test launcher đặt stub vào đó).
if [[ "${AGENT_CLI_SKIP_BUILD:-}" == 1 ]]; then
  echo "Bỏ qua go build (AGENT_CLI_SKIP_BUILD=1); dùng binary có sẵn."
else
  # Build ra tệp tạm: build lỗi thì dừng ở đây, bản đang chạy và các bản sao lưu còn nguyên.
  # Biến build như bản chính thức (.goreleaser.yml, Taskfile.yaml): CGO_ENABLED=0 và
  # GOEXPERIMENT=greenteagc. Đã kiểm với go1.27.0: tên experiment này được nhận (tên lạ bị báo
  # `unknown GOEXPERIMENT`). Phiên bản hiển thị lấy từ VCS stamping của go build, không cần -ldflags.
  (cd "$repo_dir" && CGO_ENABLED=0 GOEXPERIMENT=greenteagc GOMAXPROCS=2 go build -p 2 -o "$bin_path.new" .)
  built=1
fi
# Wrapper: mọi tiến trình agent-cli trong cùng thư mục gốc dự án dùng chung một server.
wrapper_tpl=$(cat <<'EOF'
#!/usr/bin/env bash
export CRUSH_GLOBAL_CONFIG="$HOME/.config/agent-cli"
export CRUSH_GLOBAL_DATA="$HOME/.local/share/agent-cli"
export CRUSH_CACHE_DIR="$HOME/.cache/agent-cli"
export CRUSH_DISABLE_METRICS=1
export CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1
export CRUSH_DISABLE_UPDATE_CHECK=1
export CRUSH_CLIENT_SERVER="${CRUSH_CLIENT_SERVER:-1}"
# Home mở workspace qua chính launcher này để mỗi workspace có --host riêng của nó.
export AGENT_CLI_LAUNCHER="$0"
# Giá trị hiểu đúng như strconv.ParseBool của Go (useClientServer trong internal/cmd/root.go): bật là
# 1 t T TRUE true True, tắt là 0 f F FALSE false False. Giá trị khác Go coi là tắt, nên launcher cũng
# coi là tắt (cảnh báo) thay vì thêm --host cho một tiến trình sẽ chạy in-process.
case "$CRUSH_CLIENT_SERVER" in 0 | f | F | FALSE | false | False) ;;
1 | t | T | TRUE | true | True)
  # shortcut: chỉ nhận -c/--cwd đứng riêng (không gộp cờ kiểu -dc), nâng cấp khi cần.
  have_host= cwd=$PWD sub= skip= bare=1
  args=("$@")
  for ((i = 0; i < ${#args[@]}; i++)); do
    if [[ -n "$skip" ]]; then skip=; continue; fi
    # Chỉ -d/--debug giữ được Home; cờ khác (-c, -s, -C, -y, -D, -H...) là vào thẳng workspace.
    [[ "${args[i]}" == -d || "${args[i]}" == --debug ]] || bare=
    case "${args[i]}" in
      --) break ;;
      --host | -H) have_host=1 skip=1 ;;
      --host=* | -H?*) have_host=1 ;;
      -c | --cwd) cwd=${args[i + 1]-} skip=1 ;;
      --cwd=*) cwd=${args[i]#--cwd=} ;;
      -c?*) cwd=${args[i]#-c} ;;
      -D | --data-dir | --channels | -s | --session) skip=1 ;;
      -*) ;;
      *) sub=${sub:-${args[i]}} ;;
    esac
  done
  # `ps` tự quét mọi server khi không có --host; thêm --host sẽ chỉ còn dự án hiện tại.
  # agent-cli trần trên terminal là Home (khớp shouldRunHome trong internal/cmd/home.go):
  # chưa chọn workspace nên không chọn server; Home tự gọi lại launcher với -c <dir>.
  home=
  [[ -n "$bare" && -t 0 && -t 1 ]] && home=1
  if [[ -z "$have_host" && -z "$home" && "$sub" != ps ]] && root=$(cd -- "$cwd" 2>/dev/null && { git rev-parse --show-toplevel 2>/dev/null || pwd -P; }); then
    # Không dùng /tmp: ai cũng ghi được và tên đoán được -> user khác chiếm socket trước.
    rt=${AGENT_CLI_RUNTIME_DIR:-/run/user/$UID}
    [[ -d "$rt" ]] || rt=${XDG_RUNTIME_DIR:-}
    if [[ ! -d "$rt" ]]; then
      rt=$HOME/.cache/agent-cli
      mkdir -p "$rt" && chmod 700 "$rt"
    fi
    h=$(printf %s "$root" | { sha1sum || shasum -a 1; } 2>/dev/null)
    sock="$rt/agent-cli-${h:0:8}.sock"
    # Không chọn được server riêng thì DỪNG, đừng chạy tiếp không --host: client sẽ rơi về server
    # mặc định dùng chung và mất cách ly theo dự án (session, trạng thái bận, log lẫn giữa các dự án).
    if [[ ${#h} -lt 8 ]]; then
      echo "agent-cli: thiếu sha1sum/shasum nên không chọn được server riêng cho dự án; cài một trong hai, hoặc chạy CRUSH_CLIENT_SERVER=0 agent-cli (in-process)." >&2
      exit 1
    elif [[ ${#sock} -gt 107 ]]; then # giới hạn sun_path 108 byte gồm NUL
      echo "agent-cli: đường dẫn socket quá dài (${#sock} byte, tối đa 107): $sock" >&2
      echo "agent-cli: đặt AGENT_CLI_RUNTIME_DIR trỏ tới thư mục riêng ngắn hơn (mode 700), hoặc chạy CRUSH_CLIENT_SERVER=0 agent-cli (in-process)." >&2
      exit 1
    fi
    set -- --host "unix://$sock" "$@"
  fi ;;
*) echo "agent-cli: CRUSH_CLIENT_SERVER=\"$CRUSH_CLIENT_SERVER\" không hợp lệ (dùng 1/0 hoặc true/false); chạy in-process như CRUSH_CLIENT_SERVER=0." >&2 ;;
esac
exec "__CRUSH_BIN__" "$@"
EOF
)
# Sao lưu bản đang cài ngay trước khi thay (luật dự án: mỗi lần cài có bản sao
# lưu và lệnh quay lại); giữ 3 bản mới nhất.
backup_dir=
if [[ -e "$bin_path" || -e "$wrapper_path" ]]; then
  backup_dir="$lib_dir/backup-$(date +%Y%m%d-%H%M%S)"
  mkdir -p "$backup_dir"
  [[ -e "$bin_path" ]] && cp -p -- "$bin_path" "$backup_dir/crush"
  [[ -e "$wrapper_path" ]] && cp -p -- "$wrapper_path" "$backup_dir/agent-cli"
  mapfile -t old_backups < <(ls -1dt -- "$lib_dir"/backup-* 2>/dev/null | tail -n +4)
  for d in "${old_backups[@]}"; do rm -rf -- "$d"; done
fi
# mv thay được cả binary đang chạy (cp thẳng lên nó báo "Text file busy").
[[ -n "${built:-}" ]] && mv -f -- "$bin_path.new" "$bin_path"
# Ghi tệp tạm rồi mv: nguyên tử, và thay chính symlink chứ không ghi xuyên qua nó.
printf '%s\n' "${wrapper_tpl//__CRUSH_BIN__/"$bin_path"}" > "$wrapper_path.tmp"
chmod 755 "$wrapper_path.tmp"
mv -f -- "$wrapper_path.tmp" "$wrapper_path"
# agent-cli-tmux: chép (không symlink) để bản cài không phụ thuộc thư mục checkout (đổi nhánh, di
# chuyển hay xoá repo không làm hỏng nó). Ghi tệp tạm rồi mv: thay được cả symlink của bản cài cũ
# mà không ghi xuyên qua nó vào tệp trong repo.
install -m 755 "$repo_dir/scripts/agent-cli-tmux.sh" "$install_dir/bin/agent-cli-tmux.tmp"
mv -f -- "$install_dir/bin/agent-cli-tmux.tmp" "$install_dir/bin/agent-cli-tmux"
# In lệnh quay lại TRƯỚC khi chạy thử: bản mới hỏng ngay ở --version mới là lúc cần nó.
if [[ -n "$backup_dir" ]]; then
  echo "Bản cũ đã sao lưu ở $backup_dir. Quay lại bằng:"
  [[ -e "$backup_dir/crush" ]] && printf '  cp -p %q %q && mv -f %q %q\n' "$backup_dir/crush" "$bin_path.new" "$bin_path.new" "$bin_path"
  [[ -e "$backup_dir/agent-cli" ]] && printf '  cp -p %q %q && mv -f %q %q\n' "$backup_dir/agent-cli" "$wrapper_path.tmp" "$wrapper_path.tmp" "$wrapper_path"
fi
"$wrapper_path" --version
echo "Thêm $install_dir/bin vào PATH nếu terminal chưa nhận agent-cli."
# Mẫu trong repo có thể đã thêm biến hoặc chú thích mới so với tệp bạn đang dùng. Không bao giờ
# ghi đè tệp của user: chỉ gợi ý lệnh để tự xem khác biệt rồi gộp tay.
diff_hints=()
for pair in "crushrc:agent-cli.crushrc.example" "env:agent-cli.env.example"; do
  live="$config_dir/${pair%%:*}" example="$repo_dir/${pair#*:}"
  if [[ -f "$live" ]] && ! cmp -s -- "$live" "$example"; then
    diff_hints+=("diff -u $(printf %q "$live") $(printf %q "$example")")
  fi
done
if ((${#diff_hints[@]} > 0)); then
  echo "Cấu hình đang dùng khác mẫu trong repo (installer không ghi đè tệp của bạn). Xem khác biệt:"
  printf '  %s\n' "${diff_hints[@]}"
  echo "(env chứa giá trị của bạn nên luôn khác mẫu; diff sẽ in cả key, đừng chạy khi chia sẻ màn hình.)"
fi
