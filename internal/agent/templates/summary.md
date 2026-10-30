You are writing the context checkpoint for a coding session. Assume every message above is about to be deleted: the next model to work sees your text and, usually, the last few turns verbatim, and nothing else.

The failure you are preventing is not vagueness. It is a successor that redoes work already finished, reopens an approach already rejected, or silently breaks a constraint the user already gave.

**Required sections**:

## Current State

- What task is being worked on (exact user request)
- Constraints the user stated that still apply, quoted rather than paraphrased
- Current progress and what's been completed
- What's being worked on right now (incomplete work)
- What remains to be done (specific next steps, not vague)

## Files & Changes

- Files that were modified (with brief description of changes)
- Files that were read/analyzed (why they're relevant)
- Key files not yet touched but will need changes
- File paths and line numbers for important code locations

## Technical Context

- Architecture decisions made and why
- Approaches tried and rejected, with the reason each was dropped
- Patterns being followed (with examples)
- Libraries/frameworks being used
- Commands that worked (exact commands with context)
- Commands that failed (what was tried and why it didn't work)
- Environment details (language versions, dependencies, etc.)

## Strategy & Approach

- Overall approach being taken
- Why this approach was chosen over alternatives
- Key insights or gotchas discovered
- Assumptions made
- Any blockers or risks identified

## Exact Next Steps

Be specific. Don't write "implement authentication" - write:

1. Add JWT middleware to src/middleware/auth.js:15
2. Update login handler in src/routes/user.js:45 to return token
3. Test with: npm test -- auth.test.js

**Rules**:

- Preserve exact identifiers: file paths, symbol and function names, error strings, commit hashes, config keys. One verbatim string is worth a paragraph of description.
- Anything the most recent turns already show does not need restating, but anything they imply does. Spend the words on rationale, rejections, and constraints rather than on replay.
- Never guess that a step succeeded, and never soften an uncertainty to sound finished. "Unclear whether X was verified" beats a confident wrong answer.
- Do not paste large transcripts or file bodies; the next model can read files, and a checkpoint that duplicates them helps nobody.
- Security-relevant instructions from the user (files to avoid, operations forbidden, secret handling) survive verbatim.

**Tone**: Write as if briefing a teammate taking over mid-task. Include everything they'd need to continue without asking questions. No emojis ever.

**Length**: You have a fixed output budget for this response. Spend it where it pays: rationale, rejected approaches, constraints, exact identifiers, and next steps. Do not spend it on transcript replay, file bodies, or restating what the retained turns already show.
