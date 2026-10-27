Start a sub-agent that runs on its own and reports back later.

The call returns as soon as the sub-agent starts. Its result arrives as a `<sub-agent-report>` message under the `label` you gave it, once it finishes. Keep working in the meantime, or end your turn; the report wakes you.

Use it to fan work out (a review, a search, an investigation across several scopes) or to hand off reading that would otherwise fill your context. For a quick lookup you need before doing anything else, use `agent` instead, which blocks and returns the answer.

`label` names the sub-agent for `agent_send`, `agent_status`, and `agent_stop`, and titles its report. Make it describe the scope: `auth-review`, `db-layer`. Labels are unique per session.

`access` is `read` (default) for a sub-agent that inspects and runs things but cannot edit, or `write` for one that makes a change. Give parallel `write` sub-agents scopes that do not overlap.

`model` runs this sub-agent on a different model than the session default. Name one from the list at the end of this description; it holds every provider you can use and the models each can serve. Omit `provider` to use the parent agent's provider; `provider` requires `model`. `think` and `reasoning_level` pick that model's reasoning, and when omitted the model's defaults apply. Without any of these the sub-agent runs on the session's model. The override is remembered: `agent_send` follow-ups and later resumes of the sub-agent keep running on it.

The prompt is all the sub-agent sees. It cannot see this conversation or other sub-agents. Say what to examine, what to do, and what shape the answer should take.

Sub-agents keep running if the user interrupts your turn. After it reports, a sub-agent can still take follow-ups through `agent_send`.
