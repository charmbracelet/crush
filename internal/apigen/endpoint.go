// Package apigen provides a fluent API for documenting HTTP endpoints.
//
// An [Endpoint] is the single source of truth for one route: the same
// value drives mux registration and documentation generation, so the
// two cannot drift. Endpoints are constructed with the method builders
// ([Get], [Post], [Put], [Delete]) and finalized with
// [endpointBuilder.Handle], the only way to produce an Endpoint. A
// chain without Handle does not compile into a []Endpoint, so
// incomplete definitions fail at build time.
package apigen

import "net/http"

// Param documents a single path or query parameter.
type Param struct {
	// Name is the parameter name as it appears in the path or query.
	Name string
	// In is "path" or "query".
	In string
	// Type is the JSON schema type, e.g. "string", "integer".
	Type string
	// Description is a short human-readable description.
	Description string
	// Required reports whether the parameter must be provided. Path
	// parameters are always required.
	Required bool
}

// Endpoint is the documentation and routing record for a single HTTP
// endpoint. Construct it with the fluent builders ([Get], [Post], ...);
// only [endpointBuilder.Handle] produces a complete Endpoint.
type Endpoint struct {
	method      string
	path        string
	summary     string
	description string
	tags        []string
	params      []Param
	request     any
	response    any
	status      int
	errors      []int
	sse         bool
	handler     http.HandlerFunc
}

// Method returns the HTTP method, e.g. "GET".
func (e Endpoint) Method() string { return e.method }

// Path returns the route pattern, e.g. "/v1/workspaces/{id}".
func (e Endpoint) Path() string { return e.path }

// Summary returns the one-line summary.
func (e Endpoint) Summary() string { return e.summary }

// Description returns the optional longer prose description.
func (e Endpoint) Description() string { return e.description }

// Tags returns the grouping tags.
func (e Endpoint) Tags() []string { return e.tags }

// Params returns the documented path and query parameters.
func (e Endpoint) Params() []Param { return e.params }

// Request returns the request body example value, or nil for none.
func (e Endpoint) Request() any { return e.request }

// Response returns the success response example value, or nil for an
// empty success response.
func (e Endpoint) Response() any { return e.response }

// Status returns the success status code, defaulting to 200.
func (e Endpoint) Status() int {
	if e.status == 0 {
		return http.StatusOK
	}
	return e.status
}

// Errors returns the status codes that return an error object.
func (e Endpoint) Errors() []int { return e.errors }

// SSE reports whether the endpoint streams text/event-stream instead
// of producing a JSON response.
func (e Endpoint) SSE() bool { return e.sse }

// Handler returns the endpoint's HTTP handler.
func (e Endpoint) Handler() http.HandlerFunc { return e.handler }

// endpointBuilder accumulates endpoint documentation fluently. It is
// deliberately a distinct type from [Endpoint]: a []Endpoint literal
// cannot contain a builder, so a chain missing Handle fails to
// compile.
type endpointBuilder struct{ e Endpoint }

// Get begins a GET endpoint for path, e.g. Get("/v1/workspaces/{id}").
func Get(path string) endpointBuilder {
	return endpointBuilder{Endpoint{method: http.MethodGet, path: path}}
}

// Post begins a POST endpoint for path.
func Post(path string) endpointBuilder {
	return endpointBuilder{Endpoint{method: http.MethodPost, path: path}}
}

// Put begins a PUT endpoint for path.
func Put(path string) endpointBuilder {
	return endpointBuilder{Endpoint{method: http.MethodPut, path: path}}
}

// Delete begins a DELETE endpoint for path.
func Delete(path string) endpointBuilder {
	return endpointBuilder{Endpoint{method: http.MethodDelete, path: path}}
}

// Summary sets the one-line summary.
func (b endpointBuilder) Summary(s string) endpointBuilder { b.e.summary = s; return b }

// Description sets an optional longer prose description.
func (b endpointBuilder) Description(s string) endpointBuilder { b.e.description = s; return b }

// Tags sets the grouping tags used on the docs page.
func (b endpointBuilder) Tags(t ...string) endpointBuilder { b.e.tags = t; return b }

// Accepts documents the request body with an example value, e.g.
// Accepts(proto.Workspace{}). The value's type is used to derive the
// JSON schema; the reference is compile-checked.
func (b endpointBuilder) Accepts(v any) endpointBuilder { b.e.request = v; return b }

// Responds documents the success response body with an example value.
func (b endpointBuilder) Responds(v any) endpointBuilder { b.e.response = v; return b }

// Status sets the success status code when it differs from 200, e.g.
// 202 for an accepted-but-async operation.
func (b endpointBuilder) Status(code int) endpointBuilder { b.e.status = code; return b }

// Fails documents status codes that return an error object.
func (b endpointBuilder) Fails(codes ...int) endpointBuilder { b.e.errors = codes; return b }

// SSE marks the endpoint as streaming text/event-stream.
func (b endpointBuilder) SSE() endpointBuilder { b.e.sse = true; return b }

// PathParam documents a path parameter. Path parameters are always
// required.
func (b endpointBuilder) PathParam(name, desc string) endpointBuilder {
	return b.param(Param{Name: name, In: "path", Type: "string", Description: desc, Required: true})
}

// QueryParam documents an optional query parameter.
func (b endpointBuilder) QueryParam(name, desc string) endpointBuilder {
	return b.param(Param{Name: name, In: "query", Type: "string", Description: desc})
}

// RequiredQueryParam documents a required query parameter.
func (b endpointBuilder) RequiredQueryParam(name, desc string) endpointBuilder {
	return b.param(Param{Name: name, In: "query", Type: "string", Description: desc, Required: true})
}

func (b endpointBuilder) param(p Param) endpointBuilder {
	b.e.params = append(b.e.params, p)
	return b
}

// Handle finalizes the endpoint with its HTTP handler.
func (b endpointBuilder) Handle(h http.HandlerFunc) Endpoint {
	b.e.handler = h
	return b.e
}
