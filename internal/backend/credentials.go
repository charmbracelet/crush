package backend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"
)

// ErrUnknownProvider is returned when a managed credential names a
// provider that no config knows.
var ErrUnknownProvider = errors.New("unknown provider")

// ManagedCredentials returns the server-wide managed credential set.
func (b *Backend) ManagedCredentials() *config.ManagedCredentials {
	return b.managed
}

// configStores returns the server's own config store (if any) and every
// workspace's.
func (b *Backend) configStores() []*config.ConfigStore {
	var stores []*config.ConfigStore
	if b.cfg != nil {
		stores = append(stores, b.cfg)
	}
	for _, ws := range b.workspaces.Seq2() {
		if ws.Cfg != nil {
			stores = append(stores, ws.Cfg)
		}
	}
	return stores
}

// isKnownProvider reports whether any config store knows providerID.
func (b *Backend) isKnownProvider(providerID string) bool {
	for _, s := range b.configStores() {
		if s.IsKnownProvider(providerID) {
			return true
		}
	}
	return false
}

// managedKindFor decides how a credential for providerID is presented:
// OAuth when any config holds an OAuth login for it, API key otherwise.
func (b *Backend) managedKindFor(providerID string) config.ManagedCredentialKind {
	kind := config.ManagedKindAPIKey
	for _, s := range b.configStores() {
		if k, ok := s.ManagedKindFor(providerID); ok && k == config.ManagedKindOAuth {
			return k
		}
	}
	return kind
}

// SetManagedCredential stores a credential pushed by an external owner and
// applies it to every workspace. The provider stops refreshing on its own
// and every waiting turn wakes. The credential is never persisted or
// logged.
func (b *Backend) SetManagedCredential(ctx context.Context, providerID string, cred config.ManagedCredential) error {
	if !b.isKnownProvider(providerID) {
		return ErrUnknownProvider
	}
	// The kind is decided once, here, and stored with the credential, so
	// every workspace presents the token the same way.
	if cred.Kind == "" {
		cred.Kind = b.managedKindFor(providerID)
	}
	if !config.SupportsManagedKind(providerID, cred.Kind) {
		return fmt.Errorf("%w: %s cannot take a %s credential", config.ErrUnsupportedManagedKind, providerID, cred.Kind)
	}
	stored := b.managed.Set(providerID, cred)
	b.fanOutManaged(ctx, providerID, false)
	slog.Info("Managed credential updated",
		"provider", providerID, "kind", stored.Kind,
		"expires_at", stored.ExpiresAt, "version", stored.Version)
	return nil
}

// DeleteManagedCredential drops a managed credential. Every store's
// in-memory entry falls back to its normal configuration directly; a
// reload from disk is then attempted best effort, so the provider also
// picks up anything persisted meanwhile. Models built with the dropped
// token are rebuilt before their next use, since a delete moves the
// managed version. It returns ErrUnknownProvider if the provider was not
// managed.
func (b *Backend) DeleteManagedCredential(ctx context.Context, providerID string) error {
	if !b.managed.Delete(providerID) {
		return ErrUnknownProvider
	}
	b.fanOutManaged(ctx, providerID, true)
	slog.Info("Managed credential removed", "provider", providerID)
	return nil
}

// fanOutManaged applies a change to providerID's managed credential to the
// server's store and every workspace.
func (b *Backend) fanOutManaged(ctx context.Context, providerID string, deleted bool) {
	if b.cfg != nil {
		if b.cfg.SyncManagedProvider(providerID) || deleted {
			reloadBestEffort(ctx, b.cfg, providerID)
		}
	}
	for _, ws := range b.workspaces.Seq2() {
		if ws.Cfg == nil {
			continue
		}
		ws.applyManaged(ctx, ws.Cfg.SyncManagedProvider(providerID) || deleted)
	}
}

// applyManaged follows a change in the managed set for a workspace. A
// change that needs the provider list rebuilt (a provider that was not
// configured until now, or a delete) takes the config-reload path, which
// also rebuilds the coordinator's models; a plain token change needs
// nothing here, because the coordinator rebuilds before the next request
// once the managed version moved. A workspace that opened with no
// provider gets its coordinator now that it has one.
func (w *Workspace) applyManaged(ctx context.Context, reload bool) {
	if reload && w.Cfg.WorkingDir() != "" {
		if err := w.reloadConfig(ctx); err != nil {
			slog.Warn("Failed to reload config after a managed credential change",
				"workspace", w.ID, "error", err)
		}
	}
	if w.App == nil || w.Coordinator() != nil || !w.Cfg.Config().IsConfigured() {
		return
	}
	if _, err := w.initAgent(w.ctx, false); err != nil {
		slog.Warn("Failed to initialize the agent after a managed credential arrived",
			"workspace", w.ID, "error", err)
	}
}

func reloadBestEffort(ctx context.Context, s *config.ConfigStore, providerID string) {
	if s.WorkingDir() == "" {
		return
	}
	if err := s.ReloadFromDisk(ctx); err != nil {
		slog.Warn("Failed to reload config after a managed credential change",
			"provider", providerID, "error", err)
	}
}
