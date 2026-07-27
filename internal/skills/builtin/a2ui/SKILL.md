---
name: a2ui
description: Use when the user asks you to output, speak in, or communicate using the A2UI (a2tea) format, or when you need to understand how to construct A2UI JSON components to render interactive terminal UIs for the user.
user-invocable: true
---

# A2UI / a2tea Communication Guide

Crush supports rendering rich, interactive terminal UI components embedded directly in your markdown responses using **a2tea** (a bridge to the [A2UI](https://a2ui.org) protocol).

You can emit an A2UI surface when a compact visual genuinely helps — such as a status card, an option list, a progress readout, or a form. Most of your replies should remain prose, but when a visual structure adds value, use A2UI.

## Wire Format

Embed a single inline `<a2ui-json>{...}</a2ui-json>` block in your response. Inside, provide an A2UI v0.9 payload containing an `updateComponents` message.

### Example Payload

```json
<a2ui-json>{
  "version": "v0.9",
  "updateComponents": {
    "surfaceId": "s1",
    "components": [
      {
        "component": "Card",
        "id": "root",
        "child": "col"
      },
      {
        "component": "Column",
        "id": "col",
        "children": ["title", "body", "btn-ok"]
      },
      {
        "component": "Text",
        "id": "title",
        "variant": "h2",
        "text": "Build passed"
      },
      {
        "component": "Text",
        "id": "body",
        "text": "142 tests, 0 failures."
      },
      {
        "component": "Button",
        "id": "btn-ok",
        "child": "btn-ok-label"
      },
      {
        "component": "Text",
        "id": "btn-ok-label",
        "text": "Acknowledge"
      }
    ]
  }
}</a2ui-json>
```

## Component Architecture

A2UI components live in a flat list (adjacency list) and reference their children by `id`. The renderer resolves the tree from the root (the component that nothing else references as a child).

### Core Catalog (Fully Supported)

These components render beautifully with full styling and layout:
- `Text`: Text display. Options: `text` (string), `variant` (h1-h5, caption).
- `Card`: A rounded border container. Expects a single `child` ID.
- `Column`: Lays out children vertically. Expects a `children` array of IDs.
- `Row`: Lays out children horizontally. Expects a `children` array of IDs.
- `List`: Lays out children as a list. Expects a `children` array of IDs.
- `Divider`: A horizontal rule.
- `Button`: Focusable button. Its label comes from a single `child` ID pointing at a `Text` component (there is no `label` field). When the user presses it, you receive a new message naming the pressed button and carrying the surface's current field values — a button whose id reads as a cancel (e.g. `btn-cancel`, `dismiss`, `close`) just dismisses the form instead.

### Input Components (Editable)

These components render their current values and the user can edit them; their values (keyed by component `id`) are sent back to you when a button on the surface is pressed:
- `TextField`
- `CheckBox`
- `ChoicePicker`
- `Slider`
- `DateTimeInput`

### Media & Layout Placeholders

- `Image`, `Icon`, `Video`, `AudioPlayer`: Render as compact one-line placeholders.
- `Tabs`: Render the title bar and only the *first* tab's content.
- `Modal`: Renders only its trigger, content stays hidden.

## Best Practices

1. **Keep it compact:** A2UI surfaces should be concise summaries, controls, or dashboards. Use standard markdown fences for code or long logs.
2. **One surface per block:** Provide all components inside the `components` array of a single `updateComponents` message.
3. **Link correctly:** Ensure every ID in `child` or `children` corresponds to a component in your array. Dangling IDs will break the render.
4. **Mix with Markdown:** You can put markdown text before and after the `<a2ui-json>` block.

## Common Mistakes (Anti-Patterns)

These are errors that models frequently make. Read carefully.

### Buttons do NOT have a `text` or `label` field

**NEVER put `text` or `label` on a `Button`** — the A2UI spec has **no** `text`, `label`, or `name` field on `Button` in any schema version. A button's label comes exclusively from its `child` ID pointing at a separate `Text` component. A stray `text`/`label` key is dropped at parse time and the button renders unlabeled.

**Wrong — the label will be empty:**
```json
{
  "component": "Button",
  "id": "btn",
  "text": "Submit"
}
```
The `"text"` key is silently ignored and the button renders with no visible label.

**Wrong — `label` is not a real field either:**
```json
{
  "component": "Button",
  "id": "btn",
  "label": "Submit"
}
```

**Correct — use a child Text component:**
```json
{
  "component": "Button",
  "id": "btn",
  "child": "btn-label"
},
{
  "component": "Text",
  "id": "btn-label",
  "text": "Submit"
}
```

**Complete form template — copy this shape.** Two inputs and two buttons; note how *each* button gets its own child `Text` for its label:

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

### Every `child` / `children` ID must exist in the components array

**Wrong — dangling reference:**
```json
{
  "component": "Card",
  "id": "root",
  "child": "missing-id"
}
```
There is no component with `id: "missing-id"` in the array, so the card renders `[a2tea: missing component "missing-id"]`.

**Correct:** Always include a matching component for every referenced ID.

### Do not nest components inline

Components are a **flat list** (adjacency list). Do not embed component objects inside other components.

**Wrong — inline nesting:**
```json
{
  "component": "Card",
  "id": "root",
  "child": {
    "component": "Text",
    "text": "Hello"
  }
}
```
The `child` field is a **string ID**, not an object. Inline nesting will break the parser.

**Correct:** Define each component separately and link by ID:
```json
{
  "component": "Card",
  "id": "root",
  "child": "body"
},
{
  "component": "Text",
  "id": "body",
  "text": "Hello"
}
```

### Do not put code in an A2UI surface

A2UI surfaces are for compact visual structures. Use standard markdown fenced code blocks (`` ``` ``) for code snippets, command output, or logs. Do not try to render code inside a `Text` component — it will not syntax-highlight.
