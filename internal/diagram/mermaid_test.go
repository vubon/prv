package diagram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vubon/prv/internal/models"
)

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cmd/prv", "cmd_prv"},
		{"internal/auth-service", "internal_auth_service"},
		{"pkg.v1/foo", "pkg_v1_foo"},
		{"123pkg", "pkg_123pkg"},
		{"", "root"},
		{"@org/pkg", "_org_pkg"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := SanitizeID(tc.input)
			if got != tc.expected {
				t.Errorf("SanitizeID(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestGraph_ToMermaid_Empty(t *testing.T) {
	g := NewGraph()
	if got := g.ToMermaid(); got != "" {
		t.Errorf("expected empty string for empty graph, got %q", got)
	}

	g.AddNode("cmd/prv", true)
	if got := g.ToMermaid(); got != "" {
		t.Errorf("expected empty string for graph with no edges, got %q", got)
	}
}

func TestGraph_ToMermaid_WithEdges(t *testing.T) {
	g := NewGraph()
	g.AddNode("cmd/prv", true)
	g.AddNode("internal/context", true)
	g.AddNode("internal/models", false)

	g.AddEdge("cmd/prv", "internal/context")
	g.AddEdge("internal/context", "internal/models")

	mermaid := g.ToMermaid()

	if !strings.HasPrefix(mermaid, "graph LR\n") {
		t.Errorf("expected mermaid to start with 'graph LR', got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, `cmd_prv["cmd/prv"]`) {
		t.Errorf("missing cmd_prv node definition in:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, `internal_context["internal/context"]`) {
		t.Errorf("missing internal_context node definition in:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "cmd_prv --> internal_context") {
		t.Errorf("missing cmd_prv --> internal_context edge in:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "internal_context --> internal_models") {
		t.Errorf("missing internal_context --> internal_models edge in:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "classDef touched") {
		t.Errorf("missing touched styling class in:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "class cmd_prv,internal_context touched;") {
		t.Errorf("missing touched class assignment in:\n%s", mermaid)
	}
}

func TestGenerateMermaid_GoProject(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create go.mod
	goModContent := "module example.com/testmod\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// 2. Create packages: cmd/app and internal/service and internal/models
	appDir := filepath.Join(tempDir, "cmd", "app")
	serviceDir := filepath.Join(tempDir, "internal", "service")
	modelsDir := filepath.Join(tempDir, "internal", "models")

	for _, d := range []string{appDir, serviceDir, modelsDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to mkdir %s: %v", d, err)
		}
	}

	appCode := `package main

import (
	"fmt"
	"example.com/testmod/internal/service"
)

func main() {
	fmt.Println(service.Run())
}
`
	if err := os.WriteFile(filepath.Join(appDir, "main.go"), []byte(appCode), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	serviceCode := `package service

import (
	"example.com/testmod/internal/models"
)

func Run() models.Item {
	return models.Item{}
}
`
	if err := os.WriteFile(filepath.Join(serviceDir, "service.go"), []byte(serviceCode), 0644); err != nil {
		t.Fatalf("failed to write service.go: %v", err)
	}

	modelsCode := `package models

type Item struct {
	Name string
}
`
	if err := os.WriteFile(filepath.Join(modelsDir, "item.go"), []byte(modelsCode), 0644); err != nil {
		t.Fatalf("failed to write item.go: %v", err)
	}

	// 3. Test GenerateMermaid
	files := []models.FileChange{
		{
			Path:      "cmd/app/main.go",
			Additions: 5,
			Deletions: 1,
		},
		{
			Path:      "internal/service/service.go",
			Additions: 3,
			Deletions: 0,
		},
	}

	diagram := GenerateMermaid(tempDir, files)
	if diagram == "" {
		t.Fatal("expected non-empty mermaid diagram, got empty")
	}

	if !strings.Contains(diagram, "cmd_app --> internal_service") {
		t.Errorf("expected cmd_app --> internal_service in diagram:\n%s", diagram)
	}
	if !strings.Contains(diagram, "internal_service --> internal_models") {
		t.Errorf("expected internal_service --> internal_models in diagram:\n%s", diagram)
	}
}

func TestGenerateMermaid_DiffHunkFallback(t *testing.T) {
	// Tests parsing when files do not exist on disk (e.g. purely remote PR)
	tempDir := t.TempDir()

	goModContent := "module github.com/vubon/prv\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	diffHunk := `@@ -1,5 +1,8 @@
 package prv
 
+import (
+	"github.com/vubon/prv/internal/git"
+)
`
	files := []models.FileChange{
		{
			Path:     "cmd/prv/main.go",
			DiffHunk: diffHunk,
		},
	}

	diagram := GenerateMermaid(tempDir, files)
	if diagram == "" {
		t.Fatal("expected non-empty diagram from diff hunk fallback")
	}

	if !strings.Contains(diagram, "cmd_prv --> internal_git") {
		t.Errorf("expected cmd_prv --> internal_git in diagram:\n%s", diagram)
	}
}

func TestGenerateMermaid_EmptyFiles(t *testing.T) {
	if got := GenerateMermaid(".", nil); got != "" {
		t.Errorf("expected empty string for nil files, got %q", got)
	}

	files := []models.FileChange{
		{
			Path:       "vendor/bundle.js",
			IsFiltered: true,
		},
	}
	if got := GenerateMermaid(".", files); got != "" {
		t.Errorf("expected empty string for filtered files, got %q", got)
	}
}
