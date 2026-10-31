package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vubon/prv/internal/config"
	"github.com/vubon/prv/internal/models"
)

func TestNormalizeMarkdownHeadings_ShiftsTwoLevels(t *testing.T) {
	input := `# Main Title
Some description.
## Section 2
Details here.
### Subsection 3
More details.
#### Level 4
Deep.
`
	expected := `### Main Title
Some description.
#### Section 2
Details here.
##### Subsection 3
More details.
###### Level 4
Deep.
`
	got := NormalizeMarkdownHeadings(input)
	if got != expected {
		t.Errorf("NormalizeMarkdownHeadings mismatch:\nGOT:\n%s\nWANT:\n%s", got, expected)
	}
}

func TestNormalizeMarkdownHeadings_PreservesCodeBlocks(t *testing.T) {
	input := `# Title
Here is some code:
` + "```go\n" +
		`// # This is a comment inside code block
# Also should not be touched
func main() {}
` + "```\n" +
		`## Next Section
`
	expected := `### Title
Here is some code:
` + "```go\n" +
		`// # This is a comment inside code block
# Also should not be touched
func main() {}
` + "```\n" +
		`#### Next Section
`
	got := NormalizeMarkdownHeadings(input)
	if got != expected {
		t.Errorf("code block preservation failed:\nGOT:\n%s\nWANT:\n%s", got, expected)
	}
}

func TestNormalizeMarkdownHeadings_UTF8AndSpecialCharacters(t *testing.T) {
	input := `# 🚀 Feature: বাংলা ও Emoji Support & "Quotes"`
	expected := `### 🚀 Feature: বাংলা ও Emoji Support & "Quotes"`

	got := NormalizeMarkdownHeadings(input)
	if got != expected {
		t.Errorf("UTF-8 heading normalization failed:\nGOT:\n%q\nWANT:\n%q", got, expected)
	}
}

func TestNormalizeMarkdownHeadings_Empty(t *testing.T) {
	if got := NormalizeMarkdownHeadings(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestLoadContextFiles_ExplicitExisting(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.md")
	content := "# Feature Spec\nMust handle timeout."
	if err := os.WriteFile(specFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	opts := models.BundleOptions{
		RepoPath:     tmpDir,
		ContextFiles: []string{specFile},
	}
	cfg := config.DefaultConfig()

	docs, err := LoadContextFiles(opts, cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}

	if docs[0].SourcePath != specFile {
		t.Errorf("expected source path %q, got %q", specFile, docs[0].SourcePath)
	}

	if !strings.Contains(docs[0].Content, "### Feature Spec") {
		t.Errorf("expected normalized heading '### Feature Spec', got:\n%s", docs[0].Content)
	}
}

func TestLoadContextFiles_ExplicitMissing_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	missingFile := filepath.Join(tmpDir, "missing_spec.md")

	opts := models.BundleOptions{
		RepoPath:     tmpDir,
		ContextFiles: []string{missingFile},
	}
	cfg := config.DefaultConfig()

	_, err := LoadContextFiles(opts, cfg)
	if err == nil {
		t.Fatal("expected error for missing context file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to read context file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadContextFiles_ExceedsSizeLimit_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	largeFile := filepath.Join(tmpDir, "large_spec.md")

	// Create file of 5KB
	largeData := strings.Repeat("A", 5*1024)
	if err := os.WriteFile(largeFile, []byte(largeData), 0644); err != nil {
		t.Fatalf("failed to write large spec: %v", err)
	}

	opts := models.BundleOptions{
		RepoPath:     tmpDir,
		ContextFiles: []string{largeFile},
	}
	// Configure tight 2KB limit
	cfg := &config.Config{
		Context: config.ContextConfig{
			Dir:       ".prv/context",
			MaxSizeKB: 2,
		},
	}

	_, err := LoadContextFiles(opts, cfg)
	if err == nil {
		t.Fatal("expected error for oversized context file, got nil")
	}

	if !strings.Contains(err.Error(), "exceeds size limit") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadContextFiles_UnlimitedSize(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "spec.md")
	data := strings.Repeat("A", 5*1024)
	if err := os.WriteFile(file, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	opts := models.BundleOptions{
		RepoPath:     tmpDir,
		ContextFiles: []string{file},
	}
	// MaxSizeKB = 0 (unlimited)
	cfg := &config.Config{
		Context: config.ContextConfig{
			Dir:       ".prv/context",
			MaxSizeKB: 0,
		},
	}

	docs, err := LoadContextFiles(opts, cfg)
	if err != nil {
		t.Fatalf("expected no error with unlimited size, got: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
}

func TestLoadContextFiles_AutoDiscovery_PRPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	contextDir := filepath.Join(tmpDir, ".prv", "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		t.Fatalf("failed to create context dir: %v", err)
	}

	// Case 1: Both 42.md and pr-42.md exist; 42.md must take precedence
	if err := os.WriteFile(filepath.Join(contextDir, "42.md"), []byte("# Spec 42 Number Only"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contextDir, "pr-42.md"), []byte("# Spec PR Prefix"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := models.BundleOptions{
		RepoPath:     tmpDir,
		PRIdentifier: "42",
	}
	cfg := config.DefaultConfig()

	docs, err := LoadContextFiles(opts, cfg)
	if err != nil {
		t.Fatalf("LoadContextFiles failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if !strings.Contains(docs[0].Content, "Spec 42 Number Only") {
		t.Errorf("expected 42.md precedence, got: %s", docs[0].Content)
	}

	// Case 2: Only pr_42.md exists (fallback)
	if err := os.Remove(filepath.Join(contextDir, "42.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(contextDir, "pr-42.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contextDir, "pr_42.md"), []byte("# Spec Underscore"), 0644); err != nil {
		t.Fatal(err)
	}

	docs, err = LoadContextFiles(opts, cfg)
	if err != nil {
		t.Fatalf("LoadContextFiles failed: %v", err)
	}
	if len(docs) != 1 || !strings.Contains(docs[0].Content, "Spec Underscore") {
		t.Errorf("expected fallback to pr_42.md, got: %v", docs)
	}
}

func TestLoadContextFiles_AutoDiscovery_BranchSlug(t *testing.T) {
	tmpDir := t.TempDir()
	contextDir := filepath.Join(tmpDir, ".prv", "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		t.Fatal(err)
	}

	specPath := filepath.Join(contextDir, "feat-auth-login.md")
	if err := os.WriteFile(specPath, []byte("# Auth Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := models.BundleOptions{
		RepoPath: tmpDir,
		Branch:   "feat/auth/login",
	}
	cfg := config.DefaultConfig()

	docs, err := LoadContextFiles(opts, cfg)
	if err != nil {
		t.Fatalf("LoadContextFiles failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if !strings.Contains(docs[0].Content, "Auth Spec") {
		t.Errorf("expected sanitized branch match, got: %v", docs[0])
	}
}

func TestLoadContextFiles_AutoDiscovery_Staged(t *testing.T) {
	tmpDir := t.TempDir()
	contextDir := filepath.Join(tmpDir, ".prv", "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(contextDir, "staged.md"), []byte("# Staged Review Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := models.BundleOptions{
		RepoPath: tmpDir,
		Staged:   true,
	}
	cfg := config.DefaultConfig()

	docs, err := LoadContextFiles(opts, cfg)
	if err != nil {
		t.Fatalf("LoadContextFiles failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if !strings.Contains(docs[0].Content, "Staged Review Spec") {
		t.Errorf("expected staged.md match, got: %v", docs[0])
	}
}

func TestNormalizeMarkdownHeadings_NestedCodeFences(t *testing.T) {
	input := "# Outer Spec\n" +
		"````markdown\n" +
		"# Embedded heading inside 4-fence\n" +
		"```go\n" +
		"// # comment inside 3-fence\n" +
		"func hello() {}\n" +
		"```\n" +
		"# Still inside 4-fence\n" +
		"````\n" +
		"## Real Section Outside\n"
	got := NormalizeMarkdownHeadings(input)
	if !strings.Contains(got, "### Outer Spec") {
		t.Errorf("expected outer heading shifted, got:\n%s", got)
	}
	if !strings.Contains(got, "# Embedded heading inside 4-fence") {
		t.Errorf("expected embedded heading preserved inside 4-fence, got:\n%s", got)
	}
	if !strings.Contains(got, "// # comment inside 3-fence") {
		t.Errorf("expected comment inside 3-fence preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "# Still inside 4-fence") {
		t.Errorf("expected heading inside 4-fence after 3-fence preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "#### Real Section Outside") {
		t.Errorf("expected section outside 4-fence shifted, got:\n%s", got)
	}
}

func TestNormalizeMarkdownHeadings_LongLinesWithoutLimit(t *testing.T) {
	// Line length 70 KB (> 64 KB bufio.Scanner default buffer limit)
	longLine := strings.Repeat("A", 70000)
	input := "# Heading\n" + longLine + "\n## Subheading\n"
	got := NormalizeMarkdownHeadings(input)

	if !strings.Contains(got, "### Heading") {
		t.Errorf("expected heading shifted")
	}
	if !strings.Contains(got, "#### Subheading") {
		t.Errorf("expected subheading shifted, got truncation")
	}
	if !strings.Contains(got, longLine) {
		t.Errorf("expected long line preserved without truncation")
	}
}

func TestNormalizeMarkdownHeadings_Level5And6(t *testing.T) {
	input := "##### Level 5\n###### Level 6\n"
	got := NormalizeMarkdownHeadings(input)
	expected := "###### Level 5\n###### Level 6\n"
	if got != expected {
		t.Errorf("expected capped at level 6:\nGOT:\n%s\nWANT:\n%s", got, expected)
	}
}

func TestNormalizeMarkdownHeadings_UnclosedCodeFenceAutoClosed(t *testing.T) {
	input := "# Spec\n```go\nfunc unclosed() {}\n"
	got := NormalizeMarkdownHeadings(input)
	if !strings.HasSuffix(got, "```\n") {
		t.Errorf("expected unclosed code fence to be auto-closed, got:\n%s", got)
	}
}

func TestLoadContextFiles_ExplicitBareNameResolvesInContextDir(t *testing.T) {
	tmpDir := t.TempDir()
	contextDir := filepath.Join(tmpDir, ".prv", "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contextDir, "12.md"), []byte("# PR 12 Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := models.BundleOptions{
		RepoPath:     tmpDir,
		ContextFiles: []string{"12.md"},
	}
	docs, err := LoadContextFiles(opts, config.DefaultConfig())
	if err != nil {
		t.Fatalf("expected bare filename 12.md to resolve in .prv/context, got err: %v", err)
	}
	if len(docs) != 1 || !strings.Contains(docs[0].Content, "### PR 12 Spec") {
		t.Errorf("unexpected docs: %+v", docs)
	}
}

func TestLoadContextFiles_RejectsYAMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	opts := models.BundleOptions{
		RepoPath:     tmpDir,
		ContextFiles: []string{".prv.yaml"},
	}
	_, err := LoadContextFiles(opts, config.DefaultConfig())
	if err == nil || !strings.Contains(err.Error(), "looks like a YAML config") {
		t.Fatalf("expected helpful YAML config error, got: %v", err)
	}
}
