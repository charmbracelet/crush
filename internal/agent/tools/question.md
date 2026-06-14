Ask the user a clarifying question and wait for their response before continuing.

<when_to_use>
Use this tool when you need to ask the user questions during execution.
This allows you to:
1. Gather user preferences or requirements.
2. Clarify ambiguous instructions.
3. Get decisions on implementation choices as you work.
4. Offer choices to the user about what direction to take.
</when_to_use>

<usage_notes>
- question: The question to ask the user. Keep it concise and specific.
- options: An optional list of 1–4 suggested answers. The user may select one or type a custom answer instead.
- If you recommend a specific option, make that the first option in the list and add "(Recommended)" at the end of the label.
- allow_multiple: Set to true when the user may select multiple provided options and/or add a custom free-text answer.
</usage_notes>

<behavior>
The tool blocks until the user responds. Single-answer questions return the
answer as plain text. Multiple-answer questions return a JSON array of selected
option strings, with a custom free-text answer appended when provided.
</behavior>
