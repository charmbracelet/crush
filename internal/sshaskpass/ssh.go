package sshaskpass

import "strings"

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
// prompt: the account for "user@host's password:" prompts and the key
// path for "Enter passphrase for key '...':" prompts.
func keyInfoFromPrompt(prompt string) string {
	if _, rest, ok := strings.Cut(prompt, "passphrase for key"); ok {
		if _, path, ok := strings.Cut(rest, "'"); ok {
			if key, _, ok := strings.Cut(path, "'"); ok && key != "" {
				return key
			}
		}
		return strings.Trim(strings.TrimRight(rest, ": "), " '")
	}
	// "user@host's password: " -> "user@host"
	lower := strings.ToLower(prompt)
	if account, _, ok := strings.Cut(lower, "'s password"); ok {
		return strings.TrimSpace(account)
	}
	return ""
}
