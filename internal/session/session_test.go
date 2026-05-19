package session

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) Service {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return NewService(db.New(conn), conn)
}

func TestSessionModels_SaveAndListRoundTrip(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	ctx := context.Background()

	sess, err := svc.Create(ctx, "Round trip")
	require.NoError(t, err)

	temp := 0.42
	topP := 0.9
	large := config.SelectedModel{
		Provider:        "test-provider",
		Model:           "test-large",
		ReasoningEffort: "high",
		MaxTokens:       4096,
		Temperature:     &temp,
		TopP:            &topP,
		Think:           true,
		ProviderOptions: map[string]any{"foo": "bar"},
	}
	small := config.SelectedModel{
		Provider:  "test-provider",
		Model:     "test-small",
		MaxTokens: 1024,
	}

	require.NoError(t, svc.SaveModel(ctx, sess.ID, config.SelectedModelTypeLarge, large))
	require.NoError(t, svc.SaveModel(ctx, sess.ID, config.SelectedModelTypeSmall, small))

	got, err := svc.ListModels(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byType := map[config.SelectedModelType]SessionModel{}
	for _, m := range got {
		byType[m.ModelType] = m
	}

	gotLarge, ok := byType[config.SelectedModelTypeLarge]
	require.True(t, ok, "large row missing")
	require.Equal(t, sess.ID, gotLarge.SessionID)
	require.Equal(t, "test-provider", gotLarge.Provider)
	require.Equal(t, "test-large", gotLarge.Model)
	require.Equal(t, large.ReasoningEffort, gotLarge.SelectedModel.ReasoningEffort)
	require.Equal(t, large.MaxTokens, gotLarge.SelectedModel.MaxTokens)
	require.True(t, gotLarge.SelectedModel.Think)
	require.NotNil(t, gotLarge.SelectedModel.Temperature)
	require.InDelta(t, temp, *gotLarge.SelectedModel.Temperature, 0)
	require.NotNil(t, gotLarge.SelectedModel.TopP)
	require.InDelta(t, topP, *gotLarge.SelectedModel.TopP, 0)
	require.Equal(t, "bar", gotLarge.SelectedModel.ProviderOptions["foo"])

	gotSmall, ok := byType[config.SelectedModelTypeSmall]
	require.True(t, ok, "small row missing")
	require.Equal(t, "test-small", gotSmall.Model)
	require.Equal(t, int64(1024), gotSmall.SelectedModel.MaxTokens)
}

func TestSessionModels_SaveUpsertsExisting(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	ctx := context.Background()

	sess, err := svc.Create(ctx, "Upsert")
	require.NoError(t, err)

	first := config.SelectedModel{Provider: "p1", Model: "m1", MaxTokens: 100}
	second := config.SelectedModel{Provider: "p2", Model: "m2", MaxTokens: 200}

	require.NoError(t, svc.SaveModel(ctx, sess.ID, config.SelectedModelTypeLarge, first))
	require.NoError(t, svc.SaveModel(ctx, sess.ID, config.SelectedModelTypeLarge, second))

	got, err := svc.ListModels(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, got, 1, "upsert must keep a single row per (session_id, model_type)")
	require.Equal(t, "p2", got[0].Provider)
	require.Equal(t, "m2", got[0].Model)
	require.Equal(t, int64(200), got[0].SelectedModel.MaxTokens)
}

func TestSessionModels_DeleteSessionCascades(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	ctx := context.Background()

	sess, err := svc.Create(ctx, "Cascade")
	require.NoError(t, err)

	require.NoError(t, svc.SaveModel(ctx, sess.ID, config.SelectedModelTypeLarge,
		config.SelectedModel{Provider: "p", Model: "l"}))
	require.NoError(t, svc.SaveModel(ctx, sess.ID, config.SelectedModelTypeSmall,
		config.SelectedModel{Provider: "p", Model: "s"}))

	got, err := svc.ListModels(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	require.NoError(t, svc.Delete(ctx, sess.ID))

	got, err = svc.ListModels(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, got, "session_models rows should be removed when the session is deleted")
}

func TestSessionModels_SaveValidation(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	ctx := context.Background()

	sess, err := svc.Create(ctx, "Validation")
	require.NoError(t, err)

	cases := []struct {
		name      string
		sessionID string
		modelType config.SelectedModelType
		model     config.SelectedModel
	}{
		{
			name:      "empty session id",
			sessionID: "",
			modelType: config.SelectedModelTypeLarge,
			model:     config.SelectedModel{Provider: "p", Model: "m"},
		},
		{
			name:      "empty provider",
			sessionID: sess.ID,
			modelType: config.SelectedModelTypeLarge,
			model:     config.SelectedModel{Provider: "", Model: "m"},
		},
		{
			name:      "empty model",
			sessionID: sess.ID,
			modelType: config.SelectedModelTypeLarge,
			model:     config.SelectedModel{Provider: "p", Model: ""},
		},
		{
			name:      "invalid model type",
			sessionID: sess.ID,
			modelType: config.SelectedModelType("medium"),
			model:     config.SelectedModel{Provider: "p", Model: "m"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.SaveModel(ctx, tc.sessionID, tc.modelType, tc.model)
			require.Error(t, err)
		})
	}
}

func TestSessionModels_ListEmptySession(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	ctx := context.Background()

	sess, err := svc.Create(ctx, "Empty")
	require.NoError(t, err)

	got, err := svc.ListModels(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, got)
}
