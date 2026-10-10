package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vubon/prv/internal/models"
)

// GenerateMarkdownReport creates the complete markdown document for a review result.
func GenerateMarkdownReport(res *models.ReviewResult) string {
	var sb strings.Builder

	if res.ReviewedAt.IsZero() {
		res.ReviewedAt = time.Now()
	}
	if res.RiskLevel == "" {
		res.RiskLevel = models.RiskLow
	}

	// Title & Metadata
	fmt.Fprintf(&sb, "# PR Review Report: %s\n\n", res.PRTitle)
	fmt.Fprintf(&sb, "- **Branch:** `%s` $\\rightarrow$ `%s`\n", res.Branch, res.BaseBranch)
	fmt.Fprintf(&sb, "- **Reviewed At:** %s\n", res.ReviewedAt.Format(time.RFC1123))
	fmt.Fprintf(&sb, "- **Verdict:** **%s**\n", res.Verdict)
	fmt.Fprintf(&sb, "- **Risk Level:** `%s`\n\n", res.RiskLevel)

	// Executive Summary
	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString(res.Summary + "\n\n")

	// Advisory: Weakest Link & Architectural Trade-offs
	if res.WeakestLink != "" || res.TradeOffs != "" {
		sb.WriteString("## 🎯 Staff Advisory: Weakest Link & Architectural Trade-offs\n\n")
		if res.WeakestLink != "" {
			sb.WriteString("### Weakest Link in this Change\n\n")
			sb.WriteString(res.WeakestLink + "\n\n")
		}
		if res.TradeOffs != "" {
			sb.WriteString("### Key Architectural Trade-offs\n\n")
			sb.WriteString(res.TradeOffs + "\n\n")
		}
	}

	// Architecture Diagram
	if res.ArchitectureDiagram != "" {
		sb.WriteString("## 🗺️ Architecture & Component Impact Diagram\n\n")
		sb.WriteString("```mermaid\n")
		sb.WriteString(res.ArchitectureDiagram)
		if !strings.HasSuffix(res.ArchitectureDiagram, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n\n")
	}

	// File-by-File Changes Walkthrough (Collapsible <details> blocks)
	if len(res.FileWalkthroughs) > 0 {
		sb.WriteString("## 🔍 File-by-File Changes Walkthrough\n\n")
		sb.WriteString(FormatWalkthroughMarkdown(res.FileWalkthroughs))
		sb.WriteString("\n\n")
	}
	sb.WriteString("---\n\n")

	// Issues & Findings
	if len(res.Issues) > 0 {
		sb.WriteString("## Findings & Potential Bugs\n\n")
		for i, issue := range res.Issues {
			fmt.Fprintf(&sb, "### %d. %s `%s#L%d`\n\n", i+1, issue.Title, issue.File, issue.LineStart)
			fmt.Fprintf(&sb, "- **Dimension:** `%s` | **Severity:** `%s`\n\n", issue.Dimension, issue.Severity)
			sb.WriteString(issue.Description + "\n\n")
			if issue.CodeSnippet != "" {
				sb.WriteString("```go\n" + issue.CodeSnippet + "\n```\n\n")
			}
			if issue.Suggestion != "" {
				sb.WriteString("**Suggested Fix:**\n")
				sb.WriteString("```suggestion\n" + issue.Suggestion + "\n```\n\n")
			}
			sb.WriteString("---\n\n")
		}
	}

	// Suggestions & Optimizations
	if len(res.Suggestions) > 0 {
		sb.WriteString("## Suggestions & Improvements\n\n")
		for i, sug := range res.Suggestions {
			fmt.Fprintf(&sb, "### %d. %s `%s#L%d`\n\n", i+1, sug.Title, sug.File, sug.LineStart)
			sb.WriteString(sug.Description + "\n\n")
			if sug.Suggestion != "" {
				sb.WriteString("```suggestion\n" + sug.Suggestion + "\n```\n\n")
			}
			sb.WriteString("---\n\n")
		}
	}

	// Ready-to-Copy GitHub Comments Section
	sb.WriteString("## 📋 Ready-to-Copy GitHub Review Comments\n\n")
	comments := GenerateCommentList(res)
	for _, c := range comments {
		fmt.Fprintf(&sb, "#### `%s#L%d` %s\n\n", c.File, c.Line, c.Label)
		sb.WriteString("````markdown\n")
		sb.WriteString(c.MarkdownBody)
		if !strings.HasSuffix(c.MarkdownBody, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("````\n\n")
	}

	return sb.String()
}

// SaveReportToFile writes the markdown report to the designated reviews/ directory.
func SaveReportToFile(res *models.ReviewResult, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = "reviews"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	if res.ReviewedAt.IsZero() {
		res.ReviewedAt = time.Now()
	}
	if res.Branch == "" {
		res.Branch = "review"
	}

	sanitizedBranch := strings.ReplaceAll(res.Branch, "/", "-")
	filename := fmt.Sprintf("%s-%s.md", res.ReviewedAt.Format("2006-01-02"), sanitizedBranch)
	filePath := filepath.Join(outputDir, filename)

	content := GenerateMarkdownReport(res)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", err
	}

	return filePath, nil
}
