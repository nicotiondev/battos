package contract

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

type openAPIDocument struct {
	OpenAPI    string                    `yaml:"openapi"`
	Paths      map[string]map[string]any `yaml:"paths"`
	Components struct {
		SecuritySchemes map[string]any `yaml:"securitySchemes"`
		Schemas         map[string]any `yaml:"schemas"`
	} `yaml:"components"`
}

func TestOpenAPIContractParsesAndContainsPhase3ABoundaries(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "packages", "openapi", "openapi.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}

	var doc openAPIDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}

	if doc.OpenAPI != "3.1.0" {
		t.Fatalf("unexpected OpenAPI version: %q", doc.OpenAPI)
	}

	requiredPaths := []string{
		"/health",
		"/status",
		"/memory/search",
		"/projects",
		"/tasks",
		"/knowledge/workspaces",
		"/runs",
		"/runs/{id}/approvals",
		"/novacore/chat",
	}
	for _, path := range requiredPaths {
		if _, ok := doc.Paths[path]; !ok {
			t.Errorf("contract missing path %s", path)
		}
	}

	if _, ok := doc.Components.SecuritySchemes["bearerAuth"]; !ok {
		t.Error("contract missing bearerAuth security scheme")
	}
	for _, schema := range []string{"MemoryObservation", "Project", "Task", "Run", "Approval"} {
		if _, ok := doc.Components.Schemas[schema]; !ok {
			t.Errorf("contract missing schema %s", schema)
		}
	}
}
