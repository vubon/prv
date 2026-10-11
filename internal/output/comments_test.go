package output

import (
	"strings"
	"testing"

	"github.com/vubon/prv/internal/models"
)

func TestFormatConventionalComment(t *testing.T) {
	issue := models.Issue{
		Dimension:   "Security",
		File:        "internal/auth/token.go",
		LineStart:   45,
		Severity:    models.SeverityBlocking,
		Title:       "Potential SQL Injection",
		Description: "Raw user input is concatenated into SQL query string.",
		Suggestion:  "db.QueryRowContext(ctx, \"SELECT * FROM users WHERE id = $1\", userID)",
	}

	formatted := FormatConventionalComment(issue)

	if !strings.Contains(formatted, "**security (security):** Potential SQL Injection") {
		t.Errorf("expected security header in comment, got: %s", formatted)
	}

	if !strings.Contains(formatted, "```suggestion") {
		t.Errorf("expected suggestion block, got: %s", formatted)
	}

	if !strings.Contains(formatted, "db.QueryRowContext") {
		t.Errorf("expected suggested code replacement, got: %s", formatted)
	}
}

func TestFormatConventionalComment_SubjectAndLabels(t *testing.T) {
	tests := []struct {
		name      string
		issue     models.Issue
		wantLabel string
		wantSubj  string
	}{
		{
			name: "blocking with explicit subject",
			issue: models.Issue{
				Dimension: "Logic",
				Subject:   "nil-check",
				Severity:  models.SeverityBlocking,
				Title:     "Null pointer dereference",
			},
			wantLabel: "🚨 **blocking",
			wantSubj:  "(nil-check):** Null pointer dereference",
		},
		{
			name: "question severity",
			issue: models.Issue{
				Dimension: "Architecture",
				Subject:   "context",
				Severity:  "QUESTION",
				Title:     "Why override timeout?",
			},
			wantLabel: "❓ **question",
			wantSubj:  "(context):** Why override timeout?",
		},
		{
			name: "praise severity",
			issue: models.Issue{
				Dimension: "Clean Code",
				Subject:   "design",
				Severity:  "PRAISE",
				Title:     "Great state machine refactor",
			},
			wantLabel: "👏 **praise",
			wantSubj:  "(design):** Great state machine refactor",
		},
		{
			name: "nit severity",
			issue: models.Issue{
				Dimension: "Style",
				Severity:  models.SeverityNit,
				Title:     "Trailing whitespace",
			},
			wantLabel: "🔍 **nit",
			wantSubj:  "(style):** Trailing whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := FormatConventionalComment(tt.issue)
			if !strings.Contains(formatted, tt.wantLabel) {
				t.Errorf("expected label %q in %s", tt.wantLabel, formatted)
			}
			if !strings.Contains(formatted, tt.wantSubj) {
				t.Errorf("expected subject %q in %s", tt.wantSubj, formatted)
			}
		})
	}
}

func TestGenerateInlineComments(t *testing.T) {
	res := &models.ReviewResult{
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				Dimension:   "Security",
				Subject:     "sqli",
				File:        "pkg/db/query.go",
				LineStart:   42,
				Severity:    models.SeverityBlocking,
				Title:       "SQL Injection",
				Description: "Fix raw string concat",
				Suggestion:  "db.Query(ctx, stmt, id)",
			},
			{
				Dimension: "General",
				File:      "", // No file, should be skipped
				LineStart: 10,
			},
			{
				Dimension: "General",
				File:      "pkg/db/query.go",
				LineStart: 0, // Non-positive line, should be skipped
			},
		},
		Suggestions: []models.Suggestion{
			{
				Dimension:   "Style",
				Subject:     "naming",
				File:        "pkg/util/str.go",
				LineStart:   15,
				Title:       "Rename var",
				Description: "More descriptive name",
				Suggestion:  "userName := name",
			},
		},
	}

	inline := GenerateInlineComments(res)
	if len(inline) != 2 {
		t.Fatalf("expected 2 inline comments, got %d", len(inline))
	}

	if inline[0].Path != "pkg/db/query.go" || inline[0].Line != 42 {
		t.Errorf("unexpected comment 0: %+v", inline[0])
	}
	if !strings.Contains(inline[0].Body, "🔒 **security (sqli):**") {
		t.Errorf("expected conventional comment in body, got: %s", inline[0].Body)
	}

	if inline[1].Path != "pkg/util/str.go" || inline[1].Line != 15 {
		t.Errorf("unexpected comment 1: %+v", inline[1])
	}
	if !strings.Contains(inline[1].Body, "💡 **suggestion (naming):**") {
		t.Errorf("expected conventional comment in suggestion body, got: %s", inline[1].Body)
	}
}

func TestGenerateInlineComments_MultiLine(t *testing.T) {
	res := &models.ReviewResult{
		Verdict: models.VerdictRequestChanges,
		Issues: []models.Issue{
			{
				Dimension:   "Logic",
				Subject:     "error-handling",
				File:        "pkg/client/http.go",
				LineStart:   384,
				LineEnd:     390,
				Severity:    models.SeverityMajor,
				Title:       "Missing diagnostic log",
				Description: "Surface error before retrying",
			},
		},
	}

	inline := GenerateInlineComments(res)
	if len(inline) != 1 {
		t.Fatalf("expected 1 inline comment, got %d", len(inline))
	}

	// For GitHub API multi-line comments: Line is EndLine (390), StartLine is LineStart (384)
	if inline[0].Line != 390 || inline[0].StartLine != 384 {
		t.Errorf("expected Line=390, StartLine=384, got Line=%d, StartLine=%d", inline[0].Line, inline[0].StartLine)
	}

	comments := GenerateCommentList(res)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Line != 384 || comments[0].LineEnd != 390 {
		t.Errorf("expected Line=384, LineEnd=390, got Line=%d, LineEnd=%d", comments[0].Line, comments[0].LineEnd)
	}
}
