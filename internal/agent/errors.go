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

// turnFailure settles what a failed turn reports: its finish reason, the
// heading, and the detail beneath it.
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

	// Said once, for every kind of failure. An error that was never retried
	// reports no attempts, so this costs the common case nothing.
	if attempts > 1 {
		detail = fmt.Sprintf("Failed after %d attempts. %s", attempts, detail)
	}
	return message.FinishReasonError, title, detail
}

// lastAttempt strips the retry bookkeeping off a failure, leaving the attempt
// that failed last. Its own wording already says a retry happened, which
// reads as the count twice over once the count is stated outright.
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

// failureText describes a failed turn, preferring whichever account of it the
// reader can act on.
func failureText(err error, m Model) (title, detail string) {
	// Checked before the provider branches so a deadline our own request
	// timeout imposed is never reported as a provider error.
	if timedOut, ok := errors.AsType[*requestTimeoutError](err); ok {
		return "Request timed out", timedOut.userMessage()
	}

	// A rejected managed credential is explained without the provider's
	// response body, and never as a sign-in nobody can perform.
	if managedErr, ok := errors.AsType[*managedCredentialError](err); ok {
		return "Credential rejected", stringext.Capitalize(managedErr.Error()) + "."
	}

	if providerErr, ok := errors.AsType[*fantasy.ProviderError](err); ok {
		// A provider signed in through OAuth answers 401 once its token stops
		// being honoured, and no amount of retrying fixes that.
		if m.Managed && providerErr.StatusCode == http.StatusUnauthorized {
			return "Credential rejected", fmt.Sprintf("The managed credential for %s was rejected.", m.ModelCfg.Provider)
		}
		if m.OAuth && providerErr.StatusCode == http.StatusUnauthorized {
			return reauthFinish(m)
		}
		if m.ModelCfg.Provider == string(catwalk.InferenceProviderCopilot) &&
			providerErr.Message == "The requested model is not supported." {
			return copilotNotEnabledFinish(m)
		}
		return failureTitle(providerErrorTitle(providerErr)),
			errorDetail(providerErr.Message, providerErrorRequestID(providerErr))
	}

	if fantasyErr, ok := errors.AsType[*fantasy.Error](err); ok {
		return failureTitle(fantasyErr.Title), providerErrorDetail(fantasyErr.Message)
	}
	if fantasy.IsTransportError(err) {
		wrapped := fantasy.NewTransportError(err)
		return failureTitle(wrapped.Title), providerErrorDetail(wrapped.Message)
	}
	return defaultFailureTitle, providerErrorDetail(err.Error())
}

// reauthFinish is what a turn reports when the provider rejected the
// credentials it was signed in with. It says which provider and what to do,
// because the provider's own wording for this ("OAuth access token has been
// revoked") tells the reader what happened and not how to fix it.
func reauthFinish(m Model) (title, detail string) {
	return "Unauthorized", fmt.Sprintf(
		"Please sign in to %s again. You can also run \"crush login %s --force\".",
		providerDisplayName(m), m.ModelCfg.Provider,
	)
}

// providerDisplayName names the provider a model speaks to, for messages the
// user reads, falling back to the provider ID when it has no display name.
func providerDisplayName(m Model) string {
	return cmp.Or(m.ProviderName, stringext.Capitalize(m.ModelCfg.Provider))
}

// failureTitle presents a provider's lowercase title, falling back when it
// offers none.
func failureTitle(title string) string {
	if title == "" {
		return defaultFailureTitle
	}
	return stringext.Capitalize(title)
}

// copilotNotEnabledFinish is what a turn reports when Copilot refuses a model
// the account has not switched on, which only the account holder can change.
func copilotNotEnabledFinish(m Model) (title, detail string) {
	const url = "https://github.com/settings/copilot/features"
	link := lipgloss.NewStyle().Foreground(tone.Guac).Underline(true).
		Hyperlink(url, "id=copilot").Render(url)
	return "Copilot model not enabled", fmt.Sprintf(
		"%q is not enabled in Copilot. Go to the following page to enable it. Then, wait 5 minutes before trying again. %s",
		m.CatwalkCfg.Name, link,
	)
}

// providerErrorTitle returns the title to show for a provider error, or the
// empty string when the caller's default should stand in.
//
// A mid-stream failure arrives inside a response that already went out as a
// 200, and a title derived from that status reads "ok" above the error
// contradicting it. Only a success status's own name is dropped; a title the
// provider took from the error type ("provider overloaded") still describes
// the failure.
func providerErrorTitle(providerErr *fantasy.ProviderError) string {
	status := providerErr.StatusCode
	if status >= http.StatusOK && status < http.StatusMultipleChoices &&
		strings.EqualFold(providerErr.Title, http.StatusText(status)) {
		return ""
	}
	return providerErr.Title
}

// providerErrorDetail rebuilds a provider's message around the part worth
// reading. An SDK writes the request line, status and ids first and the one
// readable sentence last; the request id stays because it is what a provider
// asks for when something needs chasing.
//
// A body that does not parse is returned as it arrived, since showing too much
// beats dropping the only account of the failure there is.
func providerErrorDetail(message string) string {
	return errorDetail(message, "")
}

// errorDetail is providerErrorDetail with a request ID found outside the
// message, which wins over one the message carries.
func errorDetail(message, requestID string) string {
	summary := message
	if body, ok := parseProviderErrorBody(message); ok {
		summary = body.Error.Message
		if summary == "" {
			// "overloaded_error" reads well enough as "Overloaded".
			summary = strings.ReplaceAll(strings.TrimSuffix(body.Error.Type, "_error"), "_", " ")
		}
		summary = sentence(summary)
		requestID = cmp.Or(requestID, body.RequestID)
	}
	if requestID == "" || strings.Contains(summary, requestID) {
		return summary
	}
	return sentence(summary) + " Request " + requestID + "."
}

// requestIDHeaders are the response headers providers name a request by:
// Anthropic sends "request-id", OpenAI and most compatible APIs
// "x-request-id".
var requestIDHeaders = []string{"request-id", "x-request-id"}

// providerErrorRequestID finds the ID a provider gave the failed request,
// reading the response headers before the body. A mid-stream failure has no
// headers of its own, so its event payload is the only place left to look.
func providerErrorRequestID(providerErr *fantasy.ProviderError) string {
	for _, name := range requestIDHeaders {
		for key, value := range providerErr.ResponseHeaders {
			if value != "" && strings.EqualFold(key, name) {
				return value
			}
		}
	}
	if body, ok := parseProviderErrorBody(string(providerErr.ResponseBody)); ok {
		return body.RequestID
	}
	return ""
}

// providerErrorBody is the error envelope providers answer with. Anthropic,
// OpenAI and Gemini all nest the readable part under "error", so one shape
// covers them.
type providerErrorBody struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

// parseProviderErrorBody recovers the envelope a provider SDK appended to its
// message, reporting false unless it describes the failure.
//
// Reaching into a human-readable string for this is the wrong layer: fantasy
// carries the type as a field as of charmbracelet/fantasy#414, so once Crush
// bumps past that release this reads ProviderError.ErrorType instead.
func parseProviderErrorBody(message string) (providerErrorBody, bool) {
	var body providerErrorBody
	start := strings.Index(message, "{")
	if start < 0 {
		return body, false
	}
	if err := json.Unmarshal([]byte(message[start:]), &body); err != nil {
		return body, false
	}
	return body, body.Error.Message != "" || body.Error.Type != ""
}

// failedAttempts returns how many times a request was tried before the error
// escaped, or zero when it was not retried. fantasy records one error per
// failed attempt and leaves a first-try failure unwrapped, so a count of one
// never arrives.
func failedAttempts(err error) int {
	var retryErr *fantasy.RetryError
	if !errors.As(err, &retryErr) {
		return 0
	}
	return len(retryErr.Errors)
}

// sentence renders text as one sentence, so details joined together do not
// read as fragments run on.
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
