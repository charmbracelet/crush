// Package callback renders the browser page a user lands on at the end of
// an OAuth redirect flow.
//
// The page is the only part of authorization the user sees outside the
// terminal, so it is worth more than a line of plain text: it reports
// whether authorization worked, names what was authorized, explains any
// failure in the provider's own words, and offers to close itself.
//
// Rendering is self-contained. Markup, styles, script, and artwork are
// embedded in the binary, so the page works with no network access beyond
// an optional web font.
package callback

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"time"
)

//go:embed page.html page.css page.js heartbit.svg heartbit-grumpy.svg charm.svg
var assets embed.FS

// closeDelay is how long the page counts down before asking the browser to
// close the tab. Long enough to read the outcome, short enough not to feel
// like waiting.
const closeDelay = 5 * time.Second

// tmpl is parsed once at startup. A parse failure means the embedded
// template is broken, which is a build-time mistake rather than anything a
// user can cause, so panicking here fails fast and loudly.
var tmpl = template.Must(template.ParseFS(assets, "page.html"))

// Result describes the outcome of an authorization attempt.
type Result struct {
	// Subject names what was being authorized, such as an MCP server name.
	// Optional; when empty the page simply omits it.
	Subject string

	// ErrorCode is the OAuth error code (for example "access_denied").
	// A non-empty value renders the page in its failure state.
	ErrorCode string

	// ErrorDescription is the provider's human-readable explanation. It is
	// shown alongside ErrorCode and may be empty.
	ErrorDescription string
}

// Failed reports whether the result describes a failed authorization.
func (r Result) Failed() bool { return r.ErrorCode != "" }

// Write renders the callback page for the given result to w. It always
// writes a complete page: if template execution somehow fails midway, the
// error is returned so the caller can log it, but the user is never shown
// a blank tab.
func Write(w io.Writer, r Result) error {
	css, err := assets.ReadFile("page.css")
	if err != nil {
		return fmt.Errorf("read callback stylesheet: %w", err)
	}
	js, err := assets.ReadFile("page.js")
	if err != nil {
		return fmt.Errorf("read callback script: %w", err)
	}
	mark, err := assets.ReadFile("heartbit.svg")
	if err != nil {
		return fmt.Errorf("read callback artwork: %w", err)
	}
	grumpy, err := assets.ReadFile("heartbit-grumpy.svg")
	if err != nil {
		return fmt.Errorf("read callback grumpy artwork: %w", err)
	}
	logo, err := assets.ReadFile("charm.svg")
	if err != nil {
		return fmt.Errorf("read callback logo: %w", err)
	}

	data := struct {
		Title            string
		Kind             string
		Heading          string
		Detail           string
		Subject          string
		ErrorCode        string
		ErrorDescription string
		Status           string
		CloseDelay       int
		CSS              template.CSS
		JS               template.JS
		Heartbit         template.HTML
		Charm            template.HTML
		Favicon          template.URL
	}{
		Subject:          r.Subject,
		ErrorCode:        r.ErrorCode,
		ErrorDescription: r.ErrorDescription,
		CSS:              template.CSS(css),
		JS:               template.JS(js),
		Charm:            template.HTML(logo),
	}

	// The artwork reflects the outcome: a smiling heart on success, a
	// grumpy one when the authorization did not go through. The favicon
	// matches so the tab itself carries the state.
	art := mark
	if r.Failed() {
		art = grumpy
	}
	data.Heartbit = template.HTML(art)
	data.Favicon = template.URL("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(art))

	if r.Failed() {
		data.Title = "Authorization failed — Crush"
		data.Kind = "failed"
		data.Heading = "Authorization failed"
		data.Detail = "Crush was not granted access to"
		if r.Subject == "" {
			data.Detail = "Crush was not granted access."
		}
		// A failed page keeps itself open: the reader needs the reason,
		// and closing the tab out from under them would take it away.
		data.Status = "Close this tab and try again from Crush."
	} else {
		data.Title = "Authorized — Crush"
		data.Kind = "ok"
		data.Heading = "You’re all set"
		data.Detail = "Crush is now connected to"
		if r.Subject == "" {
			data.Detail = "Crush is now connected."
		}
		// Replaced by the countdown as soon as the script runs, so this
		// text is what a reader without JavaScript is left with.
		data.Status = "You can close this tab."
		data.CloseDelay = int(closeDelay.Seconds())
	}

	return tmpl.Execute(w, data)
}

// Serve writes the callback page as a complete HTTP response, choosing a
// status code that matches the outcome. Errors are logged by the caller;
// the page itself is best effort because by this point the browser is
// already committed to rendering whatever arrives.
func Serve(w http.ResponseWriter, r Result) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The page reflects a one-time authorization result and must never be
	// replayed from cache on a later visit to the same localhost URL.
	w.Header().Set("Cache-Control", "no-store")
	if r.Failed() {
		w.WriteHeader(http.StatusBadRequest)
	}
	return Write(w, r)
}
