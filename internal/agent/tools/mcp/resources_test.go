package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// liveResourceSession spins up a real in-memory MCP server advertising the
// given resources and resource templates and returns a connected client
// session, mirroring liveSession for the resources side of the protocol.
func liveResourceSession(t *testing.T, resources []*mcp.Resource, templates []*mcp.ResourceTemplate) *ClientSession {
	t.Helper()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "srv"}, nil)
	handler := func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{}, nil
	}
	for _, r := range resources {
		server.AddResource(r, handler)
	}
	for _, tmpl := range templates {
		server.AddResourceTemplate(tmpl, handler)
	}
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	sess := &ClientSession{ClientSession: clientSession, cancel: cancel}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

// TestUpdateResourceTemplates_SetAndDelOnEmpty pins the registry-write
// semantics updateResourceTemplates shares with updateResources: a non-empty
// list is installed and counted, an empty (or nil) list deletes the entry
// outright — so a server that stops advertising templates does not linger
// with an empty-but-present registry row.
func TestUpdateResourceTemplates_SetAndDelOnEmpty(t *testing.T) {
	const name = "test-update-resource-templates"
	t.Cleanup(func() { allResourceTemplates.Del(name) })

	count := updateResourceTemplates(name, []*ResourceTemplate{
		{Name: "run", URITemplate: "cairn://run/{id}"},
		{Name: "bundle", URITemplate: "cairn://bundle/{id}"},
	})
	require.Equal(t, 2, count)
	got, ok := allResourceTemplates.Get(name)
	require.True(t, ok)
	require.Len(t, got, 2)

	count = updateResourceTemplates(name, nil)
	require.Equal(t, 0, count)
	_, ok = allResourceTemplates.Get(name)
	require.False(t, ok, "an empty template list must delete the registry entry")
}

// TestGetResourceTemplates_NoResourcesCapability pins the capability gate: a
// server that does not advertise the resources capability yields (nil, nil)
// without a wire call, so servers with no resources at all never produce
// warns or errors from the template fetch.
func TestGetResourceTemplates_NoResourcesCapability(t *testing.T) {
	sess, _ := liveSession(t, "no_resources_tool")
	t.Cleanup(func() { _ = sess.Close() })

	templates, err := getResourceTemplates(context.Background(), sess)
	require.NoError(t, err)
	require.Nil(t, templates)
}

// TestGetResourceTemplates_ListsTemplates pins the happy path over a real
// in-memory server: the declared templates come back with their URI
// templates intact.
func TestGetResourceTemplates_ListsTemplates(t *testing.T) {
	sess := liveResourceSession(t, nil, []*mcp.ResourceTemplate{
		{Name: "run", URITemplate: "cairn://run/{id}/a2ui"},
	})

	templates, err := getResourceTemplates(context.Background(), sess)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, "cairn://run/{id}/a2ui", templates[0].URITemplate)
}

// TestRefreshSessionResources_CountsSumResourcesAndTemplates pins the seam
// initClient and the renewal path share: both registries are populated from
// the session and the returned count — what Counts.Resources reports — is
// the sum of concrete resources and templates.
func TestRefreshSessionResources_CountsSumResourcesAndTemplates(t *testing.T) {
	const name = "test-refresh-session-resources"
	t.Cleanup(func() {
		allResources.Del(name)
		allResourceTemplates.Del(name)
	})

	sess := liveResourceSession(t,
		[]*mcp.Resource{
			{Name: "doc-a", URI: "test://doc-a"},
			{Name: "doc-b", URI: "test://doc-b"},
		},
		[]*mcp.ResourceTemplate{
			{Name: "run", URITemplate: "test://run/{id}"},
		},
	)

	count := refreshSessionResources(context.Background(), name, sess)
	require.Equal(t, 3, count, "count must sum resources and templates")

	resources, ok := allResources.Get(name)
	require.True(t, ok)
	require.Len(t, resources, 2)
	templates, ok := allResourceTemplates.Get(name)
	require.True(t, ok)
	require.Len(t, templates, 1)
}

// TestRefreshResources_UpdatesRegistriesAndCount pins the notification-driven
// refresh path end to end: RefreshResources lists both resources and
// templates from the live session and publishes their sum on the state.
func TestRefreshResources_UpdatesRegistriesAndCount(t *testing.T) {
	const name = "test-refresh-resources"
	t.Cleanup(func() {
		sessions.Del(name)
		allResources.Del(name)
		allResourceTemplates.Del(name)
		states.Del(name)
	})

	sess := liveResourceSession(t,
		[]*mcp.Resource{{Name: "doc", URI: "test://doc"}},
		[]*mcp.ResourceTemplate{{Name: "run", URITemplate: "test://run/{id}"}},
	)
	sessions.Set(name, sess)

	RefreshResources(context.Background(), name)

	_, ok := allResources.Get(name)
	require.True(t, ok, "refresh must install the listed resources")
	_, ok = allResourceTemplates.Get(name)
	require.True(t, ok, "refresh must install the listed templates")

	info, ok := GetState(name)
	require.True(t, ok)
	require.Equal(t, StateConnected, info.State)
	require.Equal(t, 2, info.Counts.Resources, "state count must sum resources and templates")
}
