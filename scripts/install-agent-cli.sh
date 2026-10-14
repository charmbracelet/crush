#!/usr/bin/env bash
# Build và cài CLI cá nhân; không ghi đè cấu hình thật.
set -euo pipefail
repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
install_dir="${AGENT_CLI_INSTALL_DIR:-$HOME/.local}"
config_dir="$HOME/.config/agent-cli"
mkdir -p "$install_dir/bin" "$install_dir/lib/agent-cli"
# Thư mục cấu hình chứa env (có key): tạo mới thì 700; đã có thì chỉ siết nếu còn mở cho nhóm/người khác.
if [[ ! -d "$config_dir" ]]; then
  install -d -m 700 "$config_dir"
elif [[ -n "$(find "$config_dir" -maxdepth 0 -perm /077)" ]]; then
  chmod go-rwx "$config_dir"
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
  (cd "$repo_dir" && GOMAXPROCS=2 go build -p 2 -o "$install_dir/lib/agent-cli/crush" .)
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
case "$CRUSH_CLIENT_SERVER" in 0 | f | F | false | FALSE | False) ;; *)
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
    if [[ ${#h} -lt 8 ]]; then
      echo "agent-cli: thiếu sha1sum/shasum, không chọn được server theo dự án." >&2
    elif [[ ${#sock} -gt 107 ]]; then # giới hạn sun_path 108 byte gồm NUL
      echo "agent-cli: đường dẫn socket quá dài (${#sock} byte), không chọn được server theo dự án." >&2
    else
      set -- --host "unix://$sock" "$@"
    fi
  fi ;;
esac
exec "__CRUSH_BIN__" "$@"
EOF
)
printf '%s\n' "${wrapper_tpl//__CRUSH_BIN__/"$install_dir/lib/agent-cli/crush"}" > "$install_dir/bin/agent-cli"
chmod 755 "$install_dir/bin/agent-cli"
ln -sfn "$repo_dir/scripts/agent-cli-tmux.sh" "$install_dir/bin/agent-cli-tmux"
"$install_dir/bin/agent-cli" --version
echo "Thêm $install_dir/bin vào PATH nếu terminal chưa nhận agent-cli."
