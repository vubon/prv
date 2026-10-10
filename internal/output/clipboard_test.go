package output

import (
	"testing"

	"github.com/vubon/prv/internal/models"
)

func TestCopyCommentsToClipboard_Empty(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle: "Empty Comments PR",
	}

	count, err := CopyCommentsToClipboard(res)
	if err != nil {
		t.Fatalf("unexpected error for empty review result: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestCopyCommentsToClipboard_Formatting(t *testing.T) {
	res := &models.ReviewResult{
		PRTitle: "Comment PR",
		Issues: []models.Issue{
			{
				Dimension:   "Security",
				File:        "auth/token.go",
				LineStart:   25,
				Severity:    models.SeverityBlocking,
				Title:       "Token leak",
				Description: "Never print token",
			},
		},
		Suggestions: []models.Suggestion{
			{
				Dimension:   "Style",
				File:        "util/math.go",
				LineStart:   10,
				Title:       "Simplify loop",
				Description: "Use range loop",
			},
		},
	}

	comments := GenerateCommentList(res)
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
}
