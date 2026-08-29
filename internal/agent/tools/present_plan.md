Use this tool when you are in plan mode and have finished investigating
and refining your plan. It presents the plan to the user for approval and
blocks until they decide.

## When to use

- You are in plan mode and your plan is complete and unambiguous: it
  covers what to change, which files to modify, and how to verify.
- You have resolved ambiguities yourself (by reading code) or asked the
  user via the `question` tool when you could not.

## When NOT to use

- For research or investigation tasks where you are just gathering
  information — do not present a plan.
- If you still have unresolved questions: use `question` first, then go
  back to exploring. Do not use this tool to ask "is this okay?".
- Do NOT use this tool merely to unblock a blocked write tool. Keep
  gathering context with read-only tools until the plan is genuinely
  complete.

## After approval

The user may:
- approve (you will be told to start implementing), or
- keep refining (stay in plan mode, address feedback, and call this
  tool again), or
- dismiss the plan (stay in plan mode and refine).

Never start implementing until the user explicitly approves the plan.
