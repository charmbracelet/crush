# Playground Component

The `playground` package implements an interactive code playground within the Blush TUI. It provides a built-in code editor, an execution environment, and basic debugging capabilities.

## `Model` Structure

The `Model` struct represents the state of the playground component:

```go
type Model struct {
	width     int
	height    int
	editor    editorModel
	output    outputModel
	focus     focusState
	keymap    keyMap
	executing bool
	language  string
	executor  *Executor
}
```

- `width`, `height`: Dimensions of the playground component.
- `editor`: The code editor sub-component (`editorModel`).
- `output`: The output display sub-component (`outputModel`).
- `focus`: Indicates which panel (editor or output) currently has focus (`focusState`).
- `keymap`: Defines the key bindings for the playground (`keyMap`).
- `executing`: A boolean indicating if code is currently being executed.
- `language`: The currently selected programming language (e.g., "go", "javascript", "python").
- `executor`: Handles the actual code execution in sandboxed environments (`Executor`).

## `focusState`

An enumeration representing the currently focused panel:

```go
const (
	editorFocus focusState = iota
	outputFocus
)
```

## `New` Function

`New()` creates and initializes a new `Model` with default settings, including an empty editor, an empty output, editor focus, and Go as the default language.

## `Init`, `Update`, `View` Methods

The `Model` implements the `tea.Model` interface:

- `Init()`: Initializes the playground. Currently, it returns `nil` as there are no initial commands.
- `Update(msg tea.Msg)`: Handles incoming `tea.Msg` messages to update the state of the playground. It processes key presses for navigation, code execution, and language switching, and delegates updates to the focused sub-component (editor or output).
- `View()`: Renders the playground's UI, including the header, editor panel, output panel, and control bar.

## `createControlBar`

This method generates the interactive control bar displayed at the bottom of the playground, containing buttons for actions like "Run", "Stop", "Debug", "Save", "Load", and "Language".

## `SetSize`

`SetSize(width, height int)`: Implements the `util.Model` interface to set the dimensions of the playground. It distributes the available width and height between the editor and output panels.

## `runCode`

`runCode()`: Executes the code currently in the editor. It sets the `executing` flag to true, appends an "Executing..." message to the output, and then calls the `Executor` to run the code in a separate goroutine. It returns `runResultMsg` or `runErrorMsg` depending on the execution outcome.

## `changeLanguage`

`changeLanguage()`: Cycles through the supported programming languages ("go", "javascript", "python"), updating the `language` field of the `Model`.

## Key Bindings

The `playground` component uses the following key bindings:

### Playground Keymap (`keyMap`)

- `ctrl+c`, `esc`: `Quit` - Exit the playground.
- `ctrl+e`: `FocusEditor` - Set focus to the code editor.
- `ctrl+o`: `FocusOutput` - Set focus to the output panel.
- `ctrl+r`: `RunCode` - Execute the code in the editor.
- `tab`: `ToggleFocus` - Toggle focus between the editor and output panels.
- `ctrl+l`: `ChangeLanguage` - Cycle through supported programming languages.

### Editor Keymap (`editorKeyMap`)

The `editorModel` (code editor) has its own set of key bindings for text manipulation:

- `enter`: `InsertNewline` - Insert a new line.
- `backspace`: `DeleteBackward` - Delete the character before the cursor.
- `delete`: `DeleteForward` - Delete the character after the cursor.
- `left`: `MoveLeft` - Move the cursor left.
- `right`: `MoveRight` - Move the cursor right.
- `up`: `MoveUp` - Move the cursor up.
- `down`: `MoveDown` - Move the cursor down.
- `home`: `MoveToStartOfLine` - Move the cursor to the beginning of the line.
- `end`: `MoveToEndOfLine` - Move the cursor to the end of the line.

## Sub-components

- **`editorModel`**: Manages the state and rendering of the interactive code editor, including cursor position and text content.
- **`outputModel`**: Handles the display of execution results and error messages, with scrolling capabilities.
- **`Executor`**: A separate component responsible for executing code in different languages (`Go`, `JavaScript`, `Python`) within temporary, sandboxed environments.
