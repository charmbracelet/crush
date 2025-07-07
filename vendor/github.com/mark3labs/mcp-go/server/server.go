// Package server provides MCP (Model Context Protocol) server implementations.
package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// resourceEntry holds both a resource and its handler
type resourceEntry struct {
	resource mcp.Resource
	handler  ResourceHandlerFunc
}

// resourceTemplateEntry holds both a template and its handler
type resourceTemplateEntry struct {
	template mcp.ResourceTemplate
	handler  ResourceTemplateHandlerFunc
}

// ServerOption is a function that configures an MCPServer.
type ServerOption func(*MCPServer)

// ResourceHandlerFunc is a function that returns resource contents.
type ResourceHandlerFunc func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)

// ResourceTemplateHandlerFunc is a function that returns a resource template.
type ResourceTemplateHandlerFunc func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)

// PromptHandlerFunc handles prompt requests with given arguments.
type PromptHandlerFunc func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// ToolHandlerFunc handles tool calls with given arguments.
type ToolHandlerFunc func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

// ToolHandlerMiddleware is a middleware function that wraps a ToolHandlerFunc.
type ToolHandlerMiddleware func(ToolHandlerFunc) ToolHandlerFunc

// ToolFilterFunc is a function that filters tools based on context, typically using session information.
type ToolFilterFunc func(ctx context.Context, tools []mcp.Tool) []mcp.Tool

// ServerTool combines a Tool with its ToolHandlerFunc.
type ServerTool struct {
	Tool    mcp.Tool
	Handler ToolHandlerFunc
}

// ServerPrompt combines a Prompt with its handler function.
type ServerPrompt struct {
	Prompt  mcp.Prompt
	Handler PromptHandlerFunc
}

// ServerResource combines a Resource with its handler function.
type ServerResource struct {
	Resource mcp.Resource
	Handler  ResourceHandlerFunc
}

// serverKey is the context key for storing the server instance
type serverKey struct{}

// ServerFromContext retrieves the MCPServer instance from a context
func ServerFromContext(ctx context.Context) *MCPServer {
	if srv, ok := ctx.Value(serverKey{}).(*MCPServer); ok {
		return srv
	}
	return nil
}

// UnparsableMessageError is attached to the RequestError when json.Unmarshal
// fails on the request.
type UnparsableMessageError struct {
	message json.RawMessage
	method  mcp.MCPMethod
	err     error
}

func (e *UnparsableMessageError) Error() string {
	return fmt.Sprintf("unparsable %s request: %s", e.method, e.err)
}

func (e *UnparsableMessageError) Unwrap() error {
	return e.err
}

func (e *UnparsableMessageError) GetMessage() json.RawMessage {
	return e.message
}

func (e *UnparsableMessageError) GetMethod() mcp.MCPMethod {
	return e.method
}

// RequestError is an error that can be converted to a JSON-RPC error.
// Implements Unwrap() to allow inspecting the error chain.
type requestError struct {
	id   any
	code int
	err  error
}

func (e *requestError) Error() string {
	return fmt.Sprintf("request error: %s", e.err)
}

func (e *requestError) ToJSONRPCError() mcp.JSONRPCError {
	return mcp.JSONRPCError{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(e.id),
		Error: struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    any    `json:"data,omitempty"`
		}{
			Code:    e.code,
			Message: e.err.Error(),
		},
	}
}

func (e *requestError) Unwrap() error {
	return e.err
}

// NotificationHandlerFunc handles incoming notifications.
type NotificationHandlerFunc func(ctx context.Context, notification mcp.JSONRPCNotification)

// MCPServer implements a Model Context Protocol server that can handle various types of requests
// including resources, prompts, and tools.
type MCPServer struct {
	// Separate mutexes for different resource types
	resourcesMu            sync.RWMutex
	promptsMu              sync.RWMutex
	toolsMu                sync.RWMutex
	middlewareMu           sync.RWMutex
	notificationHandlersMu sync.RWMutex
	capabilitiesMu         sync.RWMutex
	toolFiltersMu          sync.RWMutex

	name                   string
	version                string
	instructions           string
	resources              map[string]resourceEntry
	resourceTemplates      map[string]resourceTemplateEntry
	prompts                map[string]mcp.Prompt
	promptHandlers         map[string]PromptHandlerFunc
	tools                  map[string]ServerTool
	toolHandlerMiddlewares []ToolHandlerMiddleware
	toolFilters            []ToolFilterFunc
	notificationHandlers   map[string]NotificationHandlerFunc
	capabilities           serverCapabilities
	paginationLimit        *int
	sessions               sync.Map
	hooks                  *Hooks
}

// WithPaginationLimit sets the pagination limit for the server.
func WithPaginationLimit(limit int) ServerOption {
	return func(s *MCPServer) {
		s.paginationLimit = &limit
	}
}

// serverCapabilities defines the supported features of the MCP server
type serverCapabilities struct {
	tools     *toolCapabilities
	resources *resourceCapabilities
	prompts   *promptCapabilities
	logging   *bool
}

// resourceCapabilities defines the supported resource-related features
type resourceCapabilities struct {
	subscribe   bool
	listChanged bool
}

// promptCapabilities defines the supported prompt-related features
type promptCapabilities struct {
	listChanged bool
}

// toolCapabilities defines the supported tool-related features
type toolCapabilities struct {
	listChanged bool
}

// WithResourceCapabilities configures resource-related server capabilities
func WithResourceCapabilities(subscribe, listChanged bool) ServerOption {
	return func(s *MCPServer) {
		// Always create a non-nil capability object
		s.capabilities.resources = &resourceCapabilities{
			subscribe:   subscribe,
			listChanged: listChanged,
		}
	}
}

// WithToolHandlerMiddleware allows adding a middleware for the
// tool handler call chain.
func WithToolHandlerMiddleware(
	toolHandlerMiddleware ToolHandlerMiddleware,
) ServerOption {
	return func(s *MCPServer) {
		s.middlewareMu.Lock()
		s.toolHandlerMiddlewares = append(s.toolHandlerMiddlewares, toolHandlerMiddleware)
		s.middlewareMu.Unlock()
	}
}

// WithToolFilter adds a filter function that will be applied to tools before they are returned in list_tools
func WithToolFilter(
	toolFilter ToolFilterFunc,
) ServerOption {
	return func(s *MCPServer) {
		s.toolFiltersMu.Lock()
		s.toolFilters = append(s.toolFilters, toolFilter)
		s.toolFiltersMu.Unlock()
	}
}

// WithRecovery adds a middleware that recovers from panics in tool handlers.
func WithRecovery() ServerOption {
	return WithToolHandlerMiddleware(func(next ToolHandlerFunc) ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf(
						"panic recovered in %s tool handler: %v",
						request.Params.Name,
						r,
					)
				}
			}()
			return next(ctx, request)
		}
	})
}

// WithHooks allows adding hooks that will be called before or after
// either [all] requests or before / after specific request methods, or else
// prior to returning an error to the client.
func WithHooks(hooks *Hooks) ServerOption {
	return func(s *MCPServer) {
		s.hooks = hooks
	}
}

// WithPromptCapabilities configures prompt-related server capabilities
func WithPromptCapabilities(listChanged bool) ServerOption {
	return func(s *MCPServer) {
		// Always create a non-nil capability object
		s.capabilities.prompts = &promptCapabilities{
			listChanged: listChanged,
		}
	}
}

// WithToolCapabilities configures tool-related server capabilities
func WithToolCapabilities(listChanged bool) ServerOption {
	return func(s *MCPServer) {
		// Always create a non-nil capability object
		s.capabilities.tools = &toolCapabilities{
			listChanged: listChanged,
		}
	}
}

// WithLogging enables logging capabilities for the server
func WithLogging() ServerOption {
	return func(s *MCPServer) {
		s.capabilities.logging = mcp.ToBoolPtr(true)
	}
}

// WithInstructions sets the server instructions for the client returned in the initialize response
func WithInstructions(instructions string) ServerOption {
	return func(s *MCPServer) {
		s.instructions = instructions
	}
}

// NewMCPServer creates a new MCP server instance with the given name, version and options
func NewMCPServer(
	name, version string,
	opts ...ServerOption,
) *MCPServer {
	s := &MCPServer{
		resources:            make(map[string]resourceEntry),
		resourceTemplates:    make(map[string]resourceTemplateEntry),
		prompts:              make(map[string]mcp.Prompt),
		promptHandlers:       make(map[string]PromptHandlerFunc),
		tools:                make(map[string]ServerTool),
		name:                 name,
		version:              version,
		notificationHandlers: make(map[string]NotificationHandlerFunc),
		capabilities: serverCapabilities{
			tools:     nil,
			resources: nil,
			prompts:   nil,
			logging:   nil,
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// AddResources registers multiple resources at once
func (s *MCPServer) AddResources(resources ...ServerResource) {
	s.implicitlyRegisterResourceCapabilities()

	s.resourcesMu.Lock()
	for _, entry := range resources {
		s.resources[entry.Resource.URI] = resourceEntry{
			resource: entry.Resource,
			handler:  entry.Handler,
		}
	}
	s.resourcesMu.Unlock()

	// When the list of available resources changes, servers that declared the listChanged capability SHOULD send a notification
	if s.capabilities.resources.listChanged {
		// Send notification to all initialized sessions
		s.SendNotificationToAllClients(mcp.MethodNotificationResourcesListChanged, nil)
	}
}

// AddResource registers a new resource and its handler
func (s *MCPServer) AddResource(
	resource mcp.Resource,
	handler ResourceHandlerFunc,
) {
	s.AddResources(ServerResource{Resource: resource, Handler: handler})
}

// RemoveResource removes a resource from the server
func (s *MCPServer) RemoveResource(uri string) {
	s.resourcesMu.Lock()
	_, exists := s.resources[uri]
	if exists {
		delete(s.resources, uri)
	}
	s.resourcesMu.Unlock()

	// Send notification to all initialized sessions if listChanged capability is enabled and we actually remove a resource
	if exists && s.capabilities.resources != nil && s.capabilities.resources.listChanged {
		s.SendNotificationToAllClients(mcp.MethodNotificationResourcesListChanged, nil)
	}
}

// AddResourceTemplate registers a new resource template and its handler
func (s *MCPServer) AddResourceTemplate(
	template mcp.ResourceTemplate,
	handler ResourceTemplateHandlerFunc,
) {
	s.implicitlyRegisterResourceCapabilities()

	s.resourcesMu.Lock()
	s.resourceTemplates[template.URITemplate.Raw()] = resourceTemplateEntry{
		template: template,
		handler:  handler,
	}
	s.resourcesMu.Unlock()

	// When the list of available resources changes, servers that declared the listChanged capability SHOULD send a notification
	if s.capabilities.resources.listChanged {
		// Send notification to all initialized sessions
		s.SendNotificationToAllClients(mcp.MethodNotificationResourcesListChanged, nil)
	}
}

// AddPrompts registers multiple prompts at once
func (s *MCPServer) AddPrompts(prompts ...ServerPrompt) {
	s.implicitlyRegisterPromptCapabilities()

	s.promptsMu.Lock()
	for _, entry := range prompts {
		s.prompts[entry.Prompt.Name] = entry.Prompt
		s.promptHandlers[entry.Prompt.Name] = entry.Handler
	}
	s.promptsMu.Unlock()

	// When the list of available prompts changes, servers that declared the listChanged capability SHOULD send a notification.
	if s.capabilities.prompts.listChanged {
		// Send notification to all initialized sessions
		s.SendNotificationToAllClients(mcp.MethodNotificationPromptsListChanged, nil)
	}
}

// AddPrompt registers a new prompt handler with the given name
func (s *MCPServer) AddPrompt(prompt mcp.Prompt, handler PromptHandlerFunc) {
	s.AddPrompts(ServerPrompt{Prompt: prompt, Handler: handler})
}

// DeletePrompts removes prompts from the server
func (s *MCPServer) DeletePrompts(names ...string) {
	s.promptsMu.Lock()
	var exists bool
	for _, name := range names {
		if _, ok := s.prompts[name]; ok {
			delete(s.prompts, name)
			delete(s.promptHandlers, name)
			exists = true
		}
	}
	s.promptsMu.Unlock()

	// Send notification to all initialized sessions if listChanged capability is enabled, and we actually remove a prompt
	if exists && s.capabilities.prompts != nil && s.capabilities.prompts.listChanged {
		// Send notification to all initialized sessions
		s.SendNotificationToAllClients(mcp.MethodNotificationPromptsListChanged, nil)
	}
}

// AddTool registers a new tool and its handler
func (s *MCPServer) AddTool(tool mcp.Tool, handler ToolHandlerFunc) {
	s.AddTools(ServerTool{Tool: tool, Handler: handler})
}

// Register tool capabilities due to a tool being added.  Default to
// listChanged: true, but don't change the value if we've already explicitly
// registered tools.listChanged false.
func (s *MCPServer) implicitlyRegisterToolCapabilities() {
	s.implicitlyRegisterCapabilities(
		func() bool { return s.capabilities.tools != nil },
		func() { s.capabilities.tools = &toolCapabilities{listChanged: true} },
	)
}

func (s *MCPServer) implicitlyRegisterResourceCapabilities() {
	s.implicitlyRegisterCapabilities(
		func() bool { return s.capabilities.resources != nil },
		func() { s.capabilities.resources = &resourceCapabilities{} },
	)
}

func (s *MCPServer) implicitlyRegisterPromptCapabilities() {
	s.implicitlyRegisterCapabilities(
		func() bool { return s.capabilities.prompts != nil },
		func() { s.capabilities.prompts = &promptCapabilities{} },
	)
}

func (s *MCPServer) implicitlyRegisterCapabilities(check func() bool, register func()) {
	s.capabilitiesMu.RLock()
	if check() {
		s.capabilitiesMu.RUnlock()
		return
	}
	s.capabilitiesMu.RUnlock()

	s.capabilitiesMu.Lock()
	if !check() {
		register()
	}
	s.capabilitiesMu.Unlock()
}

// AddTools registers multiple tools at once
func (s *MCPServer) AddTools(tools ...ServerTool) {
	s.implicitlyRegisterToolCapabilities()

	s.toolsMu.Lock()
	for _, entry := range tools {
		s.tools[entry.Tool.Name] = entry
	}
	s.toolsMu.Unlock()

	// When the list of available tools changes, servers that declared the listChanged capability SHOULD send a notification.
	if s.capabilities.tools.listChanged {
		// Send notification to all initialized sessions
		s.SendNotificationToAllClients(mcp.MethodNotificationToolsListChanged, nil)
	}
}

// SetTools replaces all existing tools with the provided list
func (s *MCPServer) SetTools(tools ...ServerTool) {
	s.toolsMu.Lock()
	s.tools = make(map[string]ServerTool, len(tools))
	s.toolsMu.Unlock()
	s.AddTools(tools...)
}

// DeleteTools removes tools from the server
func (s *MCPServer) DeleteTools(names ...string) {
	s.toolsMu.Lock()
	var exists bool
	for _, name := range names {
		if _, ok := s.tools[name]; ok {
			delete(s.tools, name)
			exists = true
		}
	}
	s.toolsMu.Unlock()

	// When the list of available tools changes, servers that declared the listChanged capability SHOULD send a notification.
	if exists && s.capabilities.tools != nil && s.capabilities.tools.listChanged {
		// Send notification to all initialized sessions
		s.SendNotificationToAllClients(mcp.MethodNotificationToolsListChanged, nil)
	}
}

// AddNotificationHandler registers a new handler for incoming notifications
func (s *MCPServer) AddNotificationHandler(
	method string,
	handler NotificationHandlerFunc,
) {
	s.notificationHandlersMu.Lock()
	defer s.notificationHandlersMu.Unlock()
	s.notificationHandlers[method] = handler
}

func (s *MCPServer) handleInitialize(
	ctx context.Context,
	_ any,
	request mcp.InitializeRequest,
) (*mcp.InitializeResult, *requestError) {
	capabilities := mcp.ServerCapabilities{}

	// Only add resource capabilities if they're configured
	if s.capabilities.resources != nil {
		capabilities.Resources = &struct {
			Subscribe   bool `json:"subscribe,omitempty"`
			ListChanged bool `json:"listChanged,omitempty"`
		}{
			Subscribe:   s.capabilities.resources.subscribe,
			ListChanged: s.capabilities.resources.listChanged,
		}
	}

	// Only add prompt capabilities if they're configured
	if s.capabilities.prompts != nil {
		capabilities.Prompts = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{
			ListChanged: s.capabilities.prompts.listChanged,
		}
	}

	// Only add tool capabilities if they're configured
	if s.capabilities.tools != nil {
		capabilities.Tools = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{
			ListChanged: s.capabilities.tools.listChanged,
		}
	}

	if s.capabilities.logging != nil && *s.capabilities.logging {
		capabilities.Logging = &struct{}{}
	}

	result := mcp.InitializeResult{
		ProtocolVersion: s.protocolVersion(request.Params.ProtocolVersion),
		ServerInfo: mcp.Implementation{
			Name:    s.name,
			Version: s.version,
		},
		Capabilities: capabilities,
		Instructions: s.instructions,
	}

	if session := ClientSessionFromContext(ctx); session != nil {
		session.Initialize()

		// Store client info if the session supports it
		if sessionWithClientInfo, ok := session.(SessionWithClientInfo); ok {
			sessionWithClientInfo.SetClientInfo(request.Params.ClientInfo)
		}
	}
	return &result, nil
}

func (s *MCPServer) protocolVersion(clientVersion string) string {
	if slices.Contains(mcp.ValidProtocolVersions, clientVersion) {
		return clientVersion
	}

	return mcp.LATEST_PROTOCOL_VERSION
}

func (s *MCPServer) handlePing(
	_ context.Context,
	_ any,
	_ mcp.PingRequest,
) (*mcp.EmptyResult, *requestError) {
	return &mcp.EmptyResult{}, nil
}

func (s *MCPServer) handleSetLevel(
	ctx context.Context,
	id any,
	request mcp.SetLevelRequest,
) (*mcp.EmptyResult, *requestError) {
	clientSession := ClientSessionFromContext(ctx)
	if clientSession == nil || !clientSession.Initialized() {
		return nil, &requestError{
			id:   id,
			code: mcp.INTERNAL_ERROR,
			err:  ErrSessionNotInitialized,
		}
	}

	sessionLogging, ok := clientSession.(SessionWithLogging)
	if !ok {
		return nil, &requestError{
			id:   id,
			code: mcp.INTERNAL_ERROR,
			err:  ErrSessionDoesNotSupportLogging,
		}
	}

	level := request.Params.Level
	// Validate logging level
	switch level {
	case mcp.LoggingLevelDebug, mcp.LoggingLevelInfo, mcp.LoggingLevelNotice,
		mcp.LoggingLevelWarning, mcp.LoggingLevelError, mcp.LoggingLevelCritical,
		mcp.LoggingLevelAlert, mcp.LoggingLevelEmergency:
		// Valid level
	default:
		return nil, &requestError{
			id:   id,
			code: mcp.INVALID_PARAMS,
			err:  fmt.Errorf("invalid logging level '%s'", level),
		}
	}

	sessionLogging.SetLogLevel(level)

	return &mcp.EmptyResult{}, nil
}

func listByPagination[T mcp.Named](
	_ context.Context,
	s *MCPServer,
	cursor mcp.Cursor,
	allElements []T,
) ([]T, mcp.Cursor, error) {
	startPos := 0
	if cursor != "" {
		c, err := base64.StdEncoding.DecodeString(string(cursor))
		if err != nil {
			return nil, "", err
		}
		cString := string(c)
		startPos = sort.Search(len(allElements), func(i int) bool {
			return allElements[i].GetName() > cString
		})
	}
	endPos := len(allElements)
	if s.paginationLimit != nil {
		if len(allElements) > startPos+*s.paginationLimit {
			endPos = startPos + *s.paginationLimit
		}
	}
	elementsToReturn := allElements[startPos:endPos]
	// set the next cursor
	nextCursor := func() mcp.Cursor {
		if s.paginationLimit != nil && len(elementsToReturn) >= *s.paginationLimit {
			nc := elementsToReturn[len(elementsToReturn)-1].GetName()
			toString := base64.StdEncoding.EncodeToString([]byte(nc))
			return mcp.Cursor(toString)
		}
		return ""
	}()
	return elementsToReturn, nextCursor, nil
}

func (s *MCPServer) handleListResources(
	ctx context.Context,
	id any,
	request mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, *requestError) {
	s.resourcesMu.RLock()
	resources := make([]mcp.Resource, 0, len(s.resources))
	for _, entry := range s.resources {
		resources = append(resources, entry.resource)
	}
	s.resourcesMu.RUnlock()

	// Sort the resources by name
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})
	resourcesToReturn, nextCursor, err := listByPagination(
		ctx,
		s,
		request.Params.Cursor,
		resources,
	)
	if err != nil {
		return nil, &requestError{
			id:   id,
			code: mcp.INVALID_PARAMS,
			err:  err,
		}
	}
	result := mcp.ListResourcesResult{
		Resources: resourcesToReturn,
		PaginatedResult: mcp.PaginatedResult{
			NextCursor: nextCursor,
		},
	}
	return &result, nil
}

func (s *MCPServer) handleListResourceTemplates(
	ctx context.Context,
	id any,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, *requestError) {
	s.resourcesMu.RLock()
	templates := make([]mcp.ResourceTemplate, 0, len(s.resourceTemplates))
	for _, entry := range s.resourceTemplates {
		templates = append(templates, entry.template)
	}
	s.resourcesMu.RUnlock()
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	templatesToReturn, nextCursor, err := listByPagination(
		ctx,
		s,
		request.Params.Cursor,
		templates,
	)
	if err != nil {
		return nil, &requestError{
			id:   id,
			code: mcp.INVALID_PARAMS,
			err:  err,
		}
	}
	result := mcp.ListResourceTemplatesResult{
		ResourceTemplates: templatesToReturn,
		PaginatedResult: mcp.PaginatedResult{
			NextCursor: nextCursor,
		},
	}
	return &result, nil
}

func (s *MCPServer) handleReadResource(
	ctx context.Context,
	id any,
	request mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, *requestError) {
	s.resourcesMu.RLock()
	// First try direct resource handlers
	if entry, ok := s.resources[request.Params.URI]; ok {
		handler := entry.handler
		s.resourcesMu.RUnlock()
		contents, err := handler(ctx, request)
		if err != nil {
			return nil, &requestError{
				id:   id,
				code: mcp.INTERNAL_ERROR,
				err:  err,
			}
		}
		return &mcp.ReadResourceResult{Contents: contents}, nil
	}

	// If no direct handler found, try matching against templates
	var matchedHandler ResourceTemplateHandlerFunc
	var matched bool
	for _, entry := range s.resourceTemplates {
		template := entry.template
		if matchesTemplate(request.Params.URI, template.URITemplate) {
			matchedHandler = entry.handler
			matched = true
			matchedVars := template.URITemplate.Match(request.Params.URI)
			// Convert matched variables to a map
			request.Params.Arguments = make(map[string]any, len(matchedVars))
			for name, value := range matchedVars {
				request.Params.Arguments[name] = value.V
			}
			break
		}
	}
	s.resourcesMu.RUnlock()

	if matched {
		contents, err := matchedHandler(ctx, request)
		if err != nil {
			return nil, &requestError{
				id:   id,
				code: mcp.INTERNAL_ERROR,
				err:  err,
			}
		}
		return &mcp.ReadResourceResult{Contents: contents}, nil
	}

	return nil, &requestError{
		id:   id,
		code: mcp.RESOURCE_NOT_FOUND,
		err: fmt.Errorf(
			"handler not found for resource URI '%s': %w",
			request.Params.URI,
			ErrResourceNotFound,
		),
	}
}

// matchesTemplate checks if a URI matches a URI template pattern
func matchesTemplate(uri string, template *mcp.URITemplate) bool {
	return template.Regexp().MatchString(uri)
}

func (s *MCPServer) handleListPrompts(
	ctx context.Context,
	id any,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, *requestError) {
	s.promptsMu.RLock()
	prompts := make([]mcp.Prompt, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		prompts = append(prompts, prompt)
	}
	s.promptsMu.RUnlock()

	// sort prompts by name
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Name < prompts[j].Name
	})
	promptsToReturn, nextCursor, err := listByPagination(
		ctx,
		s,
		request.Params.Cursor,
		prompts,
	)
	if err != nil {
		return nil, &requestError{
			id:   id,
			code: mcp.INVALID_PARAMS,
			err:  err,
		}
	}
	result := mcp.ListPromptsResult{
		Prompts: promptsToReturn,
		PaginatedResult: mcp.PaginatedResult{
			NextCursor: nextCursor,
		},
	}
	return &result, nil
}

func (s *MCPServer) handleGetPrompt(
	ctx context.Context,
	id any,
	request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, *requestError) {
	s.promptsMu.RLock()
	handler, ok := s.promptHandlers[request.Params.Name]
	s.promptsMu.RUnlock()

	if !ok {
		return nil, &requestError{
			id:   id,
			code: mcp.INVALID_PARAMS,
			err:  fmt.Errorf("prompt '%s' not found: %w", request.Params.Name, ErrPromptNotFound),
		}
	}

	result, err := handler(ctx, request)
	if err != nil {
		return nil, &requestError{
			id:   id,
			code: mcp.INTERNAL_ERROR,
			err:  err,
		}
	}

	return result, nil
}

func (s *MCPServer) handleListTools(
	ctx context.Context,
	id any,
	request mcp.ListToolsRequest,
) (*mcp.ListToolsResult, *requestError) {
	// Get the base tools from the server
	s.toolsMu.RLock()
	tools := make([]mcp.Tool, 0, len(s.tools))

	// Get all tool names for consistent ordering
	toolNames := make([]string, 0, len(s.tools))
	for name := range s.tools {
		toolNames = append(toolNames, name)
	}

	// Sort the tool names for consistent ordering
	sort.Strings(toolNames)

	// Add tools in sorted order
	for _, name := range toolNames {
		tools = append(tools, s.tools[name].Tool)
	}
	s.toolsMu.RUnlock()

	// Check if there are session-specific tools
	session := ClientSessionFromContext(ctx)
	if session != nil {
		if sessionWithTools, ok := session.(SessionWithTools); ok {
			if sessionTools := sessionWithTools.GetSessionTools(); sessionTools != nil {
				// Override or add session-specific tools
				// We need to create a map first to merge the tools properly
				toolMap := make(map[string]mcp.Tool)

				// Add global tools first
				for _, tool := range tools {
					toolMap[tool.Name] = tool
				}

				// Then override with session-specific tools
				for name, serverTool := range sessionTools {
					toolMap[name] = serverTool.Tool
				}

				// Convert back to slice
				tools = make([]mcp.Tool, 0, len(toolMap))
				for _, tool := range toolMap {
					tools = append(tools, tool)
				}

				// Sort again to maintain consistent ordering
				sort.Slice(tools, func(i, j int) bool {
					return tools[i].Name < tools[j].Name
				})
			}
		}
	}

	// Apply tool filters if any are defined
	s.toolFiltersMu.RLock()
	if len(s.toolFilters) > 0 {
		for _, filter := range s.toolFilters {
			tools = filter(ctx, tools)
		}
	}
	s.toolFiltersMu.RUnlock()

	// Apply pagination
	toolsToReturn, nextCursor, err := listByPagination(
		ctx,
		s,
		request.Params.Cursor,
		tools,
	)
	if err != nil {
		return nil, &requestError{
			id:   id,
			code: mcp.INVALID_PARAMS,
			err:  err,
		}
	}

	result := mcp.ListToolsResult{
		Tools: toolsToReturn,
		PaginatedResult: mcp.PaginatedResult{
			NextCursor: nextCursor,
		},
	}
	return &result, nil
}

func (s *MCPServer) handleToolCall(
	ctx context.Context,
	id any,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, *requestError) {
	// First check session-specific tools
	var tool ServerTool
	var ok bool

	session := ClientSessionFromContext(ctx)
	if session != nil {
		if sessionWithTools, typeAssertOk := session.(SessionWithTools); typeAssertOk {
			if sessionTools := sessionWithTools.GetSessionTools(); sessionTools != nil {
				var sessionOk bool
				tool, sessionOk = sessionTools[request.Params.Name]
				if sessionOk {
					ok = true
				}
			}
		}
	}

	// If not found in session tools, check global tools
	if !ok {
		s.toolsMu.RLock()
		tool, ok = s.tools[request.Params.Name]
		s.toolsMu.RUnlock()
	}

	if !ok {
		return nil, &requestError{
			id:   id,
			code: mcp.INVALID_PARAMS,
			err:  fmt.Errorf("tool '%s' not found: %w", request.Params.Name, ErrToolNotFound),
		}
	}

	finalHandler := tool.Handler

	s.middlewareMu.RLock()
	mw := s.toolHandlerMiddlewares
	s.middlewareMu.RUnlock()

	// Apply middlewares in reverse order
	for i := len(mw) - 1; i >= 0; i-- {
		finalHandler = mw[i](finalHandler)
	}

	result, err := finalHandler(ctx, request)
	if err != nil {
		return nil, &requestError{
			id:   id,
			code: mcp.INTERNAL_ERROR,
			err:  err,
		}
	}

	return result, nil
}

func (s *MCPServer) handleNotification(
	ctx context.Context,
	notification mcp.JSONRPCNotification,
) mcp.JSONRPCMessage {
	s.notificationHandlersMu.RLock()
	handler, ok := s.notificationHandlers[notification.Method]
	s.notificationHandlersMu.RUnlock()

	if ok {
		handler(ctx, notification)
	}
	return nil
}

func createResponse(id any, result any) mcp.JSONRPCMessage {
	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(id),
		Result:  result,
	}
}

func createErrorResponse(
	id any,
	code int,
	message string,
) mcp.JSONRPCMessage {
	return mcp.JSONRPCError{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(id),
		Error: struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    any    `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
		},
	}
}
