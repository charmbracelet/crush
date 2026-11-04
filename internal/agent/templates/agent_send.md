Send a message to a background agent started with `agent` and `background: true`.

Use it to steer an agent that is still running, or to follow up on one that has finished. A running agent reads the message at its next step. A finished agent is reopened with its earlier work in context and sends a new report when it is done. This call only confirms delivery; the answer arrives later as a report under the same name.
