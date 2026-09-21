#!/bin/sh
# Forwards Crush hook events to Orca's agent-hook endpoint, so Orca can show
# per-pane status. Wire it up per docs/hooks/orca.md; without Orca's ORCA_*
# environment it does nothing.

printf '{}\n'

payload=$({ command -p cat 2>/dev/null || cat; })
if [ -z "$payload" ]; then
  exit 0
fi

# Orca may pass the endpoint in a file instead of the environment.
if [ -n "${ORCA_AGENT_HOOK_ENDPOINT:-}" ] && [ -r "$ORCA_AGENT_HOOK_ENDPOINT" ]; then
  ORCA_AGENT_HOOK_TRANSPORT=
  . "$ORCA_AGENT_HOOK_ENDPOINT" 2>/dev/null || :
fi

if [ -z "${ORCA_AGENT_HOOK_PORT:-}" ] || [ -z "${ORCA_AGENT_HOOK_TOKEN:-}" ] || [ -z "${ORCA_PANE_KEY:-}" ]; then
  exit 0
fi

route=${ORCA_CRUSH_HOOK_ROUTE:-/hook/crush}
url="http://127.0.0.1:${ORCA_AGENT_HOOK_PORT}${route}"

meta=$(printf '%s\037%s\037%s\037%s\037%s\037%s' \
  "$ORCA_PANE_KEY" \
  "${ORCA_TAB_ID:-}" \
  "${ORCA_AGENT_LAUNCH_TOKEN:-}" \
  "${ORCA_WORKTREE_ID:-}" \
  "${ORCA_AGENT_HOOK_ENV:-}" \
  "${ORCA_AGENT_HOOK_VERSION:-}")

if [ "${ORCA_AGENT_HOOK_TRANSPORT:-}" = "raw-json-v1" ] && command -v base64 >/dev/null 2>&1 && command -v tr >/dev/null 2>&1; then
  encoded=$(printf '%s' "$meta" | base64 | tr -d '\n')
  [ -n "$encoded" ] || exit 0
  printf '%s' "$payload" | curl -sS -X POST "$url" \
    --connect-timeout 0.5 --max-time 1.5 \
    --noproxy "127.0.0.1" \
    -H "Content-Type: application/json" \
    -H "X-Orca-Agent-Hook-Token: ${ORCA_AGENT_HOOK_TOKEN}" \
    -H "X-Orca-Agent-Hook-Meta-Encoding: base64" \
    -H "X-Orca-Agent-Hook-Meta: ${encoded}" \
    --data-binary @- >/dev/null 2>&1 || :
else
  printf '%s' "$payload" | curl -sS -X POST "$url" \
    --connect-timeout 0.5 --max-time 1.5 \
    --noproxy "127.0.0.1" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -H "X-Orca-Agent-Hook-Token: ${ORCA_AGENT_HOOK_TOKEN}" \
    --data-urlencode "paneKey=${ORCA_PANE_KEY}" \
    --data-urlencode "tabId=${ORCA_TAB_ID:-}" \
    --data-urlencode "launchToken=${ORCA_AGENT_LAUNCH_TOKEN:-}" \
    --data-urlencode "worktreeId=${ORCA_WORKTREE_ID:-}" \
    --data-urlencode "env=${ORCA_AGENT_HOOK_ENV:-}" \
    --data-urlencode "version=${ORCA_AGENT_HOOK_VERSION:-}" \
    --data-urlencode "payload@-" >/dev/null 2>&1 || :
fi

exit 0
