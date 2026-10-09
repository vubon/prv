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
