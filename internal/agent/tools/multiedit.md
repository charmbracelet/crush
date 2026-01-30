Multiple edits to a single file in one operation. Prefer over Edit for multiple changes to same file.

<parameters>
- file_path: Absolute path (required)
- edits: Array of {old_string, new_string, replace_all?}
</parameters>

<operation>
- Edits applied sequentially in order
- Each edit operates on result of previous edit
- PARTIAL SUCCESS: If some edits fail, successful ones are kept
- Check response for failed edits and retry with corrected context
</operation>

<critical>
All Edit tool rules apply to EACH edit:
- Exact whitespace matching required
- Plan sequence: earlier edits change content that later edits must match
- Ensure each old_string is unique at its application time
</critical>
