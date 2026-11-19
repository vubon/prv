package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vubon/prv/internal/models"
)

func TestGenerateMarkdownReport_FourBackticksNesting(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle: "Test PR",
		Branch:  "feat/test",
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				Dimension:   "Logic",
				File:        "main.go",
				LineStart:   10,
				Severity:    models.SeverityBlocking,
				Title:       "Nil check",
				Description: "Fix nil pointer",
				Suggestion:  "if x == nil { return }",
			},
		},
	}

	report := GenerateMarkdownReport(res)

	// Outer fence must use 4 backticks so inner 3-backtick ```suggestion doesn't break rendering
	if !strings.Contains(report, "````markdown") {
		t.Errorf("expected 4 backticks ````markdown in comments section, got report:\n%s", report)
	}
	if !strings.Contains(report, "````\n\n") {
		t.Errorf("expected 4 backticks closing fence ````, got report:\n%s", report)
	}

	// Should not have zero date 0001
	if strings.Contains(report, "0001") {
		t.Errorf("report contains uninitialized zero date: %s", report)
	}
}

func TestGenerateMarkdownReport_WeakestLinkAndTradeOffs(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle:     "Advisory Test PR",
		Branch:      "feat/advisory",
		Verdict:     models.VerdictApprove,
		WeakestLink: "Downstream CLI dependency failure",
		TradeOffs:   "Synchronous parsing chosen over async goroutines for simplicity",
	}

	report := GenerateMarkdownReport(res)

	if !strings.Contains(report, "## 🎯 Staff Advisory: Weakest Link & Architectural Trade-offs") {
		t.Error("expected Advisory section header in report")
	}
	if !strings.Contains(report, "Downstream CLI dependency failure") {
		t.Error("expected WeakestLink content in report")
	}
	if !strings.Contains(report, "Synchronous parsing chosen over async goroutines for simplicity") {
		t.Error("expected TradeOffs content in report")
	}
}

func TestSaveReportToFile_ZeroDateAndDefaultBranch(t *testing.T) {
	tempDir := t.TempDir()

	res := &models.ReviewResult{
		PRTitle: "Omitted Date PR",
		Branch:  "", // empty branch
		// ReviewedAt omitted (zero value)
	}

	savedPath, err := SaveReportToFile(res, tempDir)
	if err != nil {
		t.Fatalf("SaveReportToFile failed: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	expectedFilename := today + "-review.md"

	if filepath.Base(savedPath) != expectedFilename {
		t.Errorf("expected filename %q, got %q", expectedFilename, filepath.Base(savedPath))
	}

	data, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("failed to read saved report: %v", err)
	}

	if strings.Contains(string(data), "0001-01-01") {
		t.Errorf("saved report contains 0001-01-01 date")
	}
}

func TestGenerateMarkdownReport_ArchitectureDiagram(t *testing.T) {
	diag := "graph LR\n    cmd_prv --> internal_git\n"
	res := &models.ReviewResult{
		PRTitle:             "Diagram Test PR",
		Branch:              "feat/diagram",
		Verdict:             models.VerdictApprove,
		ArchitectureDiagram: diag,
	}

	report := GenerateMarkdownReport(res)

	if !strings.Contains(report, "## 🗺️ Architecture & Component Impact Diagram") {
		t.Error("expected Architecture Diagram section in report")
	}
	if !strings.Contains(report, "```mermaid\n"+diag+"```") {
		t.Errorf("expected mermaid code block in report, got:\n%s", report)
	}
}

func TestGenerateMarkdownReport_FileWalkthroughs(t *testing.T) {
	walkthroughs := []models.FileWalkthrough{
		{
			File:      "internal/output/walkthrough.go",
			Additions: 40,
			Deletions: 5,
			Topic:     "Walkthrough Engine",
			Changes: []string{
				"Implemented FormatWalkthroughMarkdown",
			},
		},
	}

	resWithWalkthrough := &models.ReviewResult{
		PRTitle:          "Walkthrough Test PR",
		Branch:           "feat/walkthrough",
		Verdict:          models.VerdictApprove,
		FileWalkthroughs: walkthroughs,
	}

	report := GenerateMarkdownReport(resWithWalkthrough)
	if !strings.Contains(report, "## 🔍 File-by-File Changes Walkthrough") {
		t.Errorf("expected File-by-File Changes Walkthrough section in report")
	}
	if !strings.Contains(report, "<details>") || !strings.Contains(report, "internal/output/walkthrough.go") {
		t.Errorf("expected details block with file in report")
	}

	// Without walkthroughs, section header should not appear
	resWithout := &models.ReviewResult{
		PRTitle: "No Walkthrough PR",
		Branch:  "feat/empty",
		Verdict: models.VerdictApprove,
	}
	reportWithout := GenerateMarkdownReport(resWithout)
	if strings.Contains(reportWithout, "## 🔍 File-by-File Changes Walkthrough") {
		t.Errorf("expected report without walkthroughs to omit section header")
	}
}

func TestBuildReviewPayload(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle: "Review Payload Test",
		Branch:  "feat/test",
		Verdict: models.VerdictApprove,
		Issues: []models.Issue{
			{
				Dimension: "Security",
				File:      "auth.go",
				LineStart: 10,
				Severity:  models.SeverityBlocking,
				Title:     "Token leak",
			},
		},
	}

	payload := BuildReviewPayload(res)
	if payload.Event != "APPROVE" {
		t.Errorf("expected event APPROVE, got %s", payload.Event)
	}
	if len(payload.Comments) != 1 {
		t.Fatalf("expected 1 inline comment, got %d", len(payload.Comments))
	}
	if payload.Comments[0].Path != "auth.go" || payload.Comments[0].Line != 10 {
		t.Errorf("unexpected comment details: %+v", payload.Comments[0])
	}
	if !strings.Contains(payload.Body, "Review Payload Test") {
		t.Error("expected PR title in payload body")
	}

	// Test REQUEST_CHANGES event mapping
	res.Verdict = models.VerdictRequestChanges
	payloadChanges := BuildReviewPayload(res)
	if payloadChanges.Event != "REQUEST_CHANGES" {
		t.Errorf("expected event REQUEST_CHANGES, got %s", payloadChanges.Event)
	}

	// Test default COMMENT event mapping
	res.Verdict = "UNKNOWN"
	payloadComment := BuildReviewPayload(res)
	if payloadComment.Event != "COMMENT" {
		t.Errorf("expected default event COMMENT, got %s", payloadComment.Event)
	}
}

func TestGenerateMarkdownReport_MultiLineAnchor(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle: "MultiLine Anchor PR",
		Branch:  "feat/anchor",
		Verdict: models.VerdictApprove,
		Suggestions: []models.Suggestion{
			{
				Dimension:   "Observability",
				Subject:     "error-diagnostics",
				File:        "internal/git/github.go",
				LineStart:   384,
				LineEnd:     390,
				Title:       "Surface fallback diagnostic",
				Description: "Print to stderr",
				Suggestion:  "fmt.Fprintf(os.Stderr, ...)",
			},
		},
	}

	report := GenerateMarkdownReport(res)
	if !strings.Contains(report, "### 1. Surface fallback diagnostic `internal/git/github.go#L384-L390`") {
		t.Errorf("expected multi-line anchor in Suggestions section, got:\n%s", report)
	}
	if !strings.Contains(report, "#### `internal/git/github.go#L384-L390` (select lines 384–390 on diff)") {
		t.Errorf("expected multi-line anchor in Ready-to-Copy comments section, got:\n%s", report)
	}
}

func TestGenerateMarkdownReport_EvidenceRendering(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle: "Evidence Render PR",
		Branch:  "feat/evidence",
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				Dimension:   "Logic",
				File:        "client.go",
				LineStart:   42,
				Severity:    models.SeverityBlocking,
				Title:       "Dropped timeout",
				Description: "HTTP client timeout removed.",
				Evidence:    "Diff removes `Timeout: 10 * time.Second`.",
			},
			{
				Dimension:   "Logic",
				File:        "parser.go",
				LineStart:   18,
				Severity:    models.SeverityBlocking,
				Title:       "Bold syntax line",
				Description: "Single-line raw diff must stay literal.",
				Evidence:    "+ **value**",
			},
			{
				Dimension:   "Logic",
				File:        "other.go",
				LineStart:   9,
				Severity:    models.SeverityMinor,
				Title:       "Minor note",
				Description: "No evidence provided.",
			},
		},
	}

	report := GenerateMarkdownReport(res)
	if !strings.Contains(report, "**Evidence:**\n\n```text\nDiff removes `Timeout: 10 * time.Second`.\n```") {
		t.Errorf("expected single-line raw Evidence rendered in literal text fence in Markdown report, got:\n%s", report)
	}
	if !strings.Contains(report, "**Evidence:**\n\n```text\n+ **value**\n```") {
		t.Errorf("expected single-line Markdown-like diff line '+ **value**' fenced as literal text, got:\n%s", report)
	}
	if strings.Count(report, "**Evidence:**") != 4 { // 2 in Findings + 2 in Ready-to-Copy Comments
		t.Errorf("expected Evidence rendered 4 times (2 in Findings + 2 in Comments), got:\n%s", report)
	}
}

func TestSaveReportToFile_WritesJSONAndHandlesCollisions(t *testing.T) {
	tempDir := t.TempDir()
	fixedTime := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)

	res1 := &models.ReviewResult{
		PRTitle:    "First Run",
		Branch:     "feat/evidence",
		BaseBranch: "main",
		Verdict:    models.VerdictComment,
		ReviewedAt: fixedTime,
		Issues: []models.Issue{
			{
				Dimension: "Logic",
				File:      "a.go",
				LineStart: 1,
				Severity:  models.SeverityBlocking,
				Title:     "First issue",
				Evidence:  "quoted diff line",
			},
		},
	}
	md1, err := SaveReportToFile(res1, tempDir)
	if err != nil {
		t.Fatalf("first SaveReportToFile failed: %v", err)
	}
	if filepath.Base(md1) != "2026-11-19-feat-evidence.md" {
		t.Fatalf("expected 2026-11-19-feat-evidence.md, got %s", filepath.Base(md1))
	}
	json1 := filepath.Join(tempDir, "2026-11-19-feat-evidence.json")
	if _, err := os.Stat(json1); err != nil {
		t.Fatalf("expected sibling JSON file %s: %v", json1, err)
	}

	// Second run on the same date & branch must use -2 without clobbering the first
	res2 := &models.ReviewResult{
		PRTitle:    "Second Run",
		Branch:     "feat/evidence",
		BaseBranch: "main",
		Verdict:    models.VerdictApprove,
		ReviewedAt: fixedTime,
	}
	md2, err := SaveReportToFile(res2, tempDir)
	if err != nil {
		t.Fatalf("second SaveReportToFile failed: %v", err)
	}
	if filepath.Base(md2) != "2026-11-19-feat-evidence-2.md" {
		t.Fatalf("expected 2026-11-19-feat-evidence-2.md, got %s", filepath.Base(md2))
	}
	json2 := filepath.Join(tempDir, "2026-11-19-feat-evidence-2.json")
	if _, err := os.Stat(json2); err != nil {
		t.Fatalf("expected sibling JSON file %s: %v", json2, err)
	}

	// Verify first JSON was NOT overwritten
	firstJSONBytes, err := os.ReadFile(json1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(firstJSONBytes), "First Run") || !strings.Contains(string(firstJSONBytes), `"evidence": "quoted diff line"`) {
		t.Errorf("first JSON was clobbered or missing evidence: %s", string(firstJSONBytes))
	}

	// Third run when only -3 is free must increment to -3
	res3 := &models.ReviewResult{
		PRTitle:    "Third Run",
		Branch:     "feat/evidence",
		BaseBranch: "main",
		Verdict:    models.VerdictApprove,
		ReviewedAt: fixedTime,
	}
	md3, err := SaveReportToFile(res3, tempDir)
	if err != nil {
		t.Fatalf("third SaveReportToFile failed: %v", err)
	}
	if filepath.Base(md3) != "2026-11-19-feat-evidence-3.md" {
		t.Fatalf("expected 2026-11-19-feat-evidence-3.md, got %s", filepath.Base(md3))
	}
}

func TestGenerateMarkdownReport_MultilineEvidenceFencing(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle: "Multiline Evidence PR",
		Branch:  "feat/ev-fence",
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				Dimension:   "Logic",
				File:        "client.go",
				LineStart:   10,
				Severity:    models.SeverityBlocking,
				Title:       "Raw diff excerpt",
				Description: "Timeout deleted.",
				Evidence:    "- Timeout: 10 * time.Second\n+ Client{}",
			},
			{
				Dimension:   "Logic",
				File:        "auth.go",
				LineStart:   20,
				Severity:    models.SeverityBlocking,
				Title:       "Already fenced excerpt",
				Description: "Guard removed.",
				Evidence:    "```diff\n- if !authed {\n-     return\n- }\n```",
			},
		},
	}

	report := GenerateMarkdownReport(res)
	if !strings.Contains(report, "**Evidence:**\n\n```text\n- Timeout: 10 * time.Second\n+ Client{}\n```") {
		t.Errorf("expected raw multiline evidence wrapped in ```text literal fence, got:\n%s", report)
	}
	if !strings.Contains(report, "**Evidence:**\n\n```diff\n- if !authed {\n-     return\n- }\n```") {
		t.Errorf("expected pre-fenced evidence preserved without double-fencing, got:\n%s", report)
	}
	if strings.Contains(report, "```text\n```diff") {
		t.Errorf("pre-fenced evidence must not be double-fenced:\n%s", report)
	}

	// Mixed excerpt (two fenced traces surrounding raw diff lines) + indented raw trace
	resMixed := &models.ReviewResult{
		PRTitle: "Mixed & Indented Evidence",
		Branch:  "feat/ev-mixed",
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				Dimension:   "Logic",
				File:        "handler.go",
				LineStart:   12,
				Severity:    models.SeverityBlocking,
				Title:       "Mixed fenced/raw trace",
				Description: "Middle lines must remain literal.",
				Evidence:    "```go\nfoo()\n```\n- removed line\n+ added line\n```go\nbar()\n```",
			},
			{
				Dimension:   "Logic",
				File:        "worker.go",
				LineStart:   33,
				Severity:    models.SeverityBlocking,
				Title:       "Indented trace",
				Description: "Leading indentation preserved.",
				Evidence:    "    if err != nil {\n        return err\n    }",
			},
		},
	}
	mixedReport := GenerateMarkdownReport(resMixed)
	if !strings.Contains(mixedReport, "**Evidence:**\n\n````text\n```go\nfoo()\n```\n- removed line\n+ added line\n```go\nbar()\n```\n````") {
		t.Errorf("expected mixed excerpt wrapped completely in ````text fence, got:\n%s", mixedReport)
	}
	if !strings.Contains(mixedReport, "**Evidence:**\n\n```text\n    if err != nil {\n        return err\n    }\n```") {
		t.Errorf("expected indented raw trace to keep leading 4 spaces on line 1, got:\n%s", mixedReport)
	}
}

func TestSaveReportToFile_BoundsFilenameBytesWithJSONAndSuffix(t *testing.T) {
	tempDir := t.TempDir()
	fixedTime := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)

	// 240-byte ASCII branch name exceeds 255 bytes once date prefix and .json extension (or -2 suffix) are added.
	longASCII := strings.Repeat("a", 240)
	for run := 1; run <= 3; run++ {
		res := &models.ReviewResult{
			PRTitle:    "Long Branch Test",
			Branch:     longASCII,
			BaseBranch: "main",
			Verdict:    models.VerdictApprove,
			ReviewedAt: fixedTime,
		}
		mdPath, err := SaveReportToFile(res, tempDir)
		if err != nil {
			t.Fatalf("run %d SaveReportToFile failed for 240-byte branch: %v", run, err)
		}
		mdBase := filepath.Base(mdPath)
		jsonBase := strings.TrimSuffix(mdBase, ".md") + ".json"
		if len(mdBase) > 255 || len(jsonBase) > 255 {
			t.Errorf("run %d filename exceeded 255 bytes: md=%d json=%d", run, len(mdBase), len(jsonBase))
		}
		if _, statErr := os.Stat(filepath.Join(tempDir, jsonBase)); statErr != nil {
			t.Errorf("run %d missing sibling JSON %s: %v", run, jsonBase, statErr)
		}
	}

	// Also verify multi-byte UTF-8 branch truncation never splits a rune.
	longUTF8 := strings.Repeat("é", 130) // 260 bytes
	resUTF8 := &models.ReviewResult{
		PRTitle:    "Long UTF8 Branch Test",
		Branch:     longUTF8,
		BaseBranch: "main",
		Verdict:    models.VerdictApprove,
		ReviewedAt: fixedTime,
	}
	mdUTF8, err := SaveReportToFile(resUTF8, tempDir)
	if err != nil {
		t.Fatalf("UTF8 SaveReportToFile failed: %v", err)
	}
	jsonUTF8 := strings.TrimSuffix(filepath.Base(mdUTF8), ".md") + ".json"
	if len(jsonUTF8) > 255 {
		t.Errorf("UTF8 json filename exceeded 255 bytes: %d", len(jsonUTF8))
	}
}
