Start a subagent that runs on its own and reports back later.

The call returns as soon as the subagent starts. Its result arrives as a `<subagent-report>` message under the `label` you gave it, once it finishes. Keep working in the meantime, or end your turn; the report wakes you.

Use it to fan work out (a review, a search, an investigation across several scopes) or to hand off reading that would otherwise fill your context. For a quick lookup you need before doing anything else, use `agent` instead, which blocks and returns the answer.

`label` names the subagent for `agent_send`, `agent_status`, and `agent_stop`, and titles its report. Make it describe the scope: `auth-review`, `db-layer`. Labels are unique per session.

`access` is `read` (default) for a subagent that inspects and runs things but cannot edit, or `write` for one that makes a change. Give parallel `write` subagents scopes that do not overlap.

`model` runs this subagent on a different model than the session default. Name one from the list at the end of this description; it holds every provider you can use and the models each can serve. Omit `provider` to use the parent agent's provider; `provider` requires `model`. `reasoning` sets how hard the subagent thinks, using one of the values in brackets after its model in that list; when omitted the model's default applies. Without any of these the subagent runs on the session's model. The override is remembered: `agent_send` follow-ups and later resumes of the subagent keep running on it.

The prompt is all the subagent sees. It cannot see this conversation or other subagents. Say what to examine, what to do, and what shape the answer should take.

Subagents keep running if the user interrupts your turn. After it reports, a subagent can still take follow-ups through `agent_send`.
