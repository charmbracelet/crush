---
name: a2ui
description: Use when the user asks you to output, speak in, or communicate using the A2UI (a2tea) format, or when you need to understand how to construct A2UI JSON components to render interactive terminal UIs for the user.
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
- `Button`: Focusable button. Its label comes from a single `child` ID pointing at a `Text` component (there is no `label` field). Pressing Enter emits a click event to the host.

### Input Components (Read-Only Visuals)

These components will render their current values, but currently do not allow the user to interactively edit them or send input back to the agent:
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
