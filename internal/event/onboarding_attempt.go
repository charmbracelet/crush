package event

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// OnboardingFileName is the file, in Crush's data directory, that keeps an
// onboarding attempt open across launches.
const OnboardingFileName = "onboarding.json"

// onboardingAttempt is an onboarding attempt that has not finished yet.
type onboardingAttempt struct {
	ID       string `json:"id"`
	Launches int    `json:"launches"`
}

// ResumeOnboarding returns the onboarding attempt kept at path and counts
// this launch, starting an attempt when none is open. An attempt lasts until
// FinishOnboarding, so a user who quits and comes back keeps one ID. Launch
// is 1 on the first launch of an attempt.
func ResumeOnboarding(path string) (id string, launch int) {
	var a onboardingAttempt
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &a)
	}
	if a.ID == "" {
		a = onboardingAttempt{ID: uuid.NewString()}
	}
	a.Launches++

	data, err := json.Marshal(a)
	if err == nil {
		if err = os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
			err = os.WriteFile(path, data, 0o600)
		}
	}
	if err != nil {
		// The attempt still works for this launch; the next one starts
		// a new attempt.
		slog.Warn("Failed to save onboarding attempt", "path", path, "error", err)
	}
	return a.ID, a.Launches
}

// FinishOnboarding closes the onboarding attempt kept at path, so a provider
// lost later starts a new one.
func FinishOnboarding(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to clear onboarding attempt", "path", path, "error", err)
	}
}
