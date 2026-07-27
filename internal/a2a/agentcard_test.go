package a2a

import (
	"testing"

	a2aspec "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
)

func TestBuildAgentCard(t *testing.T) {
	t.Parallel()

	card := BuildAgentCard(CardParams{
		Agent: config.Agent{
			ID:          "reviewer",
			Name:        "Reviewer",
			Description: "Reviews a worktree diff",
			Model:       config.SelectedModelTypeSmall,
		},
		Skills: []*skills.Skill{
			{Name: "code-review", Description: "Review a diff for bugs"},
			{Name: "run-tests", Description: "Run the test suite"},
		},
		Endpoint: "http://127.0.0.1:8080",
		Version:  "1.2.3",
	})

	require.Equal(t, "Reviewer", card.Name)
	require.Equal(t, "Reviews a worktree diff", card.Description)
	require.Equal(t, "1.2.3", card.Version)
	require.True(t, card.Capabilities.Streaming)

	require.Len(t, card.SupportedInterfaces, 1)
	iface := card.SupportedInterfaces[0]
	require.Equal(t, "http://127.0.0.1:8080", iface.URL)
	require.Equal(t, a2aspec.TransportProtocolJSONRPC, iface.ProtocolBinding)
	require.Equal(t, a2aspec.Version, iface.ProtocolVersion)

	require.Equal(t, []string{"text/plain"}, card.DefaultInputModes)
	require.Equal(t, []string{"text/plain"}, card.DefaultOutputModes)

	require.Len(t, card.Skills, 2)
	first := card.Skills[0]
	require.Equal(t, "code-review", first.ID)
	require.Equal(t, "code-review", first.Name)
	require.Equal(t, "Review a diff for bugs", first.Description)
	require.NotEmpty(t, first.Tags, "A2A requires a non-empty tags list")
}

func TestBuildAgentCardDefaultTransport(t *testing.T) {
	t.Parallel()

	card := BuildAgentCard(CardParams{
		Agent:    config.Agent{Name: "worker"},
		Endpoint: "http://127.0.0.1:9000",
	})
	require.Equal(t, a2aspec.TransportProtocolJSONRPC, card.SupportedInterfaces[0].ProtocolBinding)
}

func TestBuildAgentCardExplicitTransport(t *testing.T) {
	t.Parallel()

	card := BuildAgentCard(CardParams{
		Agent:     config.Agent{Name: "worker"},
		Endpoint:  "http://127.0.0.1:9000",
		Transport: a2aspec.TransportProtocolGRPC,
	})
	require.Equal(t, a2aspec.TransportProtocolGRPC, card.SupportedInterfaces[0].ProtocolBinding)
}

func TestBuildAgentCardNoSkills(t *testing.T) {
	t.Parallel()

	card := BuildAgentCard(CardParams{
		Agent:    config.Agent{Name: "worker"},
		Endpoint: "http://127.0.0.1:9000",
	})
	// Required field: must be non-nil so it marshals as [] not null.
	require.NotNil(t, card.Skills)
	require.Empty(t, card.Skills)
}

func TestBuildAgentCardSkipsNilSkills(t *testing.T) {
	t.Parallel()

	card := BuildAgentCard(CardParams{
		Agent:    config.Agent{Name: "worker"},
		Endpoint: "http://127.0.0.1:9000",
		Skills:   []*skills.Skill{nil, {Name: "real"}, nil},
	})
	require.Len(t, card.Skills, 1)
	require.Equal(t, "real", card.Skills[0].Name)
}

func TestCardNameFallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		agent config.Agent
		want  string
	}{
		{"name wins", config.Agent{ID: "id", Name: "Name"}, "Name"},
		{"id fallback", config.Agent{ID: "id"}, "id"},
		{"generic fallback", config.Agent{}, "crush-agent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, cardName(tc.agent))
		})
	}
}
