---
name: browser-verify
description: Use when the user asks to test, verify, inspect, or debug a web UI with Playwright/browser MCP, screenshots, console logs, network requests, forms, or responsive behavior.
---

# Browser Verification

Verify the real page, not assumptions from source code.

## Workflow

1. Confirm the target URL or local dev server.
2. Open the page and wait for the relevant UI state.
3. Capture a DOM snapshot before interacting.
4. Inspect console and network errors when behavior is broken or uncertain.
5. Interact like a user: click, type, select, submit, navigate.
6. Use screenshots for visual regressions, layout, canvas, media, or responsive checks.
7. Re-check the state after each material interaction.

## Reporting

- Say exactly what was verified.
- Include blocking errors and the URL or state where they happened.
- Do not claim browser verification passed if navigation, auth, assets, or tooling failed.

## Good Defaults

- Desktop viewport first for functional flows.
- Mobile viewport when layout, touch, keyboard, or responsive behavior matters.
- Console and network logs whenever a page appears blank, stale, or partially loaded.
