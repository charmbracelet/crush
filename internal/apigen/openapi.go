package apigen

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Info carries the top-level API metadata for the OpenAPI document.
type Info struct {
	Title       string
	Version     string
	Description string
}

// document is an OpenAPI 3.1 document rooted at the API's base path.
type document struct {
	OpenAPI string              `json:"openapi"`
	Info    Info                `json:"info"`
	Paths   map[string]pathItem `json:"paths"`
}

type pathItem map[string]operation

type operation struct {
	Summary     string                  `json:"summary"`
	Description string                  `json:"description,omitempty"`
	Tags        []string                `json:"tags,omitempty"`
	Parameters  []parameter             `json:"parameters,omitempty"`
	RequestBody *requestBody            `json:"requestBody,omitempty"`
	Responses   map[string]specResponse `json:"responses"`
}

type parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required"`
	Schema      *Schema `json:"schema"`
}

type requestBody struct {
	Required bool                 `json:"required"`
	Content  map[string]mediaType `json:"content"`
}

type mediaType struct {
	Schema *Schema `json:"schema"`
}

type specResponse struct {
	Description string               `json:"description"`
	Content     map[string]mediaType `json:"content,omitempty"`
}

var errorSchema = &Schema{
	Type: "object",
	Properties: map[string]*Schema{
		"error": {Type: "string"},
	},
	Required: []string{"error"},
}

// SpecJSON renders the endpoints as an OpenAPI 3.1 JSON document. Base
// is the API's base path (e.g. "/v1"); endpoint paths are relative to
// it. The output is deterministic: map keys are sorted by
// encoding/json.
func SpecJSON(info Info, base string, endpoints []Endpoint) ([]byte, error) {
	doc := document{
		OpenAPI: "3.2.1",
		Info:    info,
		Paths:   map[string]pathItem{},
	}
	for _, e := range endpoints {
		path := strings.TrimPrefix(e.path, base)
		op := operation{
			Summary:     e.summary,
			Description: e.description,
			Tags:        e.tags,
			Responses:   map[string]specResponse{},
		}
		for _, p := range e.params {
			op.Parameters = append(op.Parameters, parameter{
				Name:        p.Name,
				In:          p.In,
				Description: p.Description,
				Required:    p.Required,
				Schema:      &Schema{Type: p.Type},
			})
		}
		if e.request != nil {
			op.RequestBody = &requestBody{
				Required: true,
				Content: map[string]mediaType{
					"application/json": {Schema: SchemaOf(e.request)},
				},
			}
		}
		status := e.Status()
		ok := specResponse{Description: http.StatusText(status)}
		switch {
		case e.sse:
			ok.Content = map[string]mediaType{
				"text/event-stream": {Schema: &Schema{Type: "string"}},
			}
		case e.response != nil:
			ok.Content = map[string]mediaType{
				"application/json": {Schema: SchemaOf(e.response)},
			}
		}
		op.Responses[strconv.Itoa(status)] = ok
		for _, code := range e.errors {
			op.Responses[strconv.Itoa(code)] = specResponse{
				Description: http.StatusText(code),
				Content: map[string]mediaType{
					"application/json": {Schema: errorSchema},
				},
			}
		}

		item := doc.Paths[path]
		if item == nil {
			item = pathItem{}
			doc.Paths[path] = item
		}
		item[strings.ToLower(e.method)] = op
	}
	return json.MarshalIndent(doc, "", "  ")
}

// Tags returns the sorted set of tags across endpoints.
func Tags(endpoints []Endpoint) []string {
	seen := map[string]bool{}
	var tags []string
	for _, e := range endpoints {
		for _, t := range e.tags {
			if !seen[t] {
				seen[t] = true
				tags = append(tags, t)
			}
		}
	}
	sort.Strings(tags)
	return tags
}
