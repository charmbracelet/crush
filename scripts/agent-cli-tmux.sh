#!/usr/bin/env bash
# Mở nhiều dự án trong MỘT tmux session: mỗi thư mục một window chạy agent-cli.
# Dùng: agent-cli-tmux <dir>...   (prefix+w liệt kê các dự án)
#       agent-cli-tmux new <name> [--model P/M] [--repo dir] [--base ref]  (worktree + window riêng)
#       agent-cli-tmux rm <name> [--repo dir] [--force] | ls [--repo dir]
#         (rm từ chối khi worktree có file bị gitignore, vd .crush/ lịch sử session, .env; --force mới xoá)
# Env: AGENT_CLI_TMUX_SESSION (mặc định agents), AGENT_CLI_TMUX_NO_ATTACH=1 (không attach).
set -euo pipefail
session="${AGENT_CLI_TMUX_SESSION:-agents}"

if [[ $# -eq 0 || "$1" == -h || "$1" == --help ]]; then
  echo "Dùng: agent-cli-tmux <dir>... | new <name> [--model P/M] [--repo dir] [--base ref] | rm <name> [--repo dir] [--force] | ls [--repo dir]" >&2
  [[ $# -gt 0 ]] && exit 0 || exit 2
fi

# Tìm window theo @agent_dir (đường dẫn vật lý), in window_id.
find_window() {
  tmux has-session -t "=$session" 2>/dev/null || return 0
  tmux list-windows -t "=$session" -F '#{window_id}'$'\t''#{@agent_dir}' | awk -F'\t' -v d="$1" '$2 == d { print $1; exit }'
}

die() { echo "agent-cli-tmux: $2" >&2; exit "$1"; }

need_agent_cli() {
  command -v agent-cli >/dev/null || die 1 "không thấy agent-cli trên PATH (chạy scripts/install-agent-cli.sh và thêm ~/.local/bin vào PATH)."
}

# Phân nhánh con: new|rm|ls. Thư mục tên "new"/"rm"/"ls" thì viết ./new.
cmd=
case "$1" in new | rm | ls) cmd=$1; shift ;; esac
wname=
if [[ -n $cmd ]]; then
  name= model= repo= base=HEAD force=
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --force) force=1; shift ;;
      --model | --repo | --base)
        [[ $# -ge 2 ]] || die 2 "$1 cần một giá trị"
        case "$1" in --model) model=$2 ;; --repo) repo=$2 ;; --base) base=$2 ;; esac
        shift 2 ;;
      -*) die 2 "cờ không hợp lệ: $1" ;;
      *) [[ -z $name && $cmd != ls ]] || die 2 "thừa đối số: $1"; name=$1; shift ;;
    esac
  done
  if [[ $cmd != ls ]]; then
    [[ $name =~ ^[a-z0-9][a-z0-9-]{0,40}$ ]] || die 2 "tên việc phải khớp ^[a-z0-9][a-z0-9-]{0,40}\$ (nhận: '$name')"
  fi
  [[ $cmd == new ]] || [[ $model == "" && $base == HEAD ]] || die 2 "--model/--base chỉ dùng với new"
  [[ $cmd == rm || -z $force ]] || die 2 "--force chỉ dùng với rm"
  [[ -z $model || $model =~ ^[^/]+/.+$ ]] || die 2 "--model phải dạng <provider>/<model> (nhận: '$model')"
  git -C "${repo:-$PWD}" rev-parse --show-toplevel >/dev/null 2>&1 || die 2 "không phải repo git: ${repo:-$PWD}"
  # Luôn dùng worktree CHÍNH (dòng đầu của list): chạy từ trong một worktree con không được đẻ <con>-agents lồng nhau.
  top=$(git -C "${repo:-$PWD}" worktree list --porcelain | sed -n '1s/^worktree //p')
  [[ -n $top ]] || die 2 "không xác định được worktree chính: ${repo:-$PWD}"
  agents_dir="$(dirname -- "$top")/$(basename -- "$top")-agents"
  wt="$agents_dir/$name"
  branch="agent/$name"
  # Symlink ở chỗ worktree: `git worktree remove`/tạo sẽ đi theo link ra ngoài, từ chối trước khi đụng window nào.
  [[ $cmd == ls || ! -L $wt ]] || die 2 "$wt là symlink, từ chối"

  case $cmd in
    ls)
      for d in "$agents_dir"/*/; do
        [[ -e $d/.git ]] || continue
        d=${d%/}
        # Dữ liệu git của một worktree mất thì ghi broken và đi tiếp, đừng bỏ cả danh sách.
        b=$(git -C "$d" branch --show-current 2>/dev/null) || { printf '%s  (unknown)  broken\n' "$(basename -- "$d")"; continue; }
        # Không đọc được trạng thái thì coi là dirty (fail closed).
        s=dirty; st=$(git -C "$d" status --porcelain --untracked-files=normal) && [[ -z $st ]] && s=clean
        printf '%s  %s  %s\n' "$(basename -- "$d")" "${b:-(detached)}" "$s"
      done
      exit 0 ;;
    rm)
      [[ -e $wt/.git ]] || die 2 "không có worktree: $wt"
      # --untracked-files=normal: không để status.showUntrackedFiles=no (cấu hình user) che file chưa theo dõi.
      st=$(git -C "$wt" status --porcelain --untracked-files=normal) || die 3 "không đọc được trạng thái: $wt"
      [[ -z $st ]] || die 3 "worktree còn thay đổi chưa commit, không xoá: $wt"
      # git worktree remove xoá luôn file bị gitignore (.crush/crush.db lịch sử session, .env...) mà không hỏi:
      # thiếu --force thì liệt kê và từ chối. Gọi lại ngay trước khi xoá, vì agent có thể ghi lúc window bị đóng.
      check_ignored() {
        ign=$(git -C "$wt" status --porcelain --ignored --untracked-files=normal | sed -n 's/^!! //p') || die 3 "không đọc được trạng thái: $wt"
        [[ -n $ign && -z $force ]] || return 0
        { echo "agent-cli-tmux: các mục bị gitignore sẽ bị xoá vĩnh viễn:"; sed 's/^/  /' <<< "$ign"; } >&2
        die 3 "worktree có file bị gitignore, không xoá (thêm --force nếu chắc chắn): $wt"
      }
      check_ignored
      [[ -z $ign ]] || echo "agent-cli-tmux: --force: xoá kèm $(wc -l <<< "$ign") mục bị gitignore trong $wt" >&2
      abs=$(cd -- "$wt" && pwd -P)
      # Dưới CRUSH_CLIENT_SERVER=1 agent chạy ở server riêng, đóng window KHÔNG dừng nó: từ chối nếu
      # worktree còn session BUSY/WAITING (ps --json không bao giờ tự dựng server).
      if command -v agent-cli >/dev/null; then
        command -v jq >/dev/null || die 3 "cần jq để kiểm session đang chạy (agent-cli ps --json)"
        # Mọi lỗi đều fail closed: rc≠0 (gồm cờ --all không tồn tại), BẤT KỲ stderr nào (ps bỏ qua server lỗi mà vẫn rc=0), JSON hỏng.
        psfail() { rm -f "$pserr"; die 3 "$1, không biết worktree có đang chạy không: $wt"; }
        pserr=$(mktemp)
        ps=$(agent-cli ps --json --all 2>"$pserr") || psfail "agent-cli ps --json --all lỗi"
        [[ ! -s $pserr ]] || psfail "agent-cli ps báo lỗi một phần ($(head -c 200 "$pserr" | tr '\n' ' '))"
        rm -f "$pserr"
        rows=$(printf '%s' "$ps" | jq -r '.[]? | select(.busy or .waiting) | "P:" + (.path // "")') || die 3 "agent-cli ps --json không phải JSON hợp lệ, không biết worktree có đang chạy không: $wt"
        # Server lưu path đúng như client gửi (có thể symlink): chuẩn hoá bằng pwd -P rồi so; không phân giải được thì từ chối.
        while IFS= read -r p; do
          [[ -n $rows ]] || break
          p=${p#P:} # tiền tố P: để hàng BUSY có path rỗng vẫn thành một dòng, không bị nuốt
          [[ $p == /* ]] || die 3 "session đang chạy có path không tuyệt đối ('$p'), từ chối xoá: $wt"
          c=$(cd -- "$p" 2>/dev/null && pwd -P) || die 3 "session đang chạy có path không phân giải được ('$p'), từ chối xoá: $wt"
          [[ $c != "$abs" ]] || die 3 "worktree còn session BUSY/WAITING (agent-cli ps), không xoá: $wt"
        done <<< "$rows"
      else
        echo "agent-cli-tmux: không thấy agent-cli trên PATH, bỏ qua kiểm session đang chạy" >&2
      fi
      # Đóng window và đợi tiến trình pane thoát (nó có thể ghi nốt file lúc SIGHUP) rồi mới để git kiểm lại.
      id=$(find_window "$abs")
      if [[ -n $id ]]; then
        pids=$(tmux list-panes -t "$id" -F '#{pane_pid}')
        tmux kill-window -t "$id"
        for _ in $(seq 30); do
          alive=
          for pid in $pids; do kill -0 "$pid" 2>/dev/null && alive=1; done
          [[ -z $alive ]] && break
          sleep 0.1
        done
      fi
      check_ignored
      # status.showUntrackedFiles=no làm kiểm của chính git mù với file chưa theo dõi: ép normal.
      git -C "$top" -c status.showUntrackedFiles=normal worktree remove "$wt" || die 3 "git worktree remove thất bại (có thể agent vừa ghi file): $wt"
      echo "branch $branch kept"
      exit 0 ;;
  esac

  # new: kiểm hết trước khi tạo gì.
  need_agent_cli
  [[ -z $model ]] || command -v jq >/dev/null || die 1 "--model cần jq nhưng không thấy trên PATH"
  # --base phân giải tại nơi user chạy lệnh (có thể là worktree con), rồi truyền SHA.
  ref=$base
  base=$(git -C "${repo:-$PWD}" rev-parse --verify --quiet "$ref^{commit}") || die 2 "--base không phải commit: $ref"
  if [[ ! -e $wt/.git ]]; then
    if git -C "$top" show-ref --verify --quiet "refs/heads/$branch"; then
      git -C "$top" worktree add -q "$wt" "$branch"
    else
      git -C "$top" worktree add -q -b "$branch" "$wt" "$base"
    fi
  fi
  if [[ -n $model ]]; then
    mkdir -p "$wt/.crush"
    [[ -e $wt/.crush/.gitignore ]] || echo '*' > "$wt/.crush/.gitignore"
    cfg=$wt/.crush/crush.json
    [[ -s $cfg ]] || echo '{}' > "$cfg"
    jq --arg p "${model%%/*}" --arg m "${model#*/}" '. * {models: {large: {provider: $p, model: $m}}}' "$cfg" > "$cfg.tmp" &&
      mv "$cfg.tmp" "$cfg" || { rm -f "$cfg.tmp"; die 1 "không ghi được $cfg (JSON hỏng?)"; }
  fi
  set -- "$wt"
  wname=$name
fi

need_agent_cli

# Kiểm hết thư mục trước khi tạo bất cứ gì, để lỗi không để lại session nửa vời.
dirs=()
for d in "$@"; do
  if ! abs=$(cd -- "$d" 2>/dev/null && pwd -P); then
    echo "agent-cli-tmux: thư mục không tồn tại: $d" >&2
    exit 2
  fi
  dirs+=("$abs")
done

target=
for abs in "${dirs[@]}"; do
  # Nhận diện window theo đường dẫn (@agent_dir), không theo tên: hai dự án có thể trùng basename.
  id=$(find_window "$abs")
  if [[ -z "$id" ]]; then
    # env: default-shell của tmux có thể không phải POSIX (fish) nên không dùng `VAR=1 cmd`.
    cmd="env CRUSH_CLIENT_SERVER=1 agent-cli -c $(printf %q "$abs")"
    # tmux format-expand -n/-c/-s: `#` phải thành `##`, nếu không `#(cmd)` bị chạy và `#S` bị thay.
    name=${wname:-$(basename -- "$abs")}
    name=${name//#/##} esc=${abs//#/##}
    if tmux has-session -t "=$session" 2>/dev/null; then
      id=$(tmux new-window -P -F '#{window_id}' -t "=$session:" -n "$name" -c "$esc" "$cmd")
    else
      id=$(tmux new-session -d -P -F '#{window_id}' -s "${session//#/##}" -n "$name" -c "$esc" "$cmd")
    fi
    tmux set-option -w -t "$id" @agent_dir "$abs"
  fi
  target=$id
done
tmux select-window -t "$target"

[[ "${AGENT_CLI_TMUX_NO_ATTACH:-}" == 1 ]] && exit 0
if [[ -n "${TMUX:-}" ]]; then
  exec tmux switch-client -t "=$session"
fi
exec tmux attach-session -t "=$session"
