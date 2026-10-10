package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseUnifiedDiff(t *testing.T) {
	rawDiff := `diff --git a/internal/auth/token.go b/internal/auth/token.go
index 1234567..89abcdef 100644
--- a/internal/auth/token.go
+++ b/internal/auth/token.go
@@ -10,6 +10,8 @@ func ValidateToken(token string) bool {
+	if token == "" {
+		return false
+	}
 	return true
 }
diff --git a/go.sum b/go.sum
index 1111111..2222222 100644
--- a/go.sum
+++ b/go.sum
@@ -1,2 +1,3 @@
+golang.org/x/crypto v0.20.0 h1:...
`

	files, err := ParseUnifiedDiff(rawDiff, false)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// First file: token.go
	if files[0].Path != "internal/auth/token.go" {
		t.Errorf("expected path 'internal/auth/token.go', got %s", files[0].Path)
	}
	if files[0].IsFiltered {
		t.Errorf("token.go should not be filtered")
	}
	if files[0].Additions != 3 {
		t.Errorf("expected 3 additions in token.go, got %d", files[0].Additions)
	}

	// Second file: go.sum (should be filtered)
	if files[1].Path != "go.sum" {
		t.Errorf("expected path 'go.sum', got %s", files[1].Path)
	}
	if !files[1].IsFiltered {
		t.Errorf("go.sum should be filtered")
	}
}

func TestParseUnifiedDiff_PathWithSpaces(t *testing.T) {
	rawDiff := `diff --git a/docs/getting started.md b/docs/getting started.md
index 1234567..89abcdef 100644
--- a/docs/getting started.md
+++ b/docs/getting started.md
@@ -1,3 +1,4 @@
+# Overview
`

	files, err := ParseUnifiedDiff(rawDiff, false)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "docs/getting started.md" {
		t.Errorf("expected 'docs/getting started.md', got %q", files[0].Path)
	}
}

func TestParseUnifiedDiff_LongLine(t *testing.T) {
	// Generate a line exceeding 64KB (e.g. 70KB)
	longLine := make([]byte, 70*1024)
	for i := range longLine {
		longLine[i] = 'A'
	}

	rawDiff := "diff --git a/min.js b/min.js\n" +
		"--- a/min.js\n" +
		"+++ b/min.js\n" +
		"@@ -1,1 +1,1 @@\n" +
		"+" + string(longLine) + "\n"

	files, err := ParseUnifiedDiff(rawDiff, true)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed on line > 64KB: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file parsed, got %d", len(files))
	}
}

func TestParseUnifiedDiff_PlusPlusAndMinusMinusLines(t *testing.T) {
	rawDiff := `diff --git a/query.sql b/query.sql
--- a/query.sql
+++ b/query.sql
@@ -1,3 +1,3 @@
--- old sql comment
+++counter
+real add
`

	files, err := ParseUnifiedDiff(rawDiff, false)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Additions != 2 {
		t.Errorf("expected 2 additions, got %d", files[0].Additions)
	}
	if files[0].Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", files[0].Deletions)
	}
}

func TestShouldFilterFileWithExtra(t *testing.T) {
	extraPatterns := []string{
		"*.tmp",
		"generated/**",
		"custom.lock",
		"**/*_mock.go",
		"**/mocks/**",
	}

	// Should match default patterns
	if filtered, _ := ShouldFilterFileWithExtra("go.sum", extraPatterns); !filtered {
		t.Error("expected go.sum to be filtered by default patterns")
	}

	// Should match custom extra patterns
	if filtered, _ := ShouldFilterFileWithExtra("cache.tmp", extraPatterns); !filtered {
		t.Error("expected cache.tmp to be filtered by extra patterns")
	}
	if filtered, _ := ShouldFilterFileWithExtra("generated/code.go", extraPatterns); !filtered {
		t.Error("expected generated/code.go to be filtered by extra patterns")
	}
	if filtered, _ := ShouldFilterFileWithExtra("custom.lock", extraPatterns); !filtered {
		t.Error("expected custom.lock to be filtered by extra patterns")
	}

	// Nested double-star globs
	if filtered, _ := ShouldFilterFileWithExtra("internal/service/user_mock.go", extraPatterns); !filtered {
		t.Error("expected internal/service/user_mock.go to be filtered by **/*_mock.go")
	}
	if filtered, _ := ShouldFilterFileWithExtra("pkg/a/b/mocks/client.go", extraPatterns); !filtered {
		t.Error("expected pkg/a/b/mocks/client.go to be filtered by **/mocks/**")
	}

	// Normal file should not be filtered
	if filtered, _ := ShouldFilterFileWithExtra("internal/service/user.go", extraPatterns); filtered {
		t.Error("expected user.go not to be filtered")
	}
}

func TestGitRunner_GetStagedDiff(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	// Initially not a git repo
	if runner.IsGitRepo() {
		t.Error("expected tempDir not to be git repo initially")
	}

	// Initialize git repo
	if _, err := runner.runCmd("init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if _, err := runner.runCmd("config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}
	if _, err := runner.runCmd("config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	// Before staging any file, staged diff should be empty
	staged, err := runner.GetStagedDiff(false)
	if err != nil {
		t.Fatalf("GetStagedDiff failed: %v", err)
	}
	if len(staged) != 0 {
		t.Errorf("expected 0 staged files initially, got %d", len(staged))
	}

	// Create and stage a test file
	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if _, err := runner.runCmd("add", "test.go"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	// Verify staged diff
	staged, err = runner.GetStagedDiff(false)
	if err != nil {
		t.Fatalf("GetStagedDiff failed: %v", err)
	}
	if len(staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(staged))
	}
	if staged[0].Path != "test.go" {
		t.Errorf("expected path 'test.go', got %s", staged[0].Path)
	}
	if staged[0].Additions != 3 {
		t.Errorf("expected 3 additions, got %d", staged[0].Additions)
	}
}
