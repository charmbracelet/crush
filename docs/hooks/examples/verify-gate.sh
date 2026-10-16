#!/usr/bin/env bash
#
# Stop hook: cổng nghiệm thu. Chạy lệnh verify của dự án (test/lint) trước khi
# agent được phép báo "xong". Fail thì chặn: agent nhận lại output và làm tiếp,
# thay vì tự tuyên bố hoàn thành khi test đỏ.
#
# Bật trong crush.json của dự án (hoặc ~/.config/agent-cli/crush.json):
#   {
#     "hooks": {
#       "Stop": [
#         { "command": "/ĐƯỜNG/DẪN/verify-gate.sh", "timeout": 300 }
#       ]
#     }
#   }
#
# Lệnh verify lấy từ biến AGENT_CLI_VERIFY_CMD (đặt trong ~/.config/agent-cli/env),
# ví dụ: AGENT_CLI_VERIFY_CMD="go build ./... && go test ./..."
# Không đặt thì cổng tự bỏ qua (exit 0), không cản gì.
#
# Giao thức Stop hook (xem docs/hooks/README.md):
#   - exit 0, không in gì  -> cho kết thúc.
#   - in {"decision":"block","reason":"..."} -> chặn, reason thành prompt kế.
# Lệnh verify fail = NOT READY (chặn). Lệnh verify không chạy được cũng là
# NOT READY: "không kiểm được" không phải "đã pass".

set -uo pipefail

cmd="${AGENT_CLI_VERIFY_CMD:-}"
if [[ -z "$cmd" ]]; then
	exit 0 # Chưa cấu hình cổng: không cản.
fi

# stdin mang {"stop_hook_active":true} khi lượt này đã là lần làm lại do cổng
# chặn. Vẫn chạy verify lại: cổng chỉ cho qua khi lệnh thật sự xanh (trần 3 lần
# làm lại do chính Crush áp, xem README).

out="$(cd "${CRUSH_PROJECT_DIR:-.}" && eval "$cmd" 2>&1)"
code=$?

if [[ $code -eq 0 ]]; then
	exit 0 # VERDICT: READY.
fi

# Cắt output để reason không quá dài; giữ phần đuôi (nơi lỗi thường nằm).
tail_out="$(printf '%s\n' "$out" | tail -n 40)"
reason="$(printf 'VERDICT: NOT READY\nLệnh verify (%s) thoát với mã %d. Sửa cho xanh rồi mới kết thúc.\n\n%s' "$cmd" "$code" "$tail_out")"

# jq nếu có (thoát JSON an toàn); không có thì tự thoát tối thiểu.
if command -v jq >/dev/null 2>&1; then
	jq -cn --arg r "$reason" '{decision:"block", reason:$r}'
else
	esc="${reason//\\/\\\\}"
	esc="${esc//\"/\\\"}"
	esc="${esc//$'\n'/\\n}"
	printf '{"decision":"block","reason":"%s"}\n' "$esc"
fi
exit 0
