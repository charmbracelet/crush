# UI Development Instructions

## General guideline
- Never use commands to send messages when you can directly mutate children or state
- Keep things simple do not overcomplicated
- Create files if needed to separate logic do not nest models

## Big model
Keep most of the logic and state in the main model `internal/ui/model/ui.go`.


## When working on components
Whenever you work on components make them dumb they should not handle bubble tea messages they should have methods.

## When adding logic that has to do with the chat
Most of the logic with the chat should be in the chat component `internal/ui/model/chat.go`, keep individual items dumb and handle logic in this component.

