Ask the user one or more multiple-choice questions to gather information, clarify requirements, or make decisions.

<when_to_use>
Use this tool when you need user input to proceed:

- Clarifying ambiguous instructions or requirements
- Gathering user preferences for implementation choices
- Getting decisions on architectural or design approaches
- Offering choices about which direction to take
- Confirming assumptions before making significant changes
</when_to_use>

<when_not_to_use>
Do NOT use this tool when:

- The answer is clearly stated in the user's request
- You can make a reasonable decision based on best practices
- The question is purely informational (just answer directly)
- You're asking about something trivial that doesn't affect the outcome
</when_not_to_use>

<guidelines>
- Use concise, clear question text
- Provide meaningful options that cover common choices
- Headers should be brief identifiers (max 12 chars): "Framework", "Auth", "Database"
- Use multi_select only when multiple answers make sense together
- Limit to 1-4 questions per call to avoid overwhelming the user
- Each option should have 2-4 choices
- Users can always select "Other" to provide a custom response
</guidelines>

<parameters>
**questions** (array, 1-4 items): Questions to ask the user

Each question has:
- **question** (string): The full question text
- **header** (string): Short label, max 12 characters
- **options** (array, 2-4 items): Available choices
  - **label** (string): Display text for the option (1-5 words)
  - **description** (string, optional): Additional context about the option
- **multi_select** (boolean, optional): Allow multiple selections if true
</parameters>

<examples>
Single question, single select:
```json
{
  "questions": [{
    "question": "Which authentication method should we use?",
    "header": "Auth",
    "options": [
      {"label": "JWT tokens", "description": "Stateless, good for APIs"},
      {"label": "Session cookies", "description": "Traditional, server-side state"},
      {"label": "OAuth 2.0", "description": "Third-party authentication"}
    ]
  }]
}
```

Multiple questions:
```json
{
  "questions": [
    {
      "question": "Which database should we use?",
      "header": "Database",
      "options": [
        {"label": "PostgreSQL"},
        {"label": "MySQL"},
        {"label": "SQLite"}
      ]
    },
    {
      "question": "Which features should be enabled?",
      "header": "Features",
      "multi_select": true,
      "options": [
        {"label": "Logging"},
        {"label": "Metrics"},
        {"label": "Tracing"}
      ]
    }
  ]
}
```
</examples>

<behavior>
- The tool blocks execution until the user responds
- User can always provide a custom "Other" response
- Returns structured answers that you can use to proceed with implementation
</behavior>
