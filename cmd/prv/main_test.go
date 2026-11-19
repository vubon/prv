package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/vubon/prv/internal/config"
	"github.com/vubon/prv/internal/models"
)

func TestMain(m *testing.M) {
	executable := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
	if log := os.Getenv("PRV_TEST_POST_LOG"); executable == "gh" && log != "" {
		os.Exit(runReportGitHubCLIFake(os.Args[1:], log))
	}
	os.Exit(m.Run())
}

func runReportGitHubCLIFake(args []string, postLog string) int {
	if len(args) >= 3 && args[0] == "pr" && args[1] == "view" {
		metadata := os.Getenv("PRV_TEST_PR_METADATA")
		var pr models.PRMetadata
		if err := json.Unmarshal([]byte(metadata), &pr); err != nil {
			return 1
		}
		if args[2] != strconv.Itoa(pr.Number) && args[2] != pr.URL {
			return 1
		}
		_, _ = io.WriteString(os.Stdout, metadata)
		return 0
	}
	if len(args) == 0 || (args[0] != "api" && (len(args) < 2 || args[0] != "pr" || args[1] != "comment")) {
		return 1
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		return 1
	}
	f, err := os.OpenFile(postLog, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return 1
	}
	_, writeErr := io.WriteString(f, strings.Join(args, " ")+"\n")
	if err := f.Close(); err != nil || writeErr != nil {
		return 1
	}
	return 0
}

func mockReportGitHubCLI(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	name := "gh"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(binDir, name), binary, 0755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(binDir, "posted")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PRV_TEST_POST_LOG", log)
	t.Setenv("PRV_TEST_PR_METADATA", "")
	return log
}

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

func TestBundleFlags_DirtyAndMaxLines(t *testing.T) {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	var dirty bool
	var maxLines int
	fs.BoolVar(&dirty, "dirty", false, "")
	fs.BoolVar(&dirty, "w", false, "")
	fs.IntVar(&maxLines, "max-lines", 0, "")

	pos := parseFlagsAndPositionals(fs, []string{"main", "-w", "--max-lines", "1500"})
	if !dirty {
		t.Errorf("expected dirty=true from -w flag")
	}
	if maxLines != 1500 {
		t.Errorf("expected maxLines=1500, got %d", maxLines)
	}
	if len(pos) != 1 || pos[0] != "main" {
		t.Errorf("expected positional 'main', got %+v", pos)
	}
}

func TestExecuteReport_EvidencePolicyModes(t *testing.T) {
	tempDir := t.TempDir()
	reviewJSONPath := filepath.Join(tempDir, "input.json")
	outDefault := filepath.Join(tempDir, "out-default")
	outStrictFlag := filepath.Join(tempDir, "out-strict-flag")
	outStrictCfg := filepath.Join(tempDir, "out-strict-cfg")

	input := `{
  "pr_title": "Evidence CLI Test",
  "branch": "feat/ev",
  "base_branch": "main",
  "verdict": "REQUEST_CHANGES",
  "summary": "Found an unproven blocker",
  "risk_level": "HIGH",
  "issues": [
    {
      "dimension": "Logic",
      "file": "a.go",
      "line_start": 42,
      "severity": "BLOCKING",
      "title": "Unproven blocker",
      "description": "Missing evidence field"
    }
  ]
}`
	if err := os.WriteFile(reviewJSONPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Default mode: warns without mutating severity, writes both .md and .json
	var stdout, stderr bytes.Buffer
	err := executeReport([]string{"--file", reviewJSONPath, "--dir", outDefault, "--repo", tempDir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected default mode to succeed with warning, got err: %v (stderr: %s)", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "1 BLOCKING issue(s) have no evidence (a.go:42)") {
		t.Errorf("expected evidence warning on stderr, got: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), ".md") || !strings.Contains(stdout.String(), ".json") {
		t.Errorf("expected stdout to report both .md and .json saved paths, got: %q", stdout.String())
	}
	entries, err := os.ReadDir(outDefault)
	if err != nil || len(entries) != 2 {
		t.Fatalf("expected 2 output files (.md + .json), got %d (err=%v)", len(entries), err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			savedJSON, readErr := os.ReadFile(filepath.Join(outDefault, e.Name()))
			if readErr != nil {
				t.Fatal(readErr)
			}
			var savedRes models.ReviewResult
			if uErr := json.Unmarshal(savedJSON, &savedRes); uErr != nil {
				t.Fatal(uErr)
			}
			if len(savedRes.Issues) != 1 || savedRes.Issues[0].Severity != models.SeverityBlocking {
				t.Errorf("expected saved JSON severity to stay BLOCKING, got: %+v", savedRes.Issues)
			}
		}
	}

	// 2. --strict-evidence flag: exits non-zero and writes nothing
	stdout.Reset()
	stderr.Reset()
	errStrict := executeReport([]string{"--file", reviewJSONPath, "--dir", outStrictFlag, "--repo", tempDir, "--strict-evidence"}, &stdout, &stderr)
	if errStrict == nil {
		t.Fatal("expected error under --strict-evidence when BLOCKING issue lacks evidence")
	}
	if _, statErr := os.Stat(outStrictFlag); !os.IsNotExist(statErr) {
		t.Errorf("expected zero files written in strict mode, but %s exists", outStrictFlag)
	}

	// 3. Config review.require_evidence: true honored without CLI flag
	cfgPath := filepath.Join(tempDir, ".prv.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: \"1\"\nreview:\n  require_evidence: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	errCfg := executeReport([]string{"--file", reviewJSONPath, "--dir", outStrictCfg, "--repo", tempDir}, &stdout, &stderr)
	if errCfg == nil {
		t.Fatal("expected error when .prv.yaml sets require_evidence: true")
	}
	if _, statErr := os.Stat(outStrictCfg); !os.IsNotExist(statErr) {
		t.Errorf("expected zero files written when require_evidence: true fails, but %s exists", outStrictCfg)
	}
}

func TestExecuteReport_FlagsAfterPositionalPRNumber(t *testing.T) {
	mockReportGitHubCLI(t)
	t.Setenv("PRV_TEST_PR_METADATA", `{"number":42,"url":"https://reviews.example.invalid/team/project/pull/42"}`)
	tempDir := t.TempDir()
	reviewJSONPath := filepath.Join(tempDir, "input.json")
	configPath := filepath.Join(tempDir, ".prv.yaml")
	outDir := filepath.Join(tempDir, "out")
	if err := os.WriteFile(configPath, []byte("review:\n  require_evidence: false\n"), 0644); err != nil {
		t.Fatal(err)
	}

	input := `{
  "pr_number": 42,
  "pr_title": "Post PR Test",
  "branch": "feat/ev",
  "base_branch": "main",
  "verdict": "REQUEST_CHANGES",
  "summary": "Unproven blocker",
  "risk_level": "HIGH",
  "issues": [
    {
      "dimension": "Logic",
      "file": "a.go",
      "line_start": 42,
      "severity": "BLOCKING",
      "title": "Unproven blocker",
      "description": "Missing evidence field"
    }
  ]
}`
	if err := os.WriteFile(reviewJSONPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	// --strict-evidence placed AFTER positional PR number 42 must still be parsed and reject the run.
	err := executeReport([]string{"--file", reviewJSONPath, "--dir", outDir, "--repo", tempDir, "--config", configPath, "--post", "42", "--strict-evidence"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
		t.Fatalf("expected strict evidence error even after positional '42', got: %v", err)
	}
	if _, statErr := os.Stat(outDir); !os.IsNotExist(statErr) {
		t.Errorf("expected zero files written when --strict-evidence fails after positional arg")
	}

	// Non-numeric / non-URL positional argument must be rejected explicitly.
	stdout.Reset()
	stderr.Reset()
	errPos := executeReport([]string{"--file", reviewJSONPath, "--dir", outDir, "--repo", tempDir, "unexpected-arg"}, &stdout, &stderr)
	if errPos == nil || !strings.Contains(errPos.Error(), "unexpected positional argument") {
		t.Fatalf("expected unexpected positional argument error, got: %v", errPos)
	}
}

func mustGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestExecuteReport_SubdirectoryFindsGitRootConfig(t *testing.T) {
	tempDir := t.TempDir()
	mustGitCmd(t, tempDir, "init")
	mustGitCmd(t, tempDir, "config", "user.email", "test@example.com")
	mustGitCmd(t, tempDir, "config", "user.name", "Test User")
	mustGitCmd(t, tempDir, "checkout", "-b", "main")

	// Commit root .prv.yaml with require_evidence: true on trusted base revision.
	if err := os.WriteFile(filepath.Join(tempDir, ".prv.yaml"), []byte("version: \"1\"\nreview:\n  require_evidence: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, tempDir, "add", ".prv.yaml")
	mustGitCmd(t, tempDir, "commit", "-m", "baseline policy")

	subDir := filepath.Join(tempDir, "services", "api")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	reviewJSONPath := filepath.Join(tempDir, "input.json")
	input := `{"pr_title":"Subdir","branch":"feat/sub","base_branch":"main","verdict":"REQUEST_CHANGES","summary":"s","risk_level":"HIGH","issues":[{"dimension":"Logic","file":"a.go","line_start":1,"severity":"BLOCKING","title":"Blocker","description":"d"}]}`
	if err := os.WriteFile(reviewJSONPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	outDir := filepath.Join(tempDir, "out-sub")
	var stdout, stderr bytes.Buffer
	// Invoked implicitly from subdirectory without --repo or --config: must resolve Git root and enforce require_evidence.
	errReport := executeReport([]string{"--file", reviewJSONPath, "--dir", outDir}, &stdout, &stderr)
	if errReport == nil || !strings.Contains(errReport.Error(), "BLOCKING issue(s) have no evidence") {
		t.Fatalf("expected Git root require_evidence to fail when invoked from subdirectory, got: %v", errReport)
	}
	if _, statErr := os.Stat(outDir); !os.IsNotExist(statErr) {
		t.Errorf("expected zero files written from subdirectory when root config enforces evidence")
	}
}

func TestExecuteReport_UsesBaseRevisionPolicyInsteadOfFeatureBranch(t *testing.T) {
	tempDir := t.TempDir()
	mustGitCmd(t, tempDir, "init")
	mustGitCmd(t, tempDir, "config", "user.email", "test@example.com")
	mustGitCmd(t, tempDir, "config", "user.name", "Test User")
	mustGitCmd(t, tempDir, "checkout", "-b", "main")

	// Trusted base revision sets require_evidence: true.
	cfgPath := filepath.Join(tempDir, ".prv.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: \"1\"\nreview:\n  require_evidence: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, tempDir, "add", ".prv.yaml")
	mustGitCmd(t, tempDir, "commit", "-m", "main policy requires evidence")

	// Feature branch tries to relax require_evidence to false.
	mustGitCmd(t, tempDir, "checkout", "-b", "feat/relax")
	if err := os.WriteFile(cfgPath, []byte("version: \"1\"\nreview:\n  require_evidence: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, tempDir, "add", ".prv.yaml")
	mustGitCmd(t, tempDir, "commit", "-m", "relax evidence policy on branch")

	reviewJSONPath := filepath.Join(tempDir, "input.json")
	input := `{"pr_title":"Relax Attempt","branch":"feat/relax","base_branch":"main","verdict":"REQUEST_CHANGES","summary":"s","risk_level":"HIGH","issues":[{"dimension":"Logic","file":"a.go","line_start":10,"severity":"BLOCKING","title":"Unproven","description":"d"}]}`
	if err := os.WriteFile(reviewJSONPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	outStrict := filepath.Join(tempDir, "out-strict")
	var stdout, stderr bytes.Buffer
	// Without --config, policy must come from trusted base branch main (require_evidence: true).
	err := executeReport([]string{"--file", reviewJSONPath, "--dir", outStrict, "--repo", tempDir}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
		t.Fatalf("expected trusted base branch require_evidence: true to reject unproven blocker, got: %v", err)
	}
	if _, statErr := os.Stat(outStrict); !os.IsNotExist(statErr) {
		t.Errorf("expected zero files written when base branch requires evidence")
	}

	// Explicit --config override must still be honored.
	outOverride := filepath.Join(tempDir, "out-override")
	stdout.Reset()
	stderr.Reset()
	errOverride := executeReport([]string{"--file", reviewJSONPath, "--dir", outOverride, "--repo", tempDir, "--config", cfgPath}, &stdout, &stderr)
	if errOverride != nil {
		t.Fatalf("expected explicit --config override to succeed with warning, got: %v", errOverride)
	}
	if !strings.Contains(stderr.String(), "1 BLOCKING issue(s) have no evidence") {
		t.Errorf("expected warning on stderr when explicit --config relaxes policy, got: %q", stderr.String())
	}
}

func TestExecuteReport_DerivesEvidencePolicyFromTargetPR(t *testing.T) {
	tempDir := t.TempDir()
	mustGitCmd(t, tempDir, "init")
	mustGitCmd(t, tempDir, "config", "user.email", "test@example.com")
	mustGitCmd(t, tempDir, "config", "user.name", "Test User")
	mustGitCmd(t, tempDir, "checkout", "-b", "main")
	mustGitCmd(t, tempDir, "remote", "add", "origin", "https://reviews.example.invalid/contributor/project.git")
	mustGitCmd(t, tempDir, "remote", "add", "upstream", "https://reviews.example.invalid/team/project.git")
	mustGitCmd(t, tempDir, "commit", "--allow-empty", "-m", "old base without strict policy")
	mustGitCmd(t, tempDir, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Trusted base commit on main sets require_evidence: true.
	cfgPath := filepath.Join(tempDir, ".prv.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: \"1\"\nreview:\n  require_evidence: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, tempDir, "add", ".prv.yaml")
	mustGitCmd(t, tempDir, "commit", "-m", "main policy requires evidence")
	baseSHA := mustGitCmd(t, tempDir, "rev-parse", "HEAD")

	// Feature branch weakens require_evidence to false.
	mustGitCmd(t, tempDir, "checkout", "-b", "feature")
	if err := os.WriteFile(cfgPath, []byte("version: \"1\"\nreview:\n  require_evidence: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, tempDir, "add", ".prv.yaml")
	mustGitCmd(t, tempDir, "commit", "-m", "feature weakens evidence policy")

	// The configured GitHub repository is upstream, and the exact PR base is newer than the local tracking ref.
	postLog := mockReportGitHubCLI(t)
	const prURL = "https://reviews.example.invalid/team/project/pull/42"
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	prMetadata := func(commit string) string {
		return `{"number":42,"baseRefName":"main","baseRefOid":"` + commit + `","url":"` + prURL + `"}`
	}
	t.Setenv("PRV_TEST_PR_METADATA", prMetadata(baseSHA))

	// review.json falsely claims base_branch is "feature" to try loading feature's weaker config.
	reviewJSONPath := filepath.Join(tempDir, "review.json")
	input := `{"pr_title":"Tampered Base","branch":"feature","base_branch":"feature","verdict":"REQUEST_CHANGES","summary":"s","risk_level":"HIGH","issues":[{"dimension":"Logic","file":"a.go","line_start":10,"severity":"BLOCKING","title":"Unproven","description":"d"}]}`
	if err := os.WriteFile(reviewJSONPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	outPR := filepath.Join(tempDir, "out-pr")
	var stdout, stderr bytes.Buffer
	err := executeReport([]string{"--file", reviewJSONPath, "--dir", outPR, "--repo", tempDir, "--pr", "42"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
		t.Fatalf("expected target PR base branch policy (main: require_evidence=true) to reject unproven blocker even when review.json says base_branch='feature', got: %v", err)
	}
	if _, statErr := os.Stat(outPR); !os.IsNotExist(statErr) {
		t.Errorf("expected zero files written when target PR base branch requires evidence")
	}

	t.Run("explicit nested repo and rewritten upstream", func(t *testing.T) {
		subdir := filepath.Join(tempDir, "services", "api")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatal(err)
		}
		const upstreamURL = "https://reviews.example.invalid/team/project.git"
		mustGitCmd(t, tempDir, "config", "url.https://reviews.example.invalid/team/.insteadOf", "team:")
		mustGitCmd(t, tempDir, "remote", "set-url", "upstream", "team:project.git")
		t.Cleanup(func() { mustGitCmd(t, tempDir, "remote", "set-url", "upstream", upstreamURL) })
		out := filepath.Join(t.TempDir(), "reports")
		err := executeReport([]string{"--file", reviewJSONPath, "--dir", out, "--repo", subdir, "--pr", prURL}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
			t.Fatalf("expected root policy from the rewritten upstream remote, got %v", err)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("expected no saved reports, got %v", err)
		}
	})

	// Explicit --config still overrides target PR policy when specified by reviewer.
	outOverride := filepath.Join(tempDir, "out-pr-override")
	stdout.Reset()
	stderr.Reset()
	errOverride := executeReport([]string{"--file", reviewJSONPath, "--dir", outOverride, "--repo", tempDir, "--pr", "42", "--config", cfgPath}, &stdout, &stderr)
	if errOverride != nil {
		t.Fatalf("expected explicit --config override with --pr 42 to succeed with warning, got: %v", errOverride)
	}
	if !strings.Contains(stderr.String(), "1 BLOCKING issue(s) have no evidence") {
		t.Errorf("expected warning on stderr when explicit --config overrides target PR policy, got: %q", stderr.String())
	}

	for _, tc := range []struct {
		name     string
		metadata string
		wantErr  string
	}{
		{"unavailable metadata", "", "cannot verify the base commit"},
		{"missing SHA", prMetadata(""), "did not provide an exact base commit"},
		{"unavailable SHA", prMetadata(strings.Repeat("f", 40)), "cannot load PR base commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PRV_TEST_PR_METADATA", tc.metadata)
			out := filepath.Join(t.TempDir(), "reports")
			err := executeReport([]string{"--file", reviewJSONPath, "--dir", out, "--repo", tempDir, "--pr", prURL, "--post"}, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "--config") {
				t.Fatalf("expected %q with override guidance, got %v", tc.wantErr, err)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("expected no saved reports, got %v", err)
			}
			if _, err := os.Stat(postLog); !os.IsNotExist(err) {
				t.Fatalf("expected no posting attempt, got %v", err)
			}
		})
	}

	t.Run("fetch missing commit from upstream", func(t *testing.T) {
		upstream := t.TempDir()
		mustGitCmd(t, upstream, "init", "-b", "main")
		mustGitCmd(t, upstream, "config", "user.email", "test@example.com")
		mustGitCmd(t, upstream, "config", "user.name", "Test User")
		if err := os.WriteFile(filepath.Join(upstream, ".prv.yaml"), []byte("review:\n  require_evidence: true\n"), 0644); err != nil {
			t.Fatal(err)
		}
		mustGitCmd(t, upstream, "add", ".prv.yaml")
		mustGitCmd(t, upstream, "commit", "-m", "upstream evidence policy")
		sha := mustGitCmd(t, upstream, "rev-parse", "HEAD")
		probe := exec.Command("git", "cat-file", "-e", sha)
		probe.Dir = tempDir
		if err := probe.Run(); err == nil {
			t.Fatal("fixture PR base already exists in the fork")
		}
		const upstreamURL = "https://reviews.example.invalid/team/project.git"
		mustGitCmd(t, tempDir, "config", "url."+filepath.ToSlash(upstream)+".insteadOf", upstreamURL)
		t.Setenv("PRV_TEST_PR_METADATA", prMetadata(sha))
		out := filepath.Join(t.TempDir(), "reports")
		err := executeReport([]string{"--file", reviewJSONPath, "--dir", out, "--repo", tempDir, "--pr", prURL}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
			t.Fatalf("expected upstream policy to reject unproven blocker, got %v", err)
		}
		mustGitCmd(t, tempDir, "cat-file", "-e", sha)
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("expected no saved reports after applying upstream policy, got %v", err)
		}
	})

	// Both JSON identifiers agree. The policy lookup and the mocked review API must use that PR.
	consistentInput := `{"pr_number":42,"pr_url":"` + prURL + `","branch":"feature","verdict":"REQUEST_CHANGES","issues":[{"file":"a.go","line_start":1,"severity":"BLOCKING","evidence":"- old"}]}`
	if err := os.WriteFile(reviewJSONPath, []byte(consistentInput), 0644); err != nil {
		t.Fatal(err)
	}
	outPost := filepath.Join(tempDir, "out-post")
	err = executeReport([]string{"--file", reviewJSONPath, "--dir", outPost, "--repo", tempDir, "--post"}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("posting review with matching identifiers: %v", err)
	}
	log, err := os.ReadFile(postLog)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(log)); got != "api repos/team/project/pulls/42/reviews --input - --hostname reviews.example.invalid" {
		t.Fatalf("unexpected posting target: %q", got)
	}

	conflictingInput := strings.Replace(consistentInput, prURL, "https://reviews.example.invalid/foreign/project/pull/17", 1)
	if err := os.WriteFile(reviewJSONPath, []byte(conflictingInput), 0644); err != nil {
		t.Fatal(err)
	}
	outConflict := filepath.Join(tempDir, "out-conflict")
	err = executeReport([]string{"--file", reviewJSONPath, "--dir", outConflict, "--repo", tempDir, "--post"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "pr_url and pr_number refer to different PR numbers") {
		t.Fatalf("expected conflicting identifiers to fail, got %v", err)
	}
	if _, err := os.Stat(outConflict); !os.IsNotExist(err) {
		t.Fatalf("expected no saved reports with conflicting identifiers, got %v", err)
	}
	afterConflict, err := os.ReadFile(postLog)
	if err != nil || !bytes.Equal(afterConflict, log) {
		t.Fatalf("unexpected posting attempt with conflicting identifiers: %q (error %v)", afterConflict, err)
	}

	err = executeReport([]string{"--file", reviewJSONPath, "--dir", outConflict, "--repo", tempDir, "--pr", "42", "--post"}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected explicit --pr to take precedence over conflicting JSON identifiers, got %v", err)
	}
	logAfterOverride, err := os.ReadFile(postLog)
	if err != nil || string(logAfterOverride) != string(log)+string(log) {
		t.Fatalf("override should publish to upstream PR 42, got %q (error %v)", logAfterOverride, err)
	}
	saved, err := filepath.Glob(filepath.Join(outConflict, "*.json"))
	if err != nil || len(saved) != 1 {
		t.Fatalf("expected one saved JSON artifact, got %v (error %v)", saved, err)
	}
	data, err := os.ReadFile(saved[0])
	if err != nil {
		t.Fatal(err)
	}
	var savedResult models.ReviewResult
	if err := json.Unmarshal(data, &savedResult); err != nil {
		t.Fatal(err)
	}
	if savedResult.PRURL != prURL || savedResult.PRNumber != 42 {
		t.Fatalf("saved report has incorrect PR target: %s (#%d)", savedResult.PRURL, savedResult.PRNumber)
	}
	if err := executeReport([]string{"--file", saved[0], "--dir", filepath.Join(tempDir, "replay"), "--repo", tempDir}, io.Discard, io.Discard); err != nil {
		t.Fatalf("expected saved report to be reusable without an override, got %v", err)
	}
}

func TestExecuteReport_UsesSelectedBaseFromEitherCheckout(t *testing.T) {
	repo := t.TempDir()
	mustGitCmd(t, repo, "init", "-b", "main")
	mustGitCmd(t, repo, "config", "user.email", "test@example.com")
	mustGitCmd(t, repo, "config", "user.name", "Test User")
	mustGitCmd(t, repo, "commit", "--allow-empty", "-m", "main")
	mustGitCmd(t, repo, "checkout", "-b", "release")
	if err := os.WriteFile(filepath.Join(repo, ".prv.yaml"), []byte("review:\n  require_evidence: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, repo, "add", ".prv.yaml")
	mustGitCmd(t, repo, "commit", "-m", "release requires evidence")
	mustGitCmd(t, repo, "checkout", "-b", "topic")
	mustGitCmd(t, repo, "commit", "--allow-empty", "-m", "topic")

	inputPath := filepath.Join(t.TempDir(), "review.json")
	input := `{"branch":"topic","base_branch":"release","issues":[{"severity":"BLOCKING","title":"Unproven"}]}`
	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}
	for _, branch := range []string{"topic", "release"} {
		mustGitCmd(t, repo, "checkout", branch)
		out := filepath.Join(t.TempDir(), "reports")
		err := executeReport([]string{"--file", inputPath, "--dir", out, "--repo", repo}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
			t.Fatalf("expected release policy from %s checkout, got %v", branch, err)
		}
	}
	mustGitCmd(t, repo, "branch", "-D", "topic")
	out := filepath.Join(t.TempDir(), "reports")
	err := executeReport([]string{"--file", inputPath, "--dir", out, "--repo", repo}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
		t.Fatalf("expected release policy with the reviewed head unavailable, got %v", err)
	}

	mustGitCmd(t, repo, "checkout", "main")
	if err := os.WriteFile(filepath.Join(repo, ".prv.yaml"), []byte("review:\n  default_base_branch: 'invalid branch'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, repo, "add", ".prv.yaml")
	mustGitCmd(t, repo, "commit", "-m", "malformed default base")
	mustGitCmd(t, repo, "checkout", "release")
	for _, base := range []string{"release", "index (staged)", "HEAD (working tree)", ""} {
		report := models.ReviewResult{Branch: "topic", BaseBranch: base, Issues: []models.Issue{{Severity: "BLOCKING"}}}
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(inputPath, data, 0644); err != nil {
			t.Fatal(err)
		}
		wantErr := "BLOCKING issue(s) have no evidence"
		if base == "" {
			mockReportGitHubCLI(t)
			wantErr = "invalid default_base_branch"
		}
		err = executeReport([]string{"--file", inputPath, "--dir", out, "--repo", repo}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("expected %q for base %q, got %v", wantErr, base, err)
		}
	}
}

func TestExecuteReport_FetchesConfiguredDefaultBase(t *testing.T) {
	mockReportGitHubCLI(t)
	origin := t.TempDir()
	mustGitCmd(t, origin, "init", "-b", "main")
	mustGitCmd(t, origin, "config", "user.email", "test@example.com")
	mustGitCmd(t, origin, "config", "user.name", "Test User")
	policy := filepath.Join(origin, ".prv.yaml")
	if err := os.WriteFile(policy, []byte("review:\n  default_base_branch: 'release'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, origin, "add", ".prv.yaml")
	mustGitCmd(t, origin, "commit", "-m", "configure release base")
	mustGitCmd(t, origin, "checkout", "-b", "release")
	if err := os.WriteFile(policy, []byte("review:\n  require_evidence: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, origin, "add", ".prv.yaml")
	mustGitCmd(t, origin, "commit", "-m", "release requires evidence")

	parent := t.TempDir()
	mustGitCmd(t, parent, "clone", "--single-branch", "-b", "main", origin, "checkout")
	repo := filepath.Join(parent, "checkout")
	mustGitCmd(t, repo, "checkout", "-b", "topic")
	inputPath := filepath.Join(parent, "review.json")
	if err := os.WriteFile(inputPath, []byte(`{"branch":"topic","issues":[{"severity":"BLOCKING","title":"Unproven"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "reports")
	err := executeReport([]string{"--file", inputPath, "--dir", out, "--repo", repo}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
		t.Fatalf("expected fetched release policy to reject unproven blocker, got %v", err)
	}
	mustGitCmd(t, repo, "rev-parse", "--verify", "origin/release")
}

func TestExecuteReport_InferredPRMustMatchReviewedBranch(t *testing.T) {
	mockReportGitHubCLI(t)
	repo := t.TempDir()
	mustGitCmd(t, repo, "init", "-b", "main")
	mustGitCmd(t, repo, "config", "user.email", "test@example.com")
	mustGitCmd(t, repo, "config", "user.name", "Test User")
	mustGitCmd(t, repo, "remote", "add", "origin", "https://reviews.example.invalid/team/project.git")
	mustGitCmd(t, repo, "commit", "--allow-empty", "-m", "unrelated PR base")
	weakSHA := mustGitCmd(t, repo, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, ".prv.yaml"), []byte("review:\n  require_evidence: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, repo, "add", ".prv.yaml")
	mustGitCmd(t, repo, "commit", "-m", "main requires evidence")
	mustGitCmd(t, repo, "checkout", "-b", "42")

	t.Setenv("PRV_TEST_PR_METADATA", `{"number":42,"headRefName":"unrelated","baseRefOid":"`+weakSHA+`","url":"https://reviews.example.invalid/team/project/pull/42"}`)
	inputPath := filepath.Join(t.TempDir(), "review.json")
	if err := os.WriteFile(inputPath, []byte(`{"branch":"42","issues":[{"severity":"BLOCKING","title":"Unproven"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "reports")
	err := executeReport([]string{"--file", inputPath, "--dir", out, "--repo", repo}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
		t.Fatalf("expected main policy for branch 42, got %v", err)
	}
}

func TestExecuteReport_NoDefaultBaseUsesHEAD(t *testing.T) {
	mockReportGitHubCLI(t)
	repo := t.TempDir()
	mustGitCmd(t, repo, "init", "-b", "trunk")
	mustGitCmd(t, repo, "config", "user.email", "test@example.com")
	mustGitCmd(t, repo, "config", "user.name", "Test User")

	inputPath := filepath.Join(t.TempDir(), "review.json")
	input := `{"issues":[{"severity":"BLOCKING","title":"Unproven"}]}`
	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "reports")
	if err := executeReport([]string{"--file", inputPath, "--dir", out, "--repo", repo}, io.Discard, io.Discard); err != nil {
		t.Fatalf("expected built-in policy in an empty repository, got %v", err)
	}
	mustGitCmd(t, repo, "commit", "--allow-empty", "-m", "first commit on trunk")
	if err := executeReport([]string{"--file", inputPath, "--dir", out, "--repo", repo}, io.Discard, io.Discard); err != nil {
		t.Fatalf("expected report from trunk when no default ref exists, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".prv.yaml"), []byte("review:\n  require_evidence: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGitCmd(t, repo, "add", ".prv.yaml")
	mustGitCmd(t, repo, "commit", "-m", "trunk policy")
	err := executeReport([]string{"--file", inputPath, "--dir", out, "--repo", repo}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "BLOCKING issue(s) have no evidence") {
		t.Fatalf("expected report to use the committed policy on trunk, got %v", err)
	}
}

func TestRunReport_HelpExitsZero(t *testing.T) {
	if os.Getenv("PRV_TEST_REPORT_HELP") == "1" {
		runReport([]string{"--help"})
		return
	}

	var stdout, stderr bytes.Buffer
	err := executeReport([]string{"--help"}, &stdout, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp from executeReport --help, got: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRunReport_HelpExitsZero")
	cmd.Env = append(os.Environ(), "PRV_TEST_REPORT_HELP=1")
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		t.Fatalf("expected prv report --help to exit with status 0, got %v (output: %s)", runErr, string(out))
	}
}
