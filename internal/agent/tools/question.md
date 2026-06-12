Ask the user a structured question and wait for their response. Use this
when you need clarification, confirmation, or a choice before proceeding.

## How it works

Always provide a `questions` array with at least one item. A single item
renders as a plain question; multiple items render as a tabbed form with
a confirmation screen at the end.

Every question MUST include:
- `type` — `yes_no`, `single_choice`, `multi_choice`, or `free_text`
- `question` — a short, direct question (one line)
- `description` — markdown context shown below the question with details,
  tradeoffs, or examples. **Always required.** Omitting it causes an error.

## Hard limits

These are enforced. Violations return an error and waste a round trip.

- **`questions` must be a JSON array**, not a string. Pass `[{...}]`, not
  `"[{...}]"`. Double-serializing the array as a string is a common mistake.
- **Max 5 choices** per question. If you have more, group or prioritize.
- **Choices required** for `single_choice` and `multi_choice`. A
  single_choice without choices is an error.
- **Description required** on every question. Keep it under 300 chars.
- **Choice descriptions** must be under 100 chars each.
- **Max 5 questions** per batch.

## Question types

- `yes_no` — confirmation only. The question must be a proposition the user
  affirms or rejects (e.g. "Proceed with deletion?", "Enable caching?").
  Never use yes_no for A-vs-B choices, preference questions, or anything
  where both answers are valid options rather than accept/reject. If the
  question has two meaningful alternatives, use `single_choice` with two
  choices instead — even when there are exactly two options.
- `single_choice` — pick one from `choices`. Use this for any selection
  between named alternatives, including binary ones like "TypeScript or
  Go?" or "Automatic or manual?". Always provide at least 2 choices.
- `multi_choice` — pick one or more from `choices`
- `free_text` — open-ended text input. Use for questions that need a
  narrative answer (e.g. "What keeps you up at night?", "Describe your
  setup"). No choices needed. Do NOT use yes_no for open-ended questions.

Single and multi choice questions automatically include a free-text
fill-in option so the user can type a custom answer. Do not add an
"Other", "Something else", or "Custom" choice manually.

## Confirmation screen (batches only)

When asking multiple questions, a confirmation tab is **always shown**
after all questions are answered. The user sees a summary of their answers
and must confirm before submitting. If they say no, they go back to editing.

**`confirm_title` and `confirm_description` are required for batches.**
Omitting either causes an error.

- `confirm_title`: a short question like "Ready to go?" or "Sound good?"
- `confirm_description`: summarize what will happen based on the expected
  answers. Write it as if you already know what they'll pick. This gives
  the user context for their confirmation decision.

## Multiple questions

When providing multiple questions, each item can include an optional
`label` (3 words max) used as the tab header. If omitted, the first 3
words of `question` are used.

Example — single question:
```json
{
  "questions": [
    {"type": "yes_no", "question": "Enable caching?", "description": "Reduces latency for repeated queries but adds invalidation complexity."}
  ]
}
```

Example — multiple questions with confirmation:
```json
{
  "questions": [
    {"label": "Database", "type": "single_choice", "question": "Which database?", "description": "PostgreSQL for relational data, MongoDB for documents.", "choices": [{"id": "pg", "label": "PostgreSQL"}, {"id": "mongo", "label": "MongoDB"}]},
    {"label": "Caching", "type": "yes_no", "question": "Enable caching?", "description": "Reduces latency for repeated queries but adds invalidation complexity."},
    {"label": "Concerns", "type": "free_text", "question": "Any concerns about this approach?", "description": "Share any reservations or edge cases we should consider."}
  ],
  "confirm_title": "Ready to configure?",
  "confirm_description": "We'll set up PostgreSQL with query caching enabled."
}
```

## When to use

- Confirm destructive or ambiguous actions
- User's request has multiple valid interpretations
- Need the user to pick from options
- Gather multiple related answers at once

## When NOT to use

- Questions answerable by reading code or docs
- Information obtainable via other tools
- Asking permission (use the permission system)
