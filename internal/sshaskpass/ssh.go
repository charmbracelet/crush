package sshaskpass

import "strings"

// ClassifyPrompt maps an OpenSSH askpass prompt to the credential kind
// and a short key/account hint for the dialog. Confirmations (security
// key user presence, host key acceptance, ssh-add key use) become
// KindConfirm; everything else is treated as a secret prompt.
func ClassifyPrompt(prompt string) (kind Kind, keyInfo string) {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if isConfirmPrompt(lower) {
		return KindConfirm, ""
	}
	return KindPassword, keyInfoFromPrompt(prompt)
}

// isConfirmPrompt reports whether the prompt asks a yes/no question
// rather than a secret. These come from OpenSSH's confirmation dialogs:
// FIDO/security key user presence, host key acceptance, and ssh-add -c.
func isConfirmPrompt(lower string) bool {
	for _, m := range []string{
		"confirm user presence",
		"are you sure you want to continue connecting",
		"yes/no",
		"allow use of key",
		"touch the device",
		"touch your security key",
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
