package context

import (
	"strings"
	"testing"

	"github.com/vubon/prv/internal/models"
)

func TestApplyDiffBudget_UnlimitedOrWithinBudget(t *testing.T) {
	files := []models.FileChange{
		{Path: "internal/service/user.go", DiffHunk: "+line1\n+line2\n"},
		{Path: "internal/service/user_test.go", DiffHunk: "+test1\n+test2\n"},
	}

	// Case 1: maxLines = 0 (unlimited)
	out, truncated := ApplyDiffBudget(files, 0)
	if truncated != 0 {
		t.Fatalf("expected 0 truncated files for maxLines=0, got %d", truncated)
	}
	if len(out) != 2 || out[0].IsTruncated || out[1].IsTruncated {
		t.Errorf("expected no truncation when maxLines=0")
	}

	// Case 2: total lines (4) <= maxLines (10)
	out, truncated = ApplyDiffBudget(files, 10)
	if truncated != 0 {
		t.Fatalf("expected 0 truncated files when within budget, got %d", truncated)
	}
	if out[0].DiffHunk != files[0].DiffHunk || out[1].DiffHunk != files[1].DiffHunk {
		t.Errorf("expected diff hunks unchanged when within budget")
	}
}

func TestApplyDiffBudget_RiskTieredPrioritization(t *testing.T) {
	// Put low-priority test and doc files FIRST in the slice, standard file SECOND,
	// and sensitive file LAST to verify that:
	// 1. Sensitive file (Tier 1) gets budget first
	// 2. Standard file (Tier 2) gets budget second
	// 3. Test/Doc files (Tier 3) are truncated/omitted first
	// 4. Original slice ordering is preserved!
	files := []models.FileChange{
		{
			Path:        "README.md",
			Additions:   4,
			DiffHunk:    "+doc1\n+doc2\n+doc3\n+doc4\n",
			IsSensitive: false,
		},
		{
			Path:        "internal/api/handler_test.go",
			Additions:   4,
			DiffHunk:    "+test1\n+test2\n+test3\n+test4\n",
			IsSensitive: false,
		},
		{
			Path:        "internal/api/handler.go",
			Additions:   3,
			DiffHunk:    "+code1\n+code2\n+code3\n",
			IsSensitive: false,
		},
		{
			Path:        "internal/auth/jwt.go",
			Additions:   3,
			DiffHunk:    "+sec1\n+sec2\n+sec3\n",
			IsSensitive: true,
		},
	}

	// Budget = 8 lines:
	// - Tier 1 (internal/auth/jwt.go, 3 lines) -> kept in full (remaining = 5)
	// - Tier 2 (internal/api/handler.go, 3 lines) -> kept in full (remaining = 2)
	// - Tier 3 first item (README.md, 4 lines) -> partially truncated to 2 lines (remaining = 0)
	// - Tier 3 second item (internal/api/handler_test.go, 4 lines) -> omitted (0 lines remaining)
	out, truncated := ApplyDiffBudget(files, 8)
	if truncated != 2 {
		t.Fatalf("expected 2 truncated files, got %d", truncated)
	}

	// Verify original file order is preserved
	if out[0].Path != "README.md" || out[1].Path != "internal/api/handler_test.go" ||
		out[2].Path != "internal/api/handler.go" || out[3].Path != "internal/auth/jwt.go" {
		t.Fatalf("expected original file order preserved, got: %v, %v, %v, %v",
			out[0].Path, out[1].Path, out[2].Path, out[3].Path)
	}

	// Tier 1 (Sensitive) must NOT be truncated
	if out[3].IsTruncated || out[3].DiffHunk != files[3].DiffHunk {
		t.Errorf("expected sensitive file internal/auth/jwt.go kept intact, got: %+v", out[3])
	}

	// Tier 2 (Standard code) must NOT be truncated
	if out[2].IsTruncated || out[2].DiffHunk != files[2].DiffHunk {
		t.Errorf("expected standard code file internal/api/handler.go kept intact, got: %+v", out[2])
	}

	// Tier 3 first item (README.md) must be partially truncated (2/4 lines shown)
	if !out[0].IsTruncated {
		t.Errorf("expected README.md to be marked IsTruncated")
	}
	if !strings.Contains(out[0].DiffHunk, "+doc1\n+doc2") || strings.Contains(out[0].DiffHunk, "+doc3") {
		t.Errorf("expected README.md truncated after 2 lines, got:\n%s", out[0].DiffHunk)
	}
	if !strings.Contains(out[0].DiffHunk, "(2/4 lines shown)") {
		t.Errorf("expected truncation notice on README.md, got:\n%s", out[0].DiffHunk)
	}

	// Tier 3 second item (handler_test.go) must be omitted with notice
	if !out[1].IsTruncated {
		t.Errorf("expected handler_test.go to be marked IsTruncated")
	}
	if !strings.Contains(out[1].DiffHunk, "diff omitted due to --max-lines budget") {
		t.Errorf("expected omitted notice on handler_test.go, got:\n%s", out[1].DiffHunk)
	}
}
