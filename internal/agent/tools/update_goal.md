Update the status of the current goal; only "complete" is supported.

<when_to_use>
Call this tool once you have fully verified that all requirements of the active goal's
objective have been met. Do not mark a goal complete if any requirement is unresolved.
</when_to_use>

<parameters>
- `status` (required): New status for the goal. Only `"complete"` is accepted.
</parameters>

<limitations>
- Only one status value is supported: `"complete"`
- Fails if no active goal exists for the session
</limitations>

<tips>
- Verify every requirement is met before marking complete
</tips>
