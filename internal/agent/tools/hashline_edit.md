Apply line-addressable edits using hashline references from `view(hashline=true)`.

Use this tool when text matching is brittle and you want edits anchored to specific lines with staleness protection.

<workflow>
1. Read file with `view` and set `hashline=true`
2. Build one or more operations using `LINE#HASH` references from that output
3. Submit all related operations together in one call
</workflow>

<parameters>
1. `file_path` (required): absolute path to the file to modify
2. `operations` (required): ordered list of operations
</parameters>

<operation_types>
Each operation must include `operation` and `content` unless noted.

1. `replace_line`
   - Required fields: `line`, `content`
   - Replaces the referenced line with `content` (single or multiple lines)

2. `replace_range`
   - Required fields: `start`, `end`, `content`
   - Replaces inclusive range `start...end` with `content` (can be empty to delete range)

3. `prepend`
   - Required fields: `line`, `content`
   - Inserts `content` before the referenced line

4. `append`
   - Required fields: `line`, `content`
   - Inserts `content` after the referenced line
</operation_types>

<line_reference_format>
Use `LINE#HASH` references exactly as returned by `view(hashline=true)`.
Examples: `5#aa`, `142#QZ`

- `LINE` is 1-based line number from the viewed file
- `HASH` is the 2-character hash token for that line
- Hash mismatch means file changed; re-run `view(hashline=true)` and retry
</line_reference_format>

<behavior>
- All references are validated before writing
- Operations are applied sequentially in one atomic write
- Later operations correctly account for line shifts caused by earlier operations
- If any operation is invalid, no file changes are written
</behavior>

<warnings>
- Do not invent hashes
- Do not use stale references from older file views
- Keep all related edits in one call to avoid avoidable staleness failures
</warnings>

<example>
```json
{
  "file_path": "/repo/main.go",
  "operations": [
    {
      "operation": "replace_line",
      "line": "12#PV",
      "content": "func main() {"
    },
    {
      "operation": "append",
      "line": "12#PV",
      "content": "\tlog.Println(\"started\")"
    },
    {
      "operation": "replace_range",
      "start": "40#MW",
      "end": "44#QH",
      "content": "\treturn nil"
    }
  ]
}
```
</example>
