package trajectory

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"
)

//go:embed html_template.html
var htmlTemplate string

// RenderHTML renders the trajectory as a standalone HTML document.
func RenderHTML(traj *Trajectory) ([]byte, error) {
	tmpl, err := template.New("trajectory").Parse(htmlTemplate)
	if err != nil {
		return nil, err
	}

	trajJSON, err := json.Marshal(traj)
	if err != nil {
		return nil, err
	}

	data := map[string]any{
		"Title":          traj.Agent.Name + " - " + traj.SessionID,
		"TrajectoryJSON": template.JS(trajJSON),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
