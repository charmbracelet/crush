package oauth

// CatalogSpec declares the endpoint a provider publishes its model catalog at,
// and the jq program that reads it, so a provider whose listing speaks a
// dialect Crush has never seen can still fill in names, context windows, and
// capability lists at load time. The alternative was a Go enricher per provider
// type, which puts each provider's field names in Crush; here they stay with
// the provider, in the plugin.
//
// The program receives the parsed listing document as its input and returns an
// array of model objects written in the same JSON shape a config file uses for
// a model:
//
//	[{"id": "…", "name": "…", "context_window": 200000,
//	  "default_max_tokens": 8192, "can_reason": true,
//	  "reasoning_levels": ["low", "high"], "default_reasoning_effort": "low",
//	  "supports_attachments": true}]
//
// A program that returns a single object instead of an array gets one. The
// fields are optional; an entry with no id is skipped.
//
// Models the config already declares are not replaced: a declared entry keeps
// every field it sets, and the catalog supplies the rest. A declared model can
// therefore pin a name or a context window the plan reports wrongly without
// giving up the catalog's other details. Booleans are the exception — a
// declared entry can add a capability, not veto one, because "false" and
// "unset" are the same value in configuration.
type CatalogSpec struct {
	// URL of the catalog. An absolute URL is used as given; a path is joined
	// to the provider's base URL, so "/v1/models" follows wherever the
	// provider is hosted.
	URL string `json:"url" jsonschema:"required,description=Endpoint listing the provider's models,format=uri,example=https://api.example.com/v1/models"`

	// Method defaults to GET. Providers taking a JSON body usually want POST
	// with an empty Body, the same shape the quota endpoint of such a gateway
	// takes.
	Method string `json:"method,omitempty" jsonschema:"description=HTTP method for the catalog request,default=GET"`

	// Body is the request payload, defaulting to "{}" when the method posts.
	Body string `json:"body,omitempty" jsonschema:"description=Request body for the catalog call, defaults to an empty JSON object"`

	// Program is the jq program mapping the listing to model objects.
	Program string `json:"program" jsonschema:"required,description=jq program reading the listing into model objects"`

	// Headers are added to the catalog request only. Some APIs gate a
	// listing behind a version header that the inference endpoint does not
	// need, and this keeps it off the provider's other calls.
	Headers map[string]string `json:"headers,omitempty" jsonschema:"description=Headers sent only with the catalog request"`
}

// Usable reports whether the spec can fetch and read a catalog.
func (s *CatalogSpec) Usable() bool {
	return s != nil && s.URL != "" && s.Program != ""
}
