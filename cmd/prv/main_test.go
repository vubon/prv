package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/vubon/prv/internal/config"
)

func TestResolvePositionalArgs_TwoArgs(t *testing.T) {
	var branch, base, prID string
	resolvePositionalArgs([]string{"feat/payment", "staging"}, &branch, &base, &prID)

	if branch != "feat/payment" {
		t.Errorf("expected branch 'feat/payment', got %q", branch)
	}
	if base != "staging" {
		t.Errorf("expected base 'staging', got %q", base)
	}
	if prID != "" {
		t.Errorf("expected empty prID, got %q", prID)
	}
}

func TestResolvePositionalArgs_CommonBaseBranch(t *testing.T) {
	tests := []string{"staging", "dev", "develop", "main", "master", "next", "trunk"}
	for _, baseName := range tests {
		var branch, base, prID string
		resolvePositionalArgs([]string{baseName}, &branch, &base, &prID)

		if base != baseName {
			t.Errorf("expected base %q, got %q", baseName, base)
		}
		if branch != "" {
			t.Errorf("expected empty branch when base is passed alone, got %q", branch)
		}
	}
}

func TestResolvePositionalArgs_FeatureBranch(t *testing.T) {
	var branch, base, prID string
	resolvePositionalArgs([]string{"feat/user-auth"}, &branch, &base, &prID)

	if branch != "feat/user-auth" {
		t.Errorf("expected branch 'feat/user-auth', got %q", branch)
	}
	if base != "" {
		t.Errorf("expected empty base, got %q", base)
	}
}

func TestResolvePositionalArgs_PRIDAndURL(t *testing.T) {
	// Numeric PR ID
	var branch, base, prID string
	resolvePositionalArgs([]string{"42"}, &branch, &base, &prID)
	if prID != "42" {
		t.Errorf("expected prID '42', got %q", prID)
	}

	// PR URL
	var branch2, base2, prID2 string
	url := "https://github.com/vubon/prv/pull/123"
	resolvePositionalArgs([]string{url}, &branch2, &base2, &prID2)
	if prID2 != url {
		t.Errorf("expected prID %q, got %q", url, prID2)
	}
}

func TestResolvePositionalArgs_FlagPrecedence(t *testing.T) {
	// When --base is already set by flag
	branch := ""
	base := "main"
	prID := ""
	resolvePositionalArgs([]string{"feat/auth"}, &branch, &base, &prID)
	if branch != "feat/auth" || base != "main" {
		t.Errorf("expected branch 'feat/auth' and base 'main', got %q and %q", branch, base)
	}

	// When --branch is already set by flag
	branch2 := "feat/auth"
	base2 := ""
	prID2 := ""
	resolvePositionalArgs([]string{"staging"}, &branch2, &base2, &prID2)
	if branch2 != "feat/auth" || base2 != "staging" {
		t.Errorf("expected branch 'feat/auth' and base 'staging', got %q and %q", branch2, base2)
	}
}

func TestIsAllDigits(t *testing.T) {
	if !isAllDigits("12345") {
		t.Error("expected true for '12345'")
	}
	if isAllDigits("123a") {
		t.Error("expected false for '123a'")
	}
	if isAllDigits("") {
		t.Error("expected false for ''")
	}
}

func TestParseFlagsAndPositionals(t *testing.T) {
	// Case 1: Flag after single positional arg (prv bundle main --json)
	fs1 := flag.NewFlagSet("test1", flag.ContinueOnError)
	var formatJSON1 bool
	var base1, branch1, prID1 string
	fs1.BoolVar(&formatJSON1, "json", false, "")
	fs1.StringVar(&base1, "base", "", "")

	pos1 := parseFlagsAndPositionals(fs1, []string{"main", "--json"})
	resolvePositionalArgs(pos1, &branch1, &base1, &prID1)

	if !formatJSON1 {
		t.Errorf("expected formatJSON=true when --json is placed after positional arg")
	}
	if base1 != "main" {
		t.Errorf("expected base='main', got %q", base1)
	}

	// Case 2: Flag after two positional args (prv bundle feat/a main --json)
	fs2 := flag.NewFlagSet("test2", flag.ContinueOnError)
	var formatJSON2 bool
	var base2, branch2, prID2 string
	fs2.BoolVar(&formatJSON2, "json", false, "")

	pos2 := parseFlagsAndPositionals(fs2, []string{"feat/a", "main", "--json"})
	resolvePositionalArgs(pos2, &branch2, &base2, &prID2)

	if !formatJSON2 {
		t.Errorf("expected formatJSON=true when --json is placed after 2 positional args")
	}
	if branch2 != "feat/a" || base2 != "main" {
		t.Errorf("expected branch='feat/a' and base='main', got %q and %q", branch2, base2)
	}

	// Case 3: Flag with value after positional arg (prv bundle main -o out.md)
	fs3 := flag.NewFlagSet("test3", flag.ContinueOnError)
	var out3, base3, branch3, prID3 string
	fs3.StringVar(&out3, "output", "", "")
	fs3.StringVar(&out3, "o", "", "")

	pos3 := parseFlagsAndPositionals(fs3, []string{"main", "-o", "out.md"})
	resolvePositionalArgs(pos3, &branch3, &base3, &prID3)

	if out3 != "out.md" {
		t.Errorf("expected out='out.md', got %q", out3)
	}
	if base3 != "main" {
		t.Errorf("expected base='main', got %q", base3)
	}
}

func TestStringSliceFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var files stringSliceFlag
	fs.Var(&files, "context-file", "")
	fs.Var(&files, "c", "")

	err := fs.Parse([]string{"--context-file", "spec1.md", "-c", "spec2.md", "-c", "spec3.md"})
	if err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	expected := []string{"spec1.md", "spec2.md", "spec3.md"}
	for i, exp := range expected {
		if files[i] != exp {
			t.Errorf("expected files[%d] = %q, got %q", i, exp, files[i])
		}
	}
	if files.String() != "spec1.md, spec2.md, spec3.md" {
		t.Errorf("unexpected string output: %s", files.String())
	}
}

func TestCLI_Init_ScaffoldsDefaultContextFolder(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".prv.yaml")

	runInit([]string{"--repo", tempDir})

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected .prv.yaml created at %s, got err: %v", configPath, err)
	}

	contextDir := filepath.Join(tempDir, ".prv", "context")
	if info, err := os.Stat(contextDir); err != nil || !info.IsDir() {
		t.Fatalf("expected default context directory at %s, got: %v", contextDir, err)
	}

	templatePath := filepath.Join(contextDir, "template.md")
	if info, err := os.Stat(templatePath); err != nil || info.IsDir() {
		t.Fatalf("expected starter template at %s, got: %v", templatePath, err)
	}
}

func TestCLI_Init_CustomContextDirFlag(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".prv.yaml")

	runInit([]string{"--repo", tempDir, "--context-dir", "specs/reviews"})

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected .prv.yaml created at %s, got err: %v", configPath, err)
	}

	customContextDir := filepath.Join(tempDir, "specs", "reviews")
	if info, err := os.Stat(customContextDir); err != nil || !info.IsDir() {
		t.Fatalf("expected custom context directory at %s, got: %v", customContextDir, err)
	}

	templatePath := filepath.Join(customContextDir, "template.md")
	if info, err := os.Stat(templatePath); err != nil || info.IsDir() {
		t.Fatalf("expected starter template at %s, got: %v", templatePath, err)
	}

	// Verify loaded config in that repo has custom dir
	cfg, err := config.LoadConfig(tempDir, "")
	if err != nil {
		t.Fatalf("failed to load config from %s: %v", tempDir, err)
	}
	if cfg.Context.Dir != "specs/reviews" {
		t.Errorf("expected context dir 'specs/reviews', got %q", cfg.Context.Dir)
	}
}

func TestCLI_Init_FlagsAfterPositionalArg(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom.yaml")

	runInit([]string{configPath, "--context-dir", "specs/custom"})

	customContextDir := filepath.Join(tempDir, "specs", "custom")
	if info, err := os.Stat(customContextDir); err != nil || !info.IsDir() {
		t.Fatalf("expected custom context directory at %s when flag follows positional arg, got: %v", customContextDir, err)
	}
}
