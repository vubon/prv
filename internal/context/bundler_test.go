package context

import (
	"strings"
	"testing"

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
