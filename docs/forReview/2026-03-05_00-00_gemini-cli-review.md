# Dispatch System Architecture Review

**Reviewer:** Gemini CLI
**Date:** 2026-03-05

## Overall Assessment

The proposed architecture is robust, secure, and well-thought-out. It effectively addresses the challenges of distributed multi-agent orchestration. The shift from a simple local spawn model to a session-based, long-polling architecture with lease management is the right direction for scalability and reliability.

The "Mental Model" of email is a strong analogy that clarifies the asynchronous nature of the system.

## Critical Review Questions

1.  **Session model:** Yes, `worker + machine + directory` is the correct unique identifier. It maps perfectly to how developers work (one shell per project).
2.  **Directory constraints:** The "no nesting" rule is critical. Without it, a `goose` instance at `/repo` could inadvertently modify files in `/repo/backend` that another `goose` instance thinks it owns. This is a necessary constraint.
3.  **Structured specs:** This is a **huge** security win. Passing raw shell strings is a recipe for disaster (injection attacks). Structured specs `(binary, args, cwd)` are much safer and easier to audit.
4.  **Leases + fencing:** The 60s/30s split is a standard pattern and works well. The `lease_generation` token is essential for fencing off zombie handlers; without it, you'd have data corruption from "split brain" handlers.
5.  **Crash recovery:** The PID fingerprint (`pid` + `start_time`) is a clever and necessary addition. Relying on PID alone is dangerous due to OS PID recycling.
6.  **State machine:** The transition from `pending` -> `claimed` -> `started` -> `completed` is logical. The `abandoned` state (on lease expiry) is the correct way to handle failures.
7.  **Session ordering:** Blocking new commands until the current one is terminal is the safest approach. It avoids complex concurrency issues within a single session context.
8.  **Spec Authority:** Absolutely necessary. The server should not be able to force a handler to execute arbitrary binaries. Validating against the registered `binary_path` creates a strong trust boundary.
9.  **Process-tree termination:** `Setpgid` + `kill(-pid)` is the correct standard for Unix. For Windows, Job Objects are indeed the robust equivalent.
10. **Idempotency:** Keying results by `(command_id, lease_generation)` is correct. It ensures that a result from a "zombie" handler (previous generation) is rejected.

## Suggestions & Observations

### 1. Binary Path Updates
**Concern:** The design states `binary_path` is resolved at registration.
**Question:** What happens if the binary on the worker machine is updated or moved?
**Suggestion:** Consider a "re-validation" or "update registration" mechanism. If `exec.LookPath` fails during the `specMatchesSession` check, the handler might want to try re-resolving the `binary_name` and updating its registration (or failing with a clear error).

### 2. Output Limits
**Observation:** The design mentions a 10MB limit for stdout/stderr.
**Suggestion:** While reasonable for a default, some tasks (like verbose builds or huge diffs) might exceed this. Consider making this configurable per-dispatch or per-session, or implementing a "streaming only" mode where we don't buffer the whole output in memory if it's being streamed via `POST /activity`.

### 3. Network Flakiness & Heartbeats
**Observation:** 30s heartbeat with 60s lease.
**Suggestion:** In high-latency or flaky network environments (e.g., dev tunnels), a missed heartbeat could cause a lease to expire prematurely. Ensure the client (handler) has retry logic for the heartbeat `POST` itself. If the heartbeat fails due to a transient network error, it should retry immediately rather than waiting for the next ticker.

### 4. Environment Variables
**Question:** The `spec` includes `env`. How are these filtered?
**Suggestion:** We should probably allow a strict allowlist or blocklist for environment variables to prevent leaking sensitive host env vars to the worker, or conversely, to prevent the server from overriding critical host env vars (like `PATH` or `HOME`).

### 5. Zombie Processes & Reboot
**Edge Case:** If the machine reboots, the state file might persist but the process is definitely gone. The recovery logic checks `FindProcess`. On a reboot, `FindProcess(old_pid)` might find a *new* unrelated process.
**Mitigation:** The `pid_start_time` check *should* save us here, as the new process will have a much later start time. Just confirming that this check is robust enough to handle the reboot case where the PID happens to collide.

## Conclusion

This design is ready for implementation. The security and reliability mechanisms (leases, fencing, structured specs) are industry standard for this type of system.
