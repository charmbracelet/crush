package server

import (
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

// crossOriginHandler rejects requests that a web page could have made the
// user's browser send. The API has no authentication, so over TCP any
// page the user visits could otherwise read workspace environments and run
// shell commands. Browsers cannot reach a Unix socket or a named pipe, so
// requests over those pass through untouched, whatever Host they carry.
//
// The Host check defeats DNS rebinding: a page on a domain the attacker
// controls that re-resolves its own name to this machine is same-origin
// with the server as far as the browser is concerned, but its requests
// still carry that name as their Host. The Sec-Fetch-Site and Origin
// checks reject cross-site requests, such as a form or fetch POST with a
// text/plain body, which browsers send without a CORS preflight. Clients
// other than browsers normally send neither header.
func (s *Server) crossOriginHandler(next http.Handler) http.Handler {
	if s.network == "unix" || s.network == "npipe" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isAllowedHost(r.Host) {
			s.logError(r, "Rejected request with an unrecognized Host header", "host", r.Host)
			jsonError(w, http.StatusForbidden,
				"host not allowed: connect via localhost, an IP address, or the host the server listens on")
			return
		}
		if isCrossOrigin(r) {
			s.logError(r, "Rejected cross-origin request",
				"origin", r.Header.Get("Origin"),
				"sec_fetch_site", r.Header.Get("Sec-Fetch-Site"))
			jsonError(w, http.StatusForbidden, "cross-origin requests are not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isAllowedHost reports whether host, a request's Host header, names the
// server in a way DNS rebinding cannot forge: localhost, an IP literal, or
// the host the server listens on. The crush client sends its --host
// address, which passes whenever the server was started with the same one.
func (s *Server) isAllowedHost(host string) bool {
	name := (&url.URL{Host: host}).Hostname()
	if strings.EqualFold(name, "localhost") {
		return true
	}
	if _, err := netip.ParseAddr(name); err == nil {
		return true
	}
	return strings.EqualFold(name, (&url.URL{Host: s.Addr}).Hostname())
}

// isCrossOrigin reports whether a browser marked r as sent by a page from
// another origin. Fetch metadata is checked where the browser sends it,
// and the Origin header covers older browsers.
func isCrossOrigin(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err != nil || u.Host != r.Host
}
