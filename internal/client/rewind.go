package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/crush/internal/filehistory"
	"net/http"
)

func (c *Client) Rewind(ctx context.Context, workspaceID, sessionID string, request filehistory.Request) (filehistory.Result, error) {
	response, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/agent/sessions/%s/rewind", workspaceID, sessionID), nil, jsonBody(request), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return filehistory.Result{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var failure struct{ Message string }
		_ = json.NewDecoder(response.Body).Decode(&failure)
		return filehistory.Result{}, fmt.Errorf("rewind: %s", failure.Message)
	}
	var result filehistory.Result
	err = json.NewDecoder(response.Body).Decode(&result)
	return result, err
}
