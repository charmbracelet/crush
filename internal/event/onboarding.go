package event

import "time"

// Onboarding events follow a user from launching Crush without a provider to
// the end of their first turn. Together they form a funnel:
//
//	onboarding started
//	→ onboarding prompt held (optional: they typed before picking)
//	→ onboarding provider chosen
//	→ onboarding completed
//	→ onboarding first prompt sent
//	→ onboarding first turn finished
//
// Side events report the ways out of the funnel: onboarding api key failed,
// onboarding sign in ended, onboarding card dismissed and onboarding failed.
//
// Every onboarding event carries an Onboarding, so each one can be read on
// its own: a breakdown on any step shows the provider, path and model known
// at that point, without funnel attribution settings.
//
// They are sent from the TUI process, so they are reliable even when the
// agent runs in a separate server process. None of them carry prompt text or
// credentials.

// Onboarding is what is known about one onboarding attempt when an event is
// sent. Empty fields are left out of the event.
type Onboarding struct {
	// ID is random per attempt. An attempt runs from the first launch
	// without a provider until one is set up, across launches, so one ID
	// is one user's way to a working provider. A machine that loses its
	// provider later starts a new attempt.
	ID string
	// Launch counts the launches of this attempt: 1 the first time, 2 when
	// the user quit and came back, and so on.
	Launch int
	// Provider is the provider chosen so far, such as "anthropic".
	Provider string
	// Via is the path that led to the provider: "card", "card key" or
	// "more providers".
	Via string
	// Model is the model set up with the provider.
	Model string
	// Auth is the kind of credential in use: "oauth", "env var", "api key"
	// or "other". It never holds the credential itself.
	Auth string
}

// props returns the attempt's properties followed by the given pairs.
func (o Onboarding) props(pairs ...any) []any {
	props := make([]any, 0, 12+len(pairs))
	if o.ID != "" {
		props = append(props, "onboarding id", o.ID)
	}
	if o.Launch > 0 {
		props = append(props, "launch", o.Launch)
	}
	for _, p := range [][2]string{
		{"provider", o.Provider},
		{"via", o.Via},
		{"model", o.Model},
		{"auth", o.Auth},
	} {
		if p[1] != "" {
			props = append(props, p[0], p[1])
		}
	}
	return append(props, pairs...)
}

// FirstLaunch is sent once per machine, on the first launch of the TUI.
// SetupWith is the kind of credential already in place, such as "env var"
// or "oauth", or "none" when the user has to pick a provider. Users who
// already have one skip onboarding entirely, so this is the only event that
// counts them as new.
func FirstLaunch(setupWith string) {
	send("first launch", "set up with", setupWith)
}

// OnboardingStarted is sent each time the TUI opens with no provider set
// up. Filter on launch 1 to count each attempt once.
func OnboardingStarted(o Onboarding) {
	send("onboarding started", o.props()...)
}

// OnboardingPromptHeld is sent when a prompt is sent before a provider is
// set up, which is what brings up the provider card.
func OnboardingPromptHeld(o Onboarding, hasAttachments bool) {
	send("onboarding prompt held", o.props("has attachments", hasAttachments)...)
}

// OnboardingProviderChosen is sent when a row of the provider card is
// chosen. Method is how the provider signs in: "api key", "sign in" or
// "more providers" for the full list, where the provider is not known yet.
func OnboardingProviderChosen(o Onboarding, method string) {
	send("onboarding provider chosen", o.props("method", method)...)
}

// OnboardingAPIKeyFailed is sent when a key entered in the provider card does
// not work. Reason is "rejected" when the provider refused it and "error"
// when it could not be checked or saved.
func OnboardingAPIKeyFailed(o Onboarding, reason string) {
	send("onboarding api key failed", o.props("reason", reason)...)
}

// OnboardingSignInEnded is sent when a sign-in step of first run ends
// without a provider being set up. Step is the dialog that was open:
// "sign in", "api key", "auth method" or "models". Outcome is "cancelled"
// when the user backed out, "failed" when the last attempt failed and
// "model unavailable" when the account does not offer the chosen model.
// Success is reported by OnboardingCompleted instead.
func OnboardingSignInEnded(o Onboarding, step, outcome string) {
	send("onboarding sign in ended", o.props("step", step, "outcome", outcome)...)
}

// OnboardingCardDismissed is sent when the user backs out of the provider
// card without picking one.
func OnboardingCardDismissed(o Onboarding, heldPrompt bool) {
	send("onboarding card dismissed", o.props("held prompt", heldPrompt)...)
}

// OnboardingCompleted is sent once a provider and model are set up and the
// agent is ready. HeldPrompt reports whether a prompt was waiting for the
// provider, and duration is the time since this launch opened.
func OnboardingCompleted(o Onboarding, heldPrompt bool, duration time.Duration) {
	d := duration.Truncate(time.Second)
	send("onboarding completed", o.props(
		"held prompt", heldPrompt,
		"duration pretty", d.String(),
		"duration in seconds", int64(d.Seconds()),
	)...)
}

// OnboardingFailed is sent when a provider was picked but the agent could
// not start with it.
func OnboardingFailed(o Onboarding) {
	send("onboarding failed", o.props()...)
}

// OnboardingFirstPromptSent is sent for the first prompt after onboarding.
// Held reports whether it is the prompt that waited for the provider.
func OnboardingFirstPromptSent(o Onboarding, held bool, sinceCompleted time.Duration) {
	d := sinceCompleted.Truncate(time.Second)
	send("onboarding first prompt sent", o.props(
		"held prompt", held,
		"seconds since onboarding completed", int64(d.Seconds()),
	)...)
}

// OnboardingFirstTurnFinished is sent when the first turn after onboarding
// ends. Outcome is "done", "error" or "cancelled". ErrorCategory is set only
// for errors and is a coarse class such as "auth" or "rate limit", never the
// error text.
func OnboardingFirstTurnFinished(o Onboarding, outcome, errorCategory string, duration time.Duration) {
	d := duration.Truncate(time.Second)
	props := []any{"outcome", outcome}
	if errorCategory != "" {
		props = append(props, "error category", errorCategory)
	}
	props = append(props,
		"duration pretty", d.String(),
		"duration in seconds", int64(d.Seconds()),
	)
	send("onboarding first turn finished", o.props(props...)...)
}
