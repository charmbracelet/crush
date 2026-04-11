Ask user questions to gather information, clarify ambiguity, making implementation decisions,
determining user preferences and/or taste. When in doubt, ask the user.

<usage>
- Provide array of questions to ask user
- Each question needs:
  - UUID to correlate originating question and answer
  - short question text to ask user
  - array of options from which user will select one or more answers
  - boolean indicating if user can answer selecting multiple options
- Each option needs:
  - short label identifying the option
  - optional description of the option
</usage>

<features>
- More questions can be asked to user at once
- User can be allowed to select multiple options when answering
</features>

<limitations>
- Keep question and option labels short
- If option's description is too long, it will be truncated
</limitations>

<result>
- Tool will report back array of answers from user
- Each answer will include:
  - UUID of originating question
  - array of options' labels selected by user
</result>
