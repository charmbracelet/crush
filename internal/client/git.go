package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GitBranch returns the current Git branch of the workspace's working
// directory as reported by the server. It returns an empty string when
// the directory is not a Git repository or HEAD is detached.
func (c *Client) GitBranch(ctx context.Context, id string) (string, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/git/branch", id), nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get git branch: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get git branch: status code %d", rsp.StatusCode)
	}
	var result struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode git branch response: %w", err)
	}
	return result.Branch, nil
}
