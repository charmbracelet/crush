# ACP Conformance Tests

End-to-end and cross-implementation tests for Crush's ACP server and client.
These tests verify protocol compliance beyond unit tests by running actual
stdio JSON-RPC 2.0 connections against built binaries or external agents.

## Quick Start

```bash
# Run all automated test categories
./test/acp/run.sh

# Individual targets
./test/acp/run.sh build     # go build -o /tmp/crush-test-acp .
./test/acp/run.sh unit      # go test ./internal/acp/...
./test/acp/run.sh interop   # cross-SDK interop against Tangerg/acp
./test/acp/run.sh sdk       # go test github.com/coder/acp-go-sdk
./test/acp/run.sh e2e       # end-to-end against the real binary
./test/acp/run.sh zed       # recorded Zed handshake against the real binary
./test/acp/run.sh clean     # rm /tmp/crush-test-acp
```

Or run directly with Go:

```bash
go test ./internal/acp/... -count=1          # unit + integration
go test ./internal/acp/... -run TestGolden   # JSON golden serialization only
go test ./internal/acp/... -race             # race detector
```

## Test Categories

### 1. Unit & Integration Tests — `go test ./internal/acp/...`
Fast, in-process tests using in-memory pipes. No binary needed.

| Test | What it validates |
|---|---|
| `TestGoldenCrushInitializeResponse` | Initialize response is structurally valid per spec; required fields present; round-trips losslessly |
| `TestGoldenContentBlocks/*` (3) | ContentBlock JSON matches SDK reference fixtures (text, image, resource_link) |
| `TestGoldenSessionUpdates/*` (3) | SessionUpdate serialization matches SDK reference (agent_message_chunk, tool_call, edit) |
| `TestGoldenPermissionOutcome/*` (2) | Permission outcome JSON matches SDK reference (selected, cancelled) |
| `TestGoldenMethodPayloads/*` (3) | Method request/response JSON matches SDK reference (initialize, new_session, cancel) |
| `TestServerStartEndToEnd` | Full server↔client pipe: initialize → new session → cancel → disconnect |
| `TestEventBridge_StartAndStop` | Event bridge starts and stops cleanly over pipe |
| `TestEventBridge_ClientDisconnect` | Cleanup when client disconnects mid-stream |
| `TestBuildPermissionOptions` | Permission option builder logic |
| `TestMapPermissionOutcome` | Outcome-to-grant boolean mapping |
| `TestIsAllowAlways` | Allow-always detection in outcomes |
| `TestPermissionBridgeIntegration` | Full permission bridge roundtrip through service layer |
| `TestClientSubscribe` | Session update fan-out via `Subscribe()` channel |
| `TestClientPermissionAutoAllow` | Auto-allow flag selects first allow option |
| `TestClientPermissionDefaultFirstOption` | Fallback to first option when autoAllow is off |
| `TestClientPermissionNoOptions` | Returns cancelled when no options provided |
| `TestClientReadTextFile` | File read with/without line limits; missing file error |
| `TestClientWriteTextFile` | File write with mkdir parent |
| `TestClientTerminalMethods/*` (5) | Terminal stubs return "not supported" (create, kill, output, wait, release) |
| `TestClientNewClient` | Constructor validates `acp.Client` interface implementation |
| `TestDrain` | drain() helper reads until channel closed |
| `TestParseToolCall` | Tool call raw input extraction |

### 2. SDK Regression Tests — `go test github.com/coder/acp-go-sdk`
Run the SDK's own 47 tests to catch regressions in the dependency.

```bash
go test github.com/coder/acp-go-sdk -count=1
```

Covers: connection dispatch (16), error handling, notification ordering, cancel
semantics (10), defaults (3), JSON golden parity (5 test groups covering 33
fixtures). Our `TestGolden*` suite exercises a subset of these from the Crush side.

### 3. External Conformance (manual)

#### OpenAgentsInc Harness (TypeScript, 47 scenarios)
Validates initialize identity, schema compliance, platform/profile/binary contracts.

```bash
git clone https://github.com/OpenAgentsInc/openagents
cd openagents
pnpm install
pnpm --dir packages/agent-client-protocol-conformance run test
```

To test Crush, point the harness at the built binary:
```bash
CRUSH_BIN=/tmp/crush-test-acp pnpm --dir packages/agent-client-protocol-conformance run test
```

#### Tangerg/acp Interop Tests (Go, 154 fixtures)
Cross-SDK fixtures from TypeScript validators + recorded Zed 1.17.2 sessions.

```bash
go get github.com/Tangerg/acp@latest
go test github.com/Tangerg/acp/... -race -count=1
```

Files: `harness_test.go`, `golden_test.go`, `zed_test.go`, `interop_test.go`.
Add to CI by vendoring or importing the package.

### 4. Manual E2E Testing

Connect to a real IDE or reference agent:

```bash
# Build binary
go build -o /tmp/crush .

# ACP server mode (for Zed, JetBrains, Neovim)
/tmp/crush acp

# ACP client mode (driving an external agent)
/tmp/crush acp connect <agent-binary> [args...]
```

**Verification checklist:**
1. `initialize` — returns protocol version 1, agent name `"Crush"`
2. `session/new` — creates SQLite session, returns non-empty sessionId
3. `session/prompt` — runs prompt through coordinator, returns `StopReasonEndTurn`
4. `session/update` — streaming chunks arrive in real time
5. `session/request_permission` — permission dialog appears; allow/reject propagates back
6. `session/cancel` — aborts in-flight prompt, returns `StopReasonCancelled`
7. `fs/read_text_file` / `fs/write_text_file` — reads/writes actual files on disk
8. Disconnect — server exits cleanly, no goroutine leak

## Golden Fixture Reference

The 33 SDK golden fixtures live in `internal/acp/testdata/json_golden/`. They are
copied from `github.com/coder/acp-go-sdk/testdata/json_golden/`.

To update after a SDK version bump:
```bash
cp $(go env GOMODCACHE)/github.com/coder/acp-go-sdk@v0.13.5/testdata/json_golden/*.json \
   internal/acp/testdata/json_golden/
go test ./internal/acp/... -run TestGolden -update  # regenerate if structure changed
```

### 5. Hang Regression Tests — `internal/acp/hang_regression_test.go`

Five tests, each pinned to a specific way an editor ends up loading forever.
They exist because the first four would otherwise only be caught by a human
watching Zed spin.

| Test | Finding | What it pins |
|---|---|---|
| `TestServerStdoutIsPureJSONRPC` | stdout purity | Every line on the server's stdout is a JSON-RPC frame; a stray log line corrupts the stream |
| `TestInitializeResponseShape` | capability shape | `promptCapabilities`, `sessionCapabilities`, `mcpCapabilities` are present, not omitted |
| `TestPromptStreamsAgentMessageChunk` | streaming | A successful turn emits `agent_message_chunk` before the prompt response |
| `TestNewSessionReportsCwdMismatch` | workspace rooting | A `cwd` outside the workspace is reported, not silently ignored |
| `TestSessionSetupMethodsSucceed` | setup methods | `session/set_mode` and `session/set_config_option` succeed |

### 6. Cross-SDK Interop — `internal/acp/tangerg_interop_test.go`

Replays the corpus recorded by `github.com/Tangerg/acp` v0.2.4, an independent
Go implementation of the protocol, plus a real Zed 1.17.2 transcript:

- `TestZedTranscriptRejectsNothing` — every method the editor sent is recognised
- `TestZedTranscriptCancelPath` — the recorded stop reasons are ones we can produce
- `TestZedInitializeCapabilities` — the editor's capability fields survive decoding
- `TestReferenceClientHandshake` — a Tangerg/acp **client** completes `initialize`
  and opens a session against our server over a real byte stream

The reference-client test is the strongest evidence here: two Go peers sharing a
bug would pass every other test in this package.

Fixtures live in `internal/acp/testdata/tangerg/`. To refresh after a Tangerg
release:

```bash
go get github.com/Tangerg/acp@latest
cp "$(go env GOMODCACHE)"/github.com/\!tangerg/acp@*/testdata/zed/*.json \
   internal/acp/testdata/tangerg/zed/
cp "$(go env GOMODCACHE)"/github.com/\!tangerg/acp@*/testdata/fixtures/*.json \
   internal/acp/testdata/tangerg/fixtures/
```

Note the escaped module path: Go encodes the capital `T` in `Tangerg` as `!t` in
the module cache.

### 7. End-to-End Against the Real Binary — `test/acp/e2e_test.go`

Built with `-tags e2e`, these spawn the compiled `crush acp` process and speak
raw JSON-RPC at its stdin/stdout. This is the only arrangement that can catch
transport-level faults an in-process pipe test cannot see.

| Test | What it pins |
|---|---|
| `TestZedHandshake` | The recorded Zed 1.17.2 `initialize` params get a complete capability response |
| `TestZedSessionThenCancel` | `session/new`, setup methods and `session/cancel` work over the real transport |
| `TestStdoutCarriesOnlyJSONRPC` | No non-JSON-RPC bytes leak to stdout during a whole run |
| `TestUnknownMethodFails` | An unimplemented method returns a JSON-RPC error rather than silence |

```bash
CRUSH_BIN=/tmp/crush-test-acp go test -tags e2e ./test/acp/ -count=1 -v
```

A live prompt turn needs a configured provider, so these tests exercise the
handshake, session setup and cancellation paths rather than a model call.

## Known Gaps

| Gap | Status | How to close |
|---|---|---|
| Streaming prompt e2e test | Closed | `TestPromptStreamsAgentMessageChunk` asserts `agent_message_chunk` reaches the client during a turn |
| Cross-implementation conformance | Closed | `internal/acp/tangerg_interop_test.go` replays the Tangerg/acp corpus and drives our server with a Tangerg client |
| Zed reference client validation | Partial | `TestZedHandshake` replays recorded Zed bytes against the real binary; a live Zed run is still the final confirmation |
| Agent crash propagation | Open | Add `Done()` channel on `Client` so callers detect agent death |
| `TestEventBridge_ClientDisconnect` flakiness | Closed | Test rewritten to the two-pipe pattern; no longer races |
| Live model call over ACP | Open | Needs a configured provider; the e2e tests deliberately stop at cancellation |