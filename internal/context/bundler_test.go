package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vubon/prv/internal/models"
)

func TestPrepareBundle_InvalidRemotePRReturnsError(t *testing.T) {
	bundler := NewBundler(".")

	// An invalid PR identifier must return an error and not fall back to local branch review
	opts := models.BundleOptions{
		PRIdentifier: "invalid-pr-identifier-that-does-not-exist",
	}

	_, err := bundler.PrepareBundle(opts)
	if err == nil {
		t.Fatal("expected error when fetching invalid remote PR, got nil")
	}
}

func TestPrepareBundle_InvalidPRURLReturnsError(t *testing.T) {
	bundler := NewBundler(".")

	opts := models.BundleOptions{
		PRIdentifier: "https://github.com/invalid-org-12345/non-existent-repo/pull/999999",
	}

	_, err := bundler.PrepareBundle(opts)
	if err == nil {
		t.Fatal("expected error when fetching non-existent PR URL, got nil")
	}
}

func TestBundler_ToMarkdownWithCustomRulesAndSensitiveFiles(t *testing.T) {
	bundler := NewBundler(".")

	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:      "Test PR with Custom Config",
			HeadBranch: "feat/auth",
			BaseBranch: "main",
		},
		SummaryStats: models.SummaryStats{
			ReviewedFiles:  1,
			TotalAdditions: 10,
			TotalDeletions: 2,
		},
		Persona:        "staff-advisory",
		SensitiveFiles: []string{"internal/auth/jwt.go"},
		CustomRules:    []string{"All queries must be parameterized"},
		Files: []models.FileChange{
			{
				Path:        "internal/auth/jwt.go",
				Additions:   10,
				Deletions:   2,
				DiffHunk:    "+func Verify() {}",
				IsSensitive: true,
			},
		},
	}

	md := bundler.ToMarkdown(ctx)

	if !strings.Contains(md, "Security Sensitive Paths Touched") {
		t.Error("expected Sensitive Paths alert in markdown output")
	}
	if !strings.Contains(md, "internal/auth/jwt.go") {
		t.Error("expected sensitive file path in alert")
	}
	if !strings.Contains(md, "Team Engineering Standards & Custom Rules") {
		t.Error("expected Custom Rules section in markdown output")
	}
	if !strings.Contains(md, "All queries must be parameterized") {
		t.Error("expected custom rule text in markdown output")
	}
	if !strings.Contains(md, "🚨 `[SENSITIVE]`") {
		t.Error("expected sensitive badge on file header")
	}
}

func TestBundler_ToMarkdownWithArchitectureDiagram(t *testing.T) {
	bundler := NewBundler(".")
	diag := "graph LR\n    cmd_prv --> internal_context\n"
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:      "Feature PR",
			HeadBranch: "feat/diag",
			BaseBranch: "main",
		},
		ArchitectureDiagram: diag,
		Files: []models.FileChange{
			{
				Path:      "cmd/prv/main.go",
				Additions: 5,
				Deletions: 0,
				DiffHunk:  "+func main() {}",
			},
		},
	}

	md := bundler.ToMarkdown(ctx)
	if !strings.Contains(md, "## 🗺️ Architecture & Component Impact Diagram") {
		t.Error("expected architecture diagram header in bundle markdown")
	}
	if !strings.Contains(md, "```mermaid\n"+diag+"```") {
		t.Errorf("expected mermaid block in bundle markdown, got:\n%s", md)
	}
}

func TestPrepareBundle_StagedNoGitRepo(t *testing.T) {
	tempDir := t.TempDir()
	bundler := NewBundler(tempDir)

	opts := models.BundleOptions{
		Staged:   true,
		RepoPath: tempDir,
	}

	_, err := bundler.PrepareBundle(opts)
	if err == nil {
		t.Fatal("expected error when bundling staged changes in non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' in error message, got: %v", err)
	}
}

func TestPrepareBundle_StagedNoChanges(t *testing.T) {
	tempDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git init: %v", err)
	}

	bundler := NewBundler(tempDir)
	opts := models.BundleOptions{
		Staged:   true,
		RepoPath: tempDir,
	}

	_, err := bundler.PrepareBundle(opts)
	if err == nil {
		t.Fatal("expected error when no staged changes exist, got nil")
	}
	if !strings.Contains(err.Error(), "no staged changes found") {
		t.Errorf("expected 'no staged changes found' in error message, got: %v", err)
	}
}

func TestPrepareBundle_StagedWithChanges(t *testing.T) {
	tempDir := t.TempDir()

	// Init git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
		}
	}

	// Create and stage a file inside pkg/auth/
	authDir := filepath.Join(tempDir, "pkg", "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	tokenFile := filepath.Join(authDir, "token.go")
	content := "package auth\n\nfunc VerifyToken(token string) bool {\n\treturn token != \"\"\n}\n"
	if err := os.WriteFile(tokenFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	addCmd := exec.Command("git", "add", "pkg/auth/token.go")
	addCmd.Dir = tempDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	bundler := NewBundler(tempDir)
	opts := models.BundleOptions{
		Staged:   true,
		RepoPath: tempDir,
	}

	bundleCtx, err := bundler.PrepareBundle(opts)
	if err != nil {
		t.Fatalf("PrepareBundle failed: %v", err)
	}

	if bundleCtx.Metadata.BaseBranch != "index (staged)" {
		t.Errorf("expected base branch 'index (staged)', got '%s'", bundleCtx.Metadata.BaseBranch)
	}
	if bundleCtx.Metadata.Title != "Pre-commit Self-Review (Staged Changes)" {
		t.Errorf("unexpected title: %s", bundleCtx.Metadata.Title)
	}
	if len(bundleCtx.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(bundleCtx.Files))
	}
	if bundleCtx.Files[0].Path != "pkg/auth/token.go" {
		t.Errorf("expected 'pkg/auth/token.go', got '%s'", bundleCtx.Files[0].Path)
	}
	if !bundleCtx.Files[0].IsSensitive {
		t.Errorf("expected pkg/auth/token.go to be marked sensitive by default **/auth/** rule")
	}
	if len(bundleCtx.SensitiveFiles) != 1 {
		t.Errorf("expected 1 sensitive file recorded, got %d", len(bundleCtx.SensitiveFiles))
	}
}
