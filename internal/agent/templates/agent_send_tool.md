Send a message to a sub-agent you dispatched with `agent_dispatch`.

Use it to narrow a scope that turned out too broad, pass along something you learned after dispatching, or ask a follow-up question about a report you already received.

The sub-agent keeps its session, so it answers with its earlier work still in context. You do not have to re-explain what it already did.

Timing decides how the answer comes back, not whether it does:
- Still working: the message reaches it at its next step and the answer is folded into the report it was already going to send.
- Already finished: it picks the message up as a follow-up and sends a new report answering it.

Either way you get back only a delivery confirmation here. The answer arrives later as a `<sub-agent-report>` under the same label, so do not wait on this call for it. If you have nothing else to do, end your turn; the report will wake you.

A sub-agent dispatched from a different conversation, or one whose label you made up, cannot be reached. Dispatch a new one instead.
