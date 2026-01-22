Searches the full conversation transcript from a summarized session to recover specific details that were condensed in the summary.

<when_to_use>
Use this tool when you need to:
- Recover exact details that are missing from the summary
- Find specific code snippets, file paths, or commands discussed earlier
- Locate tool calls or their results from earlier in the session
- Retrieve precise error messages or decisions made earlier

DO NOT use this tool when:
- The information you need is already in the current conversation context
- You're starting a fresh session with no summarization
- You need information from a different session
</when_to_use>

<usage>
- Provide a natural language query describing the information you want
- The query is interpreted by a sub-agent that searches the transcript with grep/view
- Returns a concise answer with quoted excerpts as evidence
</usage>

<parameters>
- query: A natural language description of what you want to find (required)
</parameters>

<usage_notes>
- Only available after a session has been summarized
- The transcript contains the full conversation, including code blocks and tool calls
- The tool searches only the current session transcript
- Results include supporting excerpts to ground the answer
</usage_notes>

<limitations>
- Cannot search across multiple sessions
- Very long tool results may be truncated in the transcript
- Binary file contents are not included (only paths are recorded)
</limitations>

<tips>
- Be specific: include names, URLs, filenames, error text, or distinctive phrases
- If the first query is too broad, refine it with additional constraints
- Ask for exact quotes when you need verbatim text
</tips>

<examples>
- query: "What was the exact error message when running the tests?"
- query: "Find the implementation of the serializeTranscript function"
- query: "What file paths were modified in the refactoring?"
- query: "What approach did we try first that didn't work?"
</examples>
