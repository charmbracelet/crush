package event

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOnboardingAttemptLastsAcrossLaunches(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crush", OnboardingFileName)

	id, launch := ResumeOnboarding(path)
	require.NotEmpty(t, id)
	require.Equal(t, 1, launch)

	again, launch := ResumeOnboarding(path)
	require.Equal(t, id, again, "quitting and coming back keeps the attempt")
	require.Equal(t, 2, launch)

	FinishOnboarding(path)
	require.NoFileExists(t, path)

	next, launch := ResumeOnboarding(path)
	require.NotEqual(t, id, next, "a finished attempt is not resumed")
	require.Equal(t, 1, launch)
}

func TestOnboardingAttemptSurvivesABrokenFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), OnboardingFileName)
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	id, launch := ResumeOnboarding(path)
	require.NotEmpty(t, id)
	require.Equal(t, 1, launch)
}

func TestFinishOnboardingWithoutAnAttempt(t *testing.T) {
	t.Parallel()

	FinishOnboarding(filepath.Join(t.TempDir(), OnboardingFileName))
}
