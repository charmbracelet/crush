package agent

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CopilotHeaderTransport wraps an http.RoundTripper and injects
// VSCode Copilot-compatible headers for request grouping.
// This enables quota optimization by grouping multiple requests
// in an agent interaction into a single billing unit.
type CopilotHeaderTransport struct {
	Transport http.RoundTripper

	mu            sync.RWMutex
	interactionID string
	sessionID     string
	machineID     string
}

// NewCopilotHeaderTransport creates a new transport that wraps the given transport
// and injects Copilot-specific headers.
func NewCopilotHeaderTransport(transport http.RoundTripper) *CopilotHeaderTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	// Generate sessionID in VSCode format: UUID + timestamp in milliseconds
	// Example: "2d1443da-9168-49bf-b8c4-b5e319ad1c3b1766149919521"
	sessionID := uuid.NewString() + strconv.FormatInt(time.Now().UnixMilli(), 10)

	// Generate machineID as 64-char hex string (like VSCode's SHA256 hash)
	machineID := strings.ReplaceAll(uuid.NewString()+uuid.NewString(), "-", "")

	return &CopilotHeaderTransport{
		Transport:     transport,
		interactionID: uuid.NewString(),
		sessionID:     sessionID,
		machineID:     machineID,
	}
}

// NewInteraction generates a new interaction ID for request grouping.
// Call this at the start of each agent interaction to group all subsequent
// HTTP requests under the same interaction for quota purposes.
func (c *CopilotHeaderTransport) NewInteraction() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interactionID = uuid.NewString()
}

// SessionHeaders returns headers for the /models/session call.
// These use the same identifiers as chat/completions for consistency.
func (c *CopilotHeaderTransport) SessionHeaders() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]string{
		"vscode-sessionid":     c.sessionID,
		"vscode-machineid":     c.machineID,
		"vscode-abexpcontext":  vscodeABExpContext,
		"x-github-api-version": "2025-10-01",
	}
}

// vscodeABExpContext is the A/B experiment context from VSCode Copilot Chat.
// This helps GitHub identify the client and may enable discounted quota.
const vscodeABExpContext = "vsliv368:30146709;pythonvspyt551cf:31249601;binariesv615:30325510;nativeloc1:31344060;7d05f481:31426900;cg8ef616:31426899;copilot_t_ci:31333650;pythonrdcb7:31342333;6518g693:31436602;aj953862:31281341;nes-set-on:31351930;6abeh943:31336334;envsactivate1:31353494;cloudbuttont:31379625;todos-1:31405332;upload-service:31384080;3efgi100_wstrepl:31403338;trigger-command-fix:31379601;use-responses-api:31390855;ddidtcf:31399634;je187915:31407605;d5i5i512:31428709;ec5jj548:31422691;cp_cls_c_966_ss:31435507;copilot6169-t2000-control:31431385;c0683394:31419495;3bff2643:31431739;478ah919:31426797;30h21147:31435638;ge8j1254_inline_auto_hint_haiku:31427726;nes-autoexp-off:31439334;a5gib710:31434435;38bie571_auto:31429954;request_with_suggest:31435827;rename_enabled:31436409;nes-joint-0:31438277;7a04d226_do_not_restore_last_panel_session:31438103;anthropic_thinking_t:31432745;h0hdh950:31428394;9ab0j925:31438090;cp_cls_c_1081:31433293;copilot-nes-oct-trt:31432596;nes-slash-models-off:31439029;;cmp-ext-treat:31426748"

// RoundTrip implements http.RoundTripper, injecting Copilot headers.
func (c *CopilotHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqClone := req.Clone(req.Context())

	c.mu.RLock()
	interactionID := c.interactionID
	sessionID := c.sessionID
	machineID := c.machineID
	c.mu.RUnlock()

	// Request grouping headers for 0.1 quota per interaction
	// Based on VSCode Copilot Chat traffic analysis
	reqClone.Header.Set("x-interaction-id", interactionID)
	reqClone.Header.Set("x-interaction-type", "conversation-agent")
	reqClone.Header.Set("openai-intent", "conversation-agent")
	reqClone.Header.Set("x-initiator", "user")
	reqClone.Header.Set("x-request-id", uuid.NewString())
	reqClone.Header.Set("x-github-api-version", "2025-10-01")

	// VSCode identifiers and A/B experiment context
	reqClone.Header.Set("vscode-sessionid", sessionID)
	reqClone.Header.Set("vscode-machineid", machineID)
	reqClone.Header.Set("vscode-abexpcontext", vscodeABExpContext)
	reqClone.Header.Set("x-vscode-user-agent-library-version", "electron-fetch")

	// Browser-level headers from Electron
	reqClone.Header.Set("sec-fetch-site", "none")
	reqClone.Header.Set("sec-fetch-mode", "no-cors")
	reqClone.Header.Set("sec-fetch-dest", "empty")
	reqClone.Header.Set("accept-encoding", "gzip, deflate, br, zstd")
	reqClone.Header.Set("priority", "u=4, i")

	// Log all key headers for debugging
	slog.Debug("CopilotHeaderTransport injecting headers",
		"url", reqClone.URL.String(),
		"interactionID", interactionID,
		"sessionID", sessionID[:20]+"...",
		"User-Agent", reqClone.Header.Get("User-Agent"),
		"Copilot-Integration-Id", reqClone.Header.Get("Copilot-Integration-Id"),
		"Editor-Version", reqClone.Header.Get("Editor-Version"))

	resp, err := c.Transport.RoundTrip(reqClone)
	if err != nil {
		return resp, err
	}

	// Log quota-related response headers for debugging
	if quota := resp.Header.Get("x-quota-snapshot-premium_interactions"); quota != "" {
		slog.Info("Copilot quota snapshot", "premium_interactions", quota)
	}
	if editSession := resp.Header.Get("copilot-edits-session"); editSession != "" {
		slog.Debug("Copilot edits session", "value", editSession[:min(50, len(editSession))]+"...")
	}

	return resp, nil
}
