package team

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_Create(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	teamFile, err := svc.Create("test-team", "agent-123", "session-456")
	require.NoError(t, err)
	require.Equal(t, "test-team", teamFile.Name)
	require.Equal(t, "agent-123", teamFile.LeadAgentID)
	require.Equal(t, "session-456", teamFile.LeadSessionID)
	require.NotZero(t, teamFile.CreatedAt)

	configPath := svc.GetTeamFilePath("test-team")
	require.FileExists(t, configPath)
}

func TestService_Read(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Create("test-team", "agent-123", "session-456")
	require.NoError(t, err)

	teamFile, err := svc.Read("test-team")
	require.NoError(t, err)
	require.Equal(t, "test-team", teamFile.Name)
	require.Equal(t, "agent-123", teamFile.LeadAgentID)
}

func TestService_ReadNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Read("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestService_AddMember(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Create("test-team", "agent-123", "session-456")
	require.NoError(t, err)

	member := Member{
		AgentID:   "worker-1",
		Name:      "Worker 1",
		CWD:       "/path/to/work",
		SessionID: "session-789",
		IsActive:  true,
	}

	err = svc.AddMember("test-team", member)
	require.NoError(t, err)

	teamFile, err := svc.Read("test-team")
	require.NoError(t, err)
	require.Len(t, teamFile.Members, 1)
	require.Equal(t, "worker-1", teamFile.Members[0].AgentID)
	require.NotZero(t, teamFile.Members[0].JoinedAt)
}

func TestService_AddMemberUpdate(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Create("test-team", "agent-123", "session-456")
	require.NoError(t, err)

	member1 := Member{
		AgentID:  "worker-1",
		Name:     "Worker 1",
		CWD:      "/path/1",
		IsActive: true,
	}
	err = svc.AddMember("test-team", member1)
	require.NoError(t, err)

	// Update existing member
	member2 := Member{
		AgentID:  "worker-1",
		Name:     "Worker 1 Updated",
		CWD:      "/path/2",
		IsActive: false,
	}
	err = svc.AddMember("test-team", member2)
	require.NoError(t, err)

	teamFile, err := svc.Read("test-team")
	require.NoError(t, err)
	require.Len(t, teamFile.Members, 1)
	require.Equal(t, "Worker 1 Updated", teamFile.Members[0].Name)
	require.Equal(t, "/path/2", teamFile.Members[0].CWD)
}

func TestService_RemoveMember(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Create("test-team", "agent-123", "session-456")
	require.NoError(t, err)

	member := Member{
		AgentID:  "worker-1",
		Name:     "Worker 1",
		CWD:      "/path",
		IsActive: true,
	}
	err = svc.AddMember("test-team", member)
	require.NoError(t, err)

	err = svc.RemoveMember("test-team", "worker-1")
	require.NoError(t, err)

	teamFile, err := svc.Read("test-team")
	require.NoError(t, err)
	require.Empty(t, teamFile.Members)
}

func TestService_List(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Create("team-1", "agent-1", "session-1")
	require.NoError(t, err)

	_, err = svc.Create("team-2", "agent-2", "session-2")
	require.NoError(t, err)

	teams, err := svc.List()
	require.NoError(t, err)
	require.Len(t, teams, 2)
	require.Contains(t, teams, "team-1")
	require.Contains(t, teams, "team-2")
}

func TestService_Delete(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Create("test-team", "agent-123", "session-456")
	require.NoError(t, err)

	err = svc.Delete("test-team")
	require.NoError(t, err)

	_, err = svc.Read("test-team")
	require.Error(t, err)
}

func TestSanitizeName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"my-team", "my-team"},
		{"My Team", "My-Team"},
		{"team:with:special", "team-with-special"},
		{"  spaces  ", "spaces"},
		{"a" + string(rune(0)) + "b", "a-b"},
		{"", "team"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestService_Write(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Create("test-team", "agent-123", "session-456")
	require.NoError(t, err)

	teamFile := &TeamFile{
		Name:         "test-team",
		CreatedAt:    time.Now().UnixMilli(),
		LeadAgentID:  "new-agent",
		LeadSessionID: "new-session",
		Members: []Member{
			{AgentID: "worker-1", Name: "Worker 1"},
		},
	}

	err = svc.Write("test-team", teamFile)
	require.NoError(t, err)

	read, err := svc.Read("test-team")
	require.NoError(t, err)
	require.Equal(t, "new-agent", read.LeadAgentID)
	require.Len(t, read.Members, 1)
}

func TestService_TeamFilePath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	path := svc.GetTeamFilePath("my-team")
	expected := filepath.Join(tmpDir, "teams", "my-team", "config.json")
	require.Equal(t, expected, path)
}
