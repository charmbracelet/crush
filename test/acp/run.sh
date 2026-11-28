#!/usr/bin/env bash
set -euo pipefail

# ACP test runner — builds Crush, runs all test categories, reports results.
# Usage: ./test/acp/run.sh [target]
#   target: all (default) | unit | spec | interop | sdk | build | e2e | zed | clean

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUILD_TARGET="${1:-all}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
pass() { echo -e "${GREEN}PASS${NC} $*"; }
fail() { echo -e "${RED}FAIL${NC} $*"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC} $*"; }

run_unit() {
	info "Running unit + integration tests..."
	( cd "$PROJECT_ROOT" && go test ./internal/acp/... -count=1 -timeout=60s ) || fail "unit tests"
	pass "unit tests"
}

run_spec() {
	info "Running ACP spec conformance, golden, and interop tests..."
	( cd "$PROJECT_ROOT" && go test ./test/acp/spec/... -count=1 -timeout=60s ) || fail "spec tests"
	pass "spec tests"
}

run_interop() {
	info "Running cross-SDK interop tests (Tangerg/acp)..."
	( cd "$PROJECT_ROOT" && go test ./test/acp/spec/... -run 'TestZed|TestReferenceClient' -count=1 -timeout=60s ) \
		|| fail "interop tests"
	pass "interop tests"
}

run_sdk_regression() {
	info "Running SDK regression tests..."
	( cd "$PROJECT_ROOT" && go test github.com/coder/acp-go-sdk -count=1 -timeout=60s ) || fail "SDK regression"
	pass "SDK regression"
}

build_binary() {
	local out="${CRUSH_BIN:-/tmp/crush-test-acp}"
	if [[ ! -x "$out" ]]; then
		info "Building crush binary to $out..."
		( cd "$PROJECT_ROOT" && go build -o "$out" . ) || fail "build"
	fi
	echo "$out"
}

run_build() {
	local bin
	bin="$(build_binary)"
	pass "build ($bin)"
}

run_e2e() {
	local bin
	bin="$(build_binary)"
	info "Running end-to-end tests against the real binary..."
	( cd "$PROJECT_ROOT" && CRUSH_BIN="$bin" go test -tags e2e ./test/acp/ -count=1 -timeout=120s ) \
		|| fail "e2e tests"
	pass "e2e tests"
}

run_zed() {
	local bin
	bin="$(build_binary)"
	info "Running the recorded Zed handshake against the real binary..."
	( cd "$PROJECT_ROOT" && CRUSH_BIN="$bin" go test -tags e2e ./test/acp/ -run TestZed -count=1 -v -timeout=120s ) \
		|| fail "zed handshake"
	pass "zed handshake"
}

run_clean() {
	rm -f /tmp/crush-test-acp
	info "Cleaned up build artifacts"
}

case "$BUILD_TARGET" in
unit)
	run_unit
	;;
spec)
	run_spec
	;;
interop)
	run_interop
	;;
sdk)
	run_sdk_regression
	;;
build)
	run_build
	;;
e2e)
	run_e2e
	;;
zed)
	run_zed
	;;
clean)
	run_clean
	;;
all)
	run_build
	run_unit
	run_spec
	run_interop
	run_sdk_regression
	run_e2e
	;;
*)
	echo "Usage: $0 [all|unit|spec|interop|sdk|build|e2e|zed|clean]"
	exit 1
	;;
esac

echo ""
info "Done."
