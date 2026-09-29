package sshaskpass

import (
	"math/rand/v2"
	"strings"
)

// ClassifyPrompt maps an OpenSSH askpass prompt to the credential kind
// and a short key/account hint for the dialog. Passive security key
// touches become KindTouch (surfaced as a warning and confirmed
// automatically); genuine decision questions (host key acceptance,
// ssh-add key use) become KindConfirm; everything else is treated as a
// secret prompt.
func ClassifyPrompt(prompt string) (kind Kind, keyInfo string) {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	switch {
	case isTouchPrompt(lower):
		return KindTouch, ""
	case isConfirmPrompt(lower):
		return KindConfirm, ""
	default:
		return KindPassword, keyInfoFromPrompt(prompt)
	}
}

// ParsePrompt classifies an OpenSSH askpass prompt and fills in the
// friendly copy Crush shows in place of OpenSSH's terse phrasing: a
// short title for the dialog heading and a human-readable description
// of what is being asked for, with a playful aside suited to the
// moment. The raw prompt is preserved on the result.
func ParsePrompt(prompt string) PromptRequest {
	kind, keyInfo := ClassifyPrompt(prompt)
	lower := strings.ToLower(prompt)

	// ssh-add asks "Allow use of key ...?" with the key path inline,
	// which ClassifyPrompt leaves out because confirmations never hide
	// an account behind it.
	if kind == KindConfirm && strings.Contains(lower, "allow use of key") {
		keyInfo = keyInfoFromPrompt(prompt)
	}

	style := classifyStyle(lower, kind)
	return PromptRequest{
		Prompt:  prompt,
		Kind:    kind,
		KeyInfo: keyInfo,
		Title:   style.title(),
		Message: style.message(prompt, keyInfo),
	}
}

// KeyInfoLabel returns the label for a parsed account/key hint: "Key"
// for key-related prompts (passphrases, security keys), "Account" for
// password prompts.
func KeyInfoLabel(prompt string, kind Kind) string {
	if classifyStyle(strings.ToLower(prompt), kind) == stylePassword {
		return "Account"
	}
	return "Key"
}

// Describe returns a copy of req with the friendly copy filled in,
// parsing the raw OpenSSH prompt when the request was built in-process
// rather than by the askpass subprocess.
func Describe(req PromptRequest) PromptRequest {
	parsed := ParsePrompt(req.Prompt)
	if req.Kind == "" {
		req.Kind = parsed.Kind
	}
	if req.KeyInfo == "" {
		req.KeyInfo = parsed.KeyInfo
	}
	if req.Title == "" {
		req.Title = parsed.Title
	}
	if req.Message == "" {
		req.Message = parsed.Message
	}
	return req
}

// promptStyle is the finer-grained shape of a prompt, used to pick a
// title, message, and aside that fit the moment. Kind alone cannot tell
// a key passphrase apart from an account password.
type promptStyle int

const (
	stylePassword promptStyle = iota
	stylePassphrase
	stylePIN
	styleHostKey
	styleKeyUse
	styleTouch
)

// classifyStyle narrows a prompt to its style from the lowercased raw
// text and the coarse credential kind.
func classifyStyle(lower string, kind Kind) promptStyle {
	switch {
	case kind == KindTouch:
		return styleTouch
	case kind == KindConfirm:
		if strings.Contains(lower, "allow use of key") {
			return styleKeyUse
		}
		return styleHostKey
	case strings.Contains(lower, "passphrase"):
		return stylePassphrase
	case strings.Contains(lower, "pin"):
		return stylePIN
	default:
		return stylePassword
	}
}

// title is the short dialog heading for the prompt style.
func (s promptStyle) title() string {
	switch s {
	case styleTouch:
		return "Touch Your Security Key"
	case styleHostKey:
		return "New Host"
	case styleKeyUse:
		return "Use SSH Key"
	case stylePassphrase:
		return "SSH Passphrase"
	case stylePIN:
		return "Security Key PIN"
	default:
		return "SSH Password"
	}
}

// message describes what OpenSSH is asking for, in friendly terms,
// followed by a playful aside. Phrasing Crush does not recognize falls
// back to the raw prompt text.
func (s promptStyle) message(prompt, keyInfo string) string {
	switch s {
	case styleTouch:
		return withAside("Tap your security key to continue.", s)
	case styleHostKey:
		return withAside("OpenSSH has not seen this host before. Trust its key?", s)
	case styleKeyUse:
		if keyInfo != "" {
			return withAside("Something wants to use the SSH key at "+keyInfo+".", s)
		}
		return withAside("Something wants to use one of your SSH keys.", s)
	case stylePassphrase:
		if keyInfo != "" {
			return withAside("Unlock the SSH key at "+keyInfo+".", s)
		}
		return withAside("Unlock your SSH key.", s)
	case stylePIN:
		return withAside("Your security key wants its PIN.", s)
	default:
		if keyInfo != "" {
			return withAside("OpenSSH needs the password for "+keyInfo+".", s)
		}
		if trimmed := strings.TrimSpace(prompt); trimmed != "" {
			return withAside(trimmed, s)
		}
		return withAside("OpenSSH needs your input.", s)
	}
}

// withAside appends a playful aside to the message, in the spirit of
// Crush's exit messages.
func withAside(message string, s promptStyle) string {
	if aside := s.aside(); aside != "" {
		return message + " " + aside
	}
	return message
}

// aside returns a random line of light banter that suits the prompt.
func (s promptStyle) aside() string {
	var asides []string
	switch s {
	case stylePIN:
		asides = []string{
			"It's not getting any younger.",
			"It's been tapping its foot all day.",
			"Your key promises not to tell.",
			"A few digits of pure trust.",
			"No judgments here.",
		}
	case stylePassphrase:
		asides = []string{
			"Only you can unlock this one.",
			"Your keys, your castle.",
			"The tumblers are ready when you are.",
			"No peeking.",
		}
	case styleHostKey:
		asides = []string{
			"Trust is a two-way street.",
			"Take your time.",
			"We will wait.",
		}
	case styleKeyUse:
		asides = []string{
			"It says it comes in peace.",
			"Just between us.",
			"Nothing to see here.",
		}
	case styleTouch:
		asides = []string{
			"Your key is all ears.",
			"A gentle tap will do.",
			"Ready when you are.",
		}
	default:
		asides = []string{
			"No peeking.",
			"I won't look, promise.",
			"Your secret stays between us.",
			"Just between us.",
		}
	}
	return asides[rand.IntN(len(asides))]
}

// isTouchPrompt reports whether the prompt is a passive security key
// touch instruction (FIDO user presence). The physical touch is the
// approval, so these never need a decision from the user.
func isTouchPrompt(lower string) bool {
	for _, m := range []string{
		"confirm user presence",
		"touch the device",
		"touch your security key",
		"touch the key",
	} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// isConfirmPrompt reports whether the prompt is a decision question
// whose answer must come from the user: host key acceptance or an
// ssh-add -c key use confirmation.
func isConfirmPrompt(lower string) bool {
	for _, m := range []string{
		"are you sure you want to continue connecting",
		"yes/no",
		"allow use of key",
	} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// keyInfoFromPrompt extracts a short identifying hint from an askpass
// prompt: the account for "user@host's password:" prompts, the key path
// for "Enter passphrase for key '...':" prompts, and the key for
// ssh-add's "Allow use of key ...?" questions.
func keyInfoFromPrompt(prompt string) string {
	lower := strings.ToLower(prompt)
	for _, prefix := range []string{"passphrase for key", "passphrase for", "pin for key", "pin for", "allow use of key"} {
		if i := strings.Index(lower, prefix); i >= 0 {
			return keyPath(prompt[i+len(prefix):])
		}
	}
	if i := strings.Index(lower, "'s password"); i >= 0 {
		return strings.TrimSpace(prompt[:i])
	}
	if i := strings.Index(lower, "password for "); i >= 0 {
		return strings.TrimSpace(strings.TrimRight(prompt[i+len("password for "):], ": "))
	}
	return ""
}

// keyPath cleans the remainder of a prompt that names a key: quoted
// paths keep their content, everything else loses surrounding
// punctuation.
func keyPath(rest string) string {
	rest = strings.TrimSpace(rest)
	if _, path, ok := strings.Cut(rest, "'"); ok {
		if key, _, ok := strings.Cut(path, "'"); ok && key != "" {
			return key
		}
	}
	if end := strings.IndexAny(rest, "?\n"); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(strings.TrimRight(rest, "':"))
}
