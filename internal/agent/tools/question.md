Ask the user a structured question and wait for their response. Use this
when you need clarification, confirmation, or a choice before proceeding.

## How it works

Provide a `questions` array with at least one item; multiple items
render as a tabbed form ending in a confirmation screen.

Every question MUST include:

- `type` — `yes_no`, `single_choice`, `multi_choice`, or `free_text`
- `question` — a short, direct question (one line)
- `description` — markdown context shown below the question with details,
  tradeoffs, or examples. **Always required.** Omitting it causes an error.

## Hard limits

These are enforced. Violations return an error and waste a round trip.

- **Max 5 choices** per question. If you have more, group or prioritize.
- **Choices required** for `single_choice` and `multi_choice`. A
  single_choice without choices is an error.
- **Description required** on every question. Keep it under 300 chars.
- **Choice descriptions** must be under 100 chars each.
- **Max 5 questions** per batch. If you need more, split into multiple
  batches and tell the user there will be follow-up questions.

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

## Multiple questions

Each item can include an optional `label` (3 words max) used as the tab
header; if omitted, the first 3 words of `question` are used. For
batches, `confirm_title` is a short question like "Ready to go?" and
`confirm_description` should summarize what will happen based on the
expected answers, written as if you already know what they'll pick.

Example:

```json
{
	"questions": [
		{
			"type": "yes_no",
			"question": "Enable caching?",
			"description": "Reduces latency for repeated queries but adds invalidation complexity."
		}
	]
}
```

Batched example (tabbed form + confirmation screen):

```json
{
	"questions": [
		{
			"type": "single_choice",
			"question": "Which database?",
			"label": "Database",
			"description": "Determines the driver and migration target.",
			"choices": [
				{"id": "sqlite", "label": "SQLite"},
				{"id": "postgres", "label": "PostgreSQL"}
			]
		},
		{
			"type": "yes_no",
			"question": "Enable foreign keys?",
			"description": "Recommended unless bulk import order is unmanaged."
		}
	],
	"confirm_title": "Ready to go?",
	"confirm_description": "Creates the database with the chosen engine and settings."
}
```

## When to use

- Confirm destructive or ambiguous actions, or pick between
  interpretations
- Gather multiple related answers at once
- NOT for questions answerable by reading code or docs, information
  obtainable via other tools, or permission requests (use the
  permission system)
