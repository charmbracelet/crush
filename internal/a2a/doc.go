// Package a2a is Crush's coordination layer for dispatching agents over the
// A2A (Agent-to-Agent) protocol, using the official Go SDK
// (github.com/a2aproject/a2a-go/v2).
//
// It replaces the in-process Go call between the main agent and a dispatched
// worktree agent with a protocol boundary: each dispatched agent is described
// by an [AgentCard] and served as an A2A server; the coordinator is an A2A
// client. This unlocks process isolation, remote workers, and third-party
// agents without changing the model-facing DispatchAgent tool.
//
// This package currently implements Phase 1 foundations:
//
//   - BuildAgentCard derives a spec-valid A2A AgentCard from a dispatched
//     agent's config (issue #68).
//   - Executor adapts a Crush SessionAgent to the a2asrv.AgentExecutor
//     interface, mapping the agent run lifecycle onto A2A task states
//     (issue #69).
//
// The in-process A2A servers + discovery (#70) and the DispatchAgent A2A
// client + SSE progress (#71) build on these. See the A2A Coordination epic
// (#67) for the full plan.
//
// The SDK's core type package is imported as a2aspec throughout to avoid
// colliding with this package's own name.
package a2a
