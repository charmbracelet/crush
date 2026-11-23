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
// They are sent from the TUI process, so they are reliable even when the
// agent runs in a separate server process. None of them carry prompt text or
// credentials.

// FirstLaunch is sent once per machine, on the first launch of the TUI.
// SetupWith is the kind of credential already in place, such as "env var"
// or "ant login", or "none" when the user has to pick a provider. Users who
// already have one skip onboarding entirely, so this is the only event that
// counts them as new.
func FirstLaunch(setupWith string) {
	send("first launch", "set up with", setupWith)
}

// OnboardingStarted is sent when the TUI opens with no provider set up.
func OnboardingStarted() {
	send("onboarding started")
}

// OnboardingPromptHeld is sent when a prompt is sent before a provider is
// set up, which is what brings up the provider card.
func OnboardingPromptHeld(hasAttachments bool) {
	send("onboarding prompt held", "has attachments", hasAttachments)
}

// OnboardingProviderChosen is sent when a row of the provider card is
// chosen. Method is how the provider signs in: "api key", "sign in" or
// "more providers" for the full list.
func OnboardingProviderChosen(provider, method string) {
	send(
		"onboarding provider chosen",
		"provider", provider,
		"method", method,
	)
}

// OnboardingAPIKeyFailed is sent when a key entered in the provider card does
// not work. Reason is "rejected" when the provider refused it and "error"
// when it could not be checked or saved.
func OnboardingAPIKeyFailed(provider, reason string) {
	send(
		"onboarding api key failed",
		"provider", provider,
		"reason", reason,
	)
}

// OnboardingSignInEnded is sent when a sign-in step of first run ends
// without a provider being set up. Step is the dialog that was open:
// "sign in", "api key", "auth method" or "models". Outcome is "cancelled"
// when the user backed out, "failed" when the last attempt failed and
// "model unavailable" when the account does not offer the chosen model.
// Success is reported by OnboardingCompleted instead.
func OnboardingSignInEnded(provider, step, outcome string) {
	send(
		"onboarding sign in ended",
		"provider", provider,
		"step", step,
		"outcome", outcome,
	)
}

// OnboardingCardDismissed is sent when the user backs out of the provider
// card without picking one.
func OnboardingCardDismissed(heldPrompt bool) {
	send("onboarding card dismissed", "held prompt", heldPrompt)
}

// OnboardingCompletedProps describes a finished onboarding.
type OnboardingCompletedProps struct {
	Provider string
	Model    string
	// Auth is the kind of credential in use, see the UI's authKind.
	Auth string
	// Via is the path that led to the provider: "card", "card key" or
	// "more providers".
	Via string
	// HeldPrompt reports whether a prompt was waiting for the provider.
	HeldPrompt bool
	// Duration is the time since onboarding started.
	Duration time.Duration
}

// OnboardingCompleted is sent once a provider and model are set up and the
// agent is ready.
func OnboardingCompleted(p OnboardingCompletedProps) {
	d := p.Duration.Truncate(time.Second)
	send(
		"onboarding completed",
		"provider", p.Provider,
		"model", p.Model,
		"auth", p.Auth,
		"via", p.Via,
		"held prompt", p.HeldPrompt,
		"duration pretty", d.String(),
		"duration in seconds", int64(d.Seconds()),
	)
}

// OnboardingFailed is sent when a provider was picked but the agent could
// not start with it.
func OnboardingFailed(provider, model string) {
	send(
		"onboarding failed",
		"provider", provider,
		"model", model,
	)
}

// OnboardingFirstPromptSent is sent for the first prompt after onboarding.
// Held reports whether it is the prompt that waited for the provider.
func OnboardingFirstPromptSent(held bool, sinceCompleted time.Duration) {
	d := sinceCompleted.Truncate(time.Second)
	send(
		"onboarding first prompt sent",
		"held prompt", held,
		"seconds since completed", int64(d.Seconds()),
	)
}

// OnboardingFirstTurnFinished is sent when the first turn after onboarding
// ends. Outcome is "done", "error" or "cancelled". ErrorCategory is set only
// for errors and is a coarse class such as "auth" or "rate limit", never the
// error text.
func OnboardingFirstTurnFinished(outcome, errorCategory string, duration time.Duration) {
	d := duration.Truncate(time.Second)
	send(
		"onboarding first turn finished",
		"outcome", outcome,
		"error category", errorCategory,
		"duration pretty", d.String(),
		"duration in seconds", int64(d.Seconds()),
	)
}

// TipShown is sent the first time a one-time hint is shown on a machine.
func TipShown(name string) {
	send("tip shown", "tip", name)
}

// ProjectInitStarted is sent when /init starts writing the project's context
// file.
func ProjectInitStarted() {
	send("project init started")
}
