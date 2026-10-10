package oauth

// GatewaySpec declares that a provider's traffic passes through an in-process
// rewrite before it reaches the server, expressed as jq programs. It exists so
// a provider whose wire format differs from what its SDK speaks can be added
// from configuration: the translation is a pair of programs owned by the
// plugin, and Crush runs them without knowing anything about the provider.
//
// The request program receives
//
//	{method, url, headers, body, request_id, token}
//
// where body is the parsed JSON request (null when there is none) and token is
// the provider's credential — its login access token or resolved API key. It
// returns an object whose keys are all optional:
//
//	{url, headers, drop_headers, body}
//
// A url replaces the request's (the Host header is cleared with it, since some
// front ends route on it); headers are set; drop_headers are removed; body
// replaces the request body as JSON.
//
// The response program runs once per response unit. For an ordinary reply it
// receives {status, headers, body, request_id, token} and returns the
// replacement body. For a server-sent event stream it receives
//
//	{event: true, status, headers, data, final, request_id, token}
//
// where data is the parsed event and final marks the last event — determining
// that requires holding one event back, so a stream through a gateway lags the
// upstream by a single event. It returns an array of replacement events, which
// may be empty (drop the event) or longer than one (append synthesized ones).
// Events whose data is not JSON pass through untouched.
//
// Both programs run with two functions beyond jq itself: sha256hex(s) returns
// the lowercase hex SHA-256 of a string, and uuid returns a fresh random
// RFC 4122 version 4 UUID. They exist so a program can sign a request and
// stamp it with a per-request id.
type GatewaySpec struct {
	// Request is the jq program rewriting requests. Required.
	Request string `json:"request" jsonschema:"required,description=jq program rewriting each request, see the gateway adapter contract in the docs"`

	// Response is the jq program rewriting responses. Required.
	Response string `json:"response" jsonschema:"required,description=jq program rewriting each response or stream event, see the gateway adapter contract in the docs"`

	// HTTP1 forces the adapter to speak HTTP/1.1 rather than letting Go
	// negotiate HTTP/2. A server that fingerprints the protocol — the
	// JS-runtime clients this adapter impersonates all run on Node's
	// HTTP/1.1 stack — sees a different client otherwise.
	HTTP1 bool `json:"http1,omitempty" jsonschema:"description=Force HTTP/1.1 instead of negotiating HTTP/2"`
}

// Usable reports whether the spec carries both programs.
func (s *GatewaySpec) Usable() bool {
	return s != nil && s.Request != "" && s.Response != ""
}
