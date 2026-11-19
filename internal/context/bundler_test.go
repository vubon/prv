package context

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vubon/prv/internal/config"
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
	if len(bundleCtx.FileWalkthroughs) != 1 {
		t.Errorf("expected 1 file walkthrough generated, got %d", len(bundleCtx.FileWalkthroughs))
	}
}

func TestBundler_ToMarkdownWithFileWalkthroughs(t *testing.T) {
	bundler := NewBundler(".")
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:      "Walkthrough Bundle PR",
			HeadBranch: "feat/walkthrough",
			BaseBranch: "main",
		},
		FileWalkthroughs: []models.FileWalkthrough{
			{
				File:      "cmd/prv/main.go",
				Additions: 30,
				Deletions: 2,
				Topic:     "CLI subcommands",
				Changes:   []string{"Added runWalkthrough"},
			},
		},
	}

	md := bundler.ToMarkdown(ctx)
	if !strings.Contains(md, "## 🔍 File-by-File Changes Walkthrough Overview") {
		t.Errorf("expected File-by-File Changes Walkthrough Overview header in bundle markdown")
	}
	if !strings.Contains(md, "<summary><b>cmd/prv/main.go</b> (+30 / -2) — <i>CLI subcommands</i></summary>") {
		t.Errorf("expected details summary block in bundle markdown, got:\n%s", md)
	}
}

func TestBundler_ToJSON(t *testing.T) {
	bundler := NewBundler(".")
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:      "JSON Bundle PR",
			HeadBranch: "feat/json",
			BaseBranch: "main",
		},
		SummaryStats: models.SummaryStats{
			TotalFiles:     1,
			ReviewedFiles:  1,
			TotalAdditions: 10,
			TotalDeletions: 2,
		},
	}

	jsonStr, err := bundler.ToJSON(ctx)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if !strings.Contains(jsonStr, `"title": "JSON Bundle PR"`) {
		t.Errorf("expected title in JSON output, got:\n%s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"total_additions": 10`) {
		t.Errorf("expected total_additions in JSON output, got:\n%s", jsonStr)
	}
}

func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v, output: %s", strings.Join(args, " "), dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestBundler_PrepareBundle_CustomBaseBranch(t *testing.T) {
	tempDir := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		runGitInDir(t, tempDir, "init")
		runGitInDir(t, tempDir, "checkout", "-b", "main")
	}
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")

	// Commit on main
	_ = os.WriteFile(filepath.Join(tempDir, "base.txt"), []byte("base\n"), 0644)
	runGitInDir(t, tempDir, "add", "base.txt")
	runGitInDir(t, tempDir, "commit", "-m", "init main")

	// Branch staging
	runGitInDir(t, tempDir, "checkout", "-b", "staging")

	// Branch feat/custom
	runGitInDir(t, tempDir, "checkout", "-b", "feat/custom")
	_ = os.WriteFile(filepath.Join(tempDir, "feature.go"), []byte("package feature\n"), 0644)
	runGitInDir(t, tempDir, "add", "feature.go")
	runGitInDir(t, tempDir, "commit", "-m", "feat: custom feature")

	bundler := NewBundler(tempDir)

	// Test 1: Explicit BaseBranch in options
	opts := models.BundleOptions{
		RepoPath:   tempDir,
		BaseBranch: "staging",
	}
	ctx, err := bundler.PrepareBundle(opts)
	if err != nil {
		t.Fatalf("PrepareBundle with explicit base branch failed: %v", err)
	}

	if ctx.Metadata.BaseBranch != "staging" {
		t.Errorf("expected BaseBranch 'staging', got %q", ctx.Metadata.BaseBranch)
	}
	if ctx.Metadata.HeadBranch != "feat/custom" {
		t.Errorf("expected HeadBranch 'feat/custom', got %q", ctx.Metadata.HeadBranch)
	}
	if len(ctx.Files) != 1 || ctx.Files[0].Path != "feature.go" {
		t.Errorf("expected 1 file change 'feature.go', got %v", ctx.Files)
	}

	// Test 2: Uncommitted default_base_branch in working tree does NOT redirect implicit base (remains repo default 'main')
	cfgContent := `version: "1"
review:
  default_base_branch: "staging"
`
	cfgPath := filepath.Join(tempDir, ".prv.yaml")
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	optsImplicit := models.BundleOptions{
		RepoPath: tempDir,
	}
	ctxImplicit, err := bundler.PrepareBundle(optsImplicit)
	if err != nil {
		t.Fatalf("PrepareBundle with implicit base failed: %v", err)
	}
	if ctxImplicit.Metadata.BaseBranch != "main" {
		t.Errorf("expected BaseBranch to remain committed repository default 'main', got %q", ctxImplicit.Metadata.BaseBranch)
	}

	// Test 3: Explicit --config honors default_base_branch from specified configuration
	optsExplicitConfig := models.BundleOptions{
		RepoPath:   tempDir,
		ConfigPath: cfgPath,
	}
	ctxExplicitConfig, err := bundler.PrepareBundle(optsExplicitConfig)
	if err != nil {
		t.Fatalf("PrepareBundle with explicit --config failed: %v", err)
	}
	if ctxExplicitConfig.Metadata.BaseBranch != "staging" {
		t.Errorf("expected BaseBranch from explicit config to be 'staging', got %q", ctxExplicitConfig.Metadata.BaseBranch)
	}
}

func TestBundler_ToMarkdown_WithReviewerContext(t *testing.T) {
	bundler := NewBundler(".")
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:       "Feature Auth",
			HeadBranch:  "feat/auth",
			BaseBranch:  "main",
			Description: "Author description of auth changes.",
		},
		UserContext: []models.UserContextDoc{
			{
				SourcePath: "12.md",
				Content:    "### Goal\nImplement JWT authentication.",
			},
		},
		ArchitectureDiagram: "graph LR\n  A --> B",
	}

	md := bundler.ToMarkdown(ctx)

	// Verify Reviewer-Provided Context section is present
	if !strings.Contains(md, "## 📝 Reviewer-Provided Context (Design Intent & Spec)") {
		t.Error("expected Reviewer-Provided Context section header")
	}
	if !strings.Contains(md, "### 📄 Document: `12.md`") {
		t.Error("expected document badge with source path")
	}
	if !strings.Contains(md, "### Goal\nImplement JWT authentication.") {
		t.Error("expected normalized document content")
	}

	// Verify section ordering: PR Description -> Reviewer-Provided Context -> Architecture Diagram
	descIdx := strings.Index(md, "## PR Description / Intent")
	ctxIdx := strings.Index(md, "## 📝 Reviewer-Provided Context (Design Intent & Spec)")
	diagIdx := strings.Index(md, "## 🗺️ Architecture & Component Impact Diagram")

	if descIdx == -1 || ctxIdx == -1 || diagIdx == -1 {
		t.Fatalf("expected all 3 sections in markdown, got descIdx=%d ctxIdx=%d diagIdx=%d", descIdx, ctxIdx, diagIdx)
	}
	if descIdx >= ctxIdx || ctxIdx >= diagIdx {
		t.Errorf("expected ordering: Description (%d) < Context (%d) < Diagram (%d)", descIdx, ctxIdx, diagIdx)
	}
}

func TestBundler_ToMarkdown_MultipleContextFiles(t *testing.T) {
	bundler := NewBundler(".")
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:      "Feature Multi-Context",
			HeadBranch: "feat/multi",
			BaseBranch: "main",
		},
		UserContext: []models.UserContextDoc{
			{
				SourcePath: "specs/canvas.md",
				Content:    "### Reason Canvas",
			},
			{
				SourcePath: "specs/criteria.md",
				Content:    "### Acceptance Criteria",
			},
		},
	}

	md := bundler.ToMarkdown(ctx)

	if !strings.Contains(md, "### 📄 Document: `specs/canvas.md`") {
		t.Error("expected first document badge")
	}
	if !strings.Contains(md, "### 📄 Document: `specs/criteria.md`") {
		t.Error("expected second document badge")
	}
	if !strings.Contains(md, "### Reason Canvas") || !strings.Contains(md, "### Acceptance Criteria") {
		t.Error("expected both document contents in output")
	}
}

func TestBundler_ToMarkdown_WithoutContext_ZeroRegression(t *testing.T) {
	bundler := NewBundler(".")
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:       "Regression Check",
			HeadBranch:  "feat/reg",
			BaseBranch:  "main",
			Description: "Original PR description",
		},
		UserContext: nil,
	}

	md := bundler.ToMarkdown(ctx)

	if strings.Contains(md, "Reviewer-Provided Context") {
		t.Error("did not expect Reviewer-Provided Context section when UserContext is nil")
	}
}

func TestBundler_ToJSON_UserContextSerialization(t *testing.T) {
	bundler := NewBundler(".")
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:      "JSON Test",
			HeadBranch: "feat/json",
			BaseBranch: "main",
		},
		UserContext: []models.UserContextDoc{
			{
				SourcePath: "12.md",
				Content:    "### Test Spec",
			},
		},
	}

	jsonStr, err := bundler.ToJSON(ctx)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var parsed models.PRContext
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated JSON: %v", err)
	}

	if len(parsed.UserContext) != 1 {
		t.Fatalf("expected 1 UserContextDoc in parsed JSON, got %d", len(parsed.UserContext))
	}
	if parsed.UserContext[0].SourcePath != "12.md" {
		t.Errorf("expected SourcePath '12.md', got %q", parsed.UserContext[0].SourcePath)
	}
	if parsed.UserContext[0].Content != "### Test Spec" {
		t.Errorf("expected Content '### Test Spec', got %q", parsed.UserContext[0].Content)
	}
}

func TestBundler_PrepareBundle_WithContextFile(t *testing.T) {
	tempDir := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		runGitInDir(t, tempDir, "init")
		runGitInDir(t, tempDir, "checkout", "-b", "main")
	}
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")

	_ = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("v1\n"), 0644)
	runGitInDir(t, tempDir, "add", "file.txt")
	runGitInDir(t, tempDir, "commit", "-m", "init")

	// Create staged change
	_ = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("v2\n"), 0644)
	runGitInDir(t, tempDir, "add", "file.txt")

	// Create context file
	specPath := filepath.Join(tempDir, "spec.md")
	_ = os.WriteFile(specPath, []byte("# Feature Goal\nMust support v2."), 0644)

	bundler := NewBundler(tempDir)
	opts := models.BundleOptions{
		RepoPath:     tempDir,
		Staged:       true,
		ContextFiles: []string{specPath},
	}

	ctx, err := bundler.PrepareBundle(opts)
	if err != nil {
		t.Fatalf("PrepareBundle failed: %v", err)
	}

	if len(ctx.UserContext) != 1 {
		t.Fatalf("expected 1 UserContextDoc, got %d", len(ctx.UserContext))
	}
	if !strings.Contains(ctx.UserContext[0].Content, "### Feature Goal") {
		t.Errorf("expected normalized heading '### Feature Goal', got: %s", ctx.UserContext[0].Content)
	}
}

func TestBundler_PrepareBundle_AutoDiscovery_CurrentBranchWithoutFlag(t *testing.T) {
	tempDir := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		runGitInDir(t, tempDir, "init")
		runGitInDir(t, tempDir, "checkout", "-b", "main")
	}
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")

	_ = os.WriteFile(filepath.Join(tempDir, "base.txt"), []byte("v1\n"), 0644)
	runGitInDir(t, tempDir, "add", "base.txt")
	runGitInDir(t, tempDir, "commit", "-m", "init")

	runGitInDir(t, tempDir, "checkout", "-b", "feat/auto-branch")
	_ = os.WriteFile(filepath.Join(tempDir, "feature.go"), []byte("package feature\n"), 0644)
	runGitInDir(t, tempDir, "add", "feature.go")
	runGitInDir(t, tempDir, "commit", "-m", "add feature")

	// Drop spec in .prv/context/feat-auto-branch.md
	ctxDir := filepath.Join(tempDir, ".prv", "context")
	_ = os.MkdirAll(ctxDir, 0755)
	_ = os.WriteFile(filepath.Join(ctxDir, "feat-auto-branch.md"), []byte("# Auto Branch Spec"), 0644)

	bundler := NewBundler(tempDir)
	// Do NOT pass opts.Branch — PrepareBundle must detect current branch and auto-discover the spec
	ctx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath:   tempDir,
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareBundle failed: %v", err)
	}
	if len(ctx.UserContext) != 1 || !strings.Contains(ctx.UserContext[0].Content, "### Auto Branch Spec") {
		t.Fatalf("expected auto-discovered spec for current branch, got: %+v", ctx.UserContext)
	}
}

func TestBundler_ToMarkdown_ModifiedContextFileWarning(t *testing.T) {
	bundler := NewBundler(".")
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:      "PR Modifying Spec",
			HeadBranch: "feat/x",
			BaseBranch: "main",
		},
		UserContext: []models.UserContextDoc{
			{
				SourcePath: ".prv/context/feat-x.md",
				Content:    "### Spec",
			},
		},
		Files: []models.FileChange{
			{
				Path:      ".prv/context/feat-x.md",
				Additions: 5,
			},
		},
	}

	md := bundler.ToMarkdown(ctx)
	if !strings.Contains(md, "was modified in this change") {
		t.Errorf("expected caution banner when context file is modified in diff, got:\n%s", md)
	}
}

func TestBundler_PrepareBundle_DirtyWithChanges(t *testing.T) {
	tempDir := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		runGitInDir(t, tempDir, "init")
		runGitInDir(t, tempDir, "checkout", "-b", "main")
	}
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")

	_ = os.WriteFile(filepath.Join(tempDir, "tracked.go"), []byte("package main\n"), 0644)
	runGitInDir(t, tempDir, "add", "tracked.go")
	runGitInDir(t, tempDir, "commit", "-m", "init")

	// Modify tracked.go in working tree WITHOUT staging (unstaged modification)
	_ = os.WriteFile(filepath.Join(tempDir, "tracked.go"), []byte("package main\n\nfunc Dirty() {}\n"), 0644)

	bundler := NewBundler(tempDir)
	ctx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath: tempDir,
		Dirty:    true,
	})
	if err != nil {
		t.Fatalf("PrepareBundle with Dirty=true failed: %v", err)
	}
	if len(ctx.Files) != 1 || ctx.Files[0].Path != "tracked.go" {
		t.Fatalf("expected tracked.go in dirty bundle, got: %+v", ctx.Files)
	}
	if ctx.Metadata.BaseBranch != "HEAD (working tree)" {
		t.Errorf("unexpected BaseBranch: %q", ctx.Metadata.BaseBranch)
	}
}

func TestBundler_PrepareBundle_MaxDiffLinesAndTruncatedMarkdown(t *testing.T) {
	bundler := NewBundler(".")
	cfg := config.DefaultConfig()
	cfg.Review.MaxDiffLines = 3

	meta := &models.PRMetadata{
		Title:      "Large PR",
		HeadBranch: "feat/large",
		BaseBranch: "main",
	}
	files := []models.FileChange{
		{
			Path:      "README.md",
			Additions: 4,
			DiffHunk:  "+d1\n+d2\n+d3\n+d4\n",
		},
		{
			Path:      "internal/auth/token.go",
			Additions: 2,
			DiffHunk:  "+s1\n+s2\n",
		},
	}

	ctx := bundler.buildContext(meta, files, nil, cfg)
	if ctx.SummaryStats.TruncatedFiles != 1 {
		t.Fatalf("expected 1 truncated file, got %d", ctx.SummaryStats.TruncatedFiles)
	}
	// Sensitive file internal/auth/token.go must NOT be truncated; README.md must be truncated
	if !ctx.Files[0].IsTruncated {
		t.Errorf("expected README.md to be truncated")
	}
	if ctx.Files[1].IsTruncated {
		t.Errorf("expected sensitive file internal/auth/token.go to be kept intact")
	}

	md := bundler.ToMarkdown(ctx)
	if !strings.Contains(md, "Diff Budget:") || !strings.Contains(md, "`[TRUNCATED]`") {
		t.Errorf("expected Diff Budget summary and [TRUNCATED] badge in markdown, got:\n%s", md)
	}
}

func TestBundler_ToMarkdownAndJSON_InvariantRules(t *testing.T) {
	bundler := NewBundler(".")
	cfg := config.DefaultConfig()
	cfg.Review.CustomRules = []string{"Legacy custom rule"}
	cfg.Review.Rules = config.RulesConfig{
		Forbidden: []string{"NEVER build SQL with fmt.Sprintf"},
		Mandated:  []string{"ALWAYS wrap exported errors with %w"},
	}

	meta := &models.PRMetadata{
		Title:      "Invariant Rules PR",
		HeadBranch: "feat/rules",
		BaseBranch: "main",
	}
	ctx := bundler.buildContext(meta, nil, nil, cfg)

	md := bundler.ToMarkdown(ctx)
	for _, want := range []string{
		"## 📋 Team Engineering Standards & Custom Rules (`.prv.yaml`)",
		"- Legacy custom rule",
		"### 🚫 Forbidden Anti-Patterns",
		"`BLOCKING` finding",
		"added or deleted lines",
		"- NEVER build SQL with fmt.Sprintf",
		"### ✅ Mandated Conventions",
		"`MAJOR` issue",
		"- ALWAYS wrap exported errors with %w",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("expected %q in markdown bundle, got:\n%s", want, md)
		}
	}

	jsonOut, err := bundler.ToJSON(ctx)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	var parsed models.PRContext
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("failed to unmarshal ToJSON output: %v", err)
	}
	if parsed.RulePolicy == nil || parsed.RulePolicy.Forbidden == nil || parsed.RulePolicy.Mandated == nil {
		t.Fatalf("expected non-nil RulePolicy for forbidden and mandated tiers, got: %+v", parsed.RulePolicy)
	}
	if parsed.RulePolicy.Forbidden.Severity != models.SeverityBlocking {
		t.Errorf("expected forbidden severity BLOCKING, got %q", parsed.RulePolicy.Forbidden.Severity)
	}
	if !strings.Contains(parsed.RulePolicy.Forbidden.EvidenceRequirement, "added or deleted lines") {
		t.Errorf("expected forbidden evidence requirement in JSON, got %q", parsed.RulePolicy.Forbidden.EvidenceRequirement)
	}
	if parsed.RulePolicy.Mandated.Severity != models.SeverityMajor {
		t.Errorf("expected mandated severity MAJOR, got %q", parsed.RulePolicy.Mandated.Severity)
	}
	if parsed.RulePolicy.Mandated.EvidenceRequirement != "" {
		t.Errorf("expected empty evidence_requirement for mandated tier, got %q", parsed.RulePolicy.Mandated.EvidenceRequirement)
	}

	// Zero-regression check: without rules.forbidden / rules.mandated, JSON omits them and subsections are absent
	legacyCfg := config.DefaultConfig()
	legacyCfg.Review.CustomRules = []string{"Legacy only"}
	legacyCtx := bundler.buildContext(meta, nil, nil, legacyCfg)
	legacyMD := bundler.ToMarkdown(legacyCtx)
	if strings.Contains(legacyMD, "Forbidden Anti-Patterns") || strings.Contains(legacyMD, "Mandated Conventions") {
		t.Errorf("expected no invariant subsections when rules are empty, got:\n%s", legacyMD)
	}
	legacyJSON, err := bundler.ToJSON(legacyCtx)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	if strings.Contains(legacyJSON, "forbidden_rules") || strings.Contains(legacyJSON, "mandated_rules") || strings.Contains(legacyJSON, "rule_policy") {
		t.Errorf("expected empty invariant rules and rule_policy to be omitted from JSON, got:\n%s", legacyJSON)
	}
}

func TestBundler_PrepareBundle_BaseRevisionConfigAndOverride(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")
	runGitInDir(t, tempDir, "checkout", "-b", "main")

	baseYAML := `version: "1"
review:
  rules:
    forbidden:
      - "NEVER drop HTTP client timeouts"
    mandated:
      - "ALWAYS wrap errors with %w"
`
	cfgPath := filepath.Join(tempDir, ".prv.yaml")
	if err := os.WriteFile(cfgPath, []byte(baseYAML), 0644); err != nil {
		t.Fatal(err)
	}
	runGitInDir(t, tempDir, "add", ".prv.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "baseline config on main")

	// Branch tries to weaken rules by deleting forbidden list
	runGitInDir(t, tempDir, "checkout", "-b", "feat/weaken-rules")
	weakenedYAML := `version: "1"
review:
  rules:
    forbidden: []
    mandated: []
`
	if err := os.WriteFile(cfgPath, []byte(weakenedYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "client.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitInDir(t, tempDir, "add", ".prv.yaml", "client.go")
	runGitInDir(t, tempDir, "commit", "-m", "weaken rules")

	bundler := NewBundler(tempDir)
	ctx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath:   tempDir,
		Branch:     "feat/weaken-rules",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareBundle failed: %v", err)
	}

	if len(ctx.ForbiddenRules) != 1 || ctx.ForbiddenRules[0] != "NEVER drop HTTP client timeouts" {
		t.Fatalf("expected base revision forbidden rule to be enforced, got: %v", ctx.ForbiddenRules)
	}
	if len(ctx.MandatedRules) != 1 || ctx.MandatedRules[0] != "ALWAYS wrap errors with %w" {
		t.Fatalf("expected base revision mandated rule to be enforced, got: %v", ctx.MandatedRules)
	}
	md := bundler.ToMarkdown(ctx)
	if !strings.Contains(md, "Configuration file `.prv.yaml` was modified in this change") {
		t.Errorf("expected config modification caution notice in markdown, got:\n%s", md)
	}

	// Working-tree --dirty test on main: deleting .prv.yaml on disk still enforces HEAD's forbidden rule
	runGitInDir(t, tempDir, "checkout", "main")
	if err := os.Remove(cfgPath); err != nil {
		t.Fatal(err)
	}
	dirtyCtx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath: tempDir,
		Dirty:    true,
	})
	if err != nil {
		t.Fatalf("PrepareBundle --dirty failed: %v", err)
	}
	if len(dirtyCtx.ForbiddenRules) != 1 || dirtyCtx.ForbiddenRules[0] != "NEVER drop HTTP client timeouts" {
		t.Fatalf("expected HEAD revision forbidden rule to be enforced after deleting disk .prv.yaml, got: %v", dirtyCtx.ForbiddenRules)
	}
	runGitInDir(t, tempDir, "checkout", "--", ".prv.yaml")

	// Explicit --config override must win over base revision rules
	overridePath := filepath.Join(t.TempDir(), "override.yaml")
	overrideYAML := `version: "1"
review:
  rules:
    forbidden:
      - "NEVER use explicit override test violation"
`
	if err := os.WriteFile(overridePath, []byte(overrideYAML), 0644); err != nil {
		t.Fatal(err)
	}
	overrideCtx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath:   tempDir,
		Branch:     "feat/weaken-rules",
		BaseBranch: "main",
		ConfigPath: overridePath,
	})
	if err != nil {
		t.Fatalf("PrepareBundle with --config override failed: %v", err)
	}
	if len(overrideCtx.ForbiddenRules) != 1 || overrideCtx.ForbiddenRules[0] != "NEVER use explicit override test violation" {
		t.Fatalf("expected explicit --config override to take precedence over base revision, got: %v", overrideCtx.ForbiddenRules)
	}
}

func TestBundler_PrepareBundle_BrokenWorkingTreeYAML_DirtySucceeds(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")
	runGitInDir(t, tempDir, "checkout", "-b", "main")

	baseYAML := `version: "1"
review:
  rules:
    forbidden:
      - "NEVER skip tests"
`
	cfgPath := filepath.Join(tempDir, ".prv.yaml")
	if err := os.WriteFile(cfgPath, []byte(baseYAML), 0644); err != nil {
		t.Fatal(err)
	}
	runGitInDir(t, tempDir, "add", ".prv.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "add valid config")

	// Developer makes a syntax error in .prv.yaml while writing uncommitted changes
	brokenYAML := `version: "1"
review:
  rules: [this is invalid yaml: {broken
`
	if err := os.WriteFile(cfgPath, []byte(brokenYAML), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(tempDir, "work.go"), []byte("package work\n"), 0644)

	bundler := NewBundler(tempDir)
	// Preparing dirty bundle should not crash on the working-tree syntax error because HEAD has valid config
	ctx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath: tempDir,
		Dirty:    true,
	})
	if err != nil {
		t.Fatalf("PrepareBundle --dirty failed unexpectedly on broken working-tree YAML: %v", err)
	}
	if len(ctx.ForbiddenRules) != 1 || ctx.ForbiddenRules[0] != "NEVER skip tests" {
		t.Errorf("expected HEAD's forbidden rule to be loaded, got %v", ctx.ForbiddenRules)
	}
}

func TestBundler_ModifiedConfigCaution_RenamedAndFiltered(t *testing.T) {
	bundler := NewBundler(".")

	// Test 1: Renamed from .prv.yaml to .prv.yaml.bak
	renamedFiles := []models.FileChange{
		{
			OldPath: ".prv.yaml",
			Path:    ".prv.yaml.bak",
			Status:  "renamed",
		},
	}
	ctxRenamed := bundler.buildContext(&models.PRMetadata{Title: "Rename Test"}, renamedFiles, nil, config.DefaultConfig())
	if ctxRenamed.ModifiedConfigFile != ".prv.yaml" {
		t.Errorf("expected ModifiedConfigFile '.prv.yaml', got %q", ctxRenamed.ModifiedConfigFile)
	}
	mdRenamed := bundler.ToMarkdown(ctxRenamed)
	if !strings.Contains(mdRenamed, "Configuration file `.prv.yaml` was modified in this change") {
		t.Errorf("expected caution for renamed config in markdown, got:\n%s", mdRenamed)
	}

	// Test 2: .prv.yaml ignored by *.yaml ignore pattern
	filteredFiles := []models.FileChange{
		{
			Path:   ".prv.yaml",
			Status: "modified",
		},
	}
	cfgWithIgnore := config.DefaultConfig()
	cfgWithIgnore.Review.IgnorePatterns = []string{"*.yaml"}
	ctxFiltered := bundler.buildContext(&models.PRMetadata{Title: "Filtered Config Test"}, filteredFiles, nil, cfgWithIgnore)
	if ctxFiltered.ModifiedConfigFile != ".prv.yaml" {
		t.Errorf("expected ModifiedConfigFile '.prv.yaml' even when ignored, got %q", ctxFiltered.ModifiedConfigFile)
	}
	mdFiltered := bundler.ToMarkdown(ctxFiltered)
	if !strings.Contains(mdFiltered, "Configuration file `.prv.yaml` was modified in this change") {
		t.Errorf("expected caution for filtered config in markdown, got:\n%s", mdFiltered)
	}
}

func TestBundler_UnbornHEAD_IgnoresWorkingTreeConfig(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")

	// Write uncommitted .prv.yaml in working tree
	uncommittedYAML := `version: "1"
review:
  rules:
    forbidden:
      - "UNCOMMITTED RULE SHOULD NOT BE LOADED"
`
	if err := os.WriteFile(filepath.Join(tempDir, ".prv.yaml"), []byte(uncommittedYAML), 0644); err != nil {
		t.Fatal(err)
	}

	bundler := NewBundler(tempDir)
	// Without explicit --config, unborn HEAD must return DefaultConfig (Finding P2.4)
	cfg, _, err := bundler.resolveBaseConfig("HEAD", nil, models.BundleOptions{RepoPath: tempDir})
	if err != nil {
		t.Fatalf("resolveBaseConfig failed: %v", err)
	}
	if len(cfg.Review.Rules.Forbidden) != 0 {
		t.Fatalf("expected 0 forbidden rules from unborn HEAD without --config, got: %v", cfg.Review.Rules.Forbidden)
	}

	// With explicit --config, explicitCfg is honored
	explicitPath := filepath.Join(tempDir, ".prv.yaml")
	explicitCfg, err := config.LoadConfig(tempDir, explicitPath)
	if err != nil {
		t.Fatal(err)
	}
	cfgExplicit, _, err := bundler.resolveBaseConfig("HEAD", explicitCfg, models.BundleOptions{RepoPath: tempDir, ConfigPath: explicitPath})
	if err != nil {
		t.Fatalf("resolveBaseConfig with explicit cfg failed: %v", err)
	}
	if len(cfgExplicit.Review.Rules.Forbidden) != 1 || cfgExplicit.Review.Rules.Forbidden[0] != "UNCOMMITTED RULE SHOULD NOT BE LOADED" {
		t.Fatalf("expected explicit config to be honored on unborn HEAD, got: %v", cfgExplicit.Review.Rules.Forbidden)
	}
}

func TestBundler_ModifiedSymlinkTarget_TriggersCaution(t *testing.T) {
	bundler := NewBundler(".")

	// When a symlink target (e.g. configs/team-rules.yaml) is modified in the diff (Finding P3.5)
	symlinkTargetFiles := []models.FileChange{
		{
			Path:   "configs/team-rules.yaml",
			Status: "modified",
		},
	}
	ctx := bundler.buildContext(&models.PRMetadata{Title: "Symlink Target Edit"}, symlinkTargetFiles, nil, config.DefaultConfig(), "configs/team-rules.yaml")
	if ctx.ModifiedConfigFile != "configs/team-rules.yaml" {
		t.Fatalf("expected ModifiedConfigFile 'configs/team-rules.yaml', got %q", ctx.ModifiedConfigFile)
	}
	md := bundler.ToMarkdown(ctx)
	if !strings.Contains(md, "Configuration file `configs/team-rules.yaml` was modified in this change") {
		t.Fatalf("expected caution in markdown for modified symlink target, got:\n%s", md)
	}
}

func TestBundler_isLocalRepoMatchWithMeta(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")
	runGitInDir(t, tempDir, "remote", "add", "origin", "git@github.com:myorg/myrepo.git")

	bundler := NewBundler(tempDir)

	// Matching host and slug (Finding P2.2)
	matchMeta := &models.PRMetadata{
		Repo: "myorg/myrepo",
		URL:  "https://github.com/myorg/myrepo/pull/42",
	}
	if !bundler.isLocalRepoMatchWithMeta(matchMeta) {
		t.Errorf("expected true for matching repo, got false")
	}

	// Mismatched org/repo
	mismatchMeta := &models.PRMetadata{
		Repo: "otherorg/otherrepo",
		URL:  "https://github.com/otherorg/otherrepo/pull/42",
	}
	if bundler.isLocalRepoMatchWithMeta(mismatchMeta) {
		t.Errorf("expected false for mismatched repo, got true")
	}

	// Mismatched host (GitHub enterprise vs github.com)
	mismatchHostMeta := &models.PRMetadata{
		Repo: "myorg/myrepo",
		URL:  "https://ghe.company.internal/myorg/myrepo/pull/42",
	}
	if bundler.isLocalRepoMatchWithMeta(mismatchHostMeta) {
		t.Errorf("expected false for mismatched host, got true")
	}
}

func TestBundler_PinPRBaseCommit(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")
	runGitInDir(t, tempDir, "remote", "add", "origin", "git@github.com:test/repo.git")
	runGitInDir(t, tempDir, "checkout", "-b", "main")

	// Base commit 1 with initial rule (Finding P2.1)
	yaml1 := `version: "1"
review:
  rules:
    forbidden:
      - "RULE AT COMMIT 1"
`
	_ = os.WriteFile(filepath.Join(tempDir, ".prv.yaml"), []byte(yaml1), 0644)
	runGitInDir(t, tempDir, "add", ".prv.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "commit 1")
	commit1 := strings.TrimSpace(runGitInDir(t, tempDir, "rev-parse", "HEAD"))

	// Base commit 2 with changed rule
	yaml2 := `version: "1"
review:
  rules:
    forbidden:
      - "RULE AT COMMIT 2"
`
	_ = os.WriteFile(filepath.Join(tempDir, ".prv.yaml"), []byte(yaml2), 0644)
	runGitInDir(t, tempDir, "add", ".prv.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "commit 2")

	bundler := NewBundler(tempDir)

	// If PR targets BaseCommit commit1, resolveBaseConfig resolves rules from commit1
	cfg, _, err := bundler.resolveBaseConfig(commit1, nil, models.BundleOptions{RepoPath: tempDir})
	if err != nil {
		t.Fatalf("resolveBaseConfig for commit1 failed: %v", err)
	}
	if len(cfg.Review.Rules.Forbidden) != 1 || cfg.Review.Rules.Forbidden[0] != "RULE AT COMMIT 1" {
		t.Fatalf("expected rules from commit1, got: %v", cfg.Review.Rules.Forbidden)
	}

	// If PR targets main tip, resolves rules from commit2
	cfgTip, _, err := bundler.resolveBaseConfig("main", nil, models.BundleOptions{RepoPath: tempDir})
	if err != nil {
		t.Fatalf("resolveBaseConfig for main failed: %v", err)
	}
	if len(cfgTip.Review.Rules.Forbidden) != 1 || cfgTip.Review.Rules.Forbidden[0] != "RULE AT COMMIT 2" {
		t.Fatalf("expected rules from commit2, got: %v", cfgTip.Review.Rules.Forbidden)
	}
}

func TestBundler_HonorBaseFlag_WithDifferentPRTarget(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")
	runGitInDir(t, tempDir, "checkout", "-b", "main")

	// Commit on main with rule MAIN
	yamlMain := `version: "1"
review:
  rules:
    forbidden:
      - "RULE MAIN"
`
	_ = os.WriteFile(filepath.Join(tempDir, ".prv.yaml"), []byte(yamlMain), 0644)
	runGitInDir(t, tempDir, "add", ".prv.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "main baseline")

	// Branch staging from main, with rule STAGING
	runGitInDir(t, tempDir, "checkout", "-b", "staging")
	yamlStaging := `version: "1"
review:
  rules:
    forbidden:
      - "RULE STAGING"
`
	_ = os.WriteFile(filepath.Join(tempDir, ".prv.yaml"), []byte(yamlStaging), 0644)
	runGitInDir(t, tempDir, "add", ".prv.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "staging config")
	stagingSHA := strings.TrimSpace(runGitInDir(t, tempDir, "rev-parse", "HEAD"))

	// Branch feat/work from staging
	runGitInDir(t, tempDir, "checkout", "-b", "feat/work")
	_ = os.WriteFile(filepath.Join(tempDir, "work.go"), []byte("package main\nfunc Work(){}\n"), 0644)
	runGitInDir(t, tempDir, "add", "work.go")
	runGitInDir(t, tempDir, "commit", "-m", "add work")

	bundler := NewBundler(tempDir)
	// Even if a PR metadata targeted main, passing explicit BaseBranch: "staging" must use staging (Finding 1)
	ctx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath:   tempDir,
		Branch:     "feat/work",
		BaseBranch: "staging",
	})
	if err != nil {
		t.Fatalf("PrepareBundle failed: %v", err)
	}

	if ctx.Metadata.BaseBranch != "staging" {
		t.Errorf("expected BaseBranch 'staging', got %q", ctx.Metadata.BaseBranch)
	}
	if ctx.Metadata.BaseCommit != stagingSHA {
		t.Errorf("expected BaseCommit to match staging %s, got %s", stagingSHA, ctx.Metadata.BaseCommit)
	}
	if len(ctx.ForbiddenRules) != 1 || ctx.ForbiddenRules[0] != "RULE STAGING" {
		t.Errorf("expected staging rules, got: %v", ctx.ForbiddenRules)
	}
}

func TestBundler_ModifiedConfigCaution_SubdirectoryAndDirectorySymlink(t *testing.T) {
	bundler := NewBundler(".")

	// Case A: Diff touches services/.prv.yml from services/ (Finding 5)
	subdirFiles := []models.FileChange{
		{
			Path:   "services/.prv.yml",
			Status: "modified",
		},
	}
	ctxSub := bundler.buildContext(&models.PRMetadata{Title: "Subdir Config"}, subdirFiles, nil, config.DefaultConfig(), "services/.prv.yml")
	if ctxSub.ModifiedConfigFile != "services/.prv.yml" {
		t.Errorf("expected ModifiedConfigFile 'services/.prv.yml', got %q", ctxSub.ModifiedConfigFile)
	}
	mdSub := bundler.ToMarkdown(ctxSub)
	if !strings.Contains(mdSub, "Configuration file `services/.prv.yml` was modified in this change") {
		t.Errorf("expected caution for services/.prv.yml in markdown, got:\n%s", mdSub)
	}

	// Case B: Diff retargets an intermediate directory symlink "shared_dir" (Finding 5)
	dirSymlinkFiles := []models.FileChange{
		{
			Path:   "shared_dir",
			Status: "modified",
		},
	}
	ctxDir := bundler.buildContext(&models.PRMetadata{Title: "Dir Symlink Edit"}, dirSymlinkFiles, nil, config.DefaultConfig(), "shared_dir", "shared_dir/rules.yaml", "real_configs/rules.yaml")
	if ctxDir.ModifiedConfigFile != "shared_dir" {
		t.Errorf("expected ModifiedConfigFile 'shared_dir', got %q", ctxDir.ModifiedConfigFile)
	}
	mdDir := bundler.ToMarkdown(ctxDir)
	if !strings.Contains(mdDir, "Configuration file `shared_dir` was modified in this change") {
		t.Errorf("expected caution for shared_dir in markdown, got:\n%s", mdDir)
	}
}

func TestBundler_RejectPRAndBaseCombination(t *testing.T) {
	bundler := NewBundler(".")
	_, err := bundler.PrepareBundle(models.BundleOptions{
		PRIdentifier: "5",
		BaseBranch:   "staging",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot specify both --pr and --base") {
		t.Fatalf("expected error rejecting --pr and --base combination, got: %v", err)
	}
}

func TestBundler_LocalDiff_ReportsLocalHeadCommit(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")
	runGitInDir(t, tempDir, "checkout", "-b", "main")

	_ = os.WriteFile(filepath.Join(tempDir, "base.go"), []byte("package main\n"), 0644)
	runGitInDir(t, tempDir, "add", "base.go")
	runGitInDir(t, tempDir, "commit", "-m", "base commit")
	baseSHA := strings.TrimSpace(runGitInDir(t, tempDir, "rev-parse", "HEAD"))

	runGitInDir(t, tempDir, "checkout", "-b", "feat/local-head")
	_ = os.WriteFile(filepath.Join(tempDir, "feat.go"), []byte("package main\nfunc Feat(){}\n"), 0644)
	runGitInDir(t, tempDir, "add", "feat.go")
	runGitInDir(t, tempDir, "commit", "-m", "feat commit")
	headSHA := strings.TrimSpace(runGitInDir(t, tempDir, "rev-parse", "HEAD"))

	bundler := NewBundler(tempDir)
	ctx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath:   tempDir,
		Branch:     "feat/local-head",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareBundle failed: %v", err)
	}
	if ctx.Metadata.BaseCommit != baseSHA {
		t.Errorf("expected BaseCommit %q, got %q", baseSHA, ctx.Metadata.BaseCommit)
	}
	if ctx.Metadata.HeadCommit != headSHA {
		t.Errorf("expected HeadCommit %q, got %q", headSHA, ctx.Metadata.HeadCommit)
	}
}

func TestBundler_ExplicitAbsoluteConfigPath_TriggersModifiedConfigCaution(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")
	runGitInDir(t, tempDir, "checkout", "-b", "main")

	policyPath := filepath.Join(tempDir, "review-policy.yaml")
	_ = os.WriteFile(policyPath, []byte("version: \"1\"\nreview:\n  persona: \"strict\"\n"), 0644)
	runGitInDir(t, tempDir, "add", "review-policy.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "add policy")

	runGitInDir(t, tempDir, "checkout", "-b", "feat/edit-policy")
	_ = os.WriteFile(policyPath, []byte("version: \"1\"\nreview:\n  persona: \"relaxed\"\n"), 0644)
	runGitInDir(t, tempDir, "add", "review-policy.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "edit policy")

	bundler := NewBundler(tempDir)
	ctx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath:   tempDir,
		Branch:     "feat/edit-policy",
		BaseBranch: "main",
		ConfigPath: policyPath, // Absolute path
	})
	if err != nil {
		t.Fatalf("PrepareBundle failed: %v", err)
	}
	if ctx.ModifiedConfigFile != "review-policy.yaml" {
		t.Fatalf("expected ModifiedConfigFile 'review-policy.yaml' when passing absolute --config, got %q", ctx.ModifiedConfigFile)
	}
	md := bundler.ToMarkdown(ctx)
	if !strings.Contains(md, "Configuration file `review-policy.yaml` was modified in this change") {
		t.Errorf("expected caution in markdown for modified explicit config, got:\n%s", md)
	}
}

func TestBundler_OrphanUnbornHEAD_DoesNotFallbackToOriginHEAD(t *testing.T) {
	tempDir := t.TempDir()
	runGitInDir(t, tempDir, "init")
	runGitInDir(t, tempDir, "config", "user.email", "test@example.com")
	runGitInDir(t, tempDir, "config", "user.name", "Test User")
	runGitInDir(t, tempDir, "checkout", "-b", "main")

	// Commit a restrictive .prv.yaml on main and create refs/remotes/origin/HEAD -> origin/main
	originYAML := `version: "1"
review:
  ignore_patterns:
    - "*.go"
  rules:
    forbidden:
      - "ORIGIN HEAD RULE"
`
	_ = os.WriteFile(filepath.Join(tempDir, ".prv.yaml"), []byte(originYAML), 0644)
	runGitInDir(t, tempDir, "add", ".prv.yaml")
	runGitInDir(t, tempDir, "commit", "-m", "origin main config")
	mainSHA := strings.TrimSpace(runGitInDir(t, tempDir, "rev-parse", "HEAD"))
	runGitInDir(t, tempDir, "update-ref", "refs/remotes/origin/main", mainSHA)
	runGitInDir(t, tempDir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	// Switch to an orphan branch where local HEAD has no commits
	runGitInDir(t, tempDir, "checkout", "--orphan", "orphan-start")
	runGitInDir(t, tempDir, "rm", "-rf", ".")

	_ = os.WriteFile(filepath.Join(tempDir, "first.go"), []byte("package main\nfunc First(){}\n"), 0644)
	runGitInDir(t, tempDir, "add", "first.go")

	bundler := NewBundler(tempDir)
	ctx, err := bundler.PrepareBundle(models.BundleOptions{
		RepoPath: tempDir,
		Staged:   true,
	})
	if err != nil {
		t.Fatalf("PrepareBundle --staged on orphan branch failed: %v", err)
	}
	if len(ctx.ForbiddenRules) != 0 {
		t.Errorf("expected 0 forbidden rules on unborn orphan HEAD (should not load origin/HEAD), got %v", ctx.ForbiddenRules)
	}
	if len(ctx.Files) != 1 || ctx.Files[0].Path != "first.go" {
		t.Errorf("expected first.go to be reviewed (not ignored by origin/HEAD), got files=%v, ignored=%v", ctx.Files, ctx.IgnoredFiles)
	}
}
