# Plan Mode

Crush now uses a real session-level Plan Mode, modeled after Codex-style
planning instead of the old “pick one option and continue” flow.

## What Changed

- Removed the old pseudo plan flow based on a plan-selection tool.
- Removed the obsolete selection dialog and synthetic follow-up execution
  message.
- Stopped treating plan mode as a tool-output parsing trick or a separate
  execution session.

The new implementation keeps planning and execution inside the same session.

## Session-Level Collaboration Mode

Sessions now persist a `collaboration_mode` value:

- `default`
- `plan`

Existing sessions migrate to `default`. New sessions also start in `default`.

The active mode is part of session state, flows through the existing session
pubsub path, and is rendered by the UI directly. Dialogs do not maintain a
hidden plan-state shadow copy.

## Entering and Exiting Plan Mode

The Commands panel exposes an explicit toggle:

- `Enter Plan Mode`
- `Exit Plan Mode`

When a session is in Plan Mode, the header also shows a `PLAN` indicator.

If the latest assistant response includes a valid proposed plan, the Commands
panel also exposes `Execute Proposed Plan`.

## Agent Behavior in Plan Mode

Plan Mode is enforced in two layers.

### Prompt Rules

When a session is in `plan` mode, the agent receives dedicated planning rules:

- Explore first.
- Ask clarifying questions only when they materially affect the plan.
- Do not implement changes.
- Finish with exactly one `<proposed_plan>...</proposed_plan>` block.

### Tool Gating

Plan Mode also hard-gates tools instead of relying only on prompting.

Only read-only and analysis tools remain available. Mutating tools such as
write/edit flows, workspace-writing downloads, and execution-oriented shell
paths are not exposed to the agent while planning.

This means a model that drifts off-instruction still cannot mutate the repo in
Plan Mode.

## Structured Clarification via `request_user_input`

Plan Mode no longer uses the deleted multi-option plan-selection flow.

Instead, it has one official structured clarification tool:

- `request_user_input`

This tool is used for high-impact product or implementation decisions that
cannot be resolved by reading the repo.

The tool accepts 1–3 structured questions. Each question includes:

- a short `header`
- a stable `id`
- a `question`
- 2–3 mutually exclusive options, with the recommended option first

The UI renders the request in a dedicated dialog, allows selection or custom
input, and returns structured answers back to the agent through a request
registry instead of parsing free-form user messages.

## Proposed Plan Protocol

The output contract for Plan Mode is a plain assistant message containing a
single `<proposed_plan>` block.

Example:

```xml
<proposed_plan>
- Inspect the current session state wiring.
- Add session-level collaboration mode persistence.
- Gate mutating tools while the session is in Plan Mode.
- Add structured user-input prompts for unresolved choices.
</proposed_plan>
```

The UI enhances rendering when it detects this block, but the original message
text is preserved so content is never lost if enhanced parsing fails.

## Executing an Approved Plan

Approving a plan does not create a new session and does not clear history.

Instead, Crush:

1. switches the same session from `plan` back to `default`
2. keeps all existing messages, exploration output, and attachments
3. sends a concrete execution prompt built from the approved
   `<proposed_plan>` content

This preserves planning context and lets implementation continue naturally in
the same conversation.
