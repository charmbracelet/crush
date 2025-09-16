package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nom-nom-hub/blush/internal/permission"
)

type APIParams struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
	Format  string            `json:"format,omitempty"`
}

type APIPermissionsParams struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
	Format  string            `json:"format,omitempty"`
}

type apiTool struct {
	client      *http.Client
	permissions permission.Service
	workingDir  string
}

const (
	APIToolName    = "api"
	apiDescription = `Structured API interaction tool that makes HTTP requests with configurable headers, body content, and authentication.

WHEN TO USE THIS TOOL:
- Use when you need to make HTTP requests to APIs
- Helpful for interacting with RESTful services
- Useful for testing API endpoints
- Perfect for retrieving or sending data to web services

HOW TO USE:
- Provide the URL to make the request to
- Specify the HTTP method (GET, POST, PUT, DELETE, etc.)
- Include any required headers
- Add a request body if needed
- Set a timeout if desired
- Choose the output format (json, text, or raw)

FEATURES:
- Supports all HTTP methods (GET, POST, PUT, DELETE, PATCH, etc.)
- Allows custom headers and request bodies
- Configurable timeouts to prevent hanging
- Multiple output formats for different use cases
- Handles JSON serialization/deserialization automatically

LIMITATIONS:
- Cannot handle complex authentication flows (OAuth, etc.)
- Limited to HTTP and HTTPS protocols
- Response size is limited to 5MB

TIPS:
- Use GET for retrieving data
- Use POST for creating new resources
- Use PUT for updating existing resources
- Use DELETE for removing resources
- Always specify Content-Type header when sending data
- Set appropriate timeouts for slow APIs`
)

func NewAPITool(permissions permission.Service, workingDir string) BaseTool {
	return &apiTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func (t *apiTool) Name() string {
	return APIToolName
}

func (t *apiTool) Info() ToolInfo {
	return ToolInfo{
		Name:        APIToolName,
		Description: apiDescription,
		Parameters: map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to make the request to",
			},
			"method": map[string]any{
				"type":        "string",
				"description": "The HTTP method to use (GET, POST, PUT, DELETE, etc.)",
				"default":     "GET",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "HTTP headers to include in the request",
			},
			"body": map[string]any{
				"type":        "object",
				"description": "The request body (for POST, PUT, etc.)",
			},
			"timeout": map[string]any{
				"type":        "number",
				"description": "Timeout in seconds (max 120)",
			},
			"format": map[string]any{
				"type":        "string",
				"description": "The format to return the response in (json, text, raw)",
				"enum":        []string{"json", "text", "raw"},
				"default":     "json",
			},
		},
		Required: []string{"url"},
	}
}

func (t *apiTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params APIParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to parse API parameters: %s", err.Error())), nil
	}

	if params.URL == "" {
		return NewTextErrorResponse("URL parameter is required"), nil
	}

	method := strings.ToUpper(params.Method)
	if method == "" {
		method = "GET"
	}

	// Validate method
	validMethods := map[string]bool{
		"GET":     true,
		"POST":    true,
		"PUT":     true,
		"DELETE":  true,
		"PATCH":   true,
		"HEAD":    true,
		"OPTIONS": true,
	}
	if !validMethods[method] {
		return NewTextErrorResponse(fmt.Sprintf("Invalid HTTP method: %s", method)), nil
	}

	format := strings.ToLower(params.Format)
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "text" && format != "raw" {
		return NewTextErrorResponse("Format must be one of: json, text, raw"), nil
	}

	if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
		return NewTextErrorResponse("URL must start with http:// or https://"), nil
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for API requests")
	}

	p := t.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        t.workingDir,
			ToolCallID:  call.ID,
			ToolName:    APIToolName,
			Action:      method,
			Description: fmt.Sprintf("Make %s request to URL: %s", method, params.URL),
			Params:      APIPermissionsParams(params),
		},
	)

	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	// Handle timeout with context
	requestCtx := ctx
	if params.Timeout > 0 {
		maxTimeout := 120 // 2 minutes
		if params.Timeout > maxTimeout {
			params.Timeout = maxTimeout
		}
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
		defer cancel()
	}

	// Prepare request body
	var body io.Reader
	if params.Body != nil {
		// If body is already a string, use it directly
		if bodyStr, ok := params.Body.(string); ok {
			body = strings.NewReader(bodyStr)
		} else {
			// Otherwise, marshal it to JSON
			bodyBytes, err := json.Marshal(params.Body)
			if err != nil {
				return NewTextErrorResponse(fmt.Sprintf("Failed to marshal request body: %s", err.Error())), nil
			}
			body = bytes.NewReader(bodyBytes)
		}
	}

	req, err := http.NewRequestWithContext(requestCtx, method, params.URL, body)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if params.Headers != nil {
		for key, value := range params.Headers {
			req.Header.Set(key, value)
		}
	}

	// Set default User-Agent if not provided
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "blush/1.0")
	}

	// Set Content-Type if we have a body and it's not already set
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read error response body
		errorBody, _ := io.ReadAll(resp.Body)
		return NewTextErrorResponse(fmt.Sprintf("Request failed with status code: %d\nResponse: %s", resp.StatusCode, string(errorBody))), nil
	}

	maxSize := int64(5 * 1024 * 1024) // 5MB
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to read response body: %s", err.Error())), nil
	}

	content := string(responseBody)

	// Format response based on requested format
	switch format {
	case "json":
		// Try to pretty-print JSON
		var jsonData any
		if json.Unmarshal([]byte(content), &jsonData) == nil {
			prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
			if err == nil {
				content = string(prettyJSON)
			}
		}
		content = "```json\n" + content + "\n```"
	case "text":
		// For text format, we just return the content as-is
	case "raw":
		// For raw format, we also return as-is but might add some metadata
	}

	// Add metadata about the response
	metadata := map[string]any{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
	}

	return WithResponseMetadata(NewTextResponse(content), metadata), nil
}