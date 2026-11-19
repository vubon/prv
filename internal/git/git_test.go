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

func TestParseUnifiedDiff_PreservesSignificantPathCharacters(t *testing.T) {
	rawDiff := `diff --git "a/pkg\\app.go" "b/pkg\\app.go"
--- "a/pkg\\app.go"
+++ "b/pkg\\app.go"
@@ -1 +1 @@
-old
+new
diff --git a/ config.go b/ config.go
--- a/ config.go
+++ b/ config.go
@@ -1 +1 @@
-old
+new
` + "diff --git a/ leading.go b/renamed.go \n" + `similarity index 100%
rename from  leading.go
` + "rename to renamed.go \n"
	files, err := ParseUnifiedDiff(rawDiff, true)
	if err != nil || len(files) != 3 {
		t.Fatalf("expected three changes, got %+v with error %v", files, err)
	}
	if files[0].Path != `pkg\app.go` || files[1].Path != " config.go" {
		t.Errorf("paths lost significant characters: %q, %q", files[0].Path, files[1].Path)
	}
	if files[2].OldPath != " leading.go" || files[2].Path != "renamed.go " {
		t.Errorf("rename paths lost whitespace: %q -> %q", files[2].OldPath, files[2].Path)
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
	mustRunGit(t, runner, "config", "diff.relative", "true")
	subdir := filepath.Join(tempDir, "pkg")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(subdir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if _, err := runner.runCmd("add", "pkg/test.go"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	// Verify staged diff
	staged, err = NewGitRunner(subdir).GetStagedDiff(false)
	if err != nil {
		t.Fatalf("GetStagedDiff failed: %v", err)
	}
	if len(staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(staged))
	}
	if staged[0].Path != "pkg/test.go" {
		t.Errorf("expected path 'pkg/test.go', got %s", staged[0].Path)
	}
	if staged[0].Additions != 3 {
		t.Errorf("expected 3 additions, got %d", staged[0].Additions)
	}
}

func mustRunGit(t *testing.T, runner *GitRunner, args ...string) string {
	t.Helper()
	out, err := runner.runCmd(args...)
	if err != nil {
		t.Fatalf("git command %v failed: %v, out: %s", args, err, out)
	}
	return out
}

func TestGitRunner_GetDiff_CustomBaseBranch(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	if _, err := runner.runCmd("init", "-b", "main"); err != nil {
		// Fallback for older git versions
		mustRunGit(t, runner, "init")
		mustRunGit(t, runner, "checkout", "-b", "main")
	}
	mustRunGit(t, runner, "config", "user.email", "test@example.com")
	mustRunGit(t, runner, "config", "user.name", "Test User")

	// Commit on main
	fileA := filepath.Join(tempDir, "base.txt")
	if err := os.WriteFile(fileA, []byte("initial\n"), 0644); err != nil {
		t.Fatalf("failed to write fileA: %v", err)
	}
	mustRunGit(t, runner, "add", "base.txt")
	mustRunGit(t, runner, "commit", "-m", "init main")

	// Create staging branch
	mustRunGit(t, runner, "checkout", "-b", "staging")

	// Create feature branch from staging
	mustRunGit(t, runner, "checkout", "-b", "feat/payment")

	// Make changes on feat/payment
	mustRunGit(t, runner, "config", "diff.relative", "true")
	subdir := filepath.Join(tempDir, "pkg")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	fileB := filepath.Join(subdir, "payment.go")
	if err := os.WriteFile(fileB, []byte("package payment\n\nfunc Pay() bool { return true }\n"), 0644); err != nil {
		t.Fatalf("failed to write fileB: %v", err)
	}
	mustRunGit(t, runner, "add", "pkg/payment.go")
	mustRunGit(t, runner, "commit", "-m", "feat: add payment")

	// Test GetDiff against staging
	changes, err := NewGitRunner(subdir).GetDiff("staging", "feat/payment", false)
	if err != nil {
		t.Fatalf("GetDiff(staging, feat/payment) failed: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 file change, got %d", len(changes))
	}
	if changes[0].Path != "pkg/payment.go" {
		t.Errorf("expected changed path 'pkg/payment.go', got %s", changes[0].Path)
	}
	if changes[0].Additions != 3 {
		t.Errorf("expected 3 additions, got %d", changes[0].Additions)
	}

	// Test empty baseBranch falls back to default base branch
	changesFallback, err := runner.GetDiff("", "feat/payment", false)
	if err != nil {
		t.Fatalf("GetDiff('', feat/payment) failed: %v", err)
	}
	if len(changesFallback) != 1 {
		t.Fatalf("expected 1 file change with default base fallback, got %d", len(changesFallback))
	}

	// Test error when base branch does not exist
	_, err = runner.GetDiff("non-existent-branch", "feat/payment", false)
	if err == nil {
		t.Error("expected error when base branch does not exist, got nil")
	}
}

func TestGitRunner_GetDefaultBaseBranch(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	if _, err := runner.runCmd("init", "-b", "develop"); err != nil {
		mustRunGit(t, runner, "init")
		mustRunGit(t, runner, "checkout", "-b", "develop")
	}
	mustRunGit(t, runner, "config", "user.email", "test@example.com")
	mustRunGit(t, runner, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("failed to write readme: %v", err)
	}
	mustRunGit(t, runner, "add", "readme.txt")
	mustRunGit(t, runner, "commit", "-m", "init develop")

	detected := runner.GetDefaultBaseBranch()
	if detected != "develop" {
		t.Errorf("expected detected base branch 'develop', got %q", detected)
	}
}

func TestValidateRefName(t *testing.T) {
	validRefs := []string{
		"main",
		"master",
		"develop",
		"staging",
		"feat/payment-gateway",
		"v1.0.0",
		"release-2026.10",
		"user@work/feat",
	}
	for _, ref := range validRefs {
		if err := ValidateRefName(ref); err != nil {
			t.Errorf("expected ref %q to be valid, got error: %v", ref, err)
		}
	}

	invalidRefs := []struct {
		ref  string
		desc string
	}{
		{"", "empty ref"},
		{"-b", "starts with dash"},
		{"--output=/tmp/evil", "starts with double dash"},
		{"feat branch", "contains whitespace"},
		{"feat\tbranch", "contains tab"},
		{"feat\nbranch", "contains newline"},
		{"feat..main", "contains .."},
		{"feat~1", "contains ~"},
		{"feat^2", "contains ^"},
		{"feat:foo", "contains :"},
		{"feat?foo", "contains ?"},
		{"feat*foo", "contains *"},
		{"feat[0]", "contains ["},
		{"feat\\foo", "contains backslash"},
	}
	for _, tc := range invalidRefs {
		if err := ValidateRefName(tc.ref); err == nil {
			t.Errorf("expected ref %q (%s) to be rejected, got nil", tc.ref, tc.desc)
		}
	}
}

func TestGitRunner_GetDiff_RefValidation(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	// Validate that options starting with dash or containing illegal chars are rejected
	if _, err := runner.GetDiff("-b", "main", false); err == nil {
		t.Error("expected error for baseRef starting with '-', got nil")
	}
	if _, err := runner.GetDiff("main", "--output=/tmp/evil", false); err == nil {
		t.Error("expected error for headRef starting with '-', got nil")
	}
	if _, err := runner.GetDiff("feat..staging", "main", false); err == nil {
		t.Error("expected error for baseRef containing '..', got nil")
	}
}

func TestGitRunner_GetWorkingTreeDiff(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	if _, err := runner.runCmd("init", "-b", "main"); err != nil {
		mustRunGit(t, runner, "init")
		mustRunGit(t, runner, "checkout", "-b", "main")
	}
	mustRunGit(t, runner, "config", "user.email", "test@example.com")
	mustRunGit(t, runner, "config", "user.name", "Test User")

	// Case 1: Unborn HEAD (staged file before first commit)
	mustRunGit(t, runner, "config", "diff.relative", "true")
	subdir := filepath.Join(tempDir, "pkg")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	fileA := filepath.Join(subdir, "initial.go")
	if err := os.WriteFile(fileA, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, runner, "add", "pkg/initial.go")

	unbornChanges, err := NewGitRunner(subdir).GetWorkingTreeDiff(false)
	if err != nil {
		t.Fatalf("GetWorkingTreeDiff on unborn HEAD failed: %v", err)
	}
	if len(unbornChanges) != 1 || unbornChanges[0].Path != "pkg/initial.go" {
		t.Fatalf("expected 1 file on unborn HEAD, got: %+v", unbornChanges)
	}

	if got := unbornChanges[0].DiffHunk; got != "@@ -0,0 +1 @@\n+package main\n" {
		t.Errorf("unexpected staged hunk: %q", got)
	}

	// Commit initial.go, then make an unstaged modification
	mustRunGit(t, runner, "commit", "-m", "init")
	if err := os.WriteFile(fileA, []byte("package main\n\nfunc Main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	changes, err := NewGitRunner(subdir).GetWorkingTreeDiff(false)
	if err != nil {
		t.Fatalf("GetWorkingTreeDiff failed: %v", err)
	}
	if len(changes) != 1 || changes[0].Path != "pkg/initial.go" {
		t.Fatalf("expected unstaged modification in initial.go, got: %+v", changes)
	}
}

func TestGitRunner_ReadFileAtRef(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	if _, err := runner.runCmd("init", "-b", "main"); err != nil {
		mustRunGit(t, runner, "init")
		mustRunGit(t, runner, "checkout", "-b", "main")
	}
	mustRunGit(t, runner, "config", "user.email", "test@example.com")
	mustRunGit(t, runner, "config", "user.name", "Test User")

	// Unborn HEAD returns found=false without error
	if _, found, err := runner.ReadFileAtRef("HEAD", ".prv.yaml"); err != nil || found {
		t.Fatalf("expected unborn HEAD to return found=false, nil; got found=%v, err=%v", found, err)
	}

	// Commit .prv.yaml on main
	cfgPath := filepath.Join(tempDir, ".prv.yaml")
	wantContent := "version: \"1\"\nreview:\n  persona: \"staff-advisory\"\n"
	if err := os.WriteFile(cfgPath, []byte(wantContent), 0644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, runner, "add", ".prv.yaml")
	mustRunGit(t, runner, "commit", "-m", "add .prv.yaml")

	// Modify working tree and remove on disk -> HEAD still returns committed bytes
	if err := os.Remove(cfgPath); err != nil {
		t.Fatal(err)
	}
	got, found, err := runner.ReadFileAtRef("HEAD", ".prv.yaml")
	if err != nil || !found {
		t.Fatalf("expected .prv.yaml at HEAD, got found=%v, err=%v", found, err)
	}
	if string(got) != wantContent {
		t.Errorf("ReadFileAtRef content mismatch:\ngot:\n%q\nwant:\n%q", string(got), wantContent)
	}

	// Nonexistent path returns found=false, nil
	if _, found, err := runner.ReadFileAtRef("main", ".prv.yml"); err != nil || found {
		t.Fatalf("expected missing path to return found=false, nil; got found=%v, err=%v", found, err)
	}

	// Invalid ref / path rejected
	if _, _, err := runner.ReadFileAtRef("-bad", ".prv.yaml"); err == nil {
		t.Error("expected error for invalid ref")
	}
	if _, _, err := runner.ReadFileAtRef("main", "../secret"); err == nil {
		t.Error("expected error for invalid path")
	}

	// Symlink resolution test (Finding 8): tracked symlink pointing to another committed file
	targetPath := filepath.Join(tempDir, "shared_rules.yaml")
	targetContent := "version: \"1\"\nreview:\n  persona: \"symlink-resolved\"\n"
	if err := os.WriteFile(targetPath, []byte(targetContent), 0644); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(tempDir, "link_rules.yaml")
	if err := os.Symlink("shared_rules.yaml", symlinkPath); err != nil {
		t.Skip("symlinks not supported on this platform/filesystem")
	}
	mustRunGit(t, runner, "add", "shared_rules.yaml", "link_rules.yaml")
	mustRunGit(t, runner, "commit", "-m", "add symlink rules")

	symBytes, foundSym, errSym := runner.ReadFileAtRef("HEAD", "link_rules.yaml")
	if errSym != nil || !foundSym {
		t.Fatalf("expected symlink link_rules.yaml to resolve at HEAD, got found=%v, err=%v", foundSym, errSym)
	}
	if string(symBytes) != targetContent {
		t.Errorf("symlink content mismatch:\ngot %q\nwant %q", string(symBytes), targetContent)
	}

	// Directory symlink test (Finding 5): intermediate directory is a symlink
	realDir := filepath.Join(tempDir, "real_configs")
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatal(err)
	}
	nestedRule := filepath.Join(realDir, "rules.yaml")
	nestedContent := "version: \"1\"\nreview:\n  persona: \"dir-symlink-resolved\"\n"
	if err := os.WriteFile(nestedRule, []byte(nestedContent), 0644); err != nil {
		t.Fatal(err)
	}
	dirSymlinkPath := filepath.Join(tempDir, "shared_dir")
	if err := os.Symlink("real_configs", dirSymlinkPath); err != nil {
		t.Skip("symlinks not supported")
	}
	mustRunGit(t, runner, "add", "real_configs/rules.yaml", "shared_dir")
	mustRunGit(t, runner, "commit", "-m", "add directory symlink")

	dirSymBytes, resolvedPath, traversedPaths, foundDirSym, errDirSym := runner.ReadFileAtRefWithResolvedPath("HEAD", "shared_dir/rules.yaml")
	if errDirSym != nil || !foundDirSym {
		t.Fatalf("expected shared_dir/rules.yaml to resolve via directory symlink, got found=%v, err=%v", foundDirSym, errDirSym)
	}
	if string(dirSymBytes) != nestedContent {
		t.Errorf("dir symlink content mismatch:\ngot %q\nwant %q", string(dirSymBytes), nestedContent)
	}
	if resolvedPath != "real_configs/rules.yaml" {
		t.Errorf("expected resolvedPath 'real_configs/rules.yaml', got %q", resolvedPath)
	}
	if len(traversedPaths) == 0 {
		t.Errorf("expected traversedPaths to be populated, got empty")
	}
}

func TestGitRunner_ResolveBaseRevision(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	if _, err := runner.runCmd("init", "-b", "main"); err != nil {
		mustRunGit(t, runner, "init")
		mustRunGit(t, runner, "checkout", "-b", "main")
	}
	mustRunGit(t, runner, "config", "user.email", "test@example.com")
	mustRunGit(t, runner, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, runner, "add", "file.txt")
	mustRunGit(t, runner, "commit", "-m", "init")

	refName, sha, err := runner.ResolveBaseRevision("main")
	if err != nil {
		t.Fatalf("ResolveBaseRevision('main') failed: %v", err)
	}
	if refName != "main" {
		t.Errorf("expected refName 'main', got %q", refName)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char commit SHA, got %q", sha)
	}

	// HEAD resolution
	headRef, headSHA, err := runner.ResolveBaseRevision("HEAD")
	if err != nil || headRef != "HEAD" || headSHA != sha {
		t.Errorf("expected HEAD to resolve to commit %q, got ref=%q, sha=%q, err=%v", sha, headRef, headSHA, err)
	}

	// Nonexistent branch
	if _, _, err := runner.ResolveBaseRevision("non-existent"); err == nil {
		t.Error("expected error resolving non-existent base branch")
	}

	// Exact commit SHA (Finding 6): should resolve directly without checking origin/<sha>
	directRef, directSHA, err := runner.ResolveBaseRevision(sha)
	if err != nil || directRef != sha || directSHA != sha {
		t.Errorf("expected commit SHA %q to resolve directly, got ref=%q, sha=%q, err=%v", sha, directRef, directSHA, err)
	}
}

func TestParseUnifiedDiff_RenameHeader(t *testing.T) {
	rawDiff := `diff --git a/.prv.yaml b/.prv.yaml.bak
similarity index 100%
rename from .prv.yaml
rename to .prv.yaml.bak
`
	files, err := ParseUnifiedDiff(rawDiff, false)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Status != "renamed" {
		t.Errorf("expected status 'renamed', got %q", f.Status)
	}
	if f.OldPath != ".prv.yaml" {
		t.Errorf("expected OldPath '.prv.yaml', got %q", f.OldPath)
	}
	if f.Path != ".prv.yaml.bak" {
		t.Errorf("expected Path '.prv.yaml.bak', got %q", f.Path)
	}
}

func TestParseRemoteHostAndSlug(t *testing.T) {
	tests := []struct {
		url       string
		wantHost  string
		wantOwner string
		wantRepo  string
	}{
		{"git@github.com:org/repo.git", "github.com", "org", "repo"},
		{"https://github.com/org/repo.git", "github.com", "org", "repo"},
		{"https://gitlab.example.com/team/project.git", "gitlab.example.com", "team", "project"},
		{"git@gitlab.internal.corp:devs/engine.git", "gitlab.internal.corp", "devs", "engine"},
	}

	for _, tt := range tests {
		host, owner, repo, err := ParseRemoteHostAndSlug(tt.url)
		if err != nil {
			t.Errorf("ParseRemoteHostAndSlug(%q) unexpected error: %v", tt.url, err)
			continue
		}
		if host != tt.wantHost || owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("ParseRemoteHostAndSlug(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.url, host, owner, repo, tt.wantHost, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestGitRunner_ResolveBaseRevision_HexCommitID_DoesNotCheckOriginBranch(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	if _, err := runner.runCmd("init", "-b", "main"); err != nil {
		mustRunGit(t, runner, "init")
		mustRunGit(t, runner, "checkout", "-b", "main")
	}
	mustRunGit(t, runner, "config", "user.email", "test@example.com")
	mustRunGit(t, runner, "config", "user.name", "Test User")

	dummyFile := filepath.Join(tempDir, "file.txt")
	_ = os.WriteFile(dummyFile, []byte("content"), 0644)
	mustRunGit(t, runner, "add", "file.txt")
	mustRunGit(t, runner, "commit", "-m", "init")

	// Create a branch named origin/<dummy-40-hex-sha>
	dummySHA := "1234567890abcdef1234567890abcdef12345678"
	mustRunGit(t, runner, "branch", "origin/"+dummySHA)

	// Resolving dummySHA (which is NOT a real commit object) must NOT resolve to origin/<dummySHA> (Finding 3)
	_, _, err := runner.ResolveBaseRevision(dummySHA)
	if err == nil {
		t.Fatalf("expected error resolving non-existent commit SHA %s, but got nil (resolved to branch)", dummySHA)
	}

	// resolveCommitRef must also return "" for non-existent commit SHA
	ref := runner.resolveCommitRef(dummySHA)
	if ref != "" {
		t.Fatalf("expected resolveCommitRef to return empty for non-existent commit SHA, got %q", ref)
	}
}

func TestGitRunner_CommittedSymlink_FromSubdirectory_ResolvesAgainstRepoRoot(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	if _, err := runner.runCmd("init", "-b", "main"); err != nil {
		mustRunGit(t, runner, "init")
		mustRunGit(t, runner, "checkout", "-b", "main")
	}
	mustRunGit(t, runner, "config", "user.email", "test@example.com")
	mustRunGit(t, runner, "config", "user.name", "Test User")

	servicesDir := filepath.Join(tempDir, "services")
	sharedDir := filepath.Join(tempDir, "shared")
	if err := os.MkdirAll(servicesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}

	rulesContent := "version: \"1\"\nreview:\n  persona: \"sub-service-rules\"\n"
	if err := os.WriteFile(filepath.Join(sharedDir, "rules.yaml"), []byte(rulesContent), 0644); err != nil {
		t.Fatal(err)
	}

	// services/.prv.yml points to ../shared/rules.yaml
	symlinkPath := filepath.Join(servicesDir, ".prv.yml")
	if err := os.Symlink("../shared/rules.yaml", symlinkPath); err != nil {
		t.Skip("symlinks not supported")
	}

	mustRunGit(t, runner, "add", "shared/rules.yaml", "services/.prv.yml")
	mustRunGit(t, runner, "commit", "-m", "add services symlink")

	// Run from services subdirectory (Finding 4)
	subRunner := NewGitRunner(servicesDir)
	bytes, resolvedPath, traversedPaths, found, err := subRunner.ReadFileAtRefWithResolvedPath("HEAD", ".prv.yml")
	if err != nil || !found {
		t.Fatalf("expected .prv.yml from services subdir to resolve, got found=%v, err=%v", found, err)
	}
	if string(bytes) != rulesContent {
		t.Errorf("content mismatch: got %q, want %q", string(bytes), rulesContent)
	}
	if resolvedPath != "shared/rules.yaml" {
		t.Errorf("expected resolvedPath 'shared/rules.yaml', got %q", resolvedPath)
	}
	foundInitial := false
	foundTarget := false
	for _, p := range traversedPaths {
		if p == "services/.prv.yml" {
			foundInitial = true
		}
		if p == "shared/rules.yaml" {
			foundTarget = true
		}
	}
	if !foundInitial || !foundTarget {
		t.Errorf("expected traversedPaths to contain services/.prv.yml and shared/rules.yaml, got %v", traversedPaths)
	}
}

func TestGitRunner_CommittedSymlink_AbsoluteTarget_RejectedFromSubdirectory(t *testing.T) {
	tempDir := t.TempDir()
	runner := NewGitRunner(tempDir)

	if _, err := runner.runCmd("init", "-b", "main"); err != nil {
		mustRunGit(t, runner, "init")
		mustRunGit(t, runner, "checkout", "-b", "main")
	}
	mustRunGit(t, runner, "config", "user.email", "test@example.com")
	mustRunGit(t, runner, "config", "user.name", "Test User")

	servicesDir := filepath.Join(tempDir, "services")
	if err := os.MkdirAll(servicesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Tracked file inside services/rules.yaml
	if err := os.WriteFile(filepath.Join(servicesDir, "rules.yaml"), []byte("version: \"1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Absolute symlink services/.prv.yml -> /rules.yaml
	symlinkPath := filepath.Join(servicesDir, ".prv.yml")
	if err := os.Symlink("/rules.yaml", symlinkPath); err != nil {
		t.Skip("symlinks not supported")
	}

	mustRunGit(t, runner, "add", "services/rules.yaml", "services/.prv.yml")
	mustRunGit(t, runner, "commit", "-m", "add absolute symlink")

	subRunner := NewGitRunner(servicesDir)
	_, _, _, _, err := subRunner.ReadFileAtRefWithResolvedPath("HEAD", ".prv.yml")
	if err == nil {
		t.Fatalf("expected error rejecting absolute symlink target /rules.yaml from subdirectory, got nil")
	}
}
