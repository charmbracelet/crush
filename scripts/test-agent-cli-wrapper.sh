#!/usr/bin/env bash
# Kiểm launcher agent-cli: một server chung cho mỗi thư mục gốc dự án.
# Cài vào thư mục tạm với binary giả; không đụng bản cài thật hay repo này.
set -euo pipefail
here="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
t="$(mktemp -d)"
trap 'rm -rf "$t"' EXIT

# Repo giả để install script không tạo .crush.json trong repo thật.
mkdir -p "$t/fake/scripts" "$t/inst/lib/agent-cli" "$t/home"
cp "$here/scripts/install-agent-cli.sh" "$here/scripts/agent-cli-tmux.sh" "$t/fake/scripts/"
cp "$here"/agent-cli.crushrc.example "$here"/agent-cli.env.example "$t/fake/"
cat > "$t/inst/lib/agent-cli/crush" <<'STUB'
#!/usr/bin/env bash
echo "CS=${CRUSH_CLIENT_SERVER-unset}"
echo "L=${AGENT_CLI_LAUNCHER-unset}"
printf 'ARG=%s\n' "$@"
STUB
chmod 755 "$t/inst/lib/agent-cli/crush"
install1() { HOME="$t/home" AGENT_CLI_INSTALL_DIR="$t/inst" AGENT_CLI_SKIP_BUILD=1 bash "$t/fake/scripts/install-agent-cli.sh" 2>&1; }
msg_first="$(install1)"
wrapper="$t/inst/bin/agent-cli"

for r in repoA repoB; do
  mkdir -p "$t/$r/sub1" "$t/$r/sub2"
  git -C "$t/$r" init -q
done
mkdir -p "$t/plain/x"
ln -s "$t/plain" "$t/plainlink"

fail=0
check() { # check <tên> <điều kiện: 0 = đạt>
  if [[ "$2" == 0 ]]; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi
}
# run <thư mục> <env...> -- <args...>: chạy wrapper, in output của stub.
run() {
  local dir="$1"; shift
  local envs=()
  while [[ "$1" != -- ]]; do envs+=("$1"); shift; done
  shift
  (cd "$dir" && env HOME="$t/home" "${envs[@]}" "$wrapper" "$@")
}
same() { [[ -n "$1" && "$1" == "$2" ]]; } # rỗng == rỗng không được tính là đạt
host_of() { sed -n 's/^ARG=\(unix:.*\)$/\1/p' <<<"$1" | head -1; }
count_host() { grep -cE '^ARG=(--host|-H)' <<<"$1" || true; }
sha8() { printf %s "$1" | sha1sum | cut -c1-8; }

root_a="$(git -C "$t/repoA" rev-parse --show-toplevel)"
# Thư mục socket mong đợi, tính như launcher: /run/user/UID -> XDG_RUNTIME_DIR -> $HOME/.cache/agent-cli.
rt="/run/user/$(id -u)"
[[ -d "$rt" ]] || rt="${XDG_RUNTIME_DIR:-}"
[[ -d "$rt" ]] || rt="$t/home/.cache/agent-cli"
want="unix://$rt/agent-cli-$(sha8 "$root_a").sock"

out_a1="$(run "$t/repoA/sub1" -- run hello)"
out_a2="$(run "$t/repoA/sub2" -- run hello)"
out_b="$(run "$t/repoB/sub1" -- run hello)"
h_a1="$(host_of "$out_a1")"; h_a2="$(host_of "$out_a2")"; h_b="$(host_of "$out_b")"

check "host đúng dạng R9 (sha1 gốc git)" "$([[ "$h_a1" == "$want" ]]; echo $?)"
check "hai thư mục con cùng repo -> cùng host" "$([[ -n "$h_a1" && "$h_a1" == "$h_a2" ]]; echo $?)"
check "hai repo khác nhau -> host khác nhau" "$([[ -n "$h_b" && "$h_a1" != "$h_b" ]]; echo $?)"
check "CRUSH_CLIENT_SERVER mặc định 1" "$(grep -qx 'CS=1' <<<"$out_a1"; echo $?)"
check "đối số gốc giữ nguyên thứ tự sau --host" "$([[ "$(grep '^ARG=' <<<"$out_a1" | tail -2 | tr '\n' ' ')" == "ARG=run ARG=hello " ]]; echo $?)"

u="unix:///tmp/mine.sock"
out="$(run "$t/repoA" -- --host "$u" run x)"
check "--host của user giữ nguyên, không thêm" "$([[ "$(count_host "$out")" == 1 && "$(host_of "$out")" == "$u" ]]; echo $?)"
out="$(run "$t/repoA" -- -H "$u" run x)"
check "-H của user giữ nguyên, không thêm" "$([[ "$(count_host "$out")" == 1 && "$(host_of "$out")" == "$u" ]]; echo $?)"
out="$(run "$t/repoA" -- --host="$u" run x)"
check "--host=... của user giữ nguyên, không thêm" "$([[ "$(grep -c "^ARG=--host=" <<<"$out")" == 1 && "$(count_host "$out")" == 1 ]]; echo $?)"

out="$(run "$t" -- -c "$t/repoA/sub1" run x)"
check "-c <dir> được hiểu" "$(same "$(host_of "$out")" "$h_a1"; echo $?)"
out="$(run "$t" -- --cwd="$t/repoB/sub2" run x)"
check "--cwd=<dir> được hiểu" "$(same "$(host_of "$out")" "$h_b"; echo $?)"
out="$(run "$t" -- --cwd "$t/repoA" run x)"
check "--cwd <dir> được hiểu" "$(same "$(host_of "$out")" "$h_a1"; echo $?)"
out="$(cd "$t/repoB" && env HOME="$t/home" "$wrapper" -c sub1 run x)"
check "-c đường dẫn tương đối" "$(same "$(host_of "$out")" "$h_b"; echo $?)"

h_p="$(host_of "$(run "$t/plain/x" -- run x)")"
h_pl="$(host_of "$(run "$t/plainlink/x" -- run x)")"
check "ngoài git: realpath, symlink cùng host với đích" "$([[ -n "$h_p" && "$h_p" == "$h_pl" && "$h_p" == "unix://$rt/agent-cli-$(sha8 "$t/plain/x").sock" ]]; echo $?)"

out="$(run "$t/repoA" CRUSH_CLIENT_SERVER=0 -- run x)"
check "CRUSH_CLIENT_SERVER=0: giữ 0, không thêm --host" "$(grep -qx 'CS=0' <<<"$out" && [[ "$(count_host "$out")" == 0 ]]; echo $?)"
out="$(run "$t/repoA" -- --version)"
check "--version vẫn chạy (chỉ thêm --host phía trước)" "$(grep -qx 'ARG=--version' <<<"$out"; echo $?)"

# `ps` tự quét mọi server khi không có --host: launcher không được chèn --host cho nó.
out="$(run "$t/repoA" -- ps)"
check "ps -> không thêm --host" "$([[ "$(count_host "$out")" == 0 ]] && grep -qx 'ARG=ps' <<<"$out"; echo $?)"
out="$(run "$t/repoA" -- ps --json)"
check "ps --json -> không thêm --host" "$([[ "$(count_host "$out")" == 0 ]] && grep -qx 'ARG=--json' <<<"$out"; echo $?)"
out="$(run "$t" -- -c "$t/repoA" ps --json)"
check "-c dir ps -> không thêm --host" "$([[ "$(count_host "$out")" == 0 ]]; echo $?)"
out="$(run "$t" -- --cwd "$t/repoA" -D "$t/dd" ps)"
check "cờ có giá trị trước ps (--cwd dir -D dir) -> không thêm --host" "$([[ "$(count_host "$out")" == 0 ]]; echo $?)"
out="$(run "$t/repoA" -- ps -H "$u")"
check "ps -H x giữ host của user" "$([[ "$(count_host "$out")" == 1 && "$(host_of "$out")" == "$u" ]]; echo $?)"
out="$(run "$t/repoA" -- run ps)"
check "run ps (ps là prompt) vẫn thêm --host" "$(same "$(host_of "$out")" "$h_a1"; echo $?)"
out="$(run "$t/repoA" -- -c "$t/repoA" run x)"
check "run vẫn thêm --host" "$(same "$(host_of "$out")" "$h_a1"; echo $?)"
out="$(run "$t/repoA" --)"
check "không đối số, không phải terminal (TUI thẳng) vẫn thêm --host" "$(same "$(host_of "$out")" "$h_a1"; echo $?)"
check "launcher tự khai đường dẫn của nó (AGENT_CLI_LAUNCHER) cho Home" "$(grep -qx "L=$wrapper" <<<"$out"; echo $?)"

# Trên terminal, agent-cli trần (hoặc chỉ -d) là Home: chưa có workspace nên không chọn server.
# `script` cấp pty để stdin/stdout là terminal như khi gõ tay.
run_tty() { # run_tty <thư mục> <args...>
  local dir="$1"; shift
  (cd "$dir" && HOME="$t/home" script -qec "$(printf '%q ' "$wrapper" "$@")" /dev/null </dev/null) | tr -d '\r'
}
if command -v script >/dev/null; then
  out="$(run_tty "$t/repoA")"
  check "terminal, không đối số (Home) -> không thêm --host" "$(grep -qx 'CS=1' <<<"$out" && [[ "$(count_host "$out")" == 0 ]]; echo $?)"
  out="$(run_tty "$t/repoA" -d)"
  check "terminal, chỉ -d (Home) -> không thêm --host" "$(grep -qx 'ARG=-d' <<<"$out" && [[ "$(count_host "$out")" == 0 ]]; echo $?)"
  out="$(run_tty "$t" -c "$t/repoB/sub1")"
  check "terminal, -c dir -> --host của dir" "$(same "$(host_of "$out")" "$h_b"; echo $?)"
  for a in "-s abc" "--session abc" "-C" "--continue" "-y" "--yolo"; do
    # shellcheck disable=SC2086 # tách cờ và giá trị có chủ đích
    out="$(run_tty "$t/repoA" $a)"
    check "terminal, $a -> vẫn thêm --host của thư mục hiện tại" "$(same "$(host_of "$out")" "$h_a1"; echo $?)"
  done
  out="$(run_tty "$t/repoA" run x)"
  check "terminal, run -> vẫn thêm --host" "$(same "$(host_of "$out")" "$h_a1"; echo $?)"
else
  echo "skip ca terminal: thiếu lệnh script"
fi

# Thứ tự thư mục socket: không bao giờ /tmp, ~/.cache/agent-cli mode 0700.
out="$(run "$t/repoA" AGENT_CLI_RUNTIME_DIR=/nonexistent XDG_RUNTIME_DIR= -- run x)"
check "không có /run/user, không XDG -> \$HOME/.cache/agent-cli" "$([[ "$(host_of "$out")" == "unix://$t/home/.cache/agent-cli/agent-cli-$(sha8 "$root_a").sock" ]]; echo $?)"
check "thư mục dự phòng mode 700" "$([[ "$(stat -c %a "$t/home/.cache/agent-cli")" == 700 ]]; echo $?)"
mkdir -p "$t/xdg"
out="$(run "$t/repoA" AGENT_CLI_RUNTIME_DIR=/nonexistent XDG_RUNTIME_DIR="$t/xdg" -- run x)"
check "không có /run/user -> XDG_RUNTIME_DIR" "$([[ "$(host_of "$out")" == "unix://$t/xdg/agent-cli-$(sha8 "$root_a").sock" ]]; echo $?)"
long="$t/$(printf 'd%.0s' {1..100})"; mkdir -p "$long"
# Socket quá dài: KHÔNG chạy tiếp không --host (client sẽ rơi về server mặc định dùng chung, mất cách ly theo dự án).
rc=0; out="$(run "$t/repoA" AGENT_CLI_RUNTIME_DIR="$long" -- run x 2>"$t/err")" || rc=$?
check "socket quá 107 byte -> dừng mã 1, không chạy binary" "$([[ "$rc" == 1 && -z "$out" ]]; echo $?)"
check "socket quá dài -> lỗi nêu độ dài và AGENT_CLI_RUNTIME_DIR" "$(grep -q 'quá dài' "$t/err" && grep -q AGENT_CLI_RUNTIME_DIR "$t/err" && grep -q 'CRUSH_CLIENT_SERVER=0' "$t/err"; echo $?)"
rc=0; out="$(run "$t/repoA" AGENT_CLI_RUNTIME_DIR="$long" CRUSH_CLIENT_SERVER=0 -- run x 2>/dev/null)" || rc=$?
check "socket quá dài nhưng CRUSH_CLIENT_SERVER=0 -> vẫn chạy (không cần socket)" "$([[ "$rc" == 0 ]] && grep -qx 'CS=0' <<<"$out"; echo $?)"
rc=0; out="$(run "$t/repoA" AGENT_CLI_RUNTIME_DIR="$long" -- --host "$u" run x 2>/dev/null)" || rc=$?
check "socket quá dài nhưng user tự truyền --host -> vẫn chạy, giữ host của user" "$([[ "$rc" == 0 && "$(host_of "$out")" == "$u" ]]; echo $?)"
rc=0; out="$(run "$t/repoA" AGENT_CLI_RUNTIME_DIR="$long" -- ps 2>/dev/null)" || rc=$?
check "socket quá dài nhưng lệnh ps (không cần host) -> vẫn chạy" "$([[ "$rc" == 0 ]] && grep -qx 'ARG=ps' <<<"$out"; echo $?)"

# Repo nằm ở đường dẫn rất dài: socket vẫn ngắn vì tên chỉ dùng 8 ký tự sha1.
deep="$t/$(printf 'e%.0s' {1..60})/$(printf 'f%.0s' {1..60})/$(printf 'g%.0s' {1..60})"
mkdir -p "$deep" && git -C "$deep" init -q
out="$(run "$deep" -- run x)"
h_deep="$(host_of "$out")"
check "repo đường dẫn >150 ký tự" "$([[ ${#deep} -gt 150 ]]; echo $?)"
check "repo đường dẫn dài -> socket vẫn có và <=107 byte" "$([[ -n "$h_deep" && $((${#h_deep} - 7)) -le 107 ]]; echo $?)"

# Thiếu sha1sum/shasum -> dừng với lỗi rõ (cùng lý do với socket quá dài), trừ khi tắt client/server.
mkdir -p "$t/nohash"
for c in bash env git mkdir chmod; do ln -s "$(command -v $c)" "$t/nohash/$c"; done
rc=0; out="$(run "$t/repoA" PATH="$t/nohash" -- run x 2>"$t/err")" || rc=$?
check "thiếu công cụ hash -> dừng mã 1, không chạy binary, lỗi nêu sha1sum" "$([[ "$rc" == 1 && -z "$out" ]] && grep -q sha1sum "$t/err" && grep -q 'CRUSH_CLIENT_SERVER=0' "$t/err"; echo $?)"
rc=0; out="$(run "$t/repoA" PATH="$t/nohash" CRUSH_CLIENT_SERVER=0 -- run x 2>/dev/null)" || rc=$?
check "thiếu công cụ hash nhưng CRUSH_CLIENT_SERVER=0 -> vẫn chạy" "$([[ "$rc" == 0 ]] && grep -qx 'CS=0' <<<"$out"; echo $?)"

# CRUSH_CLIENT_SERVER hiểu đúng như strconv.ParseBool của Go: bật (1 t T TRUE true True) thì thêm --host,
# tắt (0 f F FALSE false False) thì không, giá trị lạ Go coi là tắt nên launcher cũng không thêm --host và cảnh báo.
for v in 1 t T TRUE true True; do
  out="$(run "$t/repoA" CRUSH_CLIENT_SERVER=$v -- run x 2>/dev/null)"
  check "CRUSH_CLIENT_SERVER=$v (bật theo ParseBool) -> thêm --host, giữ nguyên giá trị" "$(same "$(host_of "$out")" "$h_a1" && grep -qx "CS=$v" <<<"$out"; echo $?)"
done
for v in 0 f F FALSE false False; do
  out="$(run "$t/repoA" CRUSH_CLIENT_SERVER=$v -- run x 2>"$t/err")"
  check "CRUSH_CLIENT_SERVER=$v (tắt theo ParseBool) -> không --host, không cảnh báo" "$([[ "$(count_host "$out")" == 0 ]] && grep -qx "CS=$v" <<<"$out" && [[ ! -s "$t/err" ]]; echo $?)"
done
for v in yes no on off 2 tRuE y; do
  out="$(run "$t/repoA" CRUSH_CLIENT_SERVER=$v -- run x 2>"$t/err")"
  check "CRUSH_CLIENT_SERVER=$v (Go không hiểu, coi là tắt) -> không --host, cảnh báo stderr" "$([[ "$(count_host "$out")" == 0 ]] && grep -q 'không hợp lệ' "$t/err" && grep -qx 'ARG=run' <<<"$out"; echo $?)"
done

# AGENT_CLI_SKIP_BUILD chỉ bỏ build khi đúng bằng 1. go giả ghi dấu mỗi lần được gọi.
mkdir -p "$t/gobin" "$t/inst2/lib/agent-cli"
# go giả giữ đúng hợp đồng của go build: sinh tệp -o (installer build ra tệp tạm rồi mv).
printf '#!/bin/sh\ntouch "%s/go-called"\necho "CGO_ENABLED=$CGO_ENABLED GOEXPERIMENT=$GOEXPERIMENT" > "%s/go-env"\nwhile [ $# -gt 0 ]; do [ "$1" = -o ] && cp "%s" "$2"; shift; done\n' "$t" "$t" "$t/inst/lib/agent-cli/crush" > "$t/gobin/go"; chmod 755 "$t/gobin/go"
cp "$t/inst/lib/agent-cli/crush" "$t/inst2/lib/agent-cli/crush"
inst2() { HOME="$t/home" PATH="$t/gobin:$PATH" AGENT_CLI_INSTALL_DIR="$t/inst2" AGENT_CLI_SKIP_BUILD="$1" bash "$t/fake/scripts/install-agent-cli.sh" 2>&1; }
rm -f "$t/go-called" "$t/go-env"; msg="$(CGO_ENABLED=1 GOEXPERIMENT=nogreenteagc inst2 0)"
check "AGENT_CLI_SKIP_BUILD=0 vẫn build" "$([[ -e "$t/go-called" ]] && ! grep -q 'Bỏ qua go build' <<<"$msg"; echo $?)"
check "build như bản chính thức: CGO_ENABLED=0 GOEXPERIMENT=greenteagc (đè biến của user)" "$([[ "$(cat "$t/go-env")" == "CGO_ENABLED=0 GOEXPERIMENT=greenteagc" ]]; echo $?)"
rm -f "$t/go-called"; msg="$(inst2 1)"
check "AGENT_CLI_SKIP_BUILD=1 bỏ build và báo" "$([[ ! -e "$t/go-called" ]] && grep -q 'Bỏ qua go build' <<<"$msg"; echo $?)"

# Cài đặt cấu hình chung: crushrc + env mẫu, không .crush.json, không ghi đè lần hai.
cfg="$t/home/.config/agent-cli"
check "cài lần đầu tạo crushrc từ mẫu" "$(cmp -s "$here/agent-cli.crushrc.example" "$cfg/crushrc"; echo $?)"
check "cài lần đầu tạo env từ mẫu, mode 600" "$(cmp -s "$here/agent-cli.env.example" "$cfg/env" && [[ "$(stat -c %a "$cfg/env")" == 600 ]]; echo $?)"
check "cài đặt không tạo .crush.json" "$([[ ! -e "$t/fake/.crush.json" && ! -e "$cfg/crush.json" ]]; echo $?)"
check "không có crush.json cũ -> không in hướng dẫn chuyển đổi" "$(! grep -q 'crush.json cũ' <<<"$msg_first"; echo $?)"
echo '# của user' >> "$cfg/crushrc"; echo 'AGENT_CLI_MODEL=mine' >> "$cfg/env"
msg_second="$(install1)"
check "cài lần hai không ghi đè crushrc" "$(tail -1 "$cfg/crushrc" | grep -qx '# của user'; echo $?)"
check "cài lần hai không ghi đè env" "$(tail -1 "$cfg/env" | grep -qx 'AGENT_CLI_MODEL=mine'; echo $?)"
check "cài lần đầu (tệp = mẫu) -> không in gợi ý diff" "$(! grep -q 'diff -u' <<<"$msg_first"; echo $?)"
# repo giả là $t/fake nên mẫu trong gợi ý nằm ở đó.
check "cài lần hai, crushrc khác mẫu -> gợi ý diff -u crushrc mẫu" "$(grep -F 'diff -u' <<<"$msg_second" | grep -F "$cfg/crushrc" | grep -qF "$t/fake/agent-cli.crushrc.example"; echo $?)"
check "cài lần hai, env khác mẫu -> gợi ý diff -u env mẫu" "$(grep -F 'diff -u' <<<"$msg_second" | grep -F "$cfg/env" | grep -qF "$t/fake/agent-cli.env.example"; echo $?)"
check "gợi ý diff chỉ là gợi ý: hai tệp của user vẫn nguyên" "$(tail -1 "$cfg/crushrc" | grep -qx '# của user' && tail -1 "$cfg/env" | grep -qx 'AGENT_CLI_MODEL=mine'; echo $?)"
echo '{}' > "$cfg/crush.json"
msg_old="$(install1)"
check "có crush.json cũ -> giữ nguyên và in hướng dẫn" "$(grep -q 'crush.json cũ' <<<"$msg_old" && [[ "$(cat "$cfg/crush.json")" == '{}' ]]; echo $?)"
rm -f "$cfg/crush.json"

# Thư mục cấu hình chứa env: tạo mới -> 700; quyền hẹp hơn do user đặt giữ nguyên; quá rộng -> siết.
check "installer tạo thư mục cấu hình mode 700" "$([[ "$(stat -c %a "$cfg")" == 700 ]]; echo $?)"
chmod 500 "$cfg"; install1 >/dev/null; mode_narrow="$(stat -c %a "$cfg")"; chmod 700 "$cfg"
check "thư mục cấu hình hẹp hơn 700 (500) được giữ nguyên" "$([[ "$mode_narrow" == 500 ]]; echo $?)"
chmod 755 "$cfg"; install1 >/dev/null
check "thư mục cấu hình mở rộng (755) được siết về 700" "$([[ "$(stat -c %a "$cfg")" == 700 ]]; echo $?)"

# crush.json đang khai provider thì không tạo crushrc/env (crushrc sẽ ghi đè nó); chỉ để bản .example.
install_in() { HOME="$1" AGENT_CLI_INSTALL_DIR="$t/inst" AGENT_CLI_SKIP_BUILD=1 bash "$t/fake/scripts/install-agent-cli.sh" 2>&1; }
mkdir -p "$t/hp/.config/agent-cli"; cj='{"providers":{"mine":{"type":"openai-compat"}},"models":{"large":{"provider":"mine","model":"x"}}}'
printf '%s' "$cj" > "$t/hp/.config/agent-cli/crush.json"
msg_p="$(install_in "$t/hp")"; c="$t/hp/.config/agent-cli"
check "crush.json có provider -> không tạo crushrc/env" "$([[ ! -e "$c/crushrc" && ! -e "$c/env" ]]; echo $?)"
check "crush.json có provider -> có crushrc.example, env.example" "$([[ -f "$c/crushrc.example" && -f "$c/env.example" ]]; echo $?)"
check "crush.json có provider -> in hướng dẫn, crush.json không đổi" "$(grep -q 'KHÔNG tạo crushrc' <<<"$msg_p" && [[ "$(cat "$c/crush.json")" == "$cj" ]]; echo $?)"
echo '# sửa' >> "$c/crushrc.example"; msg_re="$(install_in "$t/hp")"
check "chạy lại không ghi đè crushrc.example" "$(tail -1 "$c/crushrc.example" | grep -qx '# sửa'; echo $?)"
check "cài lại -> sao lưu bản cũ và in lệnh quay lại" "$(ls -d "$t/inst/lib/agent-cli"/backup-*/crush >/dev/null 2>&1 && grep -q 'Quay lại' <<<"$msg_re"; echo $?)"
mkdir -p "$t/hs/.config/agent-cli" "$t/store"; printf '%s' "$cj" > "$t/store/real.json"
ln -s "$t/store/real.json" "$t/hs/.config/agent-cli/crush.json"
install_in "$t/hs" >/dev/null
check "crush.json là symlink có provider -> theo symlink, không tạo crushrc, đích không đổi" "$([[ ! -e "$t/hs/.config/agent-cli/crushrc" && -L "$t/hs/.config/agent-cli/crush.json" && "$(cat "$t/store/real.json")" == "$cj" ]]; echo $?)"
mkdir -p "$t/hn/.config/agent-cli"; printf '%s' '{"options":{"tui":{"active_theme":"x"}}}' > "$t/hn/.config/agent-cli/crush.json"
install_in "$t/hn" >/dev/null
check "crush.json không có provider -> tạo crushrc + env, giữ crush.json" "$([[ -f "$t/hn/.config/agent-cli/crushrc" && -f "$t/hn/.config/agent-cli/env" && -e "$t/hn/.config/agent-cli/crush.json" ]]; echo $?)"
mkdir -p "$t/hb/.config/agent-cli"; printf '%s' '{hỏng' > "$t/hb/.config/agent-cli/crush.json"
install_in "$t/hb" >/dev/null
check "crush.json hỏng -> coi như có provider, không tạo crushrc" "$([[ ! -e "$t/hb/.config/agent-cli/crushrc" ]]; echo $?)"
# Thiếu jq -> không xác định được provider -> an toàn: không tạo crushrc.
mkdir -p "$t/nojq" "$t/hj/.config/agent-cli"
for c in bash env dirname mkdir install find chmod ln cat cp mv rm ls tail date sha1sum cmp; do ln -sf "$(command -v $c)" "$t/nojq/$c"; done
printf '%s' "$cj" > "$t/hj/.config/agent-cli/crush.json"
msg_j="$(PATH="$t/nojq" install_in "$t/hj")"
check "thiếu jq (PATH không có jq) -> không tạo crushrc, in hướng dẫn" "$([[ ! -e "$t/hj/.config/agent-cli/crushrc" && -f "$t/hj/.config/agent-cli/crushrc.example" ]] && grep -q 'KHÔNG tạo crushrc' <<<"$msg_j"; echo $?)"

# agent-cli-tmux được chép (install -m 755), không còn symlink vào checkout.
tmux_inst="$t/inst/bin/agent-cli-tmux"
check "agent-cli-tmux là bản chép (không symlink), mode 755, giống tệp trong repo" "$([[ -f "$tmux_inst" && ! -L "$tmux_inst" && "$(stat -c %a "$tmux_inst")" == 755 ]] && cmp -s "$here/scripts/agent-cli-tmux.sh" "$tmux_inst"; echo $?)"
mv "$t/fake/scripts/agent-cli-tmux.sh" "$t/fake/scripts/agent-cli-tmux.sh.away"
out="$(bash "$tmux_inst" --help 2>&1)" || true
check "agent-cli-tmux đã cài vẫn chạy khi tệp trong repo biến mất" "$(grep -q 'Dùng:' <<<"$out"; echo $?)"
mv "$t/fake/scripts/agent-cli-tmux.sh.away" "$t/fake/scripts/agent-cli-tmux.sh"
# Bản cài cũ là symlink vào repo: cài lại phải thay chính symlink, không ghi xuyên vào tệp trong repo.
rm -f "$tmux_inst"; ln -s "$t/fake/scripts/agent-cli-tmux.sh" "$tmux_inst"
before_sum="$(cksum < "$t/fake/scripts/agent-cli-tmux.sh")"
install1 >/dev/null
check "cài lại thay symlink agent-cli-tmux của bản cài cũ bằng bản chép" "$([[ -f "$tmux_inst" && ! -L "$tmux_inst" ]]; echo $?)"
check "thay symlink không ghi xuyên vào tệp trong repo" "$([[ "$(cksum < "$t/fake/scripts/agent-cli-tmux.sh")" == "$before_sum" ]]; echo $?)"

# AGENT_CLI_INSTALL_DIR tương đối: wrapper phải ghi đường dẫn binary tuyệt đối.
mkdir -p "$t/relcwd/rel-inst/lib/agent-cli"
cp "$t/inst/lib/agent-cli/crush" "$t/relcwd/rel-inst/lib/agent-cli/crush"
(cd "$t/relcwd" && HOME="$t/home" AGENT_CLI_INSTALL_DIR=rel-inst AGENT_CLI_SKIP_BUILD=1 bash "$t/fake/scripts/install-agent-cli.sh" >/dev/null 2>&1)
check "AGENT_CLI_INSTALL_DIR tương đối -> wrapper ghi đường dẫn binary tuyệt đối" "$(grep -Fqx "exec \"$t/relcwd/rel-inst/lib/agent-cli/crush\" \"\$@\"" "$t/relcwd/rel-inst/bin/agent-cli"; echo $?)"
out="$(cd / && HOME="$t/home" "$t/relcwd/rel-inst/bin/agent-cli" run x 2>&1)" || true
check "wrapper cài bằng đường dẫn tương đối chạy được từ thư mục khác" "$(grep -qx 'ARG=run' <<<"$out"; echo $?)"

# Thư mục cấu hình là symlink: siết thư mục ĐÍCH (của user) và nói rõ; symlink hỏng thì dừng rõ ràng.
mkdir -p "$t/hl/.config" "$t/hl-real"; chmod 755 "$t/hl-real"
ln -s "$t/hl-real" "$t/hl/.config/agent-cli"
msg_l="$(install_in "$t/hl")"
check "cấu hình là symlink -> siết thư mục đích về 700, symlink còn nguyên, nói rõ" "$([[ "$(stat -c %a "$t/hl-real")" == 700 && -L "$t/hl/.config/agent-cli" ]] && grep -q 'symlink' <<<"$msg_l" && grep -qF "$t/hl-real" <<<"$msg_l"; echo $?)"
check "cấu hình là symlink -> crushrc và env tạo trong thư mục đích" "$([[ -f "$t/hl-real/crushrc" && -f "$t/hl-real/env" ]]; echo $?)"
mkdir -p "$t/hd/.config"; ln -s "$t/hd-missing" "$t/hd/.config/agent-cli"
rc=0; msg_d="$(install_in "$t/hd")" || rc=$?
check "cấu hình là symlink hỏng -> dừng mã khác 0, nói rõ, không tạo đích" "$([[ "$rc" != 0 && ! -e "$t/hd-missing" ]] && grep -q 'symlink hỏng' <<<"$msg_d"; echo $?)"

if [[ $fail == 0 ]]; then echo PASS; else echo "FAILED"; exit 1; fi
