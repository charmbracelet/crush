package spec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyzeBasicFieldChanges(t *testing.T) {
	analyzer := NewChangeAnalyzer("specs/auth-system.yaml", "1.2.0")

	oldSpec := &Spec{
		ID:          "auth-system",
		Name:        "Auth System",
		Version:     "1.1.0",
		Description: "Old description",
		Goals:       []string{"Goal 1"},
	}

	newSpec := &Spec{
		ID:          "auth-system",
		Name:        "Auth System",
		Version:     "1.2.0",
		Description: "New description",
		Goals:       []string{"Goal 1", "Goal 2"},
	}

	changes := analyzer.Analyze(oldSpec, newSpec)

	// Should detect description and goals changes
	require.NotEmpty(t, changes)

	// Find description change
	var descChange *SpecChange
	for i, c := range changes {
		if c.Path == "description" {
			descChange = &changes[i]
			break
		}
	}
	require.NotNil(t, descChange)
	require.Equal(t, "Old description", descChange.OldValue)
	require.Equal(t, "New description", descChange.NewValue)
	require.Equal(t, ImpactLow, descChange.ImpactLevel)
}

func TestAnalyzeEntityChanges(t *testing.T) {
	analyzer := NewChangeAnalyzer("specs/auth-system.yaml", "1.2.0")

	oldSpec := &Spec{
		ID:          "auth-system",
		Name:        "Auth System",
		Version:     "1.1.0",
		Description: "Auth system",
		Entities: []Entity{
			{
				Name:        "User",
				Description: "A user",
				Fields: []EntityField{
					{Name: "id", Type: "UUID", Description: "ID"},
					{Name: "email", Type: "string", Description: "Email"},
				},
			},
		},
	}

	t.Run("new entity added", func(t *testing.T) {
		newSpec := &Spec{
			ID:          "auth-system",
			Name:        "Auth System",
			Version:     "1.2.0",
			Description: "Auth system",
			Entities: []Entity{
				{Name: "User", Description: "A user", Fields: []EntityField{}},
				{Name: "Session", Description: "A session", Fields: []EntityField{}},
			},
		}

		changes := analyzer.Analyze(oldSpec, newSpec)

		var entityChange *SpecChange
		for i, c := range changes {
			if c.Path == "entities.Session" {
				entityChange = &changes[i]
				break
			}
		}
		require.NotNil(t, entityChange)
		require.Nil(t, entityChange.OldValue)
		require.NotNil(t, entityChange.NewValue)
	})

	t.Run("entity removed", func(t *testing.T) {
		newSpec := &Spec{
			ID:          "auth-system",
			Name:        "Auth System",
			Version:     "1.2.0",
			Description: "Auth system",
			Entities:    []Entity{},
		}

		changes := analyzer.Analyze(oldSpec, newSpec)

		var entityChange *SpecChange
		for i, c := range changes {
			if c.Path == "entities.User" {
				entityChange = &changes[i]
				break
			}
		}
		require.NotNil(t, entityChange)
		require.Nil(t, entityChange.NewValue)
		require.Equal(t, ImpactBreaking, entityChange.ImpactLevel)
	})

	t.Run("field type changed", func(t *testing.T) {
		newSpec := &Spec{
			ID:          "auth-system",
			Name:        "Auth System",
			Version:     "1.2.0",
			Description: "Auth system",
			Entities: []Entity{
				{
					Name:        "User",
					Description: "A user",
					Fields: []EntityField{
						{Name: "id", Type: "UUID", Description: "ID"},
						{Name: "email", Type: "int", Description: "Email"}, // Changed type
					},
				},
			},
		}

		changes := analyzer.Analyze(oldSpec, newSpec)

		var typeChange *SpecChange
		for i, c := range changes {
			if c.Path == "entities.User.fields.email.type" {
				typeChange = &changes[i]
				break
			}
		}
		require.NotNil(t, typeChange)
		require.Equal(t, "string", typeChange.OldValue)
		require.Equal(t, "int", typeChange.NewValue)
		require.Equal(t, ImpactHigh, typeChange.ImpactLevel)
	})
}

func TestAnalyzeAPIEndpointChanges(t *testing.T) {
	analyzer := NewChangeAnalyzer("specs/auth-system.yaml", "1.2.0")

	oldSpec := &Spec{
		ID:          "auth-system",
		Name:        "Auth System",
		Version:     "1.1.0",
		Description: "Auth system",
		APIEndpoints: []APIEndpoint{
			{
				Path:        "/auth/login",
				Method:      "POST",
				Description: "Login",
				Request:     map[string]string{"email": "string", "password": "string"},
			},
		},
	}

	t.Run("new endpoint added", func(t *testing.T) {
		newSpec := &Spec{
			ID:          "auth-system",
			Name:        "Auth System",
			Version:     "1.2.0",
			Description: "Auth system",
			APIEndpoints: []APIEndpoint{
				{Path: "/auth/login", Method: "POST", Description: "Login"},
				{Path: "/auth/register", Method: "POST", Description: "Register"},
			},
		}

		changes := analyzer.Analyze(oldSpec, newSpec)

		var endpointChange *SpecChange
		for i, c := range changes {
			if c.Path == "api_endpoints.POST./auth/register" {
				endpointChange = &changes[i]
				break
			}
		}
		require.NotNil(t, endpointChange)
		require.Equal(t, ImpactMedium, endpointChange.ImpactLevel)
	})

	t.Run("endpoint removed", func(t *testing.T) {
		newSpec := &Spec{
			ID:           "auth-system",
			Name:         "Auth System",
			Version:      "1.2.0",
			Description:  "Auth system",
			APIEndpoints: []APIEndpoint{},
		}

		changes := analyzer.Analyze(oldSpec, newSpec)

		var endpointChange *SpecChange
		for i, c := range changes {
			if c.Path == "api_endpoints.POST./auth/login" {
				endpointChange = &changes[i]
				break
			}
		}
		require.NotNil(t, endpointChange)
		require.Equal(t, ImpactBreaking, endpointChange.ImpactLevel)
	})
}

func TestAnalyzeRequirementChanges(t *testing.T) {
	analyzer := NewChangeAnalyzer("specs/auth-system.yaml", "1.2.0")

	oldSpec := &Spec{
		ID:          "auth-system",
		Name:        "Auth System",
		Version:     "1.1.0",
		Description: "Auth system",
		Requirements: Requirements{
			Functional: []FunctionalRequirement{
				{
					ID:                 "FR-001",
					Description:        "User can register",
					Priority:           PriorityP0,
					AcceptanceCriteria: []string{"Email validation", "Password strength"},
				},
			},
		},
	}

	t.Run("acceptance criteria changed", func(t *testing.T) {
		newSpec := &Spec{
			ID:          "auth-system",
			Name:        "Auth System",
			Version:     "1.2.0",
			Description: "Auth system",
			Requirements: Requirements{
				Functional: []FunctionalRequirement{
					{
						ID:                 "FR-001",
						Description:        "User can register",
						Priority:           PriorityP0,
						AcceptanceCriteria: []string{"Email validation", "Password strength", "New criteria"},
					},
				},
			},
		}

		changes := analyzer.Analyze(oldSpec, newSpec)

		var criteriaChange *SpecChange
		for i, c := range changes {
			if c.Path == "requirements.functional.FR-001.acceptance_criteria" {
				criteriaChange = &changes[i]
				break
			}
		}
		require.NotNil(t, criteriaChange)
		require.Len(t, criteriaChange.Affected, 1)
		require.Equal(t, "task", criteriaChange.Affected[0].Type)
		require.Equal(t, "validate", criteriaChange.Affected[0].Action)
	})
}

func TestDetermineImpactLevel(t *testing.T) {
	analyzer := NewChangeAnalyzer("", "")

	tests := []struct {
		path     string
		oldValue interface{}
		newValue interface{}
		expected ImpactLevel
	}{
		{"description", "old", "new", ImpactLow},
		{"goals", []string{}, []string{"new"}, ImpactLow},
		{"constraints.technical", "old", "new", ImpactHigh},
		{"entities.User.fields.id.type", "UUID", "string", ImpactHigh},
		{"entities.User.fields.newField", nil, "new", ImpactMedium},
		{"api_endpoints.POST./users", nil, "new", ImpactMedium},
		{"entities.User", "old", nil, ImpactBreaking},
		{"api_endpoints.GET./users", "old", nil, ImpactBreaking},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			change := SpecChange{
				Path:     tt.path,
				OldValue: tt.oldValue,
				NewValue: tt.newValue,
			}
			result := analyzer.determineImpactLevel(change)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDiff(t *testing.T) {
	analyzer := NewChangeAnalyzer("specs/auth-system.yaml", "1.2.0")

	changes := []SpecChange{
		{
			Path:        "description",
			OldValue:    "Old description",
			NewValue:    "New description",
			ImpactLevel: ImpactLow,
			Affected: []AffectedItem{
				{Type: "blueprint", Path: "README.md", Action: "regenerate"},
			},
		},
	}

	diff := analyzer.Diff(changes)
	require.Contains(t, diff, "description")
	require.Contains(t, diff, "Impact: low")
	require.Contains(t, diff, "Old description")
	require.Contains(t, diff, "New description")
	require.Contains(t, diff, "blueprint")
}
