package spec

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseSpec(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid minimal spec",
			yaml: `
id: test-spec
name: Test Spec
version: 1.0.0
description: A test specification
`,
			wantErr: false,
		},
		{
			name: "valid full spec",
			yaml: `
id: auth-system
name: User Authentication System
version: 1.2.0
status: active
owner: alice
created: 2024-01-15
description: |
  Implement a complete user authentication system with JWT tokens.

goals:
  - Secure user authentication
  - Support multiple providers

non_goals:
  - Two-factor authentication

constraints:
  technical:
    - Must use PostgreSQL
  business:
    - Launch by Q2 2024
  resources:
    - 2 developers

requirements:
  functional:
    - id: FR-001
      description: User can register
      priority: P0
      acceptance_criteria:
        - Email validation
        - Password strength

  non_functional:
    - id: NFR-001
      category: security
      description: Passwords must be hashed
      details: Use bcrypt

entities:
  - name: User
    description: A registered user
    fields:
      - name: id
        type: UUID
        description: Unique identifier
      - name: email
        type: string
        description: User email

api_endpoints:
  - path: /auth/register
    method: POST
    requirement: FR-001
    description: Register new user
    request:
      email: string
      password: string
    response:
      user: User
    errors:
      - code: EMAIL_EXISTS
        message: Email already registered

dependencies:
  internal:
    - Database pool
  external:
    - OAuth providers

metadata:
  created_by: alice
  approved: true
`,
			wantErr: false,
		},
		{
			name: "missing required fields",
			yaml: `
name: Test Spec
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			spec, err := parser.Parse([]byte(tt.yaml))

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, spec)
		})
	}
}

func TestManagerCRUD(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Create
	originalTime := time.Now()
	timeNow = func() time.Time { return originalTime }
	defer func() { timeNow = func() time.Time { return time.Now() } }()

	spec, err := manager.Create("Test Spec", "A test specification")
	require.NoError(t, err)
	require.Equal(t, "test-spec", spec.ID)
	require.Equal(t, "Test Spec", spec.Name)
	require.Equal(t, "1.0.0", spec.Version)
	require.Equal(t, SpecStatusDraft, spec.Status)

	// Load
	loaded, err := manager.Load("test-spec")
	require.NoError(t, err)
	require.Equal(t, spec.ID, loaded.ID)
	require.Equal(t, spec.Name, loaded.Name)

	// Modify and Save
	loaded.Description = "Updated description"
	loaded.Goals = []string{"Goal 1", "Goal 2"}
	err = manager.Save(loaded)
	require.NoError(t, err)

	// Reload and verify
	reloaded, err := manager.Load("test-spec")
	require.NoError(t, err)
	require.Equal(t, "Updated description", reloaded.Description)
	require.Equal(t, []string{"Goal 1", "Goal 2"}, reloaded.Goals)

	// LoadAll
	allSpecs, err := manager.LoadAll()
	require.NoError(t, err)
	require.Len(t, allSpecs, 1)

	// Create another spec
	_, err = manager.Create("Another Spec", "Another test")
	require.NoError(t, err)

	allSpecs, err = manager.LoadAll()
	require.NoError(t, err)
	require.Len(t, allSpecs, 2)
}

func TestGenerateSpecID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "Auth System",
			expected: "auth-system",
		},
		{
			name:     "with special characters",
			input:    "User @ Authentication!",
			expected: "user-authentication",
		},
		{
			name:     "already lowercase hyphenated",
			input:    "payment-processing",
			expected: "payment-processing",
		},
		{
			name:     "multiple spaces",
			input:    "Multiple   Word   Name",
			expected: "multiple-word-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSpecID(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateSpec(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		spec        *Spec
		wantErr     bool
		errContains string
	}{
		{
			name: "valid spec",
			spec: &Spec{
				ID:          "test",
				Name:        "Test",
				Version:     "1.0.0",
				Description: "Test description",
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			spec: &Spec{
				Name:        "Test",
				Version:     "1.0.0",
				Description: "Test description",
			},
			wantErr:     true,
			errContains: "ID is required",
		},
		{
			name: "missing description",
			spec: &Spec{
				ID:      "test",
				Name:    "Test",
				Version: "1.0.0",
			},
			wantErr:     true,
			errContains: "description is required",
		},
		{
			name: "entity without name",
			spec: &Spec{
				ID:          "test",
				Name:        "Test",
				Version:     "1.0.0",
				Description: "Test",
				Entities: []Entity{
					{Name: "", Description: "No name"},
				},
			},
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name: "entity field without type",
			spec: &Spec{
				ID:          "test",
				Name:        "Test",
				Version:     "1.0.0",
				Description: "Test",
				Entities: []Entity{
					{
						Name: "User",
						Fields: []EntityField{
							{Name: "id", Type: ""},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "type is required",
		},
		{
			name: "API endpoint without path",
			spec: &Spec{
				ID:          "test",
				Name:        "Test",
				Version:     "1.0.0",
				Description: "Test",
				APIEndpoints: []APIEndpoint{
					{Method: "POST", Path: ""},
				},
			},
			wantErr:     true,
			errContains: "path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.Validate(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestManagerLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	_, err := manager.Load("non-existent")
	require.Error(t, err)
}

func TestManagerLoadAllEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	specs, err := manager.LoadAll()
	require.NoError(t, err)
	require.Nil(t, specs)
}

func TestManagerLoadAllNonExistentDir(t *testing.T) {
	manager := NewManager("/non/existent/directory")

	specs, err := manager.LoadAll()
	require.NoError(t, err)
	require.Nil(t, specs)
}

func TestParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test-spec.yaml")

	yamlContent := `
id: test-spec
name: Test Spec
version: 1.0.0
description: A test specification
goals:
  - Goal 1
entities:
  - name: User
    description: A user
    fields:
      - name: id
        type: UUID
        description: ID
`

	err := os.WriteFile(specPath, []byte(yamlContent), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	spec, err := parser.ParseFile(specPath)
	require.NoError(t, err)
	require.Equal(t, "test-spec", spec.ID)
	require.Equal(t, "Test Spec", spec.Name)
	require.Len(t, spec.Entities, 1)
	require.Equal(t, "User", spec.Entities[0].Name)
}
