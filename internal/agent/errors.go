package agent

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/lipgloss/v2"
	"charm.land/tone"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/stringext"
)

var (
	ErrRequestCancelled = errors.New("request canceled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
	ErrEmptyPrompt      = errors.New("prompt is empty")
	ErrSessionMissing   = errors.New("session id is missing")
)

// defaultFailureTitle heads a failure nothing more specific describes.
const defaultFailureTitle = "Provider Error"

// turnFailure returns a failed turn's finish reason, heading and detail.
func turnFailure(err error, m Model, canceled bool) (message.FinishReason, string, string) {
	if canceled {
		return message.FinishReasonCanceled, "User canceled request", ""
	}
	if errors.Is(err, errRefused) {
		// Every retry was refused; the UI owns the refusal banner's copy.
		return message.FinishReasonContentFilter, "", ""
	}

	attempts := failedAttempts(err)
	title, detail := failureText(lastAttempt(err), m)

	if attempts > 1 {
		detail = fmt.Sprintf("Failed after %d attempts. %s", attempts, detail)
	}
	return message.FinishReasonError, title, detail
}

// lastAttempt unwraps a retry to the attempt that failed last, so "retry
// error:" does not repeat the attempt count.
func lastAttempt(err error) error {
	var retryErr *fantasy.RetryError
	if !errors.As(err, &retryErr) {
		return err
	}
	if last := retryErr.Unwrap(); last != nil {
		return last
	}
	return err
}

// failureText describes a failed turn, preferring the account the reader can
// act on.
func failureText(err error, m Model) (title, detail string) {
	// Before the provider branches, so our own deadline never reads as theirs.
	if timedOut, ok := errors.AsType[*requestTimeoutError](err); ok {
		return "Request timed out", timedOut.userMessage()
	}

	// A rejected managed credential is explained without the provider's
	// response body, and never as a sign-in nobody can perform.
	if managedErr, ok := errors.AsType[*managedCredentialError](err); ok {
		return "Credential rejected", stringext.Capitalize(managedErr.Error()) + "."
	}

	if providerErr, ok := errors.AsType[*fantasy.ProviderError](err); ok {
		if m.Managed && providerErr.StatusCode == http.StatusUnauthorized {
			return "Credential rejected", fmt.Sprintf("The managed credential for %s was rejected.", m.ModelCfg.Provider)
		}
		// An OAuth token the provider stopped honouring; retrying cannot fix it.
		if m.OAuth && providerErr.StatusCode == http.StatusUnauthorized {
			return reauthFinish(m)
		}
		if m.ModelCfg.Provider == string(catwalk.InferenceProviderCopilot) &&
			providerErr.Message == "The requested model is not supported." {
			return copilotNotEnabledFinish(m)
		}
		return failureTitle(providerErr.Title), providerErrorDetail(providerErr)
	}

	if fantasyErr, ok := errors.AsType[*fantasy.Error](err); ok {
		return failureTitle(fantasyErr.Title), fantasyErr.Message
	}
	if fantasy.IsTransportError(err) {
		wrapped := fantasy.NewTransportError(err)
		return failureTitle(wrapped.Title), wrapped.Message
	}
	return defaultFailureTitle, err.Error()
}

// reauthFinish tells the reader which provider to sign in to again, which the
// provider's own wording ("token has been revoked") leaves out.
func reauthFinish(m Model) (title, detail string) {
	return "Unauthorized", fmt.Sprintf(
		"Please sign in to %s again. You can also run \"crush login %s --force\".",
		providerDisplayName(m), m.ModelCfg.Provider,
	)
}

// providerDisplayName names a model's provider for the reader.
func providerDisplayName(m Model) string {
	return cmp.Or(m.ProviderName, stringext.Capitalize(m.ModelCfg.Provider))
}

// failureTitle capitalizes a provider's title, falling back when it has none.
func failureTitle(title string) string {
	if title == "" {
		return defaultFailureTitle
	}
	return stringext.Capitalize(title)
}

// copilotNotEnabledFinish points at the Copilot setting only the account
// holder can switch on.
func copilotNotEnabledFinish(m Model) (title, detail string) {
	const url = "https://github.com/settings/copilot/features"
	link := lipgloss.NewStyle().Foreground(tone.Guac).Underline(true).
		Hyperlink(url, "id=copilot").Render(url)
	return "Copilot model not enabled", fmt.Sprintf(
		"%q is not enabled in Copilot. Go to the following page to enable it. Then, wait 5 minutes before trying again. %s",
		m.CatwalkCfg.Name, link,
	)
}

// providerErrorDetail cuts a provider error down to its readable sentence
// plus the request ID. A message that does not parse is kept whole.
func providerErrorDetail(providerErr *fantasy.ProviderError) string {
	summary := providerErr.Message
	requestID := cmp.Or(
		providerErr.ResponseHeaders["request-id"],
		providerErr.ResponseHeaders["x-request-id"],
	)
	// Anthropic's SDK puts the request line first and the sentence last.
	if body, ok := parseProviderErrorBody(summary); ok {
		summary = body.summary()
		requestID = cmp.Or(requestID, body.RequestID)
	} else if body, ok := parseProviderErrorBody(string(providerErr.ResponseBody)); ok {
		// A mid-stream failure has no headers; its payload names the request.
		requestID = cmp.Or(requestID, body.RequestID)
	}
	if requestID == "" || strings.Contains(summary, requestID) {
		return summary
	}
	return sentence(summary) + " Request " + requestID + "."
}

// providerErrorBody is the {"error": {...}} envelope Anthropic, OpenAI and
// Gemini share.
type providerErrorBody struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

// summary is the envelope's message, or its type ("overloaded_error" reads
// as "Overloaded.") when it has none.
func (b providerErrorBody) summary() string {
	return sentence(cmp.Or(
		b.Error.Message,
		strings.ReplaceAll(strings.TrimSuffix(b.Error.Type, "_error"), "_", " "),
	))
}

// parseProviderErrorBody finds the error envelope at the end of text.
func parseProviderErrorBody(text string) (providerErrorBody, bool) {
	var body providerErrorBody
	start := strings.Index(text, "{")
	if start < 0 || json.Unmarshal([]byte(text[start:]), &body) != nil {
		return body, false
	}
	return body, body.Error.Message != "" || body.Error.Type != ""
}

// failedAttempts counts the attempts behind err, or zero if it was not
// retried; fantasy never wraps a single attempt.
func failedAttempts(err error) int {
	var retryErr *fantasy.RetryError
	if !errors.As(err, &retryErr) {
		return 0
	}
	return len(retryErr.Errors)
}

// sentence capitalizes text and ends it with punctuation.
func sentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if first, size := utf8.DecodeRuneInString(text); unicode.IsLower(first) {
		text = string(unicode.ToUpper(first)) + text[size:]
	}
	if last, _ := utf8.DecodeLastRuneInString(text); !strings.ContainsRune(".!?", last) {
		text += "."
	}
	return text
}
