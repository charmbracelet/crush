#!/usr/bin/env bash
# scripts/sync-from-crush.sh

CRUSH_REMOTE="https://github.com/charmbracelet/crush"
LAST_SYNC=$(cat .crush-sync/last-sync.txt)

# Add Crush as remote if needed
git remote add crush $CRUSH_REMOTE 2>/dev/null || true
git fetch crush

echo "ᕕ( ᐛ )ᕗ  Checking for Crush updates since $LAST_SYNC"

# Show what changed in synced directories
SYNCED_DIRS=(
    "internal/llm/agent"
    "internal/llm/tools"
    "internal/lsp"
    "internal/fsext"
)

for dir in "${SYNCED_DIRS[@]}"; do
    echo "\n📁 Changes in $dir:"
    git log $LAST_SYNC..crush/main --oneline -- $dir
done

echo "\n🤔 Review changes above, then:"
echo "  1. Cherry-pick commits you want: git cherry-pick <commit>"
echo "  2. Or merge directory: git checkout crush/main -- internal/llm/tools"
echo "  3. Update sync marker: echo <commit> > .crush-sync/last-sync.txt
"