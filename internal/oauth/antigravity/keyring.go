package antigravity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbus "github.com/godbus/dbus/v5"

	"github.com/charmbracelet/crush/internal/oauth"
)

// Antigravity keeps its OAuth tokens in the operating system keyring rather
// than in a file. Reading them lets someone who already signed in with the
// Antigravity CLI use the same session in crush without a second browser trip.
const (
	secretsService  = "org.freedesktop.secrets"
	secretsPath     = "/org/freedesktop/secrets"
	serviceIface    = "org.freedesktop.Secret.Service"
	collectionIface = "org.freedesktop.Secret.Collection"
	itemIface       = "org.freedesktop.Secret.Item"

	// antigravityService and antigravityUser are the attributes the CLI
	// files its credential under.
	antigravityService = "gemini"
	antigravityUser    = "antigravity"
)

// ErrNoKeyring says no Antigravity session could be found to import. It is a
// normal outcome — not an error worth surfacing unless the user asked to
// import — so callers can fall back to a fresh login.
var ErrNoKeyring = errors.New("no Antigravity login found in the system keyring")

// keyringEntry mirrors the JSON Antigravity stores. The credential is nested
// under "token"; the rest is identity the CLI records alongside it.
type keyringEntry struct {
	AuthMethod string          `json:"auth_method"`
	IDToken    string          `json:"id_token"`
	Token      keyringTokenObj `json:"token"`
}

type keyringTokenObj struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Expiry       string `json:"expiry"`
	Email        string `json:"email"`
	ProjectID    string `json:"project_id"`
}

// ImportFromKeyring reads an existing Antigravity CLI session.
//
// The entry is only read, never written: a later refresh here does not touch
// the CLI's copy, and the two keep working independently.
func ImportFromKeyring(ctx context.Context) (*oauth.Token, error) {
	// A locked keyring can block indefinitely inside the bus call, so the
	// read runs off-thread and the caller's timeout wins.
	type result struct {
		raw []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		raw, err := readKeyring()
		done <- result{raw, err}
	}()

	var raw []byte
	select {
	case res := <-done:
		if res.err != nil {
			return nil, res.err
		}
		raw = res.raw
	case <-ctx.Done():
		return nil, fmt.Errorf("reading the system keyring: %w", ctx.Err())
	}

	var entry keyringEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, fmt.Errorf("the Antigravity keyring entry has an unrecognised shape: %w", err)
	}
	if entry.Token.AccessToken == "" {
		return nil, fmt.Errorf("the Antigravity keyring entry holds no access token")
	}

	token := &oauth.Token{
		AccessToken:  entry.Token.AccessToken,
		RefreshToken: entry.Token.RefreshToken,
		IDToken:      entry.IDToken,
		AccountID:    orDefault(entry.Token.Email, EmailFromIDToken(entry.IDToken)),
	}
	if expiry, ok := parseExpiry(entry.Token.Expiry); ok {
		// ExpiresAt is absolute here; ExpiresIn stays zero so the
		// shared IsExpired buffer does not double-count.
		token.ExpiresAt = expiry.Unix()
	} else {
		// Without a usable expiry assume it is stale and refresh
		// eagerly rather than sending a token that will be rejected.
		token.ExpiresAt = time.Now().Unix()
	}
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("the Antigravity login cannot be refreshed (its entry has no refresh token)")
	}
	return token, nil
}

// readKeyring returns the raw Antigravity credential from the freedesktop
// Secret Service.
//
// Two constraints shape this code:
//
//   - It needs one D-Bus connection for the whole OpenSession → GetSecret
//     exchange, because the session object belongs to the connection that
//     created it. Command-line helpers that open a connection per invocation
//     cannot do this at all.
//   - Implementations disagree on method signatures: the spec has
//     SearchItems returning (ao results, o prompt), while KDE's agent
//     returns (ao, ao). Everything below therefore reads *properties*, which
//     are consistent across agents.
func readKeyring() ([]byte, error) {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return nil, fmt.Errorf("connect to the session bus: %w", err)
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return nil, fmt.Errorf("authenticate with the session bus: %w", err)
	}
	if err := conn.Hello(); err != nil {
		return nil, fmt.Errorf("register on the session bus: %w", err)
	}

	service := conn.Object(secretsService, secretsPath)

	var (
		disregard dbus.Variant
		session   dbus.ObjectPath
	)
	if err := service.Call(serviceIface+".OpenSession", 0, "plain", dbus.MakeVariant("")).
		Store(&disregard, &session); err != nil {
		return nil, fmt.Errorf("open a keyring session: %w", err)
	}

	collections, err := listCollections(conn)
	if err != nil {
		return nil, err
	}
	for _, collection := range collections {
		items, err := listItems(conn, collection)
		if err != nil {
			// An unreadable or locked collection is not fatal: another
			// one may hold the credential.
			continue
		}
		for _, item := range items {
			attrs, err := itemAttributes(conn, item)
			if err != nil {
				continue
			}
			if attrs["service"] != antigravityService || attrs["username"] != antigravityUser {
				continue
			}
			return readSecret(conn, item, session)
		}
	}
	return nil, ErrNoKeyring
}

func listCollections(conn *dbus.Conn) ([]dbus.ObjectPath, error) {
	value, err := conn.Object(secretsService, secretsPath).
		GetProperty(serviceIface + ".Collections")
	if err != nil {
		return nil, fmt.Errorf("list keyring collections: %w", err)
	}
	var paths []dbus.ObjectPath
	if err := value.Store(&paths); err != nil {
		return nil, fmt.Errorf("read keyring collections: %w", err)
	}
	return paths, nil
}

func listItems(conn *dbus.Conn, collection dbus.ObjectPath) ([]dbus.ObjectPath, error) {
	value, err := conn.Object(secretsService, collection).
		GetProperty(collectionIface + ".Items")
	if err != nil {
		return nil, err
	}
	var paths []dbus.ObjectPath
	if err := value.Store(&paths); err != nil {
		return nil, err
	}
	return paths, nil
}

func itemAttributes(conn *dbus.Conn, item dbus.ObjectPath) (map[string]string, error) {
	value, err := conn.Object(secretsService, item).
		GetProperty(itemIface + ".Attributes")
	if err != nil {
		return nil, err
	}
	attrs := map[string]string{}
	if err := value.Store(&attrs); err != nil {
		return nil, err
	}
	return attrs, nil
}

// readSecret fetches an item's payload.
//
// The Secret struct's field order is stable in the spec but implementations
// vary in how they type it, so the variant is unpacked generically and the
// payload chosen by shape: it is the longest byte string in the struct and the
// only one that parses as JSON.
func readSecret(conn *dbus.Conn, item dbus.ObjectPath, session dbus.ObjectPath) ([]byte, error) {
	var callVariant dbus.Variant
	if err := conn.Object(secretsService, item).
		Call(itemIface+".GetSecret", 0, session).Store(&callVariant); err != nil {
		return nil, fmt.Errorf("read the Antigravity credential: %w", err)
	}

	fields, ok := callVariant.Value().([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected keyring secret encoding %T", callVariant.Value())
	}

	var candidate []byte
	for _, field := range fields {
		blob, ok := field.([]byte)
		if !ok {
			continue
		}
		if len(blob) <= len(candidate) {
			// Content type ("text/plain") and revision markers are
			// both shorter than any token blob.
			continue
		}
		candidate = blob
	}
	if len(candidate) == 0 {
		return nil, ErrNoKeyring
	}
	if !json.Valid(candidate) {
		return nil, fmt.Errorf("the Antigravity credential is not readable JSON; its keyring entry may be encrypted by a newer CLI version")
	}
	return candidate, nil
}

// parseExpiry accepts the timestamps Antigravity has used across releases.
func parseExpiry(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999999-07:00",
		time.RFC3339,
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	// Unix seconds, for good measure.
	if seconds, err := json.Number(value).Int64(); err == nil && seconds > 0 {
		return time.Unix(seconds, 0), true
	}
	return time.Time{}, false
}
