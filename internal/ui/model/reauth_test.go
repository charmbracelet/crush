package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// TestReAuthenticateNotificationOpensDialog pins the wiring that turns a
// TypeReAuthenticate notification into the authentication dialog for the
// failing provider. This is the UI half of the fix for rejected
// credentials: before it, only Hyper ever published the notification, so
// every other provider surfaced raw 401 errors with no way to fix the key
// in-app.
func TestReAuthenticateNotificationOpensDialog(t *testing.T) {
	t.Parallel()

	const providerID = "kimi"

	reauthEvent := func(provider string) pubsub.Event[notify.Notification] {
		return pubsub.Event[notify.Notification]{
			Type: pubsub.CreatedEvent,
			Payload: notify.Notification{
				Type:       notify.TypeReAuthenticate,
				ProviderID: provider,
			},
		}
	}

	t.Run("api key provider opens the api key input dialog", func(t *testing.T) {
		t.Parallel()

		ws := &countingWorkspace{cfg: reauthTestConfig(providerID)}
		m := newBusyUI(ws)

		m.Update(reauthEvent(providerID))

		assert.NotNil(t, m.dialog.Dialog(dialog.APIKeyInputID))
	})

	t.Run("provider without a selected model opens nothing", func(t *testing.T) {
		t.Parallel()

		ws := &countingWorkspace{cfg: &config.Config{
			Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
				providerID: {ID: providerID, Name: "Kimi"},
			}),
		}}
		m := newBusyUI(ws)

		m.Update(reauthEvent(providerID))

		assert.Nil(t, m.dialog.Dialog(dialog.APIKeyInputID))
	})

	t.Run("unknown provider opens nothing", func(t *testing.T) {
		t.Parallel()

		ws := &countingWorkspace{cfg: &config.Config{
			Providers: csync.NewMapFrom(map[string]config.ProviderConfig{}),
		}}
		m := newBusyUI(ws)

		m.Update(reauthEvent(providerID))

		assert.Nil(t, m.dialog.Dialog(dialog.APIKeyInputID))
	})
}

// reauthTestConfig builds a config whose large model runs on providerID,
// the minimum for a re-authentication prompt to resolve a model.
func reauthTestConfig(providerID string) *config.Config {
	return &config.Config{
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			providerID: {ID: providerID, Name: "Kimi"},
		}),
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: providerID, Model: "kimi-k2"},
		},
	}
}
