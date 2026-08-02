Schedule a new scheduled task that re-runs a prompt automatically on a cron schedule, mimicking the CronCreate tool in Claude Code and Codex.

Takes a 5-field cron expression (minute hour day-of-month month day-of-week) evaluated in the user's local timezone. Supports wildcards (* and ?), single values (5), steps (*/15), ranges (1-5), comma lists (1,15,30), and three-letter names (JAN, MON). There is no seconds field, descriptors like @daily are not accepted, and extended syntax (L, W) is not supported. Day-of-week: 0 or 7 = Sunday through 6 = Saturday; when both day-of-month and day-of-week are restricted, the task fires when either matches.

Tasks fire at the top of the specified minute. If you pin the current minute (e.g. it is 06:22:50 and you schedule for minute 22), the task fires at 06:23:00, not 10 seconds later. Sub-minute precision is not supported.

**Scheduling relative to "now"**: The `<env>` block in your system prompt carries the session's START time, which goes stale the longer the session runs. Before computing cron fields, call `date` in the shell to get the actual current time — a schedule built from the stale env time lands in the past and is rejected.

To schedule N minutes from now, add N to the current minute (handling hour/day rollover). For example, if the env says "7/30/2026, 7:29:00 PM PDT" and the user asks for "1 minute from now", use minute 30 (29+1). If they ask for "5 minutes from now", use minute 34 (29+5). If the sum exceeds 59, rollover to the next hour and adjust accordingly.

**Common patterns**:
- "Remind me in 5 minutes": pin minute+5 (with rollover), same hour/day unless rolled over, recurring: false
- "Check every hour": "0 * * * *", recurring: true
- "Every day at 9am": "0 9 * * *", recurring: true
- "At 2:30pm today": "30 14 <today_dom> <today_month> *", recurring: false

Expressions that are syntactically valid but can never match — "0 0 30 2 *", February 30th — are rejected rather than accepted as a task that silently never runs.

A one-shot (recurring: false) whose pinned time has already passed today is also rejected: its next match would jump to tomorrow or next year, which is never what a one-shot means. Recompute the cron fields against the current time and try again. Recurring schedules are unaffected — a daily 9am task created at 10:30 legitimately fires tomorrow.

Set recurring to false for a one-shot reminder that fires once and deletes itself (pin minute/hour/day-of-month/month to specific values). Set durable to true to persist the task to disk so it survives restarts; otherwise it lives only in this session. A session can hold up to 50 scheduled tasks. Returns a task ID you can pass to CronDelete.
