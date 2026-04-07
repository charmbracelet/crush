// Package team provides team file persistence and management for agent teams.
package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Member represents a team member agent.
type Member struct {
	AgentID      string    `json:"agent_id"`
	Name         string    `json:"name"`
	AgentType    string    `json:"agent_type,omitempty"`
	Model        string    `json:"model,omitempty"`
	Prompt       string    `json:"prompt,omitempty"`
	Color        string    `json:"color,omitempty"`
	JoinedAt     int64     `json:"joined_at"`
	TmuxPaneID   string    `json:"tmux_pane_id,omitempty"`
	CWD          string    `json:"cwd"`
	WorktreePath string    `json:"worktree_path,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	IsActive     bool      `json:"is_active"`
}

// TeamFile represents the persistent team configuration.
type TeamFile struct {
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	CreatedAt    int64     `json:"created_at"`
	LeadAgentID  string    `json:"lead_agent_id"`
	LeadSessionID string   `json:"lead_session_id,omitempty"`
	Members      []Member  `json:"members"`
}

// Service provides team file persistence operations.
type Service struct {
	mu       sync.RWMutex
	teamsDir string
}

// NewService creates a new team service with the given global data directory.
func NewService(globalDataDir string) *Service {
	return &Service{
		teamsDir: filepath.Join(globalDataDir, "teams"),
	}
}

// GetTeamsDir returns the teams directory path.
func (s *Service) GetTeamsDir() string {
	return s.teamsDir
}

// GetTeamFilePath returns the path to a team's config file.
func (s *Service) GetTeamFilePath(teamName string) string {
	sanitized := SanitizeName(teamName)
	return filepath.Join(s.teamsDir, sanitized, "config.json")
}

// SanitizeName converts a team name to a safe directory name.
func SanitizeName(name string) string {
	// Replace non-alphanumeric characters with hyphens.
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(strings.TrimSpace(name), "-")
	// Remove leading/trailing hyphens.
	sanitized = strings.Trim(sanitized, "-")
	// Limit length.
	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}
	if sanitized == "" {
		sanitized = "team"
	}
	return sanitized
}

// Create creates a new team file.
func (s *Service) Create(teamName string, leadAgentID, leadSessionID string) (*TeamFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	teamFile := &TeamFile{
		Name:         teamName,
		CreatedAt:    time.Now().UnixMilli(),
		LeadAgentID:  leadAgentID,
		LeadSessionID: leadSessionID,
		Members:      []Member{},
	}

	if err := s.writeTeamFile(teamName, teamFile); err != nil {
		return nil, err
	}

	return teamFile, nil
}

// Read reads a team file by name.
func (s *Service) Read(teamName string) (*TeamFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readTeamFile(teamName)
}

// Write writes a team file.
func (s *Service) Write(teamName string, teamFile *TeamFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeTeamFile(teamName, teamFile)
}

// AddMember adds a member to a team.
func (s *Service) AddMember(teamName string, member Member) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	teamFile, err := s.readTeamFile(teamName)
	if err != nil {
		return err
	}

	// Check if member already exists.
	for i, m := range teamFile.Members {
		if m.AgentID == member.AgentID {
			teamFile.Members[i] = member
			return s.writeTeamFile(teamName, teamFile)
		}
	}

	member.JoinedAt = time.Now().UnixMilli()
	teamFile.Members = append(teamFile.Members, member)
	return s.writeTeamFile(teamName, teamFile)
}

// RemoveMember removes a member from a team.
func (s *Service) RemoveMember(teamName, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	teamFile, err := s.readTeamFile(teamName)
	if err != nil {
		return err
	}

	for i, m := range teamFile.Members {
		if m.AgentID == agentID {
			teamFile.Members = append(teamFile.Members[:i], teamFile.Members[i+1:]...)
			return s.writeTeamFile(teamName, teamFile)
		}
	}

	return nil
}

// List lists all teams.
func (s *Service) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.teamsDir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read teams directory: %w", err)
	}

	var teams []string
	for _, entry := range entries {
		if entry.IsDir() {
			configPath := filepath.Join(s.teamsDir, entry.Name(), "config.json")
			if _, err := os.Stat(configPath); err == nil {
				teams = append(teams, entry.Name())
			}
		}
	}

	return teams, nil
}

// Delete deletes a team and its directory.
func (s *Service) Delete(teamName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sanitized := SanitizeName(teamName)
	teamDir := filepath.Join(s.teamsDir, sanitized)
	return os.RemoveAll(teamDir)
}

// writeTeamFile writes a team file to disk.
func (s *Service) writeTeamFile(teamName string, teamFile *TeamFile) error {
	sanitized := SanitizeName(teamName)
	teamDir := filepath.Join(s.teamsDir, sanitized)

	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		return fmt.Errorf("create team directory: %w", err)
	}

	configPath := filepath.Join(teamDir, "config.json")
	data, err := json.MarshalIndent(teamFile, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal team file: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("write team file: %w", err)
	}

	return nil
}

// readTeamFile reads a team file from disk.
func (s *Service) readTeamFile(teamName string) (*TeamFile, error) {
	configPath := s.GetTeamFilePath(teamName)
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("team %q not found", teamName)
	}
	if err != nil {
		return nil, fmt.Errorf("read team file: %w", err)
	}

	var teamFile TeamFile
	if err := json.Unmarshal(data, &teamFile); err != nil {
		return nil, fmt.Errorf("parse team file: %w", err)
	}

	return &teamFile, nil
}
