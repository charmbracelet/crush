Ask user questions to gather information, clarify ambiguity, taking decisions,
determining user preferences and/or taste. When in doubt, use this tool to ask the user.

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

<when_to_use>
- Ask user questions during execution
- Gather user preferences or requirements
- Clarifying ambiguous instructions
- Get decisions on implementation choices as you work
- Offer choices to the user about what direction to take
- Determine user taste
</when_to_use>

<tips>
- If you recommend specific options, put them first in array of options and append "(Recommended)" to label
- To let user pick something else, offer "None of the above" option
- When planning
  - use this tool to clarify requirements or choose between approaches BEFORE finalizing your plan.
  - IMPORTANT: do not ask for feedback about the plan while you are working on it, because user cannot see it until you send it back 
</tips>

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
