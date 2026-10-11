package output

import (
	"fmt"
	"strings"

	"github.com/vubon/prv/internal/models"
)

// FormatConventionalComment formats an issue or suggestion into a copy-ready GitHub comment
// conforming strictly to the Conventional Comments specification:
// <label> [decorations] (<subject>): <title>
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
	case "QUESTION":
		icon = "❓"
		tag = "question"
	case "PRAISE":
		icon = "👏"
		tag = "praise"
	}

	if strings.EqualFold(issue.Dimension, "Security") {
		icon = "🔒"
		tag = "security"
	} else if strings.EqualFold(issue.Dimension, "Performance") {
		icon = "⚡"
		tag = "performance"
	}

	// Resolve subject
	subject := strings.TrimSpace(issue.Subject)
	if subject == "" && issue.Dimension != "" {
		subject = strings.ToLower(strings.TrimSpace(issue.Dimension))
	}

	if subject != "" {
		fmt.Fprintf(&sb, "%s **%s (%s):** %s\n\n", icon, tag, subject, issue.Title)
	} else {
		fmt.Fprintf(&sb, "%s **%s:** %s\n\n", icon, tag, issue.Title)
	}

	sb.WriteString(issue.Description + "\n\n")

	if issue.CodeSnippet != "" {
		sb.WriteString("```go\n" + issue.CodeSnippet + "\n```\n\n")
	}

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

// GenerateInlineComments creates a list of GitHub inline review comments from issues and suggestions.
func GenerateInlineComments(res *models.ReviewResult) []models.InlineReviewComment {
	var inlineComments []models.InlineReviewComment

	for _, issue := range res.Issues {
		if strings.TrimSpace(issue.File) == "" || issue.LineStart <= 0 {
			continue
		}
		targetLine := issue.LineStart
		startLine := 0
		if issue.LineEnd > issue.LineStart {
			targetLine = issue.LineEnd
			startLine = issue.LineStart
		}
		inlineComments = append(inlineComments, models.InlineReviewComment{
			Path:      issue.File,
			Line:      targetLine,
			Side:      "RIGHT",
			StartLine: startLine,
			Body:      FormatConventionalComment(issue),
		})
	}

	for _, sug := range res.Suggestions {
		if strings.TrimSpace(sug.File) == "" || sug.LineStart <= 0 {
			continue
		}
		targetLine := sug.LineStart
		startLine := 0
		if sug.LineEnd > sug.LineStart {
			targetLine = sug.LineEnd
			startLine = sug.LineStart
		}
		inlineComments = append(inlineComments, models.InlineReviewComment{
			Path:      sug.File,
			Line:      targetLine,
			Side:      "RIGHT",
			StartLine: startLine,
			Body: FormatConventionalComment(models.Issue{
				Dimension:   sug.Dimension,
				Subject:     sug.Subject,
				File:        sug.File,
				LineStart:   sug.LineStart,
				LineEnd:     sug.LineEnd,
				Severity:    models.SeverityMinor,
				Title:       sug.Title,
				Description: sug.Description,
				CodeSnippet: sug.CodeSnippet,
				Suggestion:  sug.Suggestion,
			}),
		})
	}

	return inlineComments
}

// GenerateCommentList creates a complete list of formatted ready-to-copy comments from a ReviewResult.
func GenerateCommentList(res *models.ReviewResult) []models.FormattedComment {
	var comments []models.FormattedComment

	for _, issue := range res.Issues {
		comments = append(comments, models.FormattedComment{
			File:         issue.File,
			Line:         issue.LineStart,
			LineEnd:      issue.LineEnd,
			Label:        fmt.Sprintf("[%s] %s", issue.Severity, issue.Dimension),
			MarkdownBody: FormatConventionalComment(issue),
		})
	}

	for _, sug := range res.Suggestions {
		comments = append(comments, models.FormattedComment{
			File:    sug.File,
			Line:    sug.LineStart,
			LineEnd: sug.LineEnd,
			Label:   fmt.Sprintf("[SUGGESTION] %s", sug.Dimension),
			MarkdownBody: FormatConventionalComment(models.Issue{
				Dimension:   sug.Dimension,
				Subject:     sug.Subject,
				File:        sug.File,
				LineStart:   sug.LineStart,
				LineEnd:     sug.LineEnd,
				Severity:    models.SeverityMinor,
				Title:       sug.Title,
				Description: sug.Description,
				CodeSnippet: sug.CodeSnippet,
				Suggestion:  sug.Suggestion,
			}),
		})
	}

	return comments
}
