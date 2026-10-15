#!/usr/bin/env bash
# Kiểm THẬT tool claude_code / codex: agent-cli (LLM chính giả có kịch bản) giao
# việc cho Claude Code / Codex THẬT đã đăng nhập trên máy. Tốn hạn mức thuê bao
# (khoảng 10 lượt gọi). Mọi thứ nằm trong thư mục tạm; không đụng repo hay cấu
# hình agent-cli thật.
#
# Dùng: scripts/test-agent-cli-delegation.sh [ca...]   (ca: claude codex sandbox registry)
#   AGENT_CLI_BIN=...   dùng binary có sẵn thay vì build
#   MODES="local server" chế độ chạy (mặc định server, như launcher)
#   AGENT_CLI_CODEX_BIN=... khi codex không nằm trong PATH (vd app ChatGPT desktop)
#   Mức quyền CLI con mặc định an toàn: acceptEdits / workspace-write; đặt
#   AGENT_CLI_CLAUDE_PERMISSION_MODE, AGENT_CLI_CODEX_SANDBOX để thử mức khác.
set -uo pipefail
here="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cases=" ${*:-claude codex sandbox registry} "
for c in $cases; do
  [[ " claude codex sandbox registry " == *" $c "* ]] || { echo "ca không có: $c (claude codex sandbox registry)"; exit 2; }
done
t="$(mktemp -d "${TMPDIR:-/tmp}/aclideleg.XXXXXX")"
# Socket ở thư mục ngắn riêng: sun_path tối đa 107 byte, TMPDIR dài là connect lỗi EINVAL.
sock_dir="$(mktemp -d /tmp/aclisock.XXXXXX)"
bin="${AGENT_CLI_BIN:-$t/agent-cli}"
fake_pid=
# Dừng server agent-cli do lần chạy này dựng. Không dùng `pgrep -f "$bin server"`: -f khớp REGEX trên cả
# dòng lệnh, nên ký tự đặc biệt trong đường dẫn (TMPDIR) khớp sai và tiến trình lạ có chứa chuỗi đó cũng
# bị bắt. Thay vào đó lọc theo TÊN tiến trình chính xác (-x), rồi chỉ giết tiến trình có exe đúng là
# $bin và environ mang CRUSH_GLOBAL_DATA nằm trong thư mục tạm của lần chạy này (so chuỗi, không regex).
stop_servers() {
  local name want p exe kv
  name="$(basename -- "$bin")"
  name="${name:0:15}" # tên tiến trình (comm) của kernel bị cắt ở 15 ký tự
  want="$(readlink -f -- "$bin" 2>/dev/null)" || return 0
  [[ -n "$want" ]] || return 0
  for p in $(pgrep -x -- "$name" 2>/dev/null); do
    exe="$(readlink -- "/proc/$p/exe" 2>/dev/null)" || continue
    [[ "$exe" == "$want" ]] || continue
    while IFS= read -r -d '' kv; do
      if [[ "$kv" == "CRUSH_GLOBAL_DATA=$t/"* ]]; then
        kill "$p" 2>/dev/null
        break
      fi
    done 2>/dev/null <"/proc/$p/environ"
  done
}
cleanup() {
  [[ -n "$fake_pid" ]] && kill "$fake_pid" 2>/dev/null
  stop_servers
  rm -rf "$sock_dir"
  [[ -n "${KEEP_TMP:-}" ]] && echo "kept $t" || rm -rf "$t"
}
trap cleanup EXIT

if [[ -z "${AGENT_CLI_BIN:-}" ]]; then
  (cd "$here" && GOMAXPROCS=2 go build -p 2 -o "$bin" .) || { echo "build failed"; exit 1; }
fi
codex_bin="${AGENT_CLI_CODEX_BIN:-$(command -v codex || true)}"
command -v claude >/dev/null || { echo "thiếu claude trong PATH"; exit 1; }

# --- LLM chính giả ------------------------------------------------------------
#   tin user "@<tool> {json}" -> một tool call <tool> với arguments = json
#   tin cuối là kết quả tool   -> "TOOLDONE <<<kết quả>>>"; còn lại -> "ECHO"
# Ghi mỗi request stream một dòng "<tool,...>|<đầu tin user>" để kiểm đăng ký tool.
cat >"$t/fake.py" <<'PY'
import http.server, json, sys
class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers.get("content-length", 0))) or b"{}")
        msgs = body.get("messages", [])
        users = [m for m in msgs if m.get("role") == "user"]
        last = users[-1]["content"] if users else ""
        if isinstance(last, list): last = "".join(p.get("text", "") for p in last)
        tools = ",".join(sorted(t.get("function", {}).get("name", "") for t in body.get("tools", [])))
        usage = {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
        if not body.get("stream"):
            self.send_response(200); self.send_header("content-type", "application/json"); self.end_headers()
            self.wfile.write(json.dumps({"id": "1", "object": "chat.completion", "model": "m", "choices": [{"index": 0, "message": {"role": "assistant", "content": "Title"}, "finish_reason": "stop"}], "usage": usage}).encode()); return
        open(sys.argv[2], "a").write(tools + "|" + last[:60].replace("\n", " ") + "\n")
        self.send_response(200); self.send_header("content-type", "text/event-stream"); self.end_headers()
        c = lambda **k: {"id": "1", "object": "chat.completion.chunk", "model": "m", **k}
        ev = lambda d: self.wfile.write(("data: " + json.dumps(d) + "\n\n").encode())
        if msgs and msgs[-1].get("role") == "tool":
            out = msgs[-1].get("content", "")
            if isinstance(out, list): out = "".join(p.get("text", "") for p in out)
            ev(c(choices=[{"index": 0, "delta": {"role": "assistant", "content": "TOOLDONE <<<" + out + ">>>"}, "finish_reason": None}]))
            ev(c(choices=[{"index": 0, "delta": {}, "finish_reason": "stop"}]))
        elif last.startswith("@"):
            name, _, args = last[1:].partition(" ")
            call = {"index": 0, "id": "call_1", "type": "function", "function": {"name": name, "arguments": args.strip()}}
            ev(c(choices=[{"index": 0, "delta": {"role": "assistant", "tool_calls": [call]}, "finish_reason": None}]))
            ev(c(choices=[{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]))
        else:
            ev(c(choices=[{"index": 0, "delta": {"role": "assistant", "content": "ECHO"}, "finish_reason": None}]))
            ev(c(choices=[{"index": 0, "delta": {}, "finish_reason": "stop"}]))
        ev(c(choices=[], usage=usage)); self.wfile.write(b"data: [DONE]\n\n")
srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), H)
open(sys.argv[1], "w").write(str(srv.server_address[1])); srv.serve_forever()
PY
python3 "$t/fake.py" "$t/port" "$t/requests.log" >/dev/null 2>&1 &
fake_pid=$!
for _ in $(seq 50); do [[ -s "$t/port" ]] && break; sleep 0.1; done
[[ -s "$t/port" ]] || { echo "fake LLM không chạy"; exit 1; }

fail=0
check() { if [[ "$2" -eq 0 ]]; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }
jtext() { python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("text") or "")' "$1" 2>/dev/null; }

suite() { # suite <local|server>
  local mode=$1 d="$t/$1" cs=0
  [[ "$mode" == server ]] && cs=1
  mkdir -p "$d"/{proj,cfg,data,cache,run} && chmod 700 "$d/run"
  cat >"$d/proj/crush.json" <<JSON
{"providers":{"fake":{"type":"openai-compat","base_url":"http://127.0.0.1:$(cat "$t/port")/v1","api_key":"x","models":[{"id":"m","name":"m","context_window":100000,"default_max_tokens":1000}]}},
 "models":{"large":{"provider":"fake","model":"m"},"small":{"provider":"fake","model":"m"}},
 "options":{"disable_provider_auto_update":true,"disable_metrics":true}}
JSON
  git -C "$d/proj" init -q
  # Môi trường như terminal mới: không mang biến CLAUDE* của phiên đang chạy script.
  local base=(env -i HOME="$HOME" PATH="$PATH" USER="${USER:-}" LOGNAME="${LOGNAME:-${USER:-}}" LANG="${LANG:-C.UTF-8}"
    TERM=xterm-256color SHELL=/bin/bash XDG_RUNTIME_DIR="$d/run"
    CRUSH_GLOBAL_CONFIG="$d/cfg" CRUSH_GLOBAL_DATA="$d/data" CRUSH_CACHE_DIR="$d/cache"
    CRUSH_DISABLE_METRICS=1 CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 CRUSH_DISABLE_UPDATE_CHECK=1 CRUSH_CLIENT_SERVER=$cs
    AGENT_CLI_CODEX_BIN="$codex_bin"
    AGENT_CLI_CLAUDE_PERMISSION_MODE="${AGENT_CLI_CLAUDE_PERMISSION_MODE:-acceptEdits}"
    AGENT_CLI_CODEX_SANDBOX="${AGENT_CLI_CODEX_SANDBOX:-workspace-write}")
  # run <tên> <prompt> [VAR=giá trị...]. Chế độ server: server giữ môi trường của
  # client đã dựng nó, nên ca đổi biến phải dùng server riêng (SOCK=<tên>).
  run() {
    local name=$1 prompt=$2; shift 2
    local host=()
    [[ $cs == 1 ]] && host=(--host "unix://$sock_dir/$mode-${SOCK:-main}.sock")
    # Trần 10 phút mỗi lượt: CLI con kẹt (hết hạn mức, chờ mạng) không treo cả script.
    (cd "$d/proj" && timeout 600 "${base[@]}" "$@" "$bin" "${host[@]}" run --json --quiet "$prompt") >"$d/$name.json" 2>"$d/$name.err"
    echo $? >"$d/$name.rc"
  }
  sid() { jtext "$d/$1.json" | grep -oE "$2 session: [^ ]+" | head -1 | cut -d' ' -f3; }
  local ask='Create a file named %s in the current directory containing exactly: %s. Create no other file. Then reply with the single word DONE.'
  local recall='What is the exact name of the file you created earlier in this conversation? Reply with the file name only.'

  if [[ "$cases" == *" claude "* ]]; then
    run c1 "@claude_code {\"prompt\":\"$(printf "$ask" hello_claude.txt 'hello from claude')\"}"
    check "$mode claude: tạo đúng tệp" "$([[ "$(cat "$d/c1.rc")" == 0 && "$(cat "$d/proj/hello_claude.txt" 2>/dev/null)" == "hello from claude"* ]]; echo $?)"
    local s; s=$(sid c1 claude_code)
    run c2 "@claude_code {\"prompt\":\"$recall\",\"resume\":\"$s\"}"
    check "$mode claude: resume nhớ hội thoại ($s)" "$([[ -n "$s" ]] && jtext "$d/c2.json" | grep -q hello_claude.txt; echo $?)"
    run c3 '@claude_code {"prompt":"- reply with the single word DASHOK"}'
    check "$mode claude: prompt bắt đầu bằng '-'" "$(jtext "$d/c3.json" | grep -q DASHOK; echo $?)"
  fi
  if [[ "$cases" == *" codex "* && -n "$codex_bin" ]]; then
    run x1 "@codex {\"prompt\":\"$(printf "$ask" hello_codex.txt 'hello from codex')\"}"
    check "$mode codex: tạo đúng tệp" "$([[ "$(cat "$d/x1.rc")" == 0 && "$(cat "$d/proj/hello_codex.txt" 2>/dev/null)" == "hello from codex"* ]]; echo $?)"
    local x; x=$(sid x1 codex)
    run x2 "@codex {\"prompt\":\"$recall\",\"resume\":\"$x\"}"
    check "$mode codex: resume nhớ hội thoại ($x)" "$([[ -n "$x" ]] && jtext "$d/x2.json" | grep -q hello_codex.txt; echo $?)"
    run x3 '@codex {"prompt":"- reply with the single word DASHOK"}'
    check "$mode codex: prompt bắt đầu bằng '-'" "$(jtext "$d/x3.json" | grep -q DASHOK; echo $?)"
  elif [[ "$cases" == *" codex "* ]]; then
    echo "skip $mode codex: không tìm thấy codex (đặt AGENT_CLI_CODEX_BIN)"
  fi
  if [[ "$cases" == *" sandbox "* ]]; then
    # Mức quyền cấu hình phải có hiệu lực thật: dontAsk / read-only thì không tạo được
    # tệp. (Không dùng plan: Claude Code ghi tệp kế hoạch vào ~/.claude/plans thật.)
    SOCK=s1 run s1 '@claude_code {"prompt":"Create a file named deny_claude.txt containing ok. Reply DONE."}' AGENT_CLI_CLAUDE_PERMISSION_MODE=dontAsk
    check "$mode claude dontAsk: CLI có chạy và không tạo được tệp" "$(jtext "$d/s1.json" | grep -q 'claude_code session:' && [[ ! -e "$d/proj/deny_claude.txt" ]]; echo $?)"
    if [[ -n "$codex_bin" ]]; then
      SOCK=s2 run s2 '@codex {"prompt":"Create a file named ro_codex.txt containing ok. Reply DONE."}' AGENT_CLI_CODEX_SANDBOX=read-only
      check "$mode codex read-only: CLI có chạy và không tạo được tệp" "$(jtext "$d/s2.json" | grep -q 'codex session:' && [[ ! -e "$d/proj/ro_codex.txt" ]]; echo $?)"
    fi
  fi
  if [[ "$cases" == *" registry "* ]]; then
    # PATH chỉ có claude, không đặt AGENT_CLI_CODEX_BIN: tool codex vắng mặt.
    mkdir -p "$d/onlyclaude" && ln -sf "$(command -v claude)" "$d/onlyclaude/claude"
    SOCK=r1 run r1 'hello' AGENT_CLI_CODEX_BIN= PATH="$d/onlyclaude"
    local tl; tl=",$(tail -1 "$t/requests.log" | cut -d'|' -f1),"
    check "$mode đăng ký: có claude_code, không có codex khi thiếu CLI" "$([[ "$tl" == *,claude_code,* && "$tl" != *,codex,* ]]; echo $?)"
    SOCK=r2 run r2 'hello' AGENT_CLI_CODEX_BIN=/nonexistent/codex
    check "$mode BIN trỏ sai: run thất bại, lỗi nêu tên biến" "$([[ "$(cat "$d/r2.rc")" != 0 ]] && cat "$d/r2.json" "$d/r2.err" | grep -q AGENT_CLI_CODEX_BIN; echo $?)"
  fi
}

for mode in ${MODES:-server}; do suite "$mode"; done
if [[ $fail == 0 ]]; then echo PASS; else echo "FAILED (KEEP_TMP=1 để giữ log)"; exit 1; fi
