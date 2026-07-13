Detach one or more sources from the current session.

Use this tool when the user says `remove_source`, asks to detach a source, or
no longer wants a source shown in the sidebar. If the request is vague, call
`sources` with `action=list`, infer the intended entry from its compact
label/path/URL metadata, and pass a specific ID, label, or unique fragment.
Ambiguous fragments are rejected instead of removing the wrong item. This
removes only the session reference and never deletes the underlying file or
modifies the URL.
