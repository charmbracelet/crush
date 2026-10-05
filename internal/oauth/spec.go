package oauth

import "strings"

// Auth flow kinds. An auth block present on a provider with no kind is an
// OAuth spec, matching the way subscription providers work.
const (
	AuthKindOAuth  = "oauth"
	AuthKindAPIKey = "api_key"
)

// Auth flow modes, selecting which OAuth grant to run.
const (
	AuthFlowAuto    = "auto"
	AuthFlowBrowser = "browser"
	AuthFlowDevice  = "device"
)

// AuthSpec declares how a provider authenticates when it is not one of the
// built-in ones. It lets a provider be added from config alone, either with
// an OAuth 2.0 flow (browser authorization code with PKCE, or RFC 8628
// device code) or, when absent, the static API key every other custom
// provider already uses.
type AuthSpec struct {
	// Kind selects the authentication mode. Empty and "oauth" mean an
	// OAuth flow; "api_key" documents a static key and disables the flow.
	Kind string `json:"kind,omitempty" jsonschema:"description=Authentication mode, either oauth or api_key,default=oauth"`

	// Flow selects the OAuth grant: "browser" (authorization code with
	// PKCE and a loopback redirect), "device" (RFC 8628 device code), or
	// "auto" (browser with a device fallback). Defaults to auto.
	Flow string `json:"flow,omitempty" jsonschema:"description=OAuth flow to run,enum=auto,enum=browser,enum=device,default=auto"`

	// Issuer is the authorization server's base URL. When the endpoint
	// URLs below are left empty they are discovered from its RFC 8414
	// metadata document.
	Issuer string `json:"issuer,omitempty" jsonschema:"description=Authorization server issuer URL, from which OAuth endpoints are discovered,format=uri,example=https://auth.example.com"`

	ClientID     string `json:"client_id,omitempty" jsonschema:"description=OAuth client ID"`
	ClientSecret string `json:"client_secret,omitempty" jsonschema:"description=OAuth client secret, for confidential clients"`

	// Scopes are requested from the provider. Servers that gate refresh
	// tokens behind offline_access need it listed explicitly.
	Scopes []string `json:"scopes,omitempty" jsonschema:"description=OAuth scopes to request,example=offline_access"`

	// Endpoint URLs. Any left empty are discovered from Issuer.
	AuthorizeURL  string `json:"authorize_url,omitempty" jsonschema:"description=Authorization endpoint URL,format=uri"`
	TokenURL      string `json:"token_url,omitempty" jsonschema:"description=Token endpoint URL,format=uri"`
	DeviceAuthURL string `json:"device_auth_url,omitempty" jsonschema:"description=Device authorization endpoint URL (RFC 8628),format=uri"`

	// RedirectURI overrides the loopback redirect URI the browser flow
	// registers. When empty it is derived from CallbackPort.
	RedirectURI string `json:"redirect_uri,omitempty" jsonschema:"description=Loopback redirect URI for the browser flow,example=http://127.0.0.1:8979/callback"`

	// CallbackPort is the loopback port the browser flow listens on. Zero
	// binds an OS-assigned port. Ignored when RedirectURI is set.
	CallbackPort int `json:"callback_port,omitempty" jsonschema:"description=Loopback callback port for the browser flow, zero picks an available port"`

	// ExtraParams are appended to the authorization request, for
	// provider-specific parameters such as audience or prompt.
	ExtraParams map[string]string `json:"extra_params,omitempty" jsonschema:"description=Extra parameters added to the authorization request"`

	// TokenHeaders are sent on every call to the token endpoint (exchange,
	// refresh, and device polling). Some authorization servers only answer a
	// request that identifies the client they know, so a plugin declares the
	// headers it demands instead of Crush hardcoding any provider's.
	TokenHeaders map[string]string `json:"token_headers,omitempty" jsonschema:"description=HTTP headers sent to the token endpoint"`

	// TokenEncoding is how the token endpoint receives its request:
	// "form" (the RFC 6749 default) or "json". A server that takes JSON
	// also expects the authorization-code exchange to carry the flow's
	// state, which Crush adds.
	TokenEncoding string `json:"token_encoding,omitempty" jsonschema:"description=How token requests are encoded,form or json,default=form"`

	// ClientSecretBasic sends client credentials with HTTP Basic auth
	// instead of in the request body, as some authorization servers
	// require.
	ClientSecretBasic bool `json:"client_secret_basic,omitempty" jsonschema:"description=Send client credentials using HTTP Basic auth instead of in the request body"`
}

// UsesOAuth reports whether the spec describes an OAuth flow. A nil spec or
// an explicit api_key kind means the provider uses a static key.
func (s *AuthSpec) UsesOAuth() bool {
	if s == nil {
		return false
	}
	return s.Kind == "" || strings.EqualFold(s.Kind, AuthKindOAuth)
}

// UsesJSONToken reports whether the token endpoint takes JSON bodies.
func (s *AuthSpec) UsesJSONToken() bool {
	return s != nil && strings.EqualFold(s.TokenEncoding, "json")
}

// FlowMode returns the normalized flow mode: auto, browser, or device.
func (s *AuthSpec) FlowMode() string {
	if s == nil {
		return AuthFlowAuto
	}
	switch strings.ToLower(s.Flow) {
	case AuthFlowBrowser, "authorization_code", "code":
		return AuthFlowBrowser
	case AuthFlowDevice, "device_code":
		return AuthFlowDevice
	default:
		return AuthFlowAuto
	}
}

// Usable reports whether the spec can drive a flow at all: it either names
// its endpoints outright or gives an issuer to discover them from. A token
// URL alone is enough for refreshing an existing sign-in.
func (s *AuthSpec) Usable() bool {
	if s == nil {
		return false
	}
	return s.TokenURL != "" || s.Issuer != ""
}

// UsageSpec declares how to read the quota a provider reports for a
// subscription-style plan, so a plugin can surface what is left without any
// Crush code. The response is walked with gjson paths: an optional list of
// groups, then the meters inside each group.
//
// The defaults are shaped for the common report: meters under "buckets[*]",
// each labelled by "displayName" and carrying "remainingFraction" and
// "resetTime".
type UsageSpec struct {
	// URL of the quota endpoint. Required.
	URL string `json:"url" jsonschema:"required,description=URL of the endpoint that reports remaining quota,format=uri,example=https://api.example.com/v1internal:retrieveUserQuotaSummary"`

	// Method defaults to GET. Providers taking a JSON body usually want POST
	// with an empty Body.
	Method string `json:"method,omitempty" jsonschema:"description=HTTP method for the quota request,default=GET"`

	// Body is the request payload, defaulting to "{}" when the method posts.
	Body string `json:"body,omitempty" jsonschema:"description=Request body for the quota call, defaults to an empty JSON object"`

	// Groups is a gjson path to the groups holding the meters, e.g.
	// "groups". Leave empty when the meters sit at the document root.
	Groups string `json:"groups,omitempty" jsonschema:"description=gjson path to the groups holding the meters,example=groups"`

	// GroupLabel names the field on each group used to label its meters.
	GroupLabel string `json:"group_label,omitempty" jsonschema:"description=Field naming each group,default=displayName"`

	// Meters is a gjson path to the meters, relative to each group when
	// Groups is set and otherwise relative to the document root. Use "." for
	// a response that is a single flat value rather than a list.
	Meters string `json:"meters,omitempty" jsonschema:"description=gjson path to the meters, or . for one flat value,default=buckets,example=buckets"`

	// Title labels the allowance when a meter carries no label of its own,
	// e.g. "Credits" for an endpoint that returns a single balance.
	Title string `json:"title,omitempty" jsonschema:"description=Label for meters that carry no label,example=Credits,default=Remaining"`

	// Window names the field carrying a short tag for the meter, such as
	// "5h" or "weekly", which is what a one-line display shows beside the
	// figure. Defaults to "window".
	Window string `json:"window,omitempty" jsonschema:"description=Field carrying a short tag for the meter,default=window"`

	// ModelGroups maps a model id prefix to the group its allowance comes
	// from, so a display can show the limits that apply to the model in
	// use rather than every plan limit. Values match a substring of the
	// group label, case-insensitively.
	ModelGroups map[string]string `json:"model_groups,omitempty" jsonschema:"description=Model id prefix to group label mapping,example=gemini=Gemini"`

	// Label, Remaining, and Reset name the fields inside each meter.
	// Remaining is a fraction of the allowance left (1 untouched, 0 spent);
	// a value above 1 is read as a raw amount, such as a credit balance.
	Label     string `json:"label,omitempty" jsonschema:"description=Field naming each meter,default=displayName"`
	Remaining string `json:"remaining,omitempty" jsonschema:"description=Field carrying the fraction or amount left,default=remainingFraction"`
	Reset     string `json:"reset,omitempty" jsonschema:"description=Field carrying the RFC 3339 reset time,example=resetTime"`

	// SpentPercent names a field carrying how much of the allowance has been
	// used, stated as a whole percentage (0 untouched, 100 spent): the shape
	// plans that report utilization rather than a balance tend to use. It
	// replaces Remaining, whose fraction it is the complement of.
	//
	// The unit is in the name on purpose. Reading a value above 1 as a
	// percentage and a value at or below 1 as a fraction, the way Remaining
	// reads a value above 1 as a raw amount, would make the one reading that
	// matters most ambiguous: a plan 1% spent and a plan fully spent are both
	// written as 1.
	SpentPercent string `json:"spent_percent,omitempty" jsonschema:"description=Field carrying the percentage of the allowance already used,example=percent"`
}

// Defaults applied to the field-name paths when a spec leaves them empty.
const (
	usageDefaultGroupLabel = "displayName"
	usageDefaultMeters     = "buckets"
	usageDefaultLabel      = "displayName"
	usageDefaultRemaining  = "remainingFraction"
	usageDefaultReset      = "resetTime"
	usageDefaultTitle      = "Remaining"
	usageDefaultWindow     = "window"

	// MetersAtRoot is the Meters path naming a response that holds a single
	// flat value instead of a list of meters.
	MetersAtRoot = "."
)

// GroupLabelPath returns the field naming each group.
func (s *UsageSpec) GroupLabelPath() string { return orDefault(s.GroupLabel, usageDefaultGroupLabel) }

// MetersPath returns the path to the meters.
func (s *UsageSpec) MetersPath() string { return orDefault(s.Meters, usageDefaultMeters) }

// LabelPath returns the field naming each meter.
func (s *UsageSpec) LabelPath() string { return orDefault(s.Label, usageDefaultLabel) }

// TitleOr returns the label used for an unlabeled meter.
func (s *UsageSpec) TitleOr() string { return orDefault(s.Title, usageDefaultTitle) }

// WindowPath returns the field carrying a meter's short tag.
func (s *UsageSpec) WindowPath() string { return orDefault(s.Window, usageDefaultWindow) }

// RemainingPath returns the field carrying the amount left.
func (s *UsageSpec) RemainingPath() string { return orDefault(s.Remaining, usageDefaultRemaining) }

// SpentPercentPath returns the field carrying the percentage already used, or
// "" when the meter reports the fraction left instead.
func (s *UsageSpec) SpentPercentPath() string { return s.SpentPercent }

// ResetPath returns the field carrying the reset time.
func (s *UsageSpec) ResetPath() string { return orDefault(s.Reset, usageDefaultReset) }

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
