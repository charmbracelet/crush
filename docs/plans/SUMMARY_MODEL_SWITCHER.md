# Summary Model Switcher — Implementation Plan - Implemented

## Goal

Add a new slash command **"Switch Model Summary"** to the `/` command
palette. When selected, it opens a model picker showing all
available/connected models. The chosen model becomes the notebook
summary model — used by the notebook generator for context entries.

No changes to the existing large/small model dialog. The summary model
is a separate slot selected through a separate command.

## Current State

- The notebook generator uses the **small model** (wired once via
  `sync.Once` in `coordinator.go:743-750`)
- The small model is shared between title generation, sub-agents, and
  notebook generation
- There's no way to switch the notebook model independently
- The `sync.Once` capture means runtime model switches don't update
  the notebook resolver (pre-existing bug)

## Design

### User flow

```
User types / in the editor
  → Command palette opens
  → User sees "Switch Model Summary" in the list
  → User selects it
  → Model picker dialog opens (all available models)
  → User picks a model
  → Model saved as summary model in config
  → Notebook generator updated to use the new model
  → Confirmation message shown
  → Dialog closes (same as Switch Model)
```

### What changes

| Component              | Change                                                                                                                                           |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `config.go`            | Add `SelectedModelTypeSummary = "summary"`, update stale comment                                                                                 |
| `commands.go` (dialog) | Add "Switch Model Summary" command item                                                                                                          |
| `models.go` (dialog)   | Add `id` field, `summaryMode` flag, `ModelTypeSummary`, hide toggle/help in summary mode                                                         |
| `ui.go`                | Handle the new command, open model dialog in summary mode, close correct dialog ID, guard auto-set-small                                         |
| `coordinator.go`       | Extract shared `buildSelectedModel` from `buildAgentModels`, resolve summary model in `UpdateModels`, use `*csync.Value[Model]` for live updates |
| `shellconfig/model.go` | Add `model summary` crushrc command (for config files)                                                                                           |
| `schema.json`          | Regenerate via `task schema` only if annotations change                                                                                          |

### Fallback chain

```
notebook generator needs model
  → summary slot absent? use small model (existing behavior)
  → summary slot present but invalid? log error, keep previous summary model,
    do NOT block the primary agent
  → summary slot present and valid? use it
```

The summary model is **auxiliary** — it must never prevent the
primary coding agent from running. `UpdateModels` is called before
every agent run (`coordinator.go:298-301`) and aborts on error.
Therefore summary resolution failures are logged but never returned
from `UpdateModels`. The previous live summary model is preserved so
the notebook generator continues working with the last known-good
model.

## Implementation Steps

### Step 1: Add `SelectedModelTypeSummary` and update comment

**File**: `internal/config/config.go`

```go
const (
    SelectedModelTypeLarge   SelectedModelType = "large"
    SelectedModelTypeSmall   SelectedModelType = "small"
    SelectedModelTypeSummary SelectedModelType = "summary" // NEW
)
```

Update the stale comment at line 746:

```go
// Before:
// We currently only support large/small as values here.
Models map[SelectedModelType]SelectedModel `json:"models,omitempty" ...`

// After:
// Supported model slots: large, small, summary.
Models map[SelectedModelType]SelectedModel `json:"models,omitempty" ...`
```

No `SummaryModel()` helper needed — the coordinator resolves it with
fallback to small.

### Step 2: Add `model summary` crushrc command

**File**: `internal/shellconfig/model.go`

Add `"summary"` to the switch in `handleModel`:

```go
case "large", "small", "summary":  // add "summary"
    return modelSelect(b, args, stdout, stderr)
```

Update usage string. No other changes — `modelSelect` is slot-agnostic
(`slot := args[1]`).

### Step 3: Add "Switch Model Summary" to command palette

**File**: `internal/ui/dialog/commands.go`

In `defaultCommands()`, add after "Switch Model". The command
includes a dynamic description that warns when no summary model is
configured:

```go
// Build description based on whether a summary model is set.
summaryDesc := ""
if _, ok := cfg.Models[config.SelectedModelTypeSummary]; !ok {
    summaryDesc = "No summary model set — using small model. Select to configure."
} else {
    summaryDesc = "Choose a model for notebook summary generation"
}

commands = append(commands, NewCommandItem(c.com.Styles, "switch_summary_model", "Switch Model Summary", "", ActionOpenDialog{
    DialogID: SummaryModelsID,
}).WithDescription(summaryDesc))
```

The description appears below the command title in the palette
(`commands_item.go:144-156`). When no summary model is configured,
the user sees the warning inline. Once configured, it shows a
neutral description. No persistent notification, no startup
interruption — the warning appears exactly when the user is looking
at the command.

### Step 4: Add `SummaryModelsID`, `id` field, and summary dialog mode

**File**: `internal/ui/dialog/models.go`

#### 4a: Add dialog ID constant

```go
const SummaryModelsID = "summary-models"
```

#### 4b: Add `id` field to `Models` struct

The current `ID()` always returns `ModelsID` (line 165-167). Dialog
operations (`ContainsDialog`, `BringToFront`, `CloseDialog`) are
entirely based on `Dialog.ID()` (`dialog.go:93-105`). Without an
overridable ID, the summary dialog would identify itself as
`"models"`, causing:

- `ContainsDialog(SummaryModelsID)` to always return false
- Duplicate stacking on reopen
- The normal model dialog treating the summary dialog as itself

Add an `id string` field initialized to `ModelsID`, and have `ID()`
return it:

```go
type Models struct {
    // ... existing fields ...
    id          string  // NEW — dialog ID, defaults to ModelsID
    summaryMode bool   // NEW — when true, dialog selects summary model
}
```

Update `ID()`:

```go
// ID implements Dialog.
func (m *Models) ID() string {
    return m.id
}
```

Set `id` in `NewModels`:

```go
func NewModels(com *common.Common, isOnboarding bool) (*Models, error) {
    // ... existing setup ...
    m.id = ModelsID  // NEW — default
    // ... rest of constructor ...
}
```

#### 4c: Add summary constructor

`NewModels` calls `setProviderItems()` at line 157 while `modelType`
is still its zero value (`ModelTypeLarge`). If `NewSummaryModels`
calls `NewModels` then changes `modelType` and calls
`setProviderItems()` again, the first call may mutate
`recent_models.large` (line 481) merely because the user opened the
summary picker. Use a private constructor that sets the ID, model
type, and summary mode before the single `setProviderItems()` call:

```go
// newModelsInternal is the shared constructor for NewModels and
// NewSummaryModels. It sets the dialog ID, model type, onboarding
// state, and summary mode before calling setProviderItems() exactly
// once, avoiding spurious recent-model mutations.
func newModelsInternal(
    com *common.Common,
    id string,
    modelType ModelType,
    isOnboarding bool,
    summaryMode bool,
) (*Models, error) {
    t := com.Styles
    m := &Models{}
    m.com = com
    m.isOnboarding = isOnboarding
    m.id = id
    m.modelType = modelType
    m.summaryMode = summaryMode

    help := help.New()
    help.Styles = t.DialogHelpStyles()
    m.help = help

    m.list = NewModelsList(t)
    m.list.Focus()
    m.list.SetSelected(0)

    m.input = textinput.New()
    m.input.SetVirtualCursor(false)
    if isOnboarding {
        m.input.Placeholder = onboardingModelInputPlaceholder
    } else {
        m.input.Placeholder = modelType.Placeholder()
    }
    m.input.SetStyles(com.Styles.TextInput)
    m.input.Focus()

    // ... key bindings setup (same as existing NewModels) ...

    var err error
    m.providers, err = config.Providers(m.com.Config())
    if err != nil {
        if len(m.providers) == 0 {
            return nil, fmt.Errorf("failed to get providers: %w", err)
        }
        slog.Warn("Listing the previously known providers", "error", err)
    }

    if err := m.setProviderItems(); err != nil {
        return nil, fmt.Errorf("failed to set provider items: %w", err)
    }

    return m, nil
}

func NewModels(com *common.Common, isOnboarding bool) (*Models, error) {
    return newModelsInternal(com, ModelsID, ModelTypeLarge, isOnboarding, false)
}

// NewSummaryModels creates a model selection dialog for the summary
// model. It's like NewModels but without the large/small toggle —
// just a flat list of all available models.
func NewSummaryModels(com *common.Common) (*Models, error) {
    return newModelsInternal(com, SummaryModelsID, ModelTypeSummary, false, true)
}
```

This replaces the existing `NewModels` body with a call to
`newModelsInternal`, preserving all existing behavior while ensuring
`setProviderItems()` runs exactly once with the correct model type.

#### 4d: Add `ModelTypeSummary` to the enum

```go
const (
    ModelTypeLarge ModelType = iota
    ModelTypeSmall
    ModelTypeSummary  // NEW
)
```

Update `Config()`:

```go
case ModelTypeSummary:
    return config.SelectedModelTypeSummary
```

Update `String()`:

```go
case ModelTypeSummary:
    return "Summary"
```

Update `Placeholder()`:

```go
case ModelTypeSummary:
    return summaryModelInputPlaceholder
```

Add placeholder:

```go
summaryModelInputPlaceholder = "Choose a model for notebook summary generation"
```

### Step 5: Hide large/small toggle and Tab help in summary mode

**File**: `internal/ui/dialog/models.go`

#### 5a: Skip tab toggle in summary mode

In `HandleMsg`:

```go
case key.Matches(msg, m.keyMap.Tab):
    if m.isOnboarding || m.summaryMode {  // add m.summaryMode
        break
    }
    // ... existing toggle logic ...
```

#### 5b: Hide radio view in summary mode

In `Draw`:

```go
if !m.isOnboarding && !m.summaryMode {  // add m.summaryMode check
    rc.TitleInfo = m.modelTypeRadioView()
}
```

#### 5c: Change title in summary mode

```go
if m.summaryMode {
    rc.Title = "Switch Model Summary"
} else {
    rc.Title = "Switch Model"
}
```

#### 5d: Exclude Tab from help in summary mode

`ShortHelp()` (line 309-327) always includes `m.keyMap.Tab` for
non-onboarding dialogs. Exclude it when `summaryMode` is true:

```go
func (m *Models) ShortHelp() []key.Binding {
    if m.isOnboarding {
        return []key.Binding{
            m.keyMap.UpDown,
            m.keyMap.Select,
        }
    }
    h := []key.Binding{
        m.keyMap.UpDown,
    }
    if !m.summaryMode {  // NEW — hide Tab in summary mode
        h = append(h, m.keyMap.Tab)
    }
    h = append(h, m.keyMap.Select)
    if m.isSelectedConfigured() {
        h = append(h, m.keyMap.Edit)
    }
    h = append(h, m.keyMap.Close)
    return h
}
```

### Step 6: Handle summary model selection, dialog close, and auto-set-small guard in UI

**File**: `internal/ui/model/ui.go`

#### 6a: Open the summary dialog

In `openDialog`, add the summary case:

```go
case dialog.SummaryModelsID:
    if cmd := m.openSummaryModelsDialog(); cmd != nil {
        cmds = append(cmds, cmd)
    }
```

Add the opener:

```go
func (m *UI) openSummaryModelsDialog() tea.Cmd {
    if m.dialog.ContainsDialog(dialog.SummaryModelsID) {
        m.dialog.BringToFront(dialog.SummaryModelsID)
        return nil
    }
    modelsDialog, err := dialog.NewSummaryModels(m.com)
    if err != nil {
        return util.ReportError(err)
    }
    m.dialog.OpenDialog(modelsDialog)
    return nil
}
```

#### 6b: Close the correct dialog after selection

`handleSelectModel` hardcodes `m.dialog.CloseDialog(dialog.ModelsID)`
in two places (~line 2350 for re-auth, ~line 2395 for success). Since
the summary dialog now correctly returns `SummaryModelsID` from
`ID()`, those calls won't close it.

`CloseDialog` is a no-op on non-open IDs (`dialog.go:151-158` — it
loops and returns if the ID isn't found). Close both IDs
unconditionally at each close site:

```go
// At ~line 2350 (re-auth branch):
m.dialog.CloseDialog(dialog.ModelsID)
m.dialog.CloseDialog(dialog.SummaryModelsID)  // NEW

// At ~line 2395 (success path):
m.dialog.CloseDialog(dialog.ModelsID)
m.dialog.CloseDialog(dialog.SummaryModelsID)  // NEW
```

#### 6c: Guard auto-set-small for summary selection

The current auto-set-small logic at lines 2367-2373 runs for **all**
model types, not just large — only `applyThemeForProvider` is inside
the large-model condition. If a user selects a summary model and
small is unset, the current code would auto-set small from the
summary provider's default, which is wrong.

Move the small-init inside the large-model condition:

```go
// Before (lines 2360-2373):
if msg.ModelType == config.SelectedModelTypeLarge {
    m.applyThemeForProvider(providerID)
}
if _, ok := cfg.Models[config.SelectedModelTypeSmall]; !ok {
    smallModel := m.com.Workspace.GetDefaultSmallModel(providerID)
    if err := m.com.Workspace.UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeSmall, smallModel); err != nil {
        cmds = append(cmds, util.ReportError(err))
    }
}

// After:
if msg.ModelType == config.SelectedModelTypeLarge {
    m.applyThemeForProvider(providerID)
    if _, ok := cfg.Models[config.SelectedModelTypeSmall]; !ok {
        smallModel := m.com.Workspace.GetDefaultSmallModel(providerID)
        if err := m.com.Workspace.UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeSmall, smallModel); err != nil {
            cmds = append(cmds, util.ReportError(err))
        }
    }
}
// Summary selection: no theme switch, no auto-set-small.
```

The existing `UpdatePreferredModel` + `UpdateAgentModel` flow is
slot-agnostic. The dialog converts its internal `ModelType` to
`config.SelectedModelType` via `ModelType.Config()` when constructing
`ActionSelectModel` (`models.go:205-210`). `ActionSelectModel.ModelType`
is already a `config.SelectedModelType` (`actions.go:43`), so
`handleSelectModel` passes it directly to `UpdatePreferredModel` —
no additional conversion needed. Summary selection flows through
without additional special-casing.

### Step 7: Extract shared `buildSelectedModel` and resolve summary model

**File**: `internal/agent/coordinator.go`

#### 7a: Add `summaryModel` field and initialize in `buildAgent`

Following the codebase convention (`sessionAgent` uses
`*csync.Value[Model]` at `agent.go:171-172`; `csync.NewValue` returns
a pointer at `value.go:19`):

```go
type coordinator struct {
    // ... existing fields ...
    summaryModel *csync.Value[Model]  // NEW — pointer, per convention
}
```

The resolver is installed during the initial `buildAgent` call
(`coordinator.go:743-750`). The constructor creates `c` first
(line 216), then calls `c.buildAgent` (line 247). `SessionAgent`
exposes `Model()` (the large/active model) but has no `SmallModel()`
method (`agent.go:136-152`).

Therefore the initialization must happen inside `buildAgent`, not in
the constructor literal. The sequence:

1. Initialize `c.summaryModel = csync.NewValue(Model{})` in the `c`
   literal (line 216-234). This ensures the pointer is non-nil when
   `buildAgent` installs the resolver closure.
2. In `buildAgent`, after `buildAgentModels` succeeds and before
   installing the resolver, populate the summary value — but **only
   for `!isSubAgent`** (sub-agent builds must not overwrite the
   notebook model):

```go
func (c *coordinator) buildAgent(ctx context.Context, prompt *prompt.Prompt, agent config.Agent, isSubAgent bool) (SessionAgent, error) {
    large, small, err := c.buildAgentModels(ctx, agent, isSubAgent)
    if err != nil {
        return nil, err
    }

    // Initialize the summary model before installing the resolver.
    // Only the top-level agent build (not sub-agents) sets this —
    // sub-agent builds share the same coordinator and must not
    // overwrite the notebook model.
    if !isSubAgent {
        summary := small // fallback
        if sel, ok := c.cfg.Config().Models[config.SelectedModelTypeSummary]; ok {
            if m, err := c.buildSelectedModel(ctx, sel, true); err == nil {
                summary = m
            } else {
                slog.Warn("Failed to resolve configured summary model, using small",
                    "provider", sel.Provider,
                    "model", sel.Model,
                    "error", err)
            }
        }
        c.summaryModel.Set(summary)
    }

    // ... existing NewSessionAgent call ...

    // Install the resolver — now c.summaryModel is populated.
    if c.notebookModelResolver != nil {
        c.notebookResolverOnce.Do(func() {
            summaryModel := c.summaryModel  // *csync.Value[Model]
            *c.notebookModelResolver = func() fantasy.LanguageModel {
                return summaryModel.Get().Model
            }
        })
    }

    // ... rest of buildAgent ...
}
```

This ensures:

- `c.summaryModel` is non-nil when the resolver closure captures it
  (no nil-pointer panic).
- The resolver returns a valid model immediately after construction,
  before any `UpdateModels` call.
- Sub-agent builds (`isSubAgent == true`) do not overwrite the
  notebook model.
- `UpdateModels` remains responsible for live updates after
  construction.

Add a test that invokes the notebook resolver immediately after
coordinator construction, before any `UpdateModels` call, to verify
it returns a valid (non-nil) model.

#### 7b: Extract `buildSelectedModel` from `buildAgentModels`

The current `buildAgentModels` (lines 924-1019) does several things
that a naive `resolveSelectedModel` helper would miss:

- Validates the model exists in the provider catalog (lines 974-980)
- Applies OpenRouter `:exacto` model-ID transformation (lines 985-991)
- Wraps the model with the configured request timeout (lines 1005-1007)
- Returns proper error identities (`errLargeModelNotFound`, etc.)

Instead of duplicating this logic, extract a shared helper that
`buildAgentModels` and the summary resolver both call. The helper
returns generic errors; `buildAgentModels` translates them back to
the existing sentinels to preserve error-identity contracts:

```go
// buildSelectedModel resolves a single model from a SelectedModel
// config entry. It validates the model exists in the provider
// catalog, applies provider-specific transformations (e.g.
// OpenRouter :exacto), wraps with the request timeout, and returns
// the fully constructed Model. Shared between buildAgentModels
// (large/small) and the summary model resolver.
func (c *coordinator) buildSelectedModel(
    ctx context.Context,
    sel config.SelectedModel,
    isSubAgent bool,
) (Model, error) {
    cfg := c.cfg.Config()

    providerCfg, ok := cfg.Providers.Get(sel.Provider)
    if !ok {
        return Model{}, errModelProviderNotConfigured
    }

    provider, err := c.buildProvider(providerCfg, sel, isSubAgent)
    if err != nil {
        return Model{}, err
    }

    // Find the catwalk model in the provider catalog.
    var catwalkModel *catwalk.Model
    for _, m := range providerCfg.Models {
        if m.ID == sel.Model {
            catwalkModel = &m
            break
        }
    }
    if catwalkModel == nil {
        return Model{}, errModelNotFoundInProvider
    }

    // Apply provider-specific model-ID transformations.
    modelID := sel.Model
    if sel.Provider == openrouter.Name && isExactoSupported(modelID) {
        modelID += ":exacto"
    }

    model, err := provider.LanguageModel(ctx, modelID)
    if err != nil {
        return Model{}, err
    }

    // Wrap with the configured request timeout.
    requestTimeout := cfg.Options.GetRequestTimeout()
    model = newRequestTimeoutModel(model, requestTimeout)

    return Model{
        Model:      model,
        CatwalkCfg: *catwalkModel,  // dereference — CatwalkCfg is a value
        ModelCfg:   sel,
        FlatRate:   providerCfg.FlatRate,
    }, nil
}
```

Add two new sentinel errors for the helper's generic failures:

```go
var (
    // ... existing sentinels ...
    errModelProviderNotConfigured = errors.New("model provider not configured")
    errModelNotFoundInProvider    = errors.New("model not found in provider config")
)
```

Note: `errModelProviderNotConfigured` already exists at line 63 —
reuse it. Only `errModelNotFoundInProvider` is new.

Then refactor `buildAgentModels` to call the helper and translate
errors back to the existing large/small sentinels:

```go
func (c *coordinator) buildAgentModels(ctx context.Context, agent config.Agent, isSubAgent bool) (Model, Model, error) {
    largeModelCfg, ok := c.cfg.Config().Models[agent.Model]
    if !ok {
        largeModelCfg, ok = c.cfg.Config().Models[config.SelectedModelTypeLarge]
    }
    if !ok {
        return Model{}, Model{}, errLargeModelNotSelected
    }
    smallModelCfg, ok := c.cfg.Config().Models[config.SelectedModelTypeSmall]
    if !ok {
        return Model{}, Model{}, errSmallModelNotSelected
    }

    large, err := c.buildSelectedModel(ctx, largeModelCfg, isSubAgent)
    if err != nil {
        // Translate generic helper errors back to large sentinels.
        if errors.Is(err, errModelProviderNotConfigured) {
            return Model{}, Model{}, errLargeModelProviderNotConfigured
        }
        if errors.Is(err, errModelNotFoundInProvider) {
            return Model{}, Model{}, errLargeModelNotFound
        }
        return Model{}, Model{}, err
    }
    small, err := c.buildSelectedModel(ctx, smallModelCfg, true)
    if err != nil {
        if errors.Is(err, errModelProviderNotConfigured) {
            return Model{}, Model{}, errSmallModelProviderNotConfigured
        }
        if errors.Is(err, errModelNotFoundInProvider) {
            return Model{}, Model{}, errSmallModelNotFound
        }
        return Model{}, Model{}, err
    }

    return large, small, nil
}
```

This preserves the existing error identities for `buildAgentModels`
callers while sharing the resolution logic.

#### 7c: Resolve summary model in `UpdateModels` without blocking the primary agent

`UpdateModels` is called before every agent run
(`coordinator.go:298-301`) and aborts on error. The summary model is
auxiliary — it must never prevent the primary coding agent from
running. Therefore summary resolution failures are logged but never
returned from `UpdateModels`. The previous live summary model is
preserved so the notebook generator continues with the last
known-good model:

```go
func (c *coordinator) UpdateModels(ctx context.Context) error {
    // ... existing large/small resolution ...
    c.currentAgent.SetModels(large, small)

    // Resolve summary model. Failures are logged but never block
    // the primary agent — the summary model is auxiliary.
    cfg := c.cfg.Config()
    if sel, ok := cfg.Models[config.SelectedModelTypeSummary]; ok {
        summary, err := c.buildSelectedModel(ctx, sel, true)
        if err != nil {
            // Log only when the failure state changes, to avoid
            // repeating the same error before every agent run.
            c.logSummaryFailureOnce(sel, err)
        } else {
            c.summaryModel.Set(summary)
            c.clearSummaryFailure()
        }
    } else {
        // Summary slot absent — fall back to small (existing behavior).
        c.summaryModel.Set(small)
        c.clearSummaryFailure()
    }

    // ... existing tool rebuild ...
}
```

Add a `summaryFailure` fingerprint and mutex-protected helpers to
suppress repeated identical logs. `UpdateModels` can be reached by
concurrent session runs, so all access to the failure state must be
synchronized:

```go
type summaryFailure struct {
    provider string
    model    string
    message  string
}

type coordinator struct {
    // ... existing fields ...
    summaryModel   *csync.Value[Model]
    summaryFailure *summaryFailure  // NEW — last logged failure, nil when healthy
    summaryErrMu   sync.Mutex      // NEW — guards summaryFailure
}

func (c *coordinator) logSummaryFailureOnce(sel config.SelectedModel, err error) {
    c.summaryErrMu.Lock()
    defer c.summaryErrMu.Unlock()
    current := &summaryFailure{
        provider: sel.Provider,
        model:    sel.Model,
        message:  err.Error(),
    }
    if c.summaryFailure != nil &&
        c.summaryFailure.provider == current.provider &&
        c.summaryFailure.model == current.model &&
        c.summaryFailure.message == current.message {
        return // identical failure, suppress
    }
    c.summaryFailure = current
    slog.Error("Failed to resolve summary model, keeping previous",
        "provider", sel.Provider,
        "model", sel.Model,
        "error", err)
}

func (c *coordinator) clearSummaryFailure() {
    c.summaryErrMu.Lock()
    defer c.summaryErrMu.Unlock()
    c.summaryFailure = nil
}
```

Using a comparable fingerprint (`provider`, `model`, `err.Error()`)
instead of `errors.Is` avoids two problems:

- Separately constructed provider errors with identical content
  generally do not match via `errors.Is`.
- Different model selections returning the same sentinel would
  incorrectly suppress the first error for the newly selected model.

This ensures:

- Summary slot absent → use small (existing behavior, unchanged).
- Summary slot present and valid → use it.
- Summary slot present but invalid → log error once (not per-run),
  keep previous summary model, primary agent runs normally.
- Recovery (error clears) → clears `summaryFailure`, next failure
  logs again.
- Different selection or different error → logs again.
- All access to `summaryFailure` is mutex-protected (no data race).

### Step 8: Separate explicit summary validation from per-run updates

**Files**: `internal/agent/coordinator.go`, `internal/app/app.go`,
`internal/workspace/workspace.go`, `internal/workspace/app_workspace.go`,
`internal/workspace/client_workspace.go`, `internal/client/proto.go`,
`internal/server/proto.go`, `internal/backend/agent.go`,
`internal/ui/model/ui.go`

#### The problem

`UpdateModels` runs before every agent run (`coordinator.go:298-301`)
and must never block the primary agent for summary failures. But
after an explicit user selection via the "Switch Model Summary"
dialog, the UI calls `UpdateAgentModel` → `UpdateModels`
(`app.go:469-474`) and shows "Summary model changed to X"
(`ui.go:2388`) if no error is returned. If the summary model is
invalid, `UpdateModels` logs but doesn't return an error — so the
user sees a false success message.

The two contexts need different error semantics:

- **Per-run (before every agent request):** summary failures are
  nonfatal, log only, keep previous model.
- **Explicit selection (after dialog pick):** the UI should know
  whether the summary model resolved successfully.

#### Solution: dedicated `UpdateSummaryModel` method (full layering)

The UI does not hold `*app.App`; it holds a `workspace.Workspace`
(`common.go:35-43`). The existing `UpdateAgentModel` spans 7 layers:
interface → AppWorkspace → App → Coordinator (local path), and
interface → ClientWorkspace → Client → server handler → Backend →
AppWorkspace (client/server path). `UpdateSummaryModel` must mirror
this exactly.

#### 8a: Coordinator method

```go
// UpdateSummaryModel resolves and applies the configured summary
// model. Unlike UpdateModels, it returns an error if the summary
// model is configured but invalid, so the UI can warn the user
// after an explicit selection. If the summary slot is absent, it
// falls back to the configured small model and returns nil.
func (c *coordinator) UpdateSummaryModel(ctx context.Context) error {
    cfg := c.cfg.Config()
    if sel, ok := cfg.Models[config.SelectedModelTypeSummary]; ok {
        summary, err := c.buildSelectedModel(ctx, sel, true)
        if err != nil {
            return fmt.Errorf("summary model %s/%s: %w", sel.Provider, sel.Model, err)
        }
        c.summaryModel.Set(summary)
        return nil
    }
    // Slot absent — fall back to the configured small model only.
    // Do not look up AgentCoder or the agent-pinned model; the
    // summary model's fallback is the small slot, not the large
    // or agent-pinned model.
    if smallCfg, ok := cfg.Models[config.SelectedModelTypeSmall]; ok {
        small, err := c.buildSelectedModel(ctx, smallCfg, true)
        if err != nil {
            // Small model also invalid — preserve the current live
            // summary model rather than failing. Return nil since
            // the summary slot is absent (no user error to report).
            return nil
        }
        c.summaryModel.Set(small)
    }
    // If small slot is also absent, preserve the current value.
    return nil
}
```

Add to the `Coordinator` interface.

#### 8b: App method

```go
// internal/app/app.go
func (app *App) UpdateSummaryModel(ctx context.Context) error {
    if app.AgentCoordinator == nil {
        return fmt.Errorf("agent configuration is missing")
    }
    return app.AgentCoordinator.UpdateSummaryModel(ctx)
}
```

#### 8c: Workspace interface

```go
// internal/workspace/workspace.go
UpdateSummaryModel(ctx context.Context) error
```

#### 8d: AppWorkspace (local path)

```go
// internal/workspace/app_workspace.go
func (w *AppWorkspace) UpdateSummaryModel(ctx context.Context) error {
    return w.app.UpdateSummaryModel(ctx)
}
```

#### 8e: ClientWorkspace (client/server path)

```go
// internal/workspace/client_workspace.go
func (w *ClientWorkspace) UpdateSummaryModel(ctx context.Context) error {
    return w.client.UpdateSummaryModel(ctx, w.workspaceID())
}
```

#### 8f: Client HTTP method

```go
// internal/client/proto.go
// UpdateSummaryModel triggers a summary model update on the server.
func (c *Client) UpdateSummaryModel(ctx context.Context, id string) error {
    rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/agent/update-summary", id), nil, nil, nil)
    if err != nil {
        return fmt.Errorf("failed to update summary model: %w", err)
    }
    defer rsp.Body.Close()
    if rsp.StatusCode != http.StatusOK {
        return fmt.Errorf("failed to update summary model: status code %d", rsp.StatusCode)
    }
    return nil
}
```

#### 8g: Server route + handler

```go
// internal/server/proto.go
//
//	@Summary		Update summary model
//	@Description	Triggers a summary model update for the workspace
//	@Tags			agent
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/update-summary [post]
func (c *controllerV1) handlePostWorkspaceAgentUpdateSummary(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if err := c.backend.UpdateSummaryModel(r.Context(), id); err != nil {
        c.handleError(w, r, err)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

Register the route in the router setup (alongside the existing
`/agent/update` route).

#### 8h: Backend method

```go
// internal/backend/agent.go
// UpdateSummaryModel reloads the summary model configuration.
func (b *Backend) UpdateSummaryModel(ctx context.Context, workspaceID string) error {
    ws, err := b.GetWorkspace(workspaceID)
    if err != nil {
        return err
    }
    return ws.UpdateSummaryModel(ctx)
}
```

#### 8i: UI calls `UpdateSummaryModel` after explicit summary selection

In `handleSelectModel`, when `msg.ModelType == config.SelectedModelTypeSummary`,
call `UpdateSummaryModel` instead of the generic `UpdateAgentModel`,
and surface a warning on failure:

```go
// After UpdatePreferredModel succeeds for summary type:
if msg.ModelType == config.SelectedModelTypeSummary {
    cmds = append(cmds, m.updateAgentModelCmd(func() tea.Msg {
        if err := m.com.Workspace.UpdateSummaryModel(context.TODO()); err != nil {
            return util.NewWarnMsg(fmt.Sprintf(
                "Summary model saved but could not be activated: %s. "+
                    "The notebook will continue using the previous model.",
                err))
        }
        modelName := msg.Model.Model
        if catwalkModel := cfg.GetModel(msg.Model.Provider, msg.Model.Model); catwalkModel != nil && catwalkModel.Name != "" {
            modelName = catwalkModel.Name
        }
        return util.NewInfoMsg(fmt.Sprintf("Summary model changed to %s", modelName))
    }))
} else {
    // ... existing UpdateAgentModel flow for large/small ...
}
```

This ensures:

- Explicit summary selection that fails → user sees a warning, not
  a false success.
- Per-run `UpdateModels` → summary failures remain nonfatal.
- The config is still persisted (the user's choice is saved), but
  the UI honestly reports whether it took effect.
- Both local and client/server workspace implementations support it.

#### Per-run `UpdateModels` keeps nonfatal summary handling

`UpdateModels` (Step 7c) continues to log summary failures without
returning an error. This is correct for the per-run path. The
dedicated `UpdateSummaryModel` handles the explicit-selection path.

### Step 9: Schema and documentation

**File**: `schema.json`

`schema.json` does not enumerate `large`/`small` — `models` uses
`additionalProperties` with a `SelectedModel` ref (lines 59-65), so
`summary` is already valid as a key. The schema is auto-generated via
`task schema` (`Taskfile.yaml:145-151`).

Actions:

- Update the stale `Config.Models` comment at `config.go:746` (done
  in Step 1).
- Update user-facing config documentation/examples to mention the
  `summary` slot.
- Regenerate schema only if source annotations changed: `task schema`.
- Do not hand-edit `schema.json`.

### Step 10: RecentModels for summary

**File**: `internal/ui/dialog/models.go`

`RecentModels` (line 356) tracks recently used models per type. The
summary dialog's `setProviderItems` already reads
`cfg.RecentModels[selectedType]` where `selectedType` is
`m.modelType.Config()`. When `modelType` is `ModelTypeSummary`, this
maps to `config.SelectedModelTypeSummary`.

No code changes needed — `setProviderItems` is slot-agnostic. The
summary slot will get its own recent-models bucket automatically
once a summary model is selected. If the bucket is empty (first
use), no "Recently used" group appears — same as the existing
behavior for large/small on first use.

### Step 11: Tests

- `model summary` crushrc command sets the summary slot
- `NewSummaryModels` creates a dialog with `ID() == SummaryModelsID`
- `NewSummaryModels` calls `setProviderItems()` exactly once (no `recent_models.large` mutation)
- `NewSummaryModels` creates a dialog without large/small toggle
- Tab key does nothing in summary mode
- Tab help is excluded in summary mode (`ShortHelp`)
- Selecting a model in summary mode saves to `models.summary`
- Command palette shows "No summary model set" warning when summary slot absent
- Command palette shows neutral description when summary slot is configured
- `handleSelectModel` closes `SummaryModelsID` after summary selection
- `handleSelectModel` does not auto-set small when selecting summary
- `handleSelectModel` shows warning (not success) when explicit summary selection fails to resolve
- `handleSelectModel` shows success when explicit summary selection resolves
- `buildSelectedModel` applies OpenRouter `:exacto` transformation
- `buildSelectedModel` wraps with request timeout
- `buildSelectedModel` returns correct `Model` struct (dereferenced `CatwalkCfg`, `FlatRate` set)
- `buildAgentModels` still returns existing sentinel errors after refactor (regression)
- `buildAgent` initializes `summaryModel` before installing resolver (non-nil pointer)
- `buildAgent` with `isSubAgent=true` does not overwrite `summaryModel`
- `UpdateModels` resolves summary model from config
- `UpdateModels` falls back to small when summary slot absent
- `UpdateModels` logs error and keeps previous summary model when summary slot present but invalid (no blocking)
- `UpdateModels` does not return an error when summary model is invalid (primary agent not blocked)
- `UpdateModels` suppresses repeated identical summary failure logs (logs once, not per-run)
- `UpdateModels` logs again when the selection or error message changes
- `UpdateModels` clears `summaryFailure` on recovery (next failure logs again)
- `UpdateModels` summary failure logging is data-race-free (run with `-race`)
- `UpdateSummaryModel` (coordinator) returns error when summary slot present but invalid
- `UpdateSummaryModel` (coordinator) returns nil and updates when summary slot present and valid
- `UpdateSummaryModel` (coordinator) returns nil and falls back to small when slot absent
- `UpdateSummaryModel` (coordinator) returns nil and preserves current value when small also absent
- `UpdateSummaryModel` (coordinator) does not look up `AgentCoder` for fallback
- `UpdateSummaryModel` (App) delegates to coordinator
- `UpdateSummaryModel` (AppWorkspace) delegates to App
- `UpdateSummaryModel` (ClientWorkspace) calls client HTTP method
- `UpdateSummaryModel` (client) sends POST to `/workspaces/{id}/agent/update-summary`
- `UpdateSummaryModel` (server handler) calls backend method
- `UpdateSummaryModel` (backend) calls workspace method
- `UpdateSummaryModel` works end-to-end in local (AppWorkspace) mode
- `UpdateSummaryModel` works end-to-end in client/server mode
- Notebook resolver returns a valid (non-nil) model immediately after coordinator construction, before any `UpdateModels` call
- Notebook resolver returns summary model after `UpdateModels` call
- Notebook resolver updates after subsequent `UpdateModels` calls
- Re-auth flow closes summary dialog (unconfigured provider)
- OAuth flow closes summary dialog (Hyper)
- Reauthentication via `ctrl+e` closes summary dialog

## Commit Plan

| Commit | Description                                                        |
| ------ | ------------------------------------------------------------------ |
| 1      | `feat: add summary model type to config`                           |
| 2      | `feat: add model summary crushrc command`                          |
| 3      | `refactor: extract buildSelectedModel from buildAgentModels`       |
| 4      | `feat: add Switch Model Summary command and dialog`                |
| 5      | `feat: wire notebook generator to summary model with live updates` |
| 6      | `feat: add UpdateSummaryModel for explicit selection validation`   |
| 7      | `test: add summary model tests`                                    |

## Risks

| Risk                                                      | Mitigation                                                                                                                                     |
| --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Existing users have no summary model                      | Fallback to small when slot absent — behavior unchanged                                                                                        |
| `sync.Once` freezes resolver                              | Use `*csync.Value[Model]` instead of captured variable                                                                                         |
| Summary model on different provider                       | Each model resolved independently — no issue                                                                                                   |
| Dialog ID collision                                       | `id` field on `Models` struct, overridden in `NewSummaryModels`                                                                                |
| `handleSelectModel` doesn't close summary dialog          | Close both `ModelsID` and `SummaryModelsID` unconditionally (no-op on non-open)                                                                |
| Re-auth flow hardcodes `ModelsID`                         | Same fix — close both IDs in re-auth branch                                                                                                    |
| Auto-set-small fires on summary selection                 | Move small-init inside `SelectedModelTypeLarge` condition                                                                                      |
| Tab help visible in summary mode                          | Exclude `m.keyMap.Tab` from `ShortHelp` when `summaryMode`                                                                                     |
| Invalid summary model blocks primary agent                | Log error, keep previous summary model, never return error from `UpdateModels` for summary failures                                            |
| UI shows false success on invalid explicit selection      | Dedicated `UpdateSummaryModel` returns error; UI shows warning on failure                                                                      |
| `UpdateSummaryModel` not wired through workspace boundary | Full 7-layer implementation: Coordinator → App → Workspace interface → AppWorkspace + ClientWorkspace → Client HTTP → Server handler → Backend |
| `buildSelectedModel` diverges from `buildAgentModels`     | Extract shared helper, refactor `buildAgentModels` to use it, translate errors back to sentinels                                               |
| `NewSummaryModels` mutates `recent_models.large`          | Private `newModelsInternal` constructor sets model type before single `setProviderItems()` call                                                |
| Resolver returns nil before first `UpdateModels`          | Initialize `summaryModel` in `buildAgent` before installing resolver, with small model fallback                                                |
| Resolver closure captures nil pointer                     | `c.summaryModel` initialized in `c` literal before `buildAgent` runs                                                                           |
| Sub-agent builds overwrite notebook model                 | Guard summary init with `!isSubAgent` in `buildAgent`                                                                                          |
| Repeated identical log lines per run                      | `summaryFailure` fingerprint suppresses duplicate logs, clears on recovery                                                                     |
| Data race on failure state                                | `summaryErrMu` mutex guards all access to `summaryFailure` (both write and clear); run tests with `-race`                                      |
| `errors.Is` unreliable for deduplication                  | Comparable fingerprint (`provider`, `model`, `err.Error()`) instead of `errors.Is`                                                             |
| Absent-slot fallback resolves wrong model                 | `UpdateSummaryModel` resolves only `SelectedModelTypeSmall`, not `AgentCoder.Model`                                                            |
| Schema drift                                              | Regenerate via `task schema` only if annotations change, don't hand-edit                                                                       |
