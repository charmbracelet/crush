#!/usr/bin/env bash
#
# update-crush.sh — rebase the delegation patch onto a newer upstream crush release
#
# The local `delegation` branch carries one commit (990d878a) that lets agents
# pin their own model. This script rebases that commit onto the latest upstream
# tag, verifies the build/tests, and reinstalls the patched binary.
#
# Usage:
#   scripts/update-crush.sh            # rebase onto latest upstream tag
#   scripts/update-crush.sh v0.90.0    # rebase onto a specific tag
#
# Requirements: git, go on PATH (Go 1.26.3+), a writable ~/.local/bin.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_TAG="v0.89.0"                 # tag the delegation commit was created from
PATCH_COMMIT="990d878a"            # "feat: let agents pin their own model for cheaper delegation"
UPSTREAM_URL="https://github.com/charmbracelet/crush.git"
INSTALL_BIN="${HOME}/.local/bin/crush"

cd "$REPO_DIR"

# 1. Make sure we have an `upstream` remote pointing at the real crush repo.
if ! git remote get-url upstream >/dev/null 2>&1; then
  echo "==> adding upstream remote: $UPSTREAM_URL"
  git remote add upstream "$UPSTREAM_URL"
fi

# 2. Fetch upstream tags.
echo "==> fetching upstream tags"
git fetch upstream --tags --prune

# 3. Pick the target tag.
NEW_TAG="${1:-}"
if [ -z "$NEW_TAG" ]; then
  NEW_TAG="$(git tag -l 'v*' | grep -v -- "$BASE_TAG" | sort -V | tail -1)"
  # prefer upstream tags over local ones
  UPSTREAM_LATEST="$(git ls-remote --tags upstream 2>/dev/null | sed -n 's#.*refs/tags/\(v[0-9][0-9.]*\)$#\1#p' | sort -V | tail -1 || true)"
  if [ -n "$UPSTREAM_LATEST" ]; then
    NEW_TAG="$UPSTREAM_LATEST"
  fi
fi

if ! git rev-parse --verify "$NEW_TAG" >/dev/null 2>&1; then
  echo "error: tag $NEW_TAG does not exist (fetched upstream tags? run without args or check the tag name)" >&2
  exit 1
fi
if [ "$NEW_TAG" = "$BASE_TAG" ]; then
  echo "already on $BASE_TAG; no update needed (pass a newer tag explicitly to force)"
  exit 0
fi

echo "==> rebasing delegation commit $PATCH_COMMIT onto $NEW_TAG"
git checkout delegation
if ! git rebase --onto "$NEW_TAG" "$BASE_TAG" delegation; then
  echo
  echo "!! rebase conflicts. Resolve them, then re-run the verify/build steps:"
  echo "     git add <resolved files> && git rebase --continue"
  echo "     go build ./... && go vet ./internal/agent/... ./internal/config/..."
  echo "     go test ./internal/config/ ./internal/agent/"
  echo "     go build -o ${HOME}/.local/bin/crush ."
  echo "   The files most likely to conflict are internal/config/config.go,"
  echo "   internal/agent/coordinator.go, and internal/agent/agentic_fetch_tool.go."
  exit 1
fi

# 4. Verify the rebased tree builds and tests pass.
echo "==> verifying build and tests"
go build ./...
go vet ./internal/agent/... ./internal/config/...
go test ./internal/config/ ./internal/agent/

# 5. Rebuild and install the patched binary.
echo "==> building $INSTALL_BIN"
go build -o "$INSTALL_BIN" .
"$INSTALL_BIN" version

echo
echo "==> done. Commit summary:"
git log --oneline "$BASE_TAG"..delegation
echo
echo "==> If this shell predates ~/.local/bin on your PATH, open a new shell"
echo "    or run: eval \"\$(command -v $INSTALL_BIN)\"  (normally: exec $SHELL)"
echo "    The delegation patch is still applied (commit $PATCH_COMMIT)."