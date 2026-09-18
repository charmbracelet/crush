#!/bin/sh
# Fake gpg for integrated pinentry tests. Behavior is chosen by the
# FAKE_GPG_MODE environment variable (the runner passes the command's
# environment through):
#   cached       - the agent already has the credential cached: succeeds
#                  regardless of the supplied passphrase.
#   locked       - only the passphrase "secret" (read from fd 3) succeeds;
#                  otherwise it emits passphrase status lines and fails.
#   unsupported  - loopback pinentry is refused: signals a fallback.
mode=${FAKE_GPG_MODE:-cached}

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
