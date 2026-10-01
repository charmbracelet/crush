package apigen

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"io"
	"sort"
)

// RenderDocs writes a plain, self-contained HTML documentation page for
// the endpoints, grouped by tag.
func RenderDocs(w io.Writer, info Info, base string, endpoints []Endpoint) error {
	return docsTemplate.Execute(w, newDocsModel(info, base, endpoints))
}

type docsModel struct {
	Info Info
	Base string
	Tags []tagGroup
}

type tagGroup struct {
	Name      string
	Endpoints []endpointView
}

type endpointView struct {
	Method      string
	Path        string
	Summary     string
	Description string
	Params      []Param
	Request     string
	Response    string
	Status      int
	Errors      []int
	SSE         bool
}

func newDocsModel(info Info, base string, endpoints []Endpoint) docsModel {
	byTag := map[string][]endpointView{}
	for _, e := range endpoints {
		view := endpointView{
			Method:      e.method,
			Path:        e.path,
			Summary:     e.summary,
			Description: e.description,
			Params:      e.params,
			Status:      e.Status(),
			Errors:      e.errors,
			SSE:         e.sse,
		}
		if e.request != nil {
			view.Request = schemaJSON(SchemaOf(e.request))
		}
		if e.response != nil {
			view.Response = schemaJSON(SchemaOf(e.response))
		}
		tags := e.tags
		if len(tags) == 0 {
			tags = []string{"other"}
		}
		for _, t := range tags {
			byTag[t] = append(byTag[t], view)
		}
	}

	model := docsModel{Info: info, Base: base}
	for _, name := range Tags(endpoints) {
		group := tagGroup{Name: name, Endpoints: byTag[name]}
		sort.Slice(group.Endpoints, func(i, j int) bool {
			return group.Endpoints[i].Path < group.Endpoints[j].Path
		})
		model.Tags = append(model.Tags, group)
	}
	if other, ok := byTag["other"]; ok {
		model.Tags = append(model.Tags, tagGroup{Name: "other", Endpoints: other})
	}
	return model
}

func schemaJSON(s *Schema) string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

//go:embed docs.html.tmpl
var docsTemplateSource string

var docsTemplate = template.Must(template.New("docs").Parse(docsTemplateSource))
