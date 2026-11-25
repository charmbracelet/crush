package dialog

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// issuerWith serves discovery metadata advertising the given endpoints and a
// device authorization response.
func issuerWith(t *testing.T, browser bool) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			authorize := ""
			if browser {
				authorize = fmt.Sprintf(`"authorization_endpoint":%q,`, server.URL+"/authorize")
			}
			fmt.Fprintf(w, `{%s"token_endpoint":%q,"device_authorization_endpoint":%q}`, authorize, server.URL+"/token", server.URL+"/device")
		case "/device":
			fmt.Fprint(w, `{"device_code":"dc","user_code":"ABCD","verification_uri":"https://example.com/activate","expires_in":60}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// freePort returns a loopback port nothing is listening on.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// portFree reports whether the loopback port can be bound.
func portFree(t *testing.T, port int) bool {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// An issuer offering only the device grant must fall back to it in auto
// mode rather than failing to build a browser URL.
func TestOAuthGenericAutoFallsBackToDevice(t *testing.T) {
	t.Parallel()
	server := issuerWith(t, false)
	m := newOAuthGeneric(&oauth.AuthSpec{ClientID: "crush", Issuer: server.URL}, "Example")
	defer m.stopPolling()

	msg := m.initiateAuth()
	got, ok := msg.(ActionInitiateOAuth)
	require.True(t, ok, "got %#v", msg)
	require.Equal(t, "ABCD", got.UserCode)
	require.Equal(t, "dc", got.DeviceCode)
}

// Closing the browser flow is a cancel, not an error, whatever order the
// context and the flow are torn down in.
func TestOAuthGenericClosedBrowserFlowIsCancel(t *testing.T) {
	t.Parallel()
	server := issuerWith(t, true)
	m := newOAuthGeneric(&oauth.AuthSpec{ClientID: "crush", Issuer: server.URL, Flow: oauth.AuthFlowBrowser}, "Example")
	defer m.stopPolling()

	msg := m.initiateAuth()
	_, ok := msg.(ActionInitiateOAuth)
	require.True(t, ok, "got %#v", msg)

	started, err := m.start()
	require.NoError(t, err)
	started.flow.Close()
	require.Nil(t, m.startPolling("", 0)())
}

// Dismissing the dialog stops the flow it started, so the callback port is
// free again for the next attempt.
func TestOAuthGenericStopPollingReleasesCallbackPort(t *testing.T) {
	t.Parallel()
	server := issuerWith(t, true)
	port := freePort(t)
	m := newOAuthGeneric(&oauth.AuthSpec{ClientID: "crush", Issuer: server.URL, CallbackPort: port}, "Example")

	_, ok := m.initiateAuth().(ActionInitiateOAuth)
	require.True(t, ok)
	require.False(t, portFree(t, port), "the flow serves its callback while the dialog is open")

	m.stopPolling()
	require.Eventually(t, func() bool { return portFree(t, port) }, 5*time.Second, 10*time.Millisecond)
}

// Esc during a slow endpoint discovery must abort it: no flow may bind the
// callback port for a dialog that no longer exists.
func TestOAuthGenericCancelDuringDiscoveryBindsNothing(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	port := freePort(t)
	m := newOAuthGeneric(&oauth.AuthSpec{ClientID: "crush", Issuer: server.URL, CallbackPort: port}, "Example")

	msgs := make(chan any, 1)
	go func() { msgs <- m.initiateAuth() }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("discovery never started")
	}

	m.stopPolling()
	select {
	case msg := <-msgs:
		require.Nil(t, msg, "a dismissed dialog reports nothing")
	case <-time.After(5 * time.Second):
		t.Fatal("initiateAuth kept running after the dialog was dismissed")
	}
	require.True(t, portFree(t, port), "no listener may outlive the dialog")
}
