package pinentry

import "math/rand/v2"

// promptMessage returns the friendly description of the credential GPG
// is asking for, followed by a playful aside suited to the moment.
func promptMessage(kind Kind) string {
	base := "Your GPG key wants its passphrase."
	if kind == KindPIN {
		base = "Your security key wants its PIN."
	}
	return base + " " + aside(kind)
}

// aside returns a random line of light banter that suits the credential,
// in the spirit of Crush's exit messages.
func aside(kind Kind) string {
	asides := passphraseAsides
	if kind == KindPIN {
		asides = pinAsides
	}
	return asides[rand.IntN(len(asides))]
}

var pinAsides = []string{
	"It's not getting any younger.",
	"Your key promises not to tell.",
	"A few digits of pure trust.",
	"No judgments here.",
}

var passphraseAsides = []string{
	"Only you can unlock this one.",
	"Your keys, your castle.",
	"The tumblers are ready when you are.",
	"No peeking.",
}
