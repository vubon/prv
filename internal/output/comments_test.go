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
