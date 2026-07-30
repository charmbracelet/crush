Schedule a new scheduled task that re-runs a prompt automatically on a cron schedule, mimicking the CronCreate tool in Claude Code and Codex.

Takes a 5-field cron expression (minute hour day-of-month month day-of-week) evaluated in the user's local timezone. Supports wildcards (* and ?), single values (5), steps (*/15), ranges (1-5), comma lists (1,15,30), and three-letter names (JAN, MON). There is no seconds field, descriptors like @daily are not accepted, and extended syntax (L, W) is not supported. Day-of-week: 0 or 7 = Sunday through 6 = Saturday; when both day-of-month and day-of-week are restricted, the task fires when either matches.

Expressions that are syntactically valid but can never match — "0 0 30 2 *", February 30th — are rejected rather than accepted as a task that silently never runs.

Set recurring to false for a one-shot reminder that fires once and deletes itself (pin minute/hour/day-of-month/month to specific values). Set durable to true to persist the task to disk so it survives restarts; otherwise it lives only in this session. A session can hold up to 50 scheduled tasks. Returns a task ID you can pass to CronDelete.

Example: "remind me at 2:30pm today to check the deploy" → cron: "30 14 <today_dom> <today_month> *", recurring: false.
