Editor navigation tool. Call this when the user asks you to show them a list of code or text locations (e.g. "where is X used", "show me all the places that...").

The attached editor (e.g. Neovim) opens a navigable picker with:
- Left pane: list of locations as filename:line.
- Right pane: live preview of the file under the selected location.
- Bottom pane: your explanation of why each location matters.

Each item must include:
- filename: absolute or workspace-relative path.
- lnum: 1-indexed line number.
- text: the relevant snippet at that location.
- note: YOUR explanation of WHY this location is relevant for the user's question. This is what the picker shows in the explanation pane and is the most important field — be specific.
- type: optional severity, one of N (note), I (info), W (warning), E (error). Defaults to N.

Use this AFTER analyzing code, to present findings with reasoning, rather than dumping raw filenames. Only available when Crush was launched from inside an editor that supports the bridge.
