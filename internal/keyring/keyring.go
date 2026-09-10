// Package keyring stores provider secrets in the OS-native credential
// store (macOS Keychain, Windows Credential Manager, or the freedesktop
// Secret Service on Linux) and lets config values reference them with a
// "keychain://" URI instead of a plaintext literal.
//
// All operations are bounded by a short timeout because keyring daemons
// are session services that can hang; an unreachable daemon degrades to
// the previous plaintext behavior rather than blocking startup.
package keyring

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	zkeyring "github.com/zalando/go-keyring"
)

// Service is the keyring service name under which all Crush secrets
// are stored.
const Service = "crush"

// scheme is the config-value prefix that marks a keyring reference.
const scheme = "keychain://"

// opTimeout bounds a single keyring operation. Keyring daemons are
// on-demand session services; when they are wedged the caller should
// fall back to plaintext instead of blocking startup. It is a variable
// so tests can shorten it.
var opTimeout = 3 * time.Second

// probeUser is a reserved keyring username used only to probe whether
// the backend answers at all. A successful probe never stores anything:
// an existing backend answers ErrNotFound, which counts as available.
const probeUser = "#probe"

// ErrUnavailable reports that no usable keyring backend exists on this
// system, or that the backend did not answer within the operation
// timeout. Callers should fall back to storing the secret in the config
// file.
var ErrUnavailable = errors.New("keyring unavailable")

// Ref returns the config-file reference for a secret stored under the
// given provider ID.
func Ref(providerID string) string {
	return scheme + providerID
}

// ParseRef splits a config value into a keyring provider ID and whether
// the value is a keyring reference at all.
func ParseRef(value string) (providerID string, ok bool) {
	if !strings.HasPrefix(value, scheme) {
		return "", false
	}
	providerID = strings.TrimPrefix(value, scheme)
	return providerID, providerID != ""
}

// IsRef reports whether the config value references a keyring entry.
func IsRef(value string) bool {
	_, ok := ParseRef(value)
	return ok
}

// Available reports whether a usable keyring backend exists on this
// system. The result is probed once and cached for the process
// lifetime; a backend that appears later (for example after unlocking a
// login keychain from a headless session) is picked up on restart.
func Available() bool {
	availableMu.Lock()
	defer availableMu.Unlock()
	if !availableDone {
		availableVal = probe()
		availableDone = true
	}
	return availableVal
}

var (
	availableMu   sync.Mutex
	availableDone bool
	availableVal  bool
)

// ResetAvailableCache clears the cached availability probe so the next
// Available call probes the backend again. Intended for tests.
func ResetAvailableCache() {
	availableMu.Lock()
	defer availableMu.Unlock()
	availableDone = false
}

// probe answers true only when the backend definitively responded:
// the secret exists (nil) or the backend looked and did not find it
// (ErrNotFound). Connection failures, unsupported platforms, and
// timeouts all mean the caller should keep using plaintext storage.
func probe() bool {
	_, err := call(func() (string, error) {
		return zkeyring.Get(Service, probeUser)
	})
	return err == nil || errors.Is(err, zkeyring.ErrNotFound)
}

// Set stores a secret under the given provider ID.
func Set(providerID, secret string) error {
	_, err := call(func() (string, error) {
		return "", zkeyring.Set(Service, providerID, secret)
	})
	if err != nil {
		return fmt.Errorf("failed to store secret for %q in keyring: %w", providerID, err)
	}
	return nil
}

// Get returns the secret stored under the given provider ID. The
// returned error wraps keyring.ErrNotFound when no entry exists.
func Get(providerID string) (string, error) {
	secret, err := call(func() (string, error) {
		return zkeyring.Get(Service, providerID)
	})
	if err != nil {
		return "", fmt.Errorf("failed to read secret for %q from keyring: %w", providerID, err)
	}
	return secret, nil
}

// Delete removes the secret stored under the given provider ID. It is
// not an error when the entry does not exist.
func Delete(providerID string) error {
	_, err := call(func() (string, error) {
		return "", zkeyring.Delete(Service, providerID)
	})
	if err != nil && !errors.Is(err, zkeyring.ErrNotFound) {
		return fmt.Errorf("failed to delete secret for %q from keyring: %w", providerID, err)
	}
	return nil
}

// Verify reports whether the keyring currently holds exactly secret
// under providerID. It reports false when no usable keyring exists,
// the entry is missing, or the stored value differs. Callers use it
// to confirm that a secret actually landed in the keychain instead of
// silently falling back to plaintext storage.
func Verify(providerID, secret string) bool {
	stored, err := Get(providerID)
	return err == nil && stored == secret
}

// IsUnavailable reports whether the error means "no usable keyring on
// this system", as opposed to a per-entry failure such as a missing
// secret.
func IsUnavailable(err error) bool {
	return errors.Is(err, ErrUnavailable) ||
		errors.Is(err, zkeyring.ErrUnsupportedPlatform)
}

// MockInit swaps the underlying backend for an in-memory store. Only
// call from test files; it also opts the test binary into keyring
// calls, which are otherwise disabled under testing.Testing so tests
// never touch the real OS keychain.
func MockInit() {
	mockActive = true
	zkeyring.MockInit()
}

// MockInitWithError swaps the underlying backend for an in-memory store
// that fails every operation with the given error. Only call from test
// files; it also opts the test binary into keyring calls.
func MockInitWithError(err error) {
	mockActive = true
	zkeyring.MockInitWithError(err)
}

// mockActive records that the current test binary explicitly opted into
// keyring calls via MockInit.
var mockActive bool

// call runs a keyring operation on its own goroutine so a wedged daemon
// cannot block the caller past opTimeout. The result channel is
// buffered so the abandoned goroutine cannot leak.
func call(op func() (string, error)) (string, error) {
	// Tests must never read or write the real OS keychain: it is shared
	// state outside the test's control, may prompt or hang on CI
	// runners, and would leak test secrets. Test binaries report every
	// operation as unavailable unless they opted in via MockInit, which
	// keeps production behavior identical while making all callers fall
	// back to plaintext storage deterministically.
	if !mockActive && testing.Testing() {
		return "", fmt.Errorf("keyring disabled under test: %w", ErrUnavailable)
	}

	type result struct {
		value string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		value, err := op()
		ch <- result{value, err}
	}()

	timer := time.NewTimer(opTimeout)
	defer timer.Stop()
	select {
	case res := <-ch:
		return res.value, res.err
	case <-timer.C:
		return "", fmt.Errorf("keyring did not answer within %s: %w", opTimeout, ErrUnavailable)
	}
}
