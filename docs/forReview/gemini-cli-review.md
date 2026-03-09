# Dispatch System Architecture Review - Gemini CLI

## Overall Assessment

The proposed Dispatch System architecture is well-thought-out, prioritizing robustness and simplicity in a distributed environment. The choice of **long-polling** over SSE/WebSockets is a pragmatic decision that ensures compatibility across various network configurations (NAT, firewalls) with minimal complexity. The **one-handler-per-machine** and **dumb-executor** models are excellent for maintainability and security, as they centralize the "brains" of the operation in the Dispatch Server.

The inclusion of **command durability (ack protocol)** and **redirect lineage** demonstrates a deep understanding of the failure modes and steering requirements of AI agent workflows.

---

## Critical Concerns

### 1. PID Reuse Risk during Recovery
The crash recovery strategy involves reading a PID from a state file and killing it on startup.
- **Problem:** On systems with high process churn, PIDs can be reused. If the handler crashes and the machine reboots or a long time passes, the PID in `handler-state.json` might belong to a new, unrelated process (e.g., a system service).
- **Impact:** The handler might accidentally kill critical system processes.
- **Suggestion:** Store a "Process Fingerprint" in the state file (e.g., `PID` + `Process Start Time`). Before killing, verify that the process at that PID was started at the recorded time.

### 2. Atomic State File Writes
The handler writes its state to `~/.config/crush/handler-state.json`.
- **Problem:** If the handler crashes *while* writing this file, the file may be corrupted or partial.
- **Impact:** Recovery fails on the next startup due to invalid JSON.
- **Suggestion:** Use atomic writes (write to `handler-state.json.tmp`, then `os.Rename`).

### 3. Thundering Herd on Reconnect
If the Dispatch Server goes down and comes back up, or if 1000 handlers are waiting on a 30s timeout that expires simultaneously.
- **Problem:** A massive spike in requests.
- **Impact:** Server saturation or DOS.
- **Suggestion:** Add a small random jitter to the poll intervals and implement exponential backoff on connection failures.

### 4. Activity Capture Mechanism
The design mentions "watching session files" to POST activity.
- **Problem:** Generic CLI tools (goose, aider) don't have a standardized session file format.
- **Impact:** Activity streaming might only work for Crush-native workers, limiting the "multi-agent" vision.
- **Suggestion:** Consider if the Handler should wrap the execution in a PTY or capture stdout/stderr to provide a "generic" activity stream (e.g., last 10 lines of output) when no specialized session watcher is available.

---

## Suggestions (Nice-to-Haves)

### 1. Machine Health Telemetry
The server currently knows which workers are available on which machine, but not their "busyness".
- **Suggestion:** Handlers should include basic telemetry (CPU/Mem usage, number of active workers) in their poll requests. This allows the server to load-balance dispatches across multiple machines with the same worker type.

### 2. Command Expiration / TTL
- **Suggestion:** Commands should have a TTL. If a machine has been offline for 24 hours, it probably shouldn't try to execute a "fix bug" command from yesterday that a human has likely already resolved or moved on from.

### 3. Worker Capability Discovery
The current design maps machines to workers by name.
- **Suggestion:** Allow workers to register "Capabilities" (e.g., `can_edit_go`, `has_internet_access`). The `POST /dispatch` could then target a capability rather than a specific worker name.

---

## Questions

1. **Multi-Machine Worker Overlap:** If 5 machines have a worker named "goose", how does the server decide which one gets the command? Is it round-robin, or does it pick the first one that polls?
2. **Interactive Workers:** How does the handler handle tools that expect TTY input? The current `exec.Command` model might hang if the tool prompts the user.
3. **Machine Identity:** What happens if a machine changes its IP or is cloned (cloning the `machine_id`)? Should there be a unique "machine secret" generated on first registration?

---

## Alternative Approaches

### 1. NATS or MQTT
Instead of a custom long-poll REST API, a lightweight message broker like **NATS** could handle the command delivery.
- **Pros:** Built-in support for request-reply, persistence (JetStream), and firewalls (via leaf nodes).
- **Cons:** Adds a heavy dependency and infrastructure overhead compared to simple HTTP.

### 2. WASM Workers
For workers that don't require full OS access, running them in a **WASM runtime** (like Extism) managed by the handler would provide much stronger security isolation than a command whitelist.

---

## Summary
The architecture is solid and follows "boring technology" principles that lead to high reliability. Addressing the PID reuse and atomic write issues should be the immediate priority for the implementation phase.
