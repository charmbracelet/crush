package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/tidwall/sjson"
)

// diagnosticsBaselineTimeout bounds the pre-mutation LSP settle wait so a
// never-opened file does not read an empty baseline forever.
const diagnosticsBaselineTimeout = 3 * time.Second

// verifyingTool wraps a file-mutation tool to record an LSP diagnostics
// delta on the result: a baseline snapshot before the inner call, a fresh
// snapshot after, and a passed/failed verdict over new errors.
type verifyingTool struct {
	inner      fantasy.AgentTool
	lspManager *lsp.Manager
	workingDir string
	// pendingChecks resolves the gate-run checks a mutation selects;
	// recorded as pending entries for the end-of-turn gate to collect.
	pendingChecks func(absPath string) []message.VerificationCheck
	// projectWide marks tools that mutate many files via workspace edits
	// (lsp_rename): the post-mutation notify refreshes all open files
	// rather than just the anchor file.
	projectWide bool
}

// wrapToolsWithVerification wraps each write-class tool in a
// verifyingTool. Unlike hooks there is no sub-agent exemption — the
// decorator fires no user code, and a sub-agent's own run gate consumes
// the metadata on its child session.
func wrapToolsWithVerification(toolList []fantasy.AgentTool, lspManager *lsp.Manager, workingDir string, pendingChecks func(absPath string) []message.VerificationCheck) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, len(toolList))
	for i, tool := range toolList {
		if tools.WriteToolNames[tool.Info().Name] {
			out[i] = &verifyingTool{
				inner:         tool,
				lspManager:    lspManager,
				workingDir:    workingDir,
				pendingChecks: pendingChecks,
				projectWide:   tool.Info().Name == "lsp_rename",
			}
		} else {
			out[i] = tool
		}
	}
	return out
}

// Unwrap returns the wrapped tool, matching hookedTool's convention for
// callers that need the concrete type.
func (v *verifyingTool) Unwrap() fantasy.AgentTool {
	return v.inner
}

func (v *verifyingTool) Info() fantasy.ToolInfo {
	return v.inner.Info()
}

func (v *verifyingTool) ProviderOptions() fantasy.ProviderOptions {
	return v.inner.ProviderOptions()
}

func (v *verifyingTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	v.inner.SetProviderOptions(opts)
}

func (v *verifyingTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	filePath := tools.ToolCallFilePath(call.Input)
	if filePath == "" {
		return v.inner.Run(ctx, call)
	}
	absPath := filepathext.SmartJoin(v.workingDir, filePath)
	// Start configured-but-not-running servers before the coverage check —
	// AnyClientHandles only sees running clients, and the pre-decorator
	// flow started them on every edit via notifyLSPs. Skipping this lost
	// lazy startup for headless runs and edit-before-view sessions.
	if v.lspManager != nil {
		v.lspManager.Start(ctx, absPath)
	}
	lspCovered := tools.AnyClientHandles(v.lspManager, absPath)

	if !lspCovered {
		resp, err := v.inner.Run(ctx, call)
		if err != nil || resp.IsError {
			return resp, err
		}
		// Keep the project-diagnostics append the tools used to produce
		// even when no client handles this file.
		resp.Content += tools.FormatDiagnostics(absPath, v.lspManager)
		// Select after the mutation so a newly created file (e.g. the
		// first _test.go in a package) is seen by the selector.
		checks := v.selectPending(absPath)
		if len(checks) == 0 {
			checks = []message.VerificationCheck{{
				Check:  "diagnostics",
				State:  message.VerificationUnverified,
				Detail: "no LSP client handles the file",
			}}
			// Surface the unverified state in the result so the model can
			// hedge — scoped to source files, where a missing check is
			// meaningful; a README edit has nothing to hedge about.
			if sourceFileExts[strings.ToLower(filepath.Ext(absPath))] {
				resp.Content += "\n\n<verification status=\"unverified\">No automated check covers this change — the end-of-turn gate has nothing to run. Claim it verified only after running a check yourself.</verification>"
			}
		}
		resp.Metadata = mergeVerificationMetadata(resp.Metadata, checks)
		return resp, nil
	}

	// Baseline before the mutation: open the file so a first-ever edit
	// does not read an empty snapshot, then wait for the publish.
	baselineSettled := tools.PrepareDiagnosticsBaseline(ctx, v.lspManager, absPath, diagnosticsBaselineTimeout)
	baseline := tools.SnapshotDiagnostics(v.lspManager)

	resp, err := v.inner.Run(ctx, call)
	if err != nil || resp.IsError {
		// Preserve the tools' early return: no diagnostics append and no
		// verification write on a failed mutation.
		return resp, err
	}

	// A workspace-edit tool touches many files — refresh all open files,
	// not just the anchor.
	notifyPath := absPath
	if v.projectWide {
		notifyPath = ""
	}
	settled := tools.NotifyLSPs(ctx, v.lspManager, notifyPath)
	after := tools.SnapshotDiagnostics(v.lspManager)
	newErrs := after.NewErrorsSince(baseline)

	resp.Content += tools.FormatDiagnostics(absPath, v.lspManager)

	state := message.VerificationPassed
	detail := ""
	switch {
	case !baselineSettled:
		// A timed-out baseline can be empty — every pre-existing error
		// would read as "new". The delta is untrustworthy in both
		// directions: record unverified, not fail or pass.
		state = message.VerificationUnverified
		detail = "baseline diagnostics did not settle before timeout"
	case !settled:
		// A timed-out settle can read a stale snapshot — record
		// unverified, not a pass that was never earned.
		state = message.VerificationUnverified
		detail = "diagnostics did not settle before timeout"
	case len(newErrs) > 0:
		state = message.VerificationFailed
		detail = fmt.Sprintf("%d new error(s)", len(newErrs))
	}
	checks := append([]message.VerificationCheck{{
		Check:  "diagnostics",
		State:  state,
		Detail: detail,
	}}, v.selectPending(absPath)...)
	resp.Metadata = mergeVerificationMetadata(resp.Metadata, checks)
	return resp, nil
}

// selectPending runs the configured check selector when one is wired.
func (v *verifyingTool) selectPending(absPath string) []message.VerificationCheck {
	if v.pendingChecks == nil {
		return nil
	}
	return v.pendingChecks(absPath)
}

// mergeVerificationMetadata sets the "verification" key on existing tool
// metadata via sjson so it composes with the "hook" key rather than
// clobbering it.
func mergeVerificationMetadata(existing string, checks []message.VerificationCheck) string {
	data, err := json.Marshal(checks)
	if err != nil {
		return existing
	}
	if existing == "" {
		existing = "{}"
	}
	merged, err := sjson.SetRaw(existing, "verification", string(data))
	if err != nil {
		return existing
	}
	return merged
}
