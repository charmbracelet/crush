package pinentry

// promptMessage returns the description of the credential GPG is asking
// for.
func promptMessage(kind Kind) string {
	if kind == KindPIN {
		return "Please enter the PIN for your security key:"
	}
	return "Please enter the passphrase for your GPG key:"
}
