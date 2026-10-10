package output

import (
	"strings"
	"testing"

	"github.com/vubon/prv/internal/models"
)

func TestFormatWalkthroughMarkdown_Empty(t *testing.T) {
	result := FormatWalkthroughMarkdown(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil slice, got %q", result)
	}

	resultEmpty := FormatWalkthroughMarkdown([]models.FileWalkthrough{})
	if resultEmpty != "" {
		t.Fatalf("expected empty string for empty slice, got %q", resultEmpty)
	}
}

func TestFormatWalkthroughMarkdown_StandardAndEdgeCases(t *testing.T) {
	walkthroughs := []models.FileWalkthrough{
		{
			File:      "internal/git/github.go",
			Additions: 22,
			Deletions: 4,
			Topic:     "Observability & PATH validation",
			Changes: []string{
				"Added `exec.LookPath(\"gh\")` validation to detect missing CLI upfront.",
				"- Captured `*exec.ExitError.Stderr` to surface full GraphQL / auth error messages.",
			},
		},
		{
			File:        "config/secrets.go",
			Additions:   5,
			Deletions:   1,
			IsSensitive: true,
			Topic:       "Credentials safety",
			Changes: []string{
				"Redacted API key in logging output.",
			},
		},
		{
			File:      "README.md",
			Additions: 10,
			Deletions: 0,
			// No Topic and No Changes provided
		},
	}

	output := FormatWalkthroughMarkdown(walkthroughs)

	// Check github.go details
	if !strings.Contains(output, "<details>") || !strings.Contains(output, "</details>") {
		t.Errorf("output must contain <details> tags")
	}
	if !strings.Contains(output, "<summary><b>internal/git/github.go</b> (+22 / -4) — <i>Observability & PATH validation</i></summary>") {
		t.Errorf("summary line for github.go did not match expected structure, got:\n%s", output)
	}
	// Check empty line after summary for GitHub Markdown compliance
	if !strings.Contains(output, "</summary>\n\n- Added `exec.LookPath") {
		t.Errorf("expected empty newline between </summary> and markdown bullet")
	}

	// Check sensitive badge
	if !strings.Contains(output, "🚨 <b>[SENSITIVE]</b>") {
		t.Errorf("sensitive file must render sensitive badge")
	}

	// Check README.md fallback when no topic and no changes
	if !strings.Contains(output, "<summary><b>README.md</b> (+10 / -0)</summary>") {
		t.Errorf("file without topic should not have trailing dashes or empty italics, got:\n%s", output)
	}
	if !strings.Contains(output, "- Updated implementation (`+10` / `-0` lines).") {
		t.Errorf("file without explicit changes should render fallback summary, got:\n%s", output)
	}
}

func TestGenerateWalkthroughFromFiles(t *testing.T) {
	files := []models.FileChange{
		{
			Path:      "internal/output/walkthrough.go",
			Status:    "added",
			Additions: 80,
			Deletions: 0,
			DiffHunk: `@@ -0,0 +1,80 @@
+package output
+
+func FormatWalkthroughMarkdown(walkthroughs []models.FileWalkthrough) string {
+	return ""
+}`,
		},
		{
			Path:      "internal/git/git.go",
			Status:    "modified",
			Additions: 15,
			Deletions: 2,
			DiffHunk: `@@ -45,6 +45,19 @@ func (g *GitRunner) GetStagedDiff(includeLocks bool) ([]models.FileChange, error) {
+	cmd := exec.Command("git", "diff", "--cached")
+	return nil, nil
 }`,
		},
		{
			Path:         "ignored_file.min.js",
			IsFiltered:   true,
			FilterReason: "minified js",
		},
		{
			Path:      "old_service.go",
			Status:    "deleted",
			Deletions: 50,
		},
		{
			Path:    "new_service.go",
			OldPath: "legacy_service.go",
			Status:  "renamed",
		},
	}

	res := GenerateWalkthroughFromFiles(files)

	// Filtered file should be skipped
	if len(res) != 4 {
		t.Fatalf("expected 4 walkthroughs (1 filtered skipped), got %d", len(res))
	}

	// Test added file
	if res[0].File != "internal/output/walkthrough.go" {
		t.Errorf("expected walkthrough.go, got %s", res[0].File)
	}
	if res[0].Topic != "Review report & formatting" {
		t.Errorf("expected 'Review report & formatting' topic, got %s", res[0].Topic)
	}
	foundAddedFunc := false
	for _, c := range res[0].Changes {
		if strings.Contains(c, "FormatWalkthroughMarkdown") {
			foundAddedFunc = true
			break
		}
	}
	if !foundAddedFunc {
		t.Errorf("expected FormatWalkthroughMarkdown in changes for added file, got %v", res[0].Changes)
	}

	// Test modified file symbol detection
	foundHunkFunc := false
	for _, c := range res[1].Changes {
		if strings.Contains(c, "GetStagedDiff") {
			foundHunkFunc = true
			break
		}
	}
	if !foundHunkFunc {
		t.Errorf("expected GetStagedDiff in changes for git.go, got %v", res[1].Changes)
	}

	// Test deleted file
	if !strings.Contains(res[2].Changes[0], "Deleted file") {
		t.Errorf("expected deleted file description, got %v", res[2].Changes)
	}

	// Test renamed file
	if !strings.Contains(res[3].Changes[0], "Renamed from `legacy_service.go`") {
		t.Errorf("expected renamed description, got %v", res[3].Changes)
	}
}

func TestInferFileTopic(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"cmd/prv/main.go", "CLI subcommands & entrypoint"},
		{"internal/diagram/mermaid.go", "Architecture & flow diagrams"},
		{"internal/output/markdown.go", "Review report & formatting"},
		{"internal/git/github.go", "Git & repository operations"},
		{"internal/context/bundler.go", "Bundle aggregation & context engine"},
		{"internal/config/config.go", "Settings & repository configuration"},
		{"internal/matrix/matrix.go", "Engineering evaluation matrix"},
		{"internal/models/review.go", "Domain data structures & schema"},
		{"internal/diagram/mermaid_test.go", "Test coverage & assertions"},
		{"docs/usage.md", "Documentation & guides"},
		{".github/workflows/ci.yml", "Configuration & workflows"},
		{"pkg/auth/token.go", "Package auth"},
		{"root.go", "Core implementation"},
	}

	for _, tt := range tests {
		got := inferFileTopic(tt.path)
		if got != tt.expected {
			t.Errorf("inferFileTopic(%q) = %q, expected %q", tt.path, got, tt.expected)
		}
	}
}
