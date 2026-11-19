package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vubon/prv/internal/models"
)

func TestValidateEvidence(t *testing.T) {
	if got := ValidateEvidence(nil); len(got) != 0 {
		t.Fatalf("expected 0 violations for nil result, got %d", len(got))
	}

	res := &models.ReviewResult{
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				File:      "client.go",
				LineStart: 42,
				Severity:  models.SeverityBlocking,
				Title:     "Timeout deleted",
				Evidence:  "- Timeout: 10 * time.Second",
			},
			{
				File:      "auth.go",
				LineStart: 12,
				Severity:  models.SeverityBlocking,
				Title:     "Missing check",
				Evidence:  "",
			},
			{
				File:      "db.go",
				LineStart: 99,
				Severity:  "blocking",
				Title:     "Whitespace-only evidence",
				Evidence:  "   \t\n  ",
			},
			{
				File:      "handler.go",
				LineStart: 5,
				Severity:  models.SeverityMajor,
				Title:     "Missing %w wrap",
				Evidence:  "",
			},
			{
				File:      "util.go",
				LineStart: 7,
				Severity:  models.SeverityMinor,
				Title:     "Minor naming",
			},
			{
				File:      "fmt.go",
				LineStart: 1,
				Severity:  models.SeverityNit,
				Title:     "Nit",
			},
		},
	}

	violations := ValidateEvidence(res)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %+v", len(violations), violations)
	}
	if violations[0].File != "auth.go" || violations[0].Line != 12 || violations[0].Index != 1 {
		t.Errorf("unexpected violation[0]: %+v", violations[0])
	}
	if violations[1].File != "db.go" || violations[1].Line != 99 || violations[1].Index != 2 {
		t.Errorf("unexpected violation[1]: %+v", violations[1])
	}
}

func TestApplyEvidencePolicy_DefaultWarnsWithoutMutation(t *testing.T) {
	res := &models.ReviewResult{
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				File:      "a.go",
				LineStart: 10,
				Severity:  models.SeverityBlocking,
				Title:     "Unproven blocker",
			},
		},
	}

	violations, err := ApplyEvidencePolicy(res, false)
	if err != nil {
		t.Fatalf("expected nil error in non-strict mode, got: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if res.Issues[0].Severity != models.SeverityBlocking {
		t.Errorf("expected severity NOT to be mutated in default warn mode, got %q", res.Issues[0].Severity)
	}
	if res.Verdict != models.VerdictRequestChanges {
		t.Errorf("expected verdict NOT to be mutated, got %q", res.Verdict)
	}

	warn := FormatEvidenceWarning(violations)
	if !strings.Contains(warn, "1 BLOCKING issue(s) have no evidence (a.go:10)") {
		t.Errorf("unexpected warning message: %q", warn)
	}
	if FormatEvidenceWarning(nil) != "" {
		t.Error("expected empty warning string when there are no violations")
	}
}

func TestApplyEvidencePolicy_StrictReturnsError(t *testing.T) {
	res := &models.ReviewResult{
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				File:      "a.go",
				LineStart: 42,
				Severity:  models.SeverityBlocking,
				Title:     "Blocker 1",
			},
			{
				File:      "b.go",
				LineStart: 7,
				Severity:  models.SeverityBlocking,
				Title:     "Blocker 2",
				Evidence:  "  ",
			},
		},
	}

	violations, err := ApplyEvidencePolicy(res, true)
	if err == nil {
		t.Fatal("expected error in strict mode when BLOCKING issues lack evidence")
	}
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}
	if !strings.Contains(err.Error(), "2 BLOCKING issue(s) have no evidence (a.go:42, b.go:7)") {
		t.Errorf("unexpected strict error message: %v", err)
	}

	// Passing case in strict mode
	res.Issues[0].Evidence = "line 42 deletes nil check"
	res.Issues[1].Evidence = "test output shows panic"
	v2, err2 := ApplyEvidencePolicy(res, true)
	if err2 != nil || len(v2) != 0 {
		t.Fatalf("expected strict mode to pass when all BLOCKING issues have evidence, got violations=%v, err=%v", v2, err2)
	}
}

func TestReviewResult_BackCompatWithoutEvidenceField(t *testing.T) {
	legacyJSON := `{"pr_title":"Old PR","branch":"main","base_branch":"main","verdict":"REQUEST_CHANGES","summary":"old","risk_level":"HIGH","issues":[{"dimension":"Logic","file":"a.go","line_start":3,"severity":"BLOCKING","title":"Bug","description":"Old format"}]}`
	var res models.ReviewResult
	if err := json.Unmarshal([]byte(legacyJSON), &res); err != nil {
		t.Fatalf("expected legacy JSON without evidence field to parse, got: %v", err)
	}
	if len(res.Issues) != 1 || res.Issues[0].Evidence != "" {
		t.Fatalf("expected 1 issue with empty evidence, got: %+v", res.Issues)
	}
}

func TestValidateEvidence_RejectsEmptyFencedBlocks(t *testing.T) {
	emptyFences := []string{
		"```diff\n   \n```",
		"```diff\n\n```",
		"~~~text\n\t \n~~~",
		"```diff\n   ",
		"```go\n```\n```diff\n  \n```",
		"```",
	}

	for _, ev := range emptyFences {
		res := &models.ReviewResult{
			Issues: []models.Issue{
				{
					File:      "a.go",
					LineStart: 10,
					Severity:  models.SeverityBlocking,
					Title:     "Empty fence",
					Evidence:  ev,
				},
			},
		}
		v := ValidateEvidence(res)
		if len(v) != 1 {
			t.Errorf("expected empty fenced block %q to fail evidence validation, got %d violations", ev, len(v))
		}
	}

	// Valid non-empty fenced block passes
	resValid := &models.ReviewResult{
		Issues: []models.Issue{
			{
				File:      "a.go",
				LineStart: 10,
				Severity:  models.SeverityBlocking,
				Title:     "Real diff evidence",
				Evidence:  "```diff\n- Timeout: 10 * time.Second\n```",
			},
		},
	}
	if v := ValidateEvidence(resValid); len(v) != 0 {
		t.Errorf("expected non-empty fenced block to pass, got %+v", v)
	}
}

func TestFormatEvidenceSection_ClosingFenceWhitespace(t *testing.T) {
	const body = "```diff\n- old line\n```"
	if rendered := formatEvidenceSection(body + " \t"); !strings.Contains(rendered, "\n\n"+body+"\n\n") {
		t.Fatalf("expected a closing fence with spaces and tabs to be preserved, got %q", rendered)
	}
	const withNonbreakingSpace = body + "\u00a0"
	want := "````text\n" + withNonbreakingSpace + "\n````"
	if rendered := formatEvidenceSection(withNonbreakingSpace); !strings.Contains(rendered, want) {
		t.Fatalf("expected a nonbreaking space after the inner fence to need an outer fence, got %q", rendered)
	}
}
