#!/usr/bin/env bash
# E2E cho `agent-cli run --json` với server OpenAI-compat giả, cả đường local
# lẫn client/server (CRUSH_CLIENT_SERVER=1). Chứng minh mỗi tiến trình trả
# đúng session ID của nó. Mọi thứ nằm trong thư mục tạm; không đụng repo.
# Dùng: scripts/test-agent-cli-run-json.sh   (AGENT_CLI_BIN=... để bỏ qua build)
set -uo pipefail
here="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
t="$(mktemp -d /tmp/aclijson.XXXXXX)"
fake_pid=""
cleanup() {
  [ -n "$fake_pid" ] && kill "$fake_pid" 2>/dev/null
  pkill -f -- "$t/s.sock" 2>/dev/null
  [ -n "${KEEP_TMP:-}" ] && echo "kept $t" || rm -rf "$t"
}
trap cleanup EXIT

fail=0
check() { # check <tên> <điều kiện: 0=đúng>
  if [ "$2" -eq 0 ]; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi
}
# tokens <file>: "prompt/completion" đọc được, để in khi kiểm token đỏ.
tokens() { echo "$(jget "$1" prompt_tokens)/$(jget "$1" completion_tokens)"; }
# jget <file> <key>: in giá trị của key; lỗi nếu stdout không đúng 1 object.
jget() {
  python3 - "$1" "$2" <<'PY'
import json, sys
dec = json.JSONDecoder()
raw = open(sys.argv[1]).read()
obj, end = dec.raw_decode(raw)
if raw[end:].strip():
    sys.exit("stdout holds more than one JSON object")
print(obj.get(sys.argv[2], ""))
PY
}

# replies <dir> <session>: mỗi tin assistant một dòng "<finish reason>|<text>".
replies() {
  (cd "$1" && "$bin" session show "$2" --json 2>/dev/null) | python3 -c '
import json, sys
for m in json.load(sys.stdin)["messages"]:
    if m["role"] != "assistant": continue
    pick = lambda k, ty: "".join(p.get(k, "") for p in m["parts"] if p["type"] == ty)
    print(pick("reason", "finish") + "|" + pick("text", "text"))'
}
# last_reply <dir> <session>: dòng của tin assistant cuối ("|" khi chưa có).
last_reply() { local r; r="$(replies "$@" | tail -1)"; echo "${r:-|}"; }

bin="${AGENT_CLI_BIN:-$t/agent-cli}"
if [ -z "${AGENT_CLI_BIN:-}" ]; then
  (cd "$here" && GOMAXPROCS=2 go build -p 2 -o "$bin" .) || { echo "build failed"; exit 1; }
fi

# --- server giả -------------------------------------------------------------
cat > "$t/fake.py" <<'PY'
import http.server, json, sys, time
class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers.get('content-length', 0))) or b'{}')
        users = [m for m in body.get('messages', []) if m.get('role') == 'user']
        last = users[-1]['content']
        if isinstance(last, list): last = ''.join(p.get('text', '') for p in last)
        tools = ",".join(t.get('function', {}).get('name', '') for t in body.get('tools', []))
        if body.get('stream'):
            open(sys.argv[2], 'a').write(last[:30].replace("\n", " ") + "|" + tools + "\n")
        if 'SLOW' in last and body.get('stream'): time.sleep(30)
        if 'FAIL' in last:
            self.send_response(400); self.send_header('content-type', 'application/json'); self.end_headers()
            self.wfile.write(json.dumps({"error": {"message": "boom", "type": "invalid_request_error"}}).encode()); return
        msgs = body.get('messages', [])
        text = "ECHO(%d): %s" % (len(users), last[:40])
        if msgs and msgs[-1].get('role') == 'tool': text = "TOOLDONE"
        usage = {"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18}
        # Sinh tiêu đề chạy tách rời và cộng dồn token vào session mới; usage 0
        # để kết quả không phụ thuộc thứ tự commit.
        if last.startswith("Generate a concise title"):
            usage = {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
        c = lambda d, **k: {"id": "1", "object": "chat.completion.chunk", "model": "m", **k}
        if body.get('stream'):
            self.send_response(200); self.send_header('content-type', 'text/event-stream'); self.end_headers()
            ev = lambda d: self.wfile.write(("data: " + json.dumps(d) + "\n\n").encode())
            if 'TOOLCALL' in last and msgs[-1].get('role') != 'tool':
                call = {"index": 0, "id": "call_1", "type": "function", "function": {"name": "bash", "arguments": json.dumps({"command": "touch e2e-perm-probe", "description": "probe"})}}
                ev(c(0, choices=[{"index": 0, "delta": {"role": "assistant", "tool_calls": [call]}, "finish_reason": None}]))
                ev(c(0, choices=[{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]))
            else:
                ev(c(0, choices=[{"index": 0, "delta": {"role": "assistant", "content": text}, "finish_reason": None}]))
                ev(c(0, choices=[{"index": 0, "delta": {}, "finish_reason": "stop"}]))
            ev(c(0, choices=[], usage=usage))
            self.wfile.write(b"data: [DONE]\n\n")
        else:
            self.send_response(200); self.send_header('content-type', 'application/json'); self.end_headers()
            self.wfile.write(json.dumps({"id": "1", "object": "chat.completion", "model": "m", "choices": [{"index": 0, "message": {"role": "assistant", "content": "Title"}, "finish_reason": "stop"}], "usage": usage}).encode())
srv = http.server.ThreadingHTTPServer(('127.0.0.1', 0), H)
open(sys.argv[1], 'w').write(str(srv.server_address[1]))
srv.serve_forever()
PY
python3 "$t/fake.py" "$t/port" "$t/requests.log" >/dev/null 2>&1 &
fake_pid=$!
for _ in $(seq 50); do [ -s "$t/port" ] && break; sleep 0.1; done
[ -s "$t/port" ] || { echo "fake server did not start"; exit 1; }

# --- môi trường cô lập ------------------------------------------------------
mkdir -p "$t/home" "$t/proj"
export HOME="$t/home" XDG_CONFIG_HOME="$t/home/c" XDG_DATA_HOME="$t/home/d" XDG_CACHE_HOME="$t/home/x"
export XDG_RUNTIME_DIR="$t/run"; mkdir -p "$t/run" && chmod 700 "$t/run"
export CRUSH_DISABLE_METRICS=1 DO_NOT_TRACK=1 CRUSH_SERVER_IDLE_TIMEOUT=1
cat > "$t/proj/crush.json" <<JSON
{"providers":{"fake":{"type":"openai-compat","base_url":"http://127.0.0.1:$(cat "$t/port")/v1","api_key":"x","models":[{"id":"m","name":"m","context_window":100000,"default_max_tokens":1000}]}},
 "models":{"large":{"provider":"fake","model":"m"},"small":{"provider":"fake","model":"m"}},
 "options":{"disable_provider_auto_update":true,"disable_metrics":true}}
JSON
cd "$t/proj" || exit 1

run_suite() { # run_suite <tên> [cờ toàn cục...]
  local mode="$1"; shift
  local o="$t/$mode"; mkdir -p "$o"
  local rc

  # Làm ấm DB trước: hai tiến trình local khởi động lạnh cùng lúc tranh nhau
  # chạy migration (lỗi có sẵn, không thuộc --json).
  "$bin" "$@" run --json "warm" >/dev/null 2>&1

  # Hai tiến trình song song, cùng dự án: mỗi cái phải trả session của chính nó.
  "$bin" "$@" run --json "AAA" >"$o/a.json" 2>"$o/a.err" &
  local pa=$!
  "$bin" "$@" run --json "BBB" >"$o/b.json" 2>"$o/b.err"; rc=$?
  wait "$pa"; local rca=$?
  check "$mode: song song rc=0" $((rc + rca))
  local sa sb
  sa="$(jget "$o/a.json" session_id)"; sb="$(jget "$o/b.json" session_id)"
  [ -n "$sa" ] && [ -n "$sb" ] && [ "$sa" != "$sb" ]; check "$mode: song song 2 session_id khác nhau" $?
  [[ "$(jget "$o/a.json" text)" == *AAA* && "$(jget "$o/a.json" text)" != *BBB* ]]; check "$mode: A nhận text của A" $?
  [[ "$(jget "$o/b.json" text)" == *BBB* && "$(jget "$o/b.json" text)" != *AAA* ]]; check "$mode: B nhận text của B" $?
  local tok
  tok="$(tokens "$o/a.json")"
  [ "$tok" = 11/7 ]; check "$mode: token session mới = 11/7 (quan sát $tok)" $?

  # -s <id> tiếp tục đúng session đó.
  "$bin" "$@" run --json -s "$sa" "again" >"$o/s.json" 2>/dev/null; rc=$?
  [ "$rc" -eq 0 ] && [ "$(jget "$o/s.json" session_id)" = "$sa" ] && [[ "$(jget "$o/s.json" text)" == *again* ]]; check "$mode: -s tiếp tục đúng session" $?
  tok="$(tokens "$o/s.json")"
  [ "$tok" = 11/7 ]; check "$mode: token kết quả -s = 11/7 (quan sát $tok)" $?

  # -C = session mới nhất; chạy tuần tự nên biết chắc là cái nào.
  sleep 1.1 # updated_at tính theo giây; tránh hoà với session vừa chạy.
  "$bin" "$@" run --json "seed" >"$o/seed.json" 2>/dev/null
  "$bin" "$@" run --json -C "last" >"$o/c.json" 2>/dev/null; rc=$?
  [ "$rc" -eq 0 ] && [ "$(jget "$o/c.json" session_id)" = "$(jget "$o/seed.json" session_id)" ]; check "$mode: -C trả session mới nhất" $?

  # Lỗi provider: rc=1, vẫn có JSON với error + session thật.
  "$bin" "$@" run --json "FAIL" >"$o/e.json" 2>/dev/null; rc=$?
  [ "$rc" -eq 1 ] && [ -n "$(jget "$o/e.json" error)" ] && [ -n "$(jget "$o/e.json" session_id)" ]; check "$mode: lỗi provider rc=1 + JSON" $?

  # -s không tồn tại: lỗi trước khi chạy, vẫn phải có JSON.
  "$bin" "$@" run --json -s doesnotexist "x" >"$o/n.json" 2>/dev/null; rc=$?
  [ "$rc" -eq 1 ] && [ -n "$(jget "$o/n.json" error)" ]; check "$mode: -s sai rc=1 + JSON" $?

  # Tool cần quyền: run tự duyệt trong cả hai chế độ (như AutoApproveSession).
  timeout 60 "$bin" "$@" run --json "TOOLCALL" >"$o/p.json" 2>/dev/null; rc=$?
  [ "$rc" -eq 0 ] && [[ "$(jget "$o/p.json" text)" == *TOOLDONE* ]] && [ -e "$t/proj/e2e-perm-probe" ]; check "$mode: tool cần quyền được tự duyệt và chạy" $?
  rm -f "$t/proj/e2e-perm-probe"

  # Sai cú pháp: rc=2, stdout rỗng.
  "$bin" "$@" run --json </dev/null >"$o/u.out" 2>/dev/null; rc=$?
  [ "$rc" -eq 2 ] && [ ! -s "$o/u.out" ]; check "$mode: không prompt rc=2" $?
  "$bin" "$@" run --json -s x -C "y" >"$o/u2.out" 2>/dev/null; rc=$?
  [ "$rc" -eq 2 ] && [ ! -s "$o/u2.out" ]; check "$mode: -s với -C rc=2" $?

  # SIGTERM giữa chừng: vẫn đúng 1 JSON có error, rc=1.
  "$bin" "$@" run --json "SLOW" >"$o/t.json" 2>/dev/null &
  local pt=$!
  sleep 4
  kill -TERM "$pt" 2>/dev/null
  ( sleep 15; kill -9 "$pt" 2>/dev/null ) & local wd=$!
  wait "$pt"; rc=$?
  kill "$wd" 2>/dev/null; wait "$wd" 2>/dev/null
  [ "$rc" -eq 1 ] && [ -n "$(jget "$o/t.json" error)" ]; check "$mode: SIGTERM -> rc=1 + JSON có error" $?
  # Server giữ run khi client rời, nên Ctrl+C phải tự huỷ run của nó.
  local st=""
  for _ in $(seq 20); do st="$(last_reply "$t/proj" "$(jget "$o/t.json" session_id)")"; [[ "$st" == ?*\|* ]] && break; sleep 0.5; done
  [[ "$st" == canceled\|* ]]; check "$mode: SIGTERM huỷ luôn run (quan sát: $st)" $?

  # `run` không được khởi tạo lại agent đã sẵn sàng (sẽ thay coordinator của
  # TUI đang mở: mất tool question). Giả lập TUI: init interactive, rồi run.
  if [ "$mode" = clientserver ]; then
    "$bin" "$@" run --json "SLOW holder" >"$o/h.json" 2>/dev/null &
    local ph=$! wsid code
    sleep 3
    wsid="$(curl -s --unix-socket "$t/s.sock" http://x/v1/workspaces | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["id"])')"
    code="$(curl -s -o /dev/null -w '%{http_code}' --unix-socket "$t/s.sock" -X POST -H 'content-type: application/json' -d '{"interactive":true}' "http://x/v1/workspaces/$wsid/agent/init")"
    local busy; busy="$(curl -s --unix-socket "$t/s.sock" "http://x/v1/workspaces/$wsid/agent" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("is_busy"))')"
    [ "$busy" = True ]; check "$mode: init giữa run, agent vẫn báo bận (is_busy=$busy)" $?
    "$bin" "$@" run --json "probe" >"$o/f.json" 2>/dev/null; rc=$?
    local tools; tools="$(grep '^probe|' "$t/requests.log" | tail -1)"
    [ "$code" = 200 ] && [ "$rc" -eq 0 ] && [[ "$tools" == *question* ]]; check "$mode: run không init lại agent đang sẵn sàng (init=$code, tools: $tools)" $?
    kill -TERM "$ph" 2>/dev/null; wait "$ph" 2>/dev/null
    # Init giữa run không được thay coordinator (run cũ thành bóng ma: không
    # huỷ được, is_busy=false): SIGTERM holder vẫn phải huỷ được run của nó.
    local hs=""
    for _ in $(seq 20); do hs="$(last_reply "$t/proj" "$(jget "$o/h.json" session_id)")"; [[ "$hs" == ?*\|* ]] && break; sleep 0.5; done
    [[ "$hs" == canceled\|* ]]; check "$mode: init giữa run không làm mất run, SIGTERM vẫn huỷ (quan sát: $hs)" $?
  fi
}

unset CRUSH_CLIENT_SERVER
run_suite local
export CRUSH_CLIENT_SERVER=1
run_suite clientserver --host "unix://$t/s.sock"

# Client bị SIGKILL giữa run: hết detach grace (1s) server vẫn phải giữ
# workspace cho tới khi run xong, không huỷ nó. Dự án + server riêng để biết
# chắc session nào; tên socket bắt đầu bằng s.sock để cleanup và kiểm "không
# còn server" bên dưới phủ luôn.
h0="$t/h0"; mkdir -p "$h0"; cp "$t/proj/crush.json" "$h0/"
(cd "$h0" && CRUSH_SERVER_DETACH_GRACE=1 exec "$bin" --host "unix://$t/s.sock-h0" run --json "SLOW orphan") >/dev/null 2>&1 &
pk=$!
for _ in $(seq 150); do grep -q '^SLOW orphan|' "$t/requests.log" && break; sleep 0.1; done
kill -9 "$pk" 2>/dev/null; wait "$pk" 2>/dev/null
h0_sid="$(cd "$h0" && "$bin" session list --json 2>/dev/null | python3 -c 'import json,sys; l=json.load(sys.stdin); print(l[0]["uuid"] if len(l) == 1 else "")')"
# send --wait bị Ctrl+C: chỉ thôi chờ, run vẫn chạy tiếp (send giao việc cho
# session). Chạy song song với run trên để dùng chung 30s chờ.
(cd "$h0" && exec "$bin" --host "unix://$t/s.sock-h0" send --new --wait --json "SLOW send") >"$t/h0-send.json" 2>/dev/null &
pw=$!
for _ in $(seq 150); do grep -q '^SLOW send|' "$t/requests.log" && break; sleep 0.1; done
kill -INT "$pw" 2>/dev/null; wait "$pw"; rc=$?
send_sid="$(jget "$t/h0-send.json" session_id)" send_err="$(jget "$t/h0-send.json" error)"
# Không đặt $(...) trong tên check: nó ghi đè $? trước khi check đọc.
[ "$rc" -eq 1 ] && [ "$send_err" = "stopped waiting; the run keeps going on session $send_sid" ]; check "clientserver: Ctrl+C send --wait chỉ thôi chờ (quan sát: rc=$rc $send_err)" $?
# run -s vào session đang chạy lượt của người khác (lượt SLOW send): Ctrl+C
# không được huỷ lượt đó hay hàng đợi, chỉ thôi chờ.
(cd "$h0" && exec "$bin" --host "unix://$t/s.sock-h0" run --json -s "$send_sid" "queued from run") >"$t/h0-runs.json" 2>/dev/null &
pr=$!
sleep 3
kill -INT "$pr" 2>/dev/null; wait "$pr"; rc=$?
runs_err="$(jget "$t/h0-runs.json" error)"
[ "$rc" -eq 1 ] && [ "$runs_err" = "stopped waiting; the run keeps going on session $send_sid" ]; check "clientserver: Ctrl+C run -s chỉ thôi chờ (quan sát: rc=$rc $runs_err)" $?
h0_state="" send_all=""
for _ in $(seq 60); do
  h0_state="$(last_reply "$h0" "$h0_sid")"; send_all="$(replies "$h0" "$send_sid" | tr '\n' ';')"
  [[ "$h0_state" == ?*\|* && "$send_all" == *"queued from run"* ]] && break; sleep 1
done
[[ "$h0_state" == "end_turn|ECHO("*"SLOW orphan"* ]]; check "clientserver: client bị SIGKILL giữa run -> run vẫn xong (quan sát: $h0_state)" $?
[[ "$send_all" == "end_turn|ECHO("*"SLOW send;"* ]]; check "clientserver: lượt của send vẫn xong sau Ctrl+C của send và của run -s (quan sát: $send_all)" $?
[[ "$send_all" == *"end_turn|ECHO("*"queued from run;" ]]; check "clientserver: prompt xếp hàng của run -s vẫn chạy (quan sát: $send_all)" $?

# Server idle-timeout 1s phải tự thoát; không để lại tiến trình hay socket.
for _ in $(seq 30); do pgrep -f -- "$t/s.sock" >/dev/null || break; sleep 0.5; done
pgrep -f -- "$t/s.sock" >/dev/null; [ $? -ne 0 ]; check "không còn server process" $?

if [ "$fail" -eq 0 ]; then echo PASS; else echo "FAILED"; fi
exit "$fail"
