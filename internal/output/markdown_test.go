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
