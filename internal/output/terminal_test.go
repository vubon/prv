package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vubon/prv/internal/models"
)

// captureStdout captures stdout printed during execution of fn.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintBundleSummary(t *testing.T) {
	ctx := &models.PRContext{
		Metadata: models.PRMetadata{
			Title:      "Feature Auth",
			HeadBranch: "feat/auth",
			BaseBranch: "main",
			Author:     "vubon",
		},
		SummaryStats: models.SummaryStats{
			ReviewedFiles:  2,
			FilteredFiles:  1,
			TotalAdditions: 50,
			TotalDeletions: 10,
		},
		SensitiveFiles:      []string{"internal/auth/jwt.go"},
		CustomRules:         []string{"Rule 1"},
		ArchitectureDiagram: "graph LR\n    a --> b\n",
		Files: []models.FileChange{
			{
				Path:        "internal/auth/jwt.go",
				Additions:   30,
				Deletions:   5,
				IsSensitive: true,
			},
			{
				Path:      "cmd/prv/main.go",
				Additions: 20,
				Deletions: 5,
			},
		},
		IgnoredFiles: []string{"go.sum (lockfile)"},
		BundledAt:    time.Now(),
	}

	out := captureStdout(func() {
		PrintBundleSummary(ctx)
	})

	if !strings.Contains(out, "PR Review Context Bundle Prepared") {
		t.Error("expected banner in bundle summary")
	}
	if !strings.Contains(out, "Feature Auth") {
		t.Error("expected title in bundle summary")
	}
	if !strings.Contains(out, "internal/auth/jwt.go") {
		t.Error("expected sensitive file path in bundle summary")
	}
	if !strings.Contains(out, "go.sum") {
		t.Error("expected ignored file in bundle summary")
	}
}

func TestPrintReviewSummary(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle:     "Payment PR",
		Branch:      "feat/payment",
		BaseBranch:  "main",
		Verdict:     models.VerdictRequestChanges,
		RiskLevel:   models.RiskHigh,
		Summary:     "Executive payment summary",
		WeakestLink: "Stripe webhook secret validation",
		TradeOffs:   "Async callback vs polling",
		FileWalkthroughs: []models.FileWalkthrough{
			{
				File:        "pkg/payment/stripe.go",
				Additions:   100,
				Deletions:   10,
				IsSensitive: true,
				Topic:       "Payment integration",
			},
		},
		Issues: []models.Issue{
			{
				Dimension: "Security",
				File:      "pkg/payment/stripe.go",
				LineStart: 45,
				Severity:  models.SeverityBlocking,
				Title:     "Missing signature verification",
			},
		},
	}

	out := captureStdout(func() {
		PrintReviewSummary(res)
	})

	if !strings.Contains(out, "REQUEST_CHANGES") {
		t.Error("expected verdict in review summary")
	}
	if !strings.Contains(out, "Weakest Link in Change") {
		t.Error("expected Weakest Link in review summary")
	}
	if !strings.Contains(out, "Architectural Trade-offs") {
		t.Error("expected Trade-offs in review summary")
	}
	if !strings.Contains(out, "File-by-File Changes Walkthrough") {
		t.Error("expected File Walkthrough in review summary")
	}
	if !strings.Contains(out, "pkg/payment/stripe.go") {
		t.Error("expected file in review summary")
	}
}
