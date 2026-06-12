You are performing a CONTEXT CHECKPOINT COMPACTION. Create a handoff summary for another LLM that will resume this task.

**This summary will be ONLY context available when work resumes.** All previous messages will be replaced by this summary.

## Instructions

Before writing the summary, analyze the conversation. Think through:
- What is the user's actual goal?
- What has been accomplished vs what remains?
- What errors were encountered and how were they resolved?
- What files are most important for continuing?
- What would a new assistant need to know to avoid repeating mistakes?

## Constraints

- **Length budget: ~1500 tokens maximum.** Be concise. Every token in this summary consumes space needed for future work.
- Include exact file paths and line numbers where applicable.
- Include commands that worked (exact syntax). Skip commands that failed unless the failure reveals an important constraint.
- Preserve code snippets only when they represent non-obvious patterns or critical state. Don't reproduce entire files.
- If there is a todo list, include task statuses using standard markdown checkboxes: `- [x]` for completed, `- [ ]` for pending/in-progress. The resuming assistant will also see the todo list separately via the `todos` tool.
- Write as if briefing a teammate taking over mid-task. No emojis. No filler.

## Output Format

Use exactly these section headings. Do not add extra sections.

### Goal
(One sentence: what the user asked for)

### Completed
(Numbered list of what was done, with file paths)

### Errors & Fixes
(Bullet list. Only include errors that affected the current approach or revealed important constraints. Skip transient failures that were immediately resolved.)

### Remaining
(Specific next steps with file paths and line numbers. Not "implement auth" but "add JWT middleware to src/middleware/auth.js:15")

### Key Paths
(List of files most relevant to continuing work)
