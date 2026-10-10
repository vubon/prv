package output

import (
	"fmt"
	"strings"

	"github.com/vubon/prv/internal/models"
)

// FormatConventionalComment formats an issue or suggestion into a copy-ready GitHub comment.
func FormatConventionalComment(issue models.Issue) string {
	var sb strings.Builder

	// Label icon & tag
	icon := "💡"
	tag := "suggestion"
	switch strings.ToUpper(issue.Severity) {
	case models.SeverityBlocking:
		icon = "🚨"
		tag = "blocking"
	case models.SeverityMajor:
		icon = "⚠️"
		tag = "issue"
	case models.SeverityNit:
		icon = "🔍"
		tag = "nit"
	}

	if strings.EqualFold(issue.Dimension, "Security") {
		icon = "🔒"
		tag = "security"
	} else if strings.EqualFold(issue.Dimension, "Performance") {
		icon = "⚡"
		tag = "performance"
	}

	fmt.Fprintf(&sb, "%s **%s (%s):** %s\n\n", icon, tag, strings.ToLower(issue.Dimension), issue.Title)
	sb.WriteString(issue.Description + "\n\n")

	if issue.Suggestion != "" {
		sb.WriteString("```suggestion\n")
		sb.WriteString(issue.Suggestion)
		if !strings.HasSuffix(issue.Suggestion, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n")
	}

	return sb.String()
}

// GenerateCommentList creates a complete list of formatted ready-to-copy comments from a ReviewResult.
func GenerateCommentList(res *models.ReviewResult) []models.FormattedComment {
	var comments []models.FormattedComment

	for _, issue := range res.Issues {
		comments = append(comments, models.FormattedComment{
			File:         issue.File,
			Line:         issue.LineStart,
			Label:        fmt.Sprintf("[%s] %s", issue.Severity, issue.Dimension),
			MarkdownBody: FormatConventionalComment(issue),
		})
	}

	for _, sug := range res.Suggestions {
		comments = append(comments, models.FormattedComment{
			File:  sug.File,
			Line:  sug.LineStart,
			Label: fmt.Sprintf("[SUGGESTION] %s", sug.Dimension),
			MarkdownBody: FormatConventionalComment(models.Issue{
				Dimension:   sug.Dimension,
				File:        sug.File,
				LineStart:   sug.LineStart,
				LineEnd:     sug.LineEnd,
				Severity:    models.SeverityMinor,
				Title:       sug.Title,
				Description: sug.Description,
				Suggestion:  sug.Suggestion,
			}),
		})
	}

	return comments
}
