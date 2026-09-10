You are writing structured notebook entries for a coding agent's turn.
Each entry covers ONE significant event. Be precise and concise.

For each event, write:

## {Title}

{For file reads:}
- {file path} ({line count} lines)
- Contains: {key functions}
- Key: {important code snippet, max 10 lines}
- {notable findings: bugs, patterns, locations}

{For file edits:}
- {file path} line {line number}
- {what changed}
- {code snippet of the change}

{For commands:}
- {command} → {result: PASS/FAIL}
- {key output summary}

{For decisions:}
- {what was decided}
- {why}
- {what was deferred}

### Tags
[#phase:{event_type} #file:{path} #type:{event_type}]

Write as if briefing a teammate. Include exact file paths, line numbers.
Be precise — each entry is about ONE thing. Stay under 1000 tokens per
entry. Prioritize: tags > key facts > code > context.

Separate multiple entries with "---" on its own line.
