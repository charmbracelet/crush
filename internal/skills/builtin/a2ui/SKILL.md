---
name: a2ui
description: Use when the user asks you to output, speak in, or communicate using the A2UI (a2tea) format, or when you need to understand how to construct A2UI JSON components to render interactive terminal UIs for the user.
---

# A2UI / a2tea Communication Guide

Crush renders rich, interactive terminal UI from **A2UI** (the [A2UI protocol](https://a2ui.org), via the a2tea bridge). A surface is a compact visual — a status card, an option list, a progress readout, a short form. Most replies should stay prose; reach for a surface only when the structure genuinely helps.

## Two delivery paths — pick the right one

There are two ways an A2UI surface reaches the user. **Prefer reading an MCP resource when the data lives on a server**; emit inline JSON only for ad-hoc visuals no server provides.

### 1. Read an MCP `/a2ui` resource (preferred for server data)

Servers that own data (a todo queue, a bundle, a trace, a dashboard) expose **A2UI resource templates** — URIs ending in `/a2ui` — that render their data as a surface. Read one with `read_mcp_resource`:

```
read_mcp_resource(mcp_name="todos", uri="todos://queue/<name>/a2ui")
```

What happens:
- The host renders the surface for the **user** (audience `user`) and hands **you** a one-line placeholder, e.g. `[A2UI surface rendered in the chat UI from todos://queue/<name>/a2ui — the user can already see it; do not repeat or echo its JSON payload]`.
- You do **not** receive the surface JSON. **Never re-echo, re-emit, or paraphrase the surface** — the user can already see it. Continue in prose.
- Resource templates are listed by `list_mcp_resources`. A template with `{id}` is filled by you (e.g. `.../artifact/<id>/a2ui`); one with `{?w}` accepts an optional width hint the host manages — **do not** append `?w=` yourself.

Use this path whenever a server offers an `/a2ui` view of its data. It is richer (server-rendered, sized to the host) and cheaper (no JSON through your context) than reconstructing the same view inline.

### 2. Emit your own inline surface (ad-hoc visuals)

For a visual no server provides, embed a single inline `<a2ui-json>{...}</a2ui-json>` block containing one `updateComponents` message:

```json
<a2ui-json>{
  "version": "v0.9",
  "updateComponents": {
    "surfaceId": "s1",
    "components": [
      {"component": "Card", "id": "root", "child": "col"},
      {"component": "Column", "id": "col", "children": ["title", "body", "btn-ok"]},
      {"component": "Text", "id": "title", "variant": "h2", "text": "Build passed"},
      {"component": "Text", "id": "body", "text": "142 tests, 0 failures."},
      {"component": "Button", "id": "btn-ok", "child": "btn-ok-label"},
      {"component": "Text", "id": "btn-ok-label", "text": "Acknowledge"}
    ]
  }
}</a2ui-json>
```

This is the **chat-scanned** path: the surface is part of your reply. Use it for summaries, dashboards, and one-shot forms you author yourself.

## Component architecture

Components live in a **flat adjacency list** and reference children by `id`. The renderer resolves the tree from the root (the component nothing else references as a child).

### Core catalog (fully supported)

- `Text`: text display. Fields: `text` (string), `variant` (`h1`–`h5`, `caption`; omit for body).
- `Card`: rounded-border container. Single `child` id.
- `Column`: lays out `children` (array of ids) vertically.
- `Row`: lays out `children` horizontally.
- `List`: lays out `children` as a list (divider between rows).
- `Divider`: a horizontal rule.
- `Button`: focusable button. Its label is a single `child` id pointing at a `Text` component — there is **no** `label`/`text` field on `Button`. On press, the surface's current field values come back to you; a button whose id reads as cancel (`btn-cancel`, `dismiss`, `close`) just dismisses the form.

### Input components (editable)

Render their current value; the user can edit; values (keyed by component `id`) return when a button on the surface is pressed:
- `TextField`, `CheckBox`, `ChoicePicker`, `Slider`, `DateTimeInput`

### Media & layout placeholders

- `Image`, `Icon`, `Video`, `AudioPlayer`: compact one-line placeholders.
- `Tabs`: renders the title bar and only the *first* tab's content.
- `Modal`: renders only its trigger; content stays hidden.

## Forms (the submission loop)

A form is a surface with input components plus a `Button`. When the user presses the button you receive a message naming the button and carrying the field values keyed by component `id`. A cancel-reading button dismisses without a submission.

**Complete form template — copy this shape.** Two inputs, two buttons; each button gets its own child `Text` label:

```json
<a2ui-json>{
  "version": "v0.9",
  "updateComponents": {
    "surfaceId": "form",
    "components": [
      {"component": "Card", "id": "root", "child": "col"},
      {"component": "Column", "id": "col", "children": ["name", "email", "actions"]},
      {"component": "TextField", "id": "name", "label": "Name"},
      {"component": "TextField", "id": "email", "label": "Email"},
      {"component": "Row", "id": "actions", "children": ["btn-send", "btn-cancel"]},
      {"component": "Button", "id": "btn-send", "child": "btn-send-label"},
      {"component": "Text", "id": "btn-send-label", "text": "Send"},
      {"component": "Button", "id": "btn-cancel", "child": "btn-cancel-label"},
      {"component": "Text", "id": "btn-cancel-label", "text": "Cancel"}
    ]
  }
}</a2ui-json>
```

## A2UI over MCP (the contract)

When you build or reason about server-side surfaces, the A2UI-over-MCP contract (https://a2ui.org/guides/a2ui_over_mcp/) governs:

- **MIME type** `application/a2ui+json` (legacy `application/json+a2ui`) identifies a surface, delivered by `resources/read` (an `/a2ui` resource) or by a `tools/call` result carrying an `EmbeddedResource` of that type.
- **Audience**: a surface annotated `["user"]` renders for the human and its raw JSON is kept out of your context; `["assistant"]` is for your reasoning and is not rendered; empty means both.
- **Interactivity round-trip**: a surface's `Button` carries an `action` = `{event: {name, context}}`. When the user clicks, the host calls the owning server's `a2ui_action` tool with `{name, context}` (data bindings resolved against surface state) and feeds any returned A2UI payload back into the **same** surface — no agent turn. Render failures go back via an `a2ui_error` tool call.
- **Width hint**: a server may size pre-rendered geometry (e.g. a flame graph) to an optional `?w=N` hint the **host** appends. Never add `?w=` yourself; only templates declaring `{?w}` accept it.

You normally only *consume* server surfaces (path 1). Build them only when you are authoring an MCP server.

## Best practices

1. **Keep it compact.** Surfaces are concise summaries, controls, or dashboards. Use fenced code blocks for code or long logs.
2. **One surface per block.** All components inside the `components` array of a single `updateComponents` message.
3. **Link correctly.** Every `child`/`children` id must exist in your array — a dangling id renders `[a2tea: missing component ...]`.
4. **Mix with Markdown.** Prose before and after the `<a2ui-json>` block is fine.
5. **Don't re-emit server surfaces.** If you read an `/a2ui` resource, the user already sees it — move on in prose.

## Common mistakes (anti-patterns)

### Buttons do NOT have a `text` or `label` field

**NEVER put `text`/`label` on a `Button`** — the spec has no such field; the label comes only from a child `Text`. A stray key is dropped and the button renders unlabeled.

**Wrong — the label will be empty:**
```json
{"component": "Button", "id": "btn", "text": "Submit"}
```

**Correct — a child Text component:**
```json
{"component": "Button", "id": "btn", "child": "btn-label"},
{"component": "Text", "id": "btn-label", "text": "Submit"}
```

### Every `child` / `children` id must exist

**Wrong — dangling reference:**
```json
{"component": "Card", "id": "root", "child": "missing-id"}
```
No component `missing-id` exists, so the card renders `[a2tea: missing component "missing-id"]`. Always include a matching component for every referenced id.

### Do not nest components inline

Components are a **flat list**; `child` is a **string id**, not an object.

**Wrong — inline nesting:**
```json
{"component": "Card", "id": "root", "child": {"component": "Text", "text": "Hello"}}
```

**Correct — define separately, link by id:**
```json
{"component": "Card", "id": "root", "child": "body"},
{"component": "Text", "id": "body", "text": "Hello"}
```

### Do not put code in a surface

A2UI surfaces are for compact visual structure. Use fenced code blocks (`` ``` ``) for code, command output, or logs — a `Text` component will not syntax-highlight.
