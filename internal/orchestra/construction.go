package orchestra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/spec"
)

// ConstructionGenerator generates construction documents from specs.
type ConstructionGenerator struct {
	outputDir string
}

// NewConstructionGenerator creates a new construction generator.
func NewConstructionGenerator(outputDir string) *ConstructionGenerator {
	return &ConstructionGenerator{
		outputDir: outputDir,
	}
}

// Generate generates all construction documents for a spec.
func (g *ConstructionGenerator) Generate(s spec.Spec) error {
	projectDir := filepath.Join(g.outputDir, s.ID)

	// Create directories
	dirs := []string{
		filepath.Join(projectDir, "prisma"),
		filepath.Join(projectDir, "api"),
		filepath.Join(projectDir, "contracts"),
		filepath.Join(projectDir, "infrastructure"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Generate Prisma schema
	if err := g.generatePrismaSchema(projectDir, s); err != nil {
		return err
	}

	// Generate OpenAPI spec
	if err := g.generateOpenAPI(projectDir, s); err != nil {
		return err
	}

	// Generate TypeScript contracts
	if err := g.generateTypeScriptContracts(projectDir, s); err != nil {
		return err
	}

	return nil
}

func (g *ConstructionGenerator) generatePrismaSchema(dir string, s spec.Spec) error {
	var content strings.Builder

	content.WriteString(fmt.Sprintf(`// construction/%s/prisma/schema.prisma
// Generated from: specs/%s.yaml v%s
// DO NOT EDIT DIRECTLY - Update spec and regenerate

generator client {
  provider = "go"
  output   = "./db"
}

datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

`, s.ID, s.ID, s.Version))

	// Generate enums first
	enums := make(map[string][]string)
	for _, entity := range s.Entities {
		for _, field := range entity.Fields {
			if field.Type == "enum" && len(field.Values) > 0 {
				enums[field.Name] = field.Values
			}
		}
	}

	for enumName, values := range enums {
		content.WriteString(fmt.Sprintf("enum %s {\n", pascalCase(enumName)))
		for _, v := range values {
			content.WriteString(fmt.Sprintf("  %s\n", strings.ToUpper(v)))
		}
		content.WriteString("}\n\n")
	}

	// Generate models
	for _, entity := range s.Entities {
		content.WriteString(fmt.Sprintf("// Entity: %s\n", entity.Name))
		content.WriteString(fmt.Sprintf("// Source: specs/%s.yaml#entities\n", s.ID))
		content.WriteString(fmt.Sprintf("model %s {\n", entity.Name))

		for _, field := range entity.Fields {
			prismaType := g.toPrismaType(field.Type, field.Name)
			fieldName := g.toPrismaFieldName(field.Name)

			content.WriteString(fmt.Sprintf("  %s %s", fieldName, prismaType))

			// Add modifiers
			if field.Name == "id" {
				content.WriteString(" @id @default(uuid())")
			}
			if strings.Contains(strings.ToLower(field.Name), "email") {
				content.WriteString(" @unique")
			}
			if field.References != "" {
				// Extract referenced entity
				parts := strings.Split(field.References, ".")
				if len(parts) >= 2 {
					refEntity := parts[0]
					content.WriteString(fmt.Sprintf(" @relation(fields: [%s], references: [id])", fieldName))
					// Add relation field
					defer func() {
						content.WriteString(fmt.Sprintf("  %s %s @relation(fields: [%s], references: [id], onDelete: Cascade)\n",
							refEntity, refEntity+"?", fieldName))
					}()
				}
			}

			// Map snake_case to database
			dbName := g.toDBName(field.Name)
			if dbName != field.Name {
				content.WriteString(fmt.Sprintf(" @map(\"%s\")", dbName))
			}

			content.WriteString("\n")
		}

		// Add map to table name
		tableName := g.toTableName(entity.Name)
		content.WriteString(fmt.Sprintf("\n  @@map(\"%s\")\n", tableName))
		content.WriteString("}\n\n")
	}

	return os.WriteFile(filepath.Join(dir, "prisma", "schema.prisma"), []byte(content.String()), 0o644)
}

func (g *ConstructionGenerator) toPrismaType(typ, name string) string {
	switch strings.ToLower(typ) {
	case "uuid":
		return "String"
	case "string":
		if strings.Contains(strings.ToLower(name), "email") {
			return "String"
		}
		return "String"
	case "boolean":
		return "Boolean"
	case "timestamp", "datetime":
		return "DateTime"
	case "integer", "int":
		return "Int"
	case "float", "double":
		return "Float"
	case "enum":
		return pascalCase(name)
	default:
		return "String"
	}
}

func (g *ConstructionGenerator) toPrismaFieldName(name string) string {
	return pascalCase(name)
}

func (g *ConstructionGenerator) toDBName(name string) string {
	// Convert camelCase to snake_case
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func (g *ConstructionGenerator) toTableName(name string) string {
	// Convert PascalCase to snake_case and pluralize
	singular := g.toDBName(name)
	return singular + "s"
}

func (g *ConstructionGenerator) generateOpenAPI(dir string, s spec.Spec) error {
	var content strings.Builder

	content.WriteString(fmt.Sprintf(`# construction/%s/api/openapi.yaml
# Generated from: specs/%s.yaml v%s
# DO NOT EDIT DIRECTLY - Update spec and regenerate

openapi: 3.1.0
info:
  title: %s API
  version: %s
  description: |
    %s

servers:
  - url: /api/v1

paths:
`, s.ID, s.ID, s.Version, s.Name, s.Version, indent(s.Description, 4)))

	// Generate paths
	for _, endpoint := range s.APIEndpoints {
		content.WriteString(fmt.Sprintf("  %s:\n", endpoint.Path))
		content.WriteString(fmt.Sprintf("    %s:\n", strings.ToLower(endpoint.Method)))
		content.WriteString(fmt.Sprintf("      summary: %s\n", endpoint.Description))
		content.WriteString(fmt.Sprintf("      operationId: %s\n", g.toOperationID(endpoint)))
		content.WriteString(fmt.Sprintf("      tags:\n        - %s\n", g.getTag(endpoint.Path)))
		content.WriteString("      responses:\n")
		content.WriteString("        '200':\n")
		content.WriteString("          description: Success\n")
		content.WriteString("          content:\n")
		content.WriteString("            application/json:\n")
		content.WriteString("              schema:\n")
		content.WriteString(fmt.Sprintf("                $ref: '#/components/schemas/%sResponse'\n",
			g.toOperationID(endpoint)))
		content.WriteString("\n")
	}

	// Generate components
	content.WriteString("components:\n  schemas:\n")

	// Generate entity schemas
	for _, entity := range s.Entities {
		content.WriteString(fmt.Sprintf("    %s:\n", entity.Name))
		content.WriteString("      type: object\n")
		content.WriteString("      properties:\n")
		for _, field := range entity.Fields {
			content.WriteString(fmt.Sprintf("        %s:\n", field.Name))
			content.WriteString(fmt.Sprintf("          type: %s\n", g.toOpenAPIType(field.Type)))
			if field.Description != "" {
				content.WriteString(fmt.Sprintf("          description: %s\n", field.Description))
			}
		}
		content.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(dir, "api", "openapi.yaml"), []byte(content.String()), 0o644)
}

func (g *ConstructionGenerator) toOpenAPIType(typ string) string {
	switch strings.ToLower(typ) {
	case "uuid":
		return "string"
	case "string":
		return "string"
	case "boolean":
		return "boolean"
	case "timestamp", "datetime":
		return "string"
	case "integer", "int":
		return "integer"
	case "float", "double":
		return "number"
	case "enum":
		return "string"
	default:
		return "string"
	}
}

func (g *ConstructionGenerator) toOperationID(endpoint spec.APIEndpoint) string {
	path := strings.ReplaceAll(endpoint.Path, "/", "_")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.Trim(path, "_")
	return strings.ToLower(endpoint.Method) + pascalCase(path)
}

func (g *ConstructionGenerator) getTag(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 {
		return pascalCase(parts[0])
	}
	return "Default"
}

func (g *ConstructionGenerator) generateTypeScriptContracts(dir string, s spec.Spec) error {
	var content strings.Builder

	content.WriteString(fmt.Sprintf(`// construction/%s/contracts/types.ts
// Generated from: specs/%s.yaml v%s
// DO NOT EDIT DIRECTLY - Update spec and regenerate

`, s.ID, s.ID, s.Version))

	// Generate enums
	for _, entity := range s.Entities {
		for _, field := range entity.Fields {
			if field.Type == "enum" && len(field.Values) > 0 {
				content.WriteString(fmt.Sprintf("export type %s = \n", pascalCase(field.Name)))
				for i, v := range field.Values {
					if i < len(field.Values)-1 {
						content.WriteString(fmt.Sprintf("  | '%s'\n", v))
					} else {
						content.WriteString(fmt.Sprintf("  | '%s';\n\n", v))
					}
				}
			}
		}
	}

	// Generate interfaces
	for _, entity := range s.Entities {
		content.WriteString(fmt.Sprintf("export interface %s {\n", entity.Name))
		for _, field := range entity.Fields {
			tsType := g.toTypeScriptType(field.Type)
			optional := ""
			if field.Name != "id" {
				optional = "?"
			}
			content.WriteString(fmt.Sprintf("  %s%s: %s;\n", field.Name, optional, tsType))
		}
		content.WriteString("}\n\n")
	}

	// Generate API types
	for _, endpoint := range s.APIEndpoints {
		opID := g.toOperationID(endpoint)
		content.WriteString(fmt.Sprintf("export interface %sRequest {\n", pascalCase(opID)))
		for name, typ := range endpoint.Request {
			content.WriteString(fmt.Sprintf("  %s: %s;\n", name, g.toTypeScriptType(typ)))
		}
		content.WriteString("}\n\n")

		content.WriteString(fmt.Sprintf("export interface %sResponse {\n", pascalCase(opID)))
		for name, typ := range endpoint.Response {
			content.WriteString(fmt.Sprintf("  %s: %s;\n", name, g.toTypeScriptType(typ)))
		}
		content.WriteString("}\n\n")
	}

	return os.WriteFile(filepath.Join(dir, "contracts", "types.ts"), []byte(content.String()), 0o644)
}

func (g *ConstructionGenerator) toTypeScriptType(typ string) string {
	switch strings.ToLower(typ) {
	case "uuid":
		return "string"
	case "string":
		return "string"
	case "boolean":
		return "boolean"
	case "timestamp", "datetime":
		return "Date"
	case "integer", "int":
		return "number"
	case "float", "double":
		return "number"
	case "User":
		return "User"
	default:
		if strings.Contains(typ, "[]") {
			return g.toTypeScriptType(strings.TrimSuffix(typ, "[]")) + "[]"
		}
		return typ
	}
}

func pascalCase(s string) string {
	if len(s) == 0 {
		return s
	}
	words := strings.Fields(s)
	var result strings.Builder
	for _, word := range words {
		if len(word) > 0 {
			result.WriteString(strings.ToUpper(string(word[0])))
			if len(word) > 1 {
				result.WriteString(word[1:])
			}
		}
	}
	return result.String()
}

func indent(s string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}
