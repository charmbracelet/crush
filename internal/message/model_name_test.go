package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestModelNamePersists covers the model_name column: the display name
// set at creation survives a read, and an update that fills it in later
// persists too, so the UI can keep showing a friendly name after the
// model is renamed or removed from the config.
func TestModelNamePersists(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t)

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{
		Role:      Assistant,
		Model:     "model-id",
		ModelName: "Friendly Name",
		Provider:  "test-provider",
	})
	require.NoError(t, err)

	got, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Equal(t, "Friendly Name", got.ModelName)
	require.Equal(t, "model-id", got.Model)
	require.Equal(t, "test-provider", got.Provider)

	msg.ModelName = "Renamed Name"
	require.NoError(t, svc.Update(t.Context(), msg))
	require.NoError(t, svc.Flush(t.Context(), msg.ID))

	got, err = svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed Name", got.ModelName)

	// Legacy messages created without a name read back empty, not
	// garbage, so the UI falls back to the model ID.
	legacy, err := svc.Create(t.Context(), sessionID, CreateMessageParams{
		Role:     Assistant,
		Model:    "legacy-model",
		Provider: "test-provider",
	})
	require.NoError(t, err)
	got, err = svc.Get(t.Context(), legacy.ID)
	require.NoError(t, err)
	require.Empty(t, got.ModelName)
	require.Equal(t, "legacy-model", got.Model)
}
