#!/usr/bin/env bash
# Kiểm agent-cli-tmux bằng tmux server riêng (-L agentcli-test) và agent-cli giả.
set -euo pipefail
repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
real_tmux=$(command -v tmux)
work=$(mktemp -d)
sock="$work/tmux.sock" # trong $work: rm -rf dọn luôn, không để rác ở /tmp/tmux-UID
# Wrapper tmux ép dùng socket riêng: script thật gọi `tmux` trần, không bao giờ chạm server của user.
trap '"$real_tmux" -S "$sock" kill-server 2>/dev/null || true; rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/a" "$work/b"
# Nếu TMPDIR nằm trong một repo thật, git không được leo ra ngoài $work (ca not-git sẽ tạo branch trong repo đó).
export GIT_CEILING_DIRECTORIES="$work"
printf '#!/bin/sh\nexec %s -S %s "$@"\n' "$real_tmux" "$sock" > "$work/bin/tmux"
# Stub: `ps` in nội dung $work/ps.json; có --host hoặc thiếu --all là lỗi (--host còn để lại $work/host-seen);
# ps-fail = rc 1; ps-stderr = lỗi một phần (stderr + [] + rc 0). Nhận SIGHUP thì ghi late.txt nếu có $work/hup-<tên thư mục>.
cat > "$work/bin/agent-cli" <<STUB
#!/bin/sh
if [ "\$1" = ps ]; then
  case " \$* " in *" --host"*) touch $work/host-seen; exit 1 ;; esac
  case " \$* " in *" --all "*) ;; *) exit 1 ;; esac
  [ -e $work/ps-fail ] && exit 1
  [ -e $work/ps-stderr ] && echo "Skipping unix:///x.sock: status code 500" >&2
  cat $work/ps.json 2>/dev/null || echo '[]'
  exit 0
fi
trap '[ -e $work/hup-\$(basename "\$PWD") ] && echo late > late.txt; exit 0' HUP
echo "\$CRUSH_CLIENT_SERVER \$*" >> $work/calls
sleep 30 & wait
STUB
chmod +x "$work/bin/tmux" "$work/bin/agent-cli"
export PATH="$work/bin:$PATH" AGENT_CLI_TMUX_NO_ATTACH=1
unset TMUX
tmux kill-server 2>/dev/null || true

fail() { echo "FAIL: $*" >&2; exit 1; }
run() { bash "$repo_dir/scripts/agent-cli-tmux.sh" "$@"; }
windows() { tmux list-windows -t =agents | wc -l | tr -d ' '; }

run "$work/a" "$work/b"
[[ $(windows) == 2 ]] || fail "mong 2 window, có $(windows)"
names=$(tmux list-windows -t =agents -F '#{window_name}' | sort | tr '\n' ' ')
[[ $names == "a b " ]] || fail "tên window sai: $names"
for _ in $(seq 50); do [[ $(wc -l < "$work/calls" 2>/dev/null || echo 0) -ge 2 ]] && break; sleep 0.1; done
for d in a b; do
  grep -qxF "1 -c $work/$d" "$work/calls" || fail "stub không thấy '1 -c $work/$d': $(cat "$work/calls")"
done

run "$work/a"
[[ $(windows) == 2 ]] || fail "chạy lại đẻ thêm window: $(windows)"
[[ $(tmux display-message -p -t =agents: '#{window_name}') == a ]] || fail "không chọn window a"

rc=0; run "$work/khong-ton-tai" 2>"$work/err" || rc=$?
[[ $rc == 2 ]] || fail "thư mục sai: rc=$rc, mong 2"
[[ -s "$work/err" ]] || fail "thư mục sai không in lỗi ra stderr"
[[ $(windows) == 2 ]] || fail "thư mục sai làm đổi số window"

# F4: thư mục có dấu cách; cùng thư mục qua đường tương đối / symlink / lặp trong một lệnh -> 1 window.
mkdir -p "$work/with space"
run "$work/with space"
[[ $(windows) == 3 ]] || fail "thư mục có dấu cách: $(windows) window"
ln -s "$work/a" "$work/link-a"
(cd "$work" && run a ./a link-a "$work/a" "$work/a")
[[ $(windows) == 3 ]] || fail "a qua tương đối/symlink/lặp đẻ thêm window: $(windows)"
run "$work/b" "$work/b"
[[ $(windows) == 3 ]] || fail "b lặp trong một lệnh đẻ thêm window: $(windows)"

# F1: `#` trong tên thư mục không được bị tmux format-expand (không chạy #(cmd), không thay #S).
marker="$work/INJECTED"
mkdir -p "$work/a#S b" "$work/x#(touch $marker)"
run "$work/a#S b" "$work/x#(touch $marker)"
sleep 0.5
[[ ! -e $marker ]] || fail "tmux đã chạy lệnh chèn qua tên thư mục"
[[ $(windows) == 5 ]] || fail "mong 5 window, có $(windows)"
for d in "a#S b" "x#(touch $marker)"; do
  want=$(cd "$work/$d" && pwd -P)
  got=$(tmux list-windows -t =agents -F '#{window_name}|#{pane_current_path}' | grep -F -- "|$want" || true)
  [[ $got == "$d|$want" || $got == "${d##*/}|$want" ]] || fail "window/cwd sai cho '$d': '$got'"
done

# F2: thiếu agent-cli trên PATH -> rc=1 và có thông báo.
rc=0; PATH="$work/bin-empty:/usr/bin:/bin" bash "$repo_dir/scripts/agent-cli-tmux.sh" "$work/a" 2>"$work/err" || rc=$?
[[ $rc == 1 && -s "$work/err" ]] || fail "thiếu agent-cli: rc=$rc"

# --- new | rm | ls: repo git tạm (1 commit), worktree nằm cạnh repo trong $work/proj-agents ---
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
proj="$work/proj"
git init -q "$proj" && git -C "$proj" commit -q --allow-empty -m init
wtd="$work/proj-agents"
nwin() { tmux list-windows -t =agents -F '#{window_name}' | grep -cxF -- "$1" || true; }
nbr() { git -C "$proj" branch --list 'agent/*' | wc -l | tr -d ' '; }
rc_of() { rc=0; run "$@" >"$work/out" 2>"$work/err" || rc=$?; }

# new-basic
rc_of new fix1 --repo "$proj"
[[ $rc == 0 ]] || fail "new-basic: rc=$rc: $(cat "$work/err")"
[[ -e $wtd/fix1/.git ]] || fail "new-basic: thiếu worktree $wtd/fix1"
git -C "$proj" show-ref --verify --quiet refs/heads/agent/fix1 || fail "new-basic: thiếu branch agent/fix1"
[[ $(git -C "$wtd/fix1" branch --show-current) == agent/fix1 ]] || fail "new-basic: worktree không ở agent/fix1"
[[ $(nwin fix1) == 1 ]] || fail "new-basic: window fix1 = $(nwin fix1)"
for _ in $(seq 50); do grep -qxF "1 -c $wtd/fix1" "$work/calls" && break; sleep 0.1; done
grep -qxF "1 -c $wtd/fix1" "$work/calls" || fail "new-basic: stub không nhận -c $wtd/fix1: $(cat "$work/calls")"
[[ $(tmux display-message -p -t =agents: '#{window_name}') == fix1 ]] || fail "new-basic: không chọn window fix1"

# new-model: ghi crush.json, gộp với file có sẵn, tách provider ở dấu / đầu tiên, worktree vẫn sạch
git -C "$proj" worktree add -q -b agent/pre "$wtd/pre"
mkdir -p "$wtd/pre/.crush"
echo '{"options":{"debug":true},"models":{"large":{"provider":"old","max_tokens":9},"small":{"provider":"s","model":"sm"}}}' > "$wtd/pre/.crush/crush.json"
rc_of new pre --repo "$proj" --model openrouter/anthropic/claude-x
[[ $rc == 0 ]] || fail "new-model: rc=$rc: $(cat "$work/err")"
got=$(jq -c '[.models.large.provider, .models.large.model, .models.large.max_tokens, .models.small.model, .options.debug]' "$wtd/pre/.crush/crush.json")
[[ $got == '["openrouter","anthropic/claude-x",9,"sm",true]' ]] || fail "new-model: gộp sai: $got"
[[ $(cat "$wtd/pre/.crush/.gitignore") == '*' ]] || fail "new-model: .crush/.gitignore sai"
rc_of new m1 --repo "$proj" --model main/model-x
[[ $rc == 0 ]] || fail "new-model: m1 rc=$rc: $(cat "$work/err")"
[[ $(jq -c .models.large "$wtd/m1/.crush/crush.json") == '{"provider":"main","model":"model-x"}' ]] || fail "new-model: crush.json m1 sai"
[[ -z $(git -C "$wtd/m1" status --porcelain) ]] || fail "new-model: .crush làm worktree dơ"

# new-twice: không đẻ branch/window thừa
b0=$(nbr)
rc_of new fix1 --repo "$proj"
[[ $rc == 0 ]] || fail "new-twice: rc=$rc"
[[ $(nbr) == "$b0" ]] || fail "new-twice: đẻ thêm branch"
[[ $(nwin fix1) == 1 ]] || fail "new-twice: window fix1 = $(nwin fix1)"

# bad-name / bad-model / not-git / bad-base: rc=2 và không tạo gì
w0=$(windows); b0=$(nbr)
for bad in Bad_Name -x "-" "$(printf 'a%.0s' $(seq 42))" ""; do
  rc_of new "$bad" --repo "$proj"; [[ $rc == 2 ]] || fail "bad-name '$bad': rc=$rc"
done
rc_of new nomodel --repo "$proj" --model nosep
[[ $rc == 2 ]] || fail "bad-model: rc=$rc"
[[ ! -e $wtd/nomodel ]] || fail "bad-model: vẫn tạo worktree"
rc_of new nomodel --repo "$proj" --model /x; [[ $rc == 2 ]] || fail "bad-model '/x': rc=$rc"
rc_of new nomodel --repo "$proj" --model main/; [[ $rc == 2 ]] || fail "bad-model 'main/': rc=$rc"
rc_of new nobase --repo "$proj" --base no-such-ref
[[ $rc == 2 && ! -e $wtd/nobase ]] || fail "bad-base: rc=$rc"
grep -q 'no-such-ref' "$work/err" || fail "bad-base: thông báo thiếu ref gốc: $(cat "$work/err")"
rc_of new x --repo "$work/a"; [[ $rc == 2 && -s $work/err ]] || fail "not-git: rc=$rc"
(cd "$work/a" && rc=0; run new x >/dev/null 2>&1 || rc=$?; [[ $rc == 2 ]]) || fail "not-git (PWD): rc khác 2"
[[ $(nbr) == "$b0" && $(windows) == "$w0" ]] || fail "ca lỗi làm đổi branch/window"

# F4/F5: thiếu agent-cli hoặc jq -> rc=1 và KHÔNG tạo worktree/branch
b0=$(nbr)
rc=0; PATH="$work/bin-empty:/usr/bin:/bin" bash "$repo_dir/scripts/agent-cli-tmux.sh" new nocli --repo "$proj" 2>"$work/err" >/dev/null || rc=$?
[[ $rc == 1 && -s $work/err && ! -e $wtd/nocli && $(nbr) == "$b0" ]] || fail "new thiếu agent-cli: rc=$rc, đã tạo worktree/branch"
mkdir -p "$work/bin-nojq"
for t in git sed dirname basename awk grep; do ln -sf "$(command -v $t)" "$work/bin-nojq/$t"; done
ln -sf "$work/bin/agent-cli" "$work/bin-nojq/agent-cli"
rc=0; PATH="$work/bin-nojq" "$(command -v bash)" "$repo_dir/scripts/agent-cli-tmux.sh" new nojq --repo "$proj" --model a/b 2>"$work/err" >/dev/null || rc=$?
[[ $rc == 1 && $(cat "$work/err") == *jq* && ! -e $wtd/nojq && $(nbr) == "$b0" ]] || fail "new --model thiếu jq: rc=$rc, $(cat "$work/err")"

# F2: new chạy từ TRONG một worktree con vẫn đặt ở <repo chính>-agents, không lồng
(cd "$wtd/pre" && run new child >/dev/null 2>"$work/err") || fail "new từ worktree con: $(cat "$work/err")"
[[ -e $wtd/child/.git && ! -e $wtd/pre-agents && ! -e $work/pre-agents ]] || fail "new từ worktree con đặt sai chỗ"
rc_of ls --repo "$proj"
grep -qxF 'child  agent/child  clean' "$work/out" || fail "ls không thấy child tạo từ worktree con: $(cat "$work/out")"
(cd "$wtd/pre" && run rm child >/dev/null 2>&1) || fail "rm child từ worktree con"
[[ ! -e $wtd/child ]] || fail "rm child từ worktree con: còn"

# N2: --base phân giải ở nơi chạy lệnh: new từ trong worktree feat -> branch child2 trỏ HEAD của feat
git -C "$proj" worktree add -q -b feat "$work/featwt"
git -C "$work/featwt" commit -q --allow-empty -m feat
(cd "$work/featwt" && run new child2 >/dev/null 2>"$work/err") || fail "new từ feat: $(cat "$work/err")"
[[ $(git -C "$proj" rev-parse agent/child2) == "$(git -C "$work/featwt" rev-parse HEAD)" ]] || fail "N2: child2 không phân nhánh từ HEAD của feat"
[[ $(git -C "$proj" rev-parse agent/child2) != "$(git -C "$proj" rev-parse HEAD)" ]] || fail "N2: child2 lấy HEAD của worktree chính"

# N3: symlink ở chỗ worktree -> rc=2, link và đích còn nguyên
ln -s "$wtd/pre" "$wtd/sym" # trỏ vào worktree thật: chỉ kiểm -L mới chặn được
rc_of rm sym --repo "$proj"; [[ $rc == 2 && -L $wtd/sym && -e $wtd/pre/.git ]] || fail "N3 rm symlink: rc=$rc"
rc_of new sym --repo "$proj"; [[ $rc == 2 && -L $wtd/sym ]] || fail "N3 new symlink: rc=$rc"

# F1: status.showUntrackedFiles=no làm git mù với file chưa theo dõi; kiểm của script phải tự bắt
git -C "$proj" config status.showUntrackedFiles no

# ls
rc_of ls --repo "$proj"
[[ $rc == 0 ]] || fail "ls: rc=$rc"
grep -qxF 'fix1  agent/fix1  clean' "$work/out" || fail "ls: thiếu dòng clean: $(cat "$work/out")"
echo x > "$wtd/fix1/dirty.txt"
rc_of ls --repo "$proj"
grep -qxF 'fix1  agent/fix1  dirty' "$work/out" || fail "ls: thiếu dòng dirty: $(cat "$work/out")"

# rm-dirty: từ chối, không xoá gì
rc_of rm fix1 --repo "$proj"
[[ $rc == 3 ]] || fail "rm-dirty: rc=$rc"
grep -q 'chưa commit' "$work/err" || fail "rm-dirty: thiếu thông báo của script: $(cat "$work/err")"
[[ -e $wtd/fix1/dirty.txt && $(nwin fix1) == 1 ]] || fail "rm-dirty: đã xoá worktree/window"
rc_of rm khong-co --repo "$proj"; [[ $rc == 2 ]] || fail "rm worktree không có: rc=$rc"

# git status lỗi (index hỏng) -> fail closed: rc=3, worktree còn, ls báo dirty
rc_of new corrupt --repo "$proj"
echo garbage > "$(git -C "$wtd/corrupt" rev-parse --absolute-git-dir)/index"
rc_of rm corrupt --repo "$proj"
[[ $rc == 3 && -e $wtd/corrupt/.git ]] || fail "rm index hỏng: rc=$rc"
grep -q 'không đọc được' "$work/err" || fail "rm index hỏng: thiếu thông báo: $(cat "$work/err")"
rc_of ls --repo "$proj"
grep -qxF 'corrupt  agent/corrupt  dirty' "$work/out" || fail "ls index hỏng phải báo dirty: $(cat "$work/out")"

# N5: session BUSY/WAITING (agent-cli ps --json) -> rc=3, worktree còn; ps lỗi -> cũng từ chối
rc_of new busy --repo "$proj"
for flag in busy waiting; do
  echo "[{\"path\":\"$work/a\",\"busy\":true,\"waiting\":false},{\"path\":\"$wtd/busy\",\"busy\":$([[ $flag == busy ]] && echo true || echo false),\"waiting\":$([[ $flag == waiting ]] && echo true || echo false)}]" > "$work/ps.json"
  rc_of rm busy --repo "$proj"
  [[ $rc == 3 && -e $wtd/busy/.git && $(nwin busy) == 1 ]] || fail "N5 $flag: rc=$rc"
  grep -q 'BUSY/WAITING' "$work/err" || fail "N5 $flag: thiếu thông báo: $(cat "$work/err")"
done
touch "$work/ps-fail"; rc_of rm busy --repo "$proj"
[[ $rc == 3 && -e $wtd/busy/.git ]] || fail "N5 ps lỗi: rc=$rc"
rm -f "$work/ps-fail"
# (b) path của hàng BUSY qua symlink / tương đối / không phân giải được -> vẫn từ chối (so path đã chuẩn hoá, fail closed)
ln -s "$wtd/busy" "$work/busy-link"
for p in "$work/busy-link" "$work/busy-link/" "proj-agents/busy" "$work/khong/ton/tai" ""; do
  echo '[{"path":"'"$p"'","busy":true,"waiting":false}]' > "$work/ps.json"
  rc_of rm busy --repo "$proj"
  [[ $rc == 3 && -e $wtd/busy/.git && $(nwin busy) == 1 ]] || fail "N5 path '$p': rc=$rc"
done
# (c) ps lỗi một phần: stderr + [] + rc=0 không được hiểu là "không có gì chạy"
echo '[]' > "$work/ps.json"; touch "$work/ps-stderr"
rc_of rm busy --repo "$proj"
[[ $rc == 3 && -e $wtd/busy/.git && $(nwin busy) == 1 ]] || fail "N5 ps stderr một phần: rc=$rc"
rm -f "$work/ps-stderr"
echo '[{"path":"'"$wtd/busy"'","busy":false,"waiting":false}]' > "$work/ps.json"  # IDLE -> được xoá
rc_of rm busy --repo "$proj"; [[ $rc == 0 && ! -e $wtd/busy ]] || fail "N5 idle: rc=$rc: $(cat "$work/err")"
echo '[]' > "$work/ps.json"
[[ ! -e $work/host-seen ]] || fail "N5: script đưa --host cho agent-cli ps"

# N1: agent ghi file lúc SIGHUP (window bị kill) -> git remove phải từ chối, file còn (showUntrackedFiles=no vẫn đang bật)
rc_of new late --repo "$proj"
touch "$work/hup-late"
for _ in $(seq 50); do grep -qxF "1 -c $wtd/late" "$work/calls" && break; sleep 0.1; done
rc_of rm late --repo "$proj"
[[ $rc == 3 && -e $wtd/late/late.txt ]] || fail "N1 file ghi lúc SIGHUP: rc=$rc, late.txt=$(ls "$wtd/late" 2>&1)"

# rm-ignored: file bị gitignore (.crush/crush.db lịch sử session) -> thiếu --force thì từ chối, liệt kê, giữ nguyên
echo db > "$wtd/m1/.crush/crush.db"
rc_of rm m1 --repo "$proj"
[[ $rc == 3 && -e $wtd/m1/.crush/crush.db ]] || fail "rm-ignored: rc=$rc, crush.db mất: $(cat "$work/err")"
grep -qxF '  .crush/' "$work/err" || fail "rm-ignored: không liệt kê .crush/: $(cat "$work/err")"
grep -q -- '--force' "$work/err" || fail "rm-ignored: không gợi ý --force: $(cat "$work/err")"
rc_of ls --force --repo "$proj"; [[ $rc == 2 ]] || fail "--force ngoài rm: rc=$rc"

# rm-clean (--force): worktree + window mất, branch còn
rc_of rm m1 --repo "$proj" --force
[[ $rc == 0 ]] || fail "rm-clean: rc=$rc: $(cat "$work/err")"
grep -q 'bị gitignore' "$work/err" || fail "rm-clean: thiếu cảnh báo file gitignore (.crush): $(cat "$work/err")"
[[ ! -e $wtd/m1 ]] || fail "rm-clean: worktree còn"
git -C "$proj" show-ref --verify --quiet refs/heads/agent/m1 || fail "rm-clean: branch mất"
grep -qxF 'branch agent/m1 kept' "$work/out" || fail "rm-clean: thiếu dòng kept: $(cat "$work/out")"
[[ $(nwin m1) == 0 ]] || fail "rm-clean: window còn"

# N4: git data của một worktree mất -> ls ghi broken và vẫn liệt kê các worktree khác
rc_of new gone --repo "$proj"
rm -rf "$(git -C "$wtd/gone" rev-parse --absolute-git-dir)"
rc_of ls --repo "$proj"
[[ $rc == 0 ]] || fail "N4 ls: rc=$rc: $(cat "$work/err")"
grep -qxF 'gone  (unknown)  broken' "$work/out" || fail "N4 thiếu dòng broken: $(cat "$work/out")"
grep -qxF 'fix1  agent/fix1  dirty' "$work/out" || fail "N4 ls bỏ cả danh sách: $(cat "$work/out")"

# branch có sẵn mà chưa có worktree -> gắn lại, không tạo branch mới
b0=$(nbr)
rc_of new m1 --repo "$proj"
[[ $rc == 0 && -e $wtd/m1/.git && $(nbr) == "$b0" ]] || fail "new trên branch có sẵn: rc=$rc"

echo PASS
