#!/usr/bin/env bash
# Regenerate internal/themes/schemes.json from the iTerm2-Color-Schemes
# repository (Windows Terminal JSON format). Requires git and node.
#
# The catalog is embedded at build time via go:embed; this script is not part
# of the runtime path. Run it to pull in new or updated upstream schemes.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

git clone --depth 1 --filter=blob:none --sparse \
  https://github.com/mbadolato/iTerm2-Color-Schemes "$tmpdir/repo"
(
  cd "$tmpdir/repo"
  git sparse-checkout set windowsterminal
)

node - "$tmpdir/repo/windowsterminal" "$repo_root/internal/themes/schemes.json" <<'NODE'
const fs = require("fs");
const path = require("path");
const [, , dir, out] = process.argv;

const schemes = [];
for (const f of fs.readdirSync(dir)) {
  if (!f.endsWith(".json")) continue;
  try {
    schemes.push(JSON.parse(fs.readFileSync(path.join(dir, f), "utf8")));
  } catch (e) {
    console.error("skip", f, e.message);
  }
}
schemes.sort((a, b) => (a.name || "").localeCompare(b.name || ""));
fs.writeFileSync(out, JSON.stringify(schemes));
console.log(`wrote ${schemes.length} schemes to ${out}`);
NODE
