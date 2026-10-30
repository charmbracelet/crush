#!/bin/sh
# Fake gpg for integrated pinentry tests. Behavior is chosen by the
# FAKE_GPG_MODE environment variable (the runner passes the command's
# environment through):
#   cached       - the agent already has the credential cached: succeeds
#                  regardless of the supplied passphrase.
#   locked       - only the passphrase "secret" (read from fd 3) succeeds;
#                  otherwise it leaves partial output on stdout (as a
#                  clearsign does), emits passphrase status lines and fails.
#   unsupported  - loopback pinentry is refused: signals a fallback.
# A plain run (no --pinentry-mode, so the external pinentry would ask)
# prints "plain" and echoes stdin, so tests can check what reached it.
mode=${FAKE_GPG_MODE:-cached}

case " $* " in
*" --pinentry-mode "*) ;;
*)
  echo plain
  cat
  exit 0
  ;;
esac

# Real gpg stops parsing options at the first operand, so the runner
# must put its loopback flags before the caller's arguments.
if [ "$1" != "--batch" ]; then
  echo "gpg: Note: '--batch' is not considered an option" >&2
  exit 2
fi

# Read the passphrase from the dedicated fd the runner always provides.
if read -r pass <&3 2>/dev/null; then
  :
fi

case "$mode" in
  cached)
    echo "[GNUPG:] SIG_CREATED ok"
    exit 0
    ;;
  locked)
    if [ "$pass" = "secret" ]; then
      echo "[GNUPG:] USERID_HINT ABC123 Test Key <test@example.com>" >&2
      echo "[GNUPG:] SIG_CREATED ok"
      exit 0
    fi
    echo "partial"
    echo "[GNUPG:] USERID_HINT ABC123 Test Key <test@example.com>" >&2
    echo "[GNUPG:] BAD_PASSPHRASE ABC123" >&2
    echo "gpg: signing failed: Bad passphrase" >&2
    exit 2
    ;;
  unsupported)
    echo "gpg: setting 'pinentry-mode' to 'loopback' is not allowed" >&2
    exit 2
    ;;
  *)
    echo "gpg: unknown subcommand" >&2
    exit 1
    ;;
esac
