Launch a sub-agent that works in its own context window and reports back a single result.

Use this to:
- Fan out independent work. Emit several agent calls in one message and they run concurrently (up to 5 at a time). Reviewing several aspects of a change, searching several subsystems, or checking several hypotheses are all a good fit.
- Keep your own context clean. A sub-agent can read a large amount of material and hand back only the conclusion, which extends how long this session runs before it needs compaction.

Set `access` to choose how much the sub-agent can do:
- `read` (the default) gives it glob, grep, ls, view, bash, and the LSP tools. It can inspect and run things but cannot change files. Use it for review, search, and investigation.
- `write` adds the editing tools, so it can carry out a change end to end. Use it when the task is a self-contained piece of work you want done rather than described. Give it a scope it will not collide with, since parallel write sub-agents editing the same files will clobber each other.

Either way the sub-agent cannot launch further sub-agents.

Each call is independent: sub-agents cannot see your conversation, each other, or anything you did not put in the prompt. Write the prompt as a standalone brief. Say what to examine, what to do, and what shape the answer should take. State the scope explicitly, since two sub-agents given overlapping scopes will duplicate each other's work.

You get one text response back per call and cannot reply to it or ask a follow-up. The call does not return until the sub-agent finishes. If the work needs a conversation, do it yourself.
