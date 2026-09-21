Report how memory is configured and whether it has caught up on recent turns.

This is the diagnostic tool. It answers "why doesn't it remember me" by showing
where memory is being written and how much of it has been processed yet.

<shows>
- The deployment URL, workspace, and session key
- The user and agent peer identifiers
- Observation mode, session strategy, and recall mode
- The background reasoning backlog: how much is still pending
</shows>

<when_to_use>
- The user says memory is not working, or asks what you remember
- `honcho_search` or `honcho_chat` returned nothing and you want to know whether
  that means "no memory" or "wrong session"
- The user just changed memory configuration and wants to confirm it took
</when_to_use>

<reading_the_result>
- A non-empty reasoning queue means recent turns are stored but have not yet
  shaped what memory knows. Give it a moment before concluding it forgot.
- The session key encodes the session strategy. A different working directory or
  branch can produce a different key, which looks like amnesia but is really a
  different notebook.
- The API key is never shown. If you need to check it, tell the user to look at
  their own configuration.
</reading_the_result>
