package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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
			if ev := formatEvidenceSection(issue.Evidence); ev != "" {
				sb.WriteString(ev)
			}
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
			if sug.LineEnd > sug.LineStart {
				fmt.Fprintf(&sb, "### %d. %s `%s#L%d-L%d`\n\n", i+1, sug.Title, sug.File, sug.LineStart, sug.LineEnd)
			} else {
				fmt.Fprintf(&sb, "### %d. %s `%s#L%d`\n\n", i+1, sug.Title, sug.File, sug.LineStart)
			}
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
		if c.LineEnd > c.Line {
			fmt.Fprintf(&sb, "#### `%s#L%d-L%d` (select lines %d–%d on diff) %s\n\n", c.File, c.Line, c.LineEnd, c.Line, c.LineEnd, c.Label)
		} else {
			fmt.Fprintf(&sb, "#### `%s#L%d` %s\n\n", c.File, c.Line, c.Label)
		}
		fence := backtickFenceFor(c.MarkdownBody, 4)
		sb.WriteString(fence + "markdown\n")
		sb.WriteString(c.MarkdownBody)
		if !strings.HasSuffix(c.MarkdownBody, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString(fence + "\n\n")
	}

	return sb.String()
}

const maxReportFilenameBytes = 255

// SaveReportToFile writes the markdown report and a matching .json sibling file
// to the designated reviews/ directory using collision-safe filenames (-2, -3, ...).
func SaveReportToFile(res *models.ReviewResult, outputDir string) (string, error) {
	if res == nil {
		return "", fmt.Errorf("cannot save nil review result")
	}
	if outputDir == "" {
		outputDir = "reviews"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	if res.ReviewedAt.IsZero() {
		res.ReviewedAt = time.Now()
	}
	if res.Branch == "" {
		res.Branch = "review"
	}

	sanitizedBranch := sanitizeBranchFilename(res.Branch)
	datePrefix := res.ReviewedAt.Format("2006-01-02")

	// Generate Markdown before marshaling JSON so defaulted RiskLevel/ReviewedAt match across both artifacts.
	content := GenerateMarkdownReport(res)
	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal review JSON: %w", err)
	}
	jsonBytes = append(jsonBytes, '\n')

	const longestExt = ".json"
	for n := 1; ; n++ {
		suffix := ""
		if n > 1 {
			suffix = fmt.Sprintf("-%d", n)
		}
		maxBranchBytes := maxReportFilenameBytes - len(datePrefix) - 1 - len(suffix) - len(longestExt)
		if maxBranchBytes < 1 {
			return "", fmt.Errorf("unable to construct unique report filename within %d bytes", maxReportFilenameBytes)
		}
		boundedBranch := truncateUTF8Bytes(sanitizedBranch, maxBranchBytes)
		if boundedBranch == "" {
			boundedBranch = "r"
		}
		stem := fmt.Sprintf("%s-%s%s", datePrefix, boundedBranch, suffix)
		mdPath := filepath.Join(outputDir, stem+".md")
		jsonPath := filepath.Join(outputDir, stem+".json")

		exists, statErr := anyReportFileExists(mdPath, jsonPath)
		if statErr != nil {
			return "", statErr
		}
		if exists {
			continue
		}

		if err := writeExclusiveFile(mdPath, []byte(content)); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", fmt.Errorf("failed to write markdown report %s: %w", mdPath, err)
		}
		if err := writeExclusiveFile(jsonPath, jsonBytes); err != nil {
			_ = os.Remove(mdPath)
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", fmt.Errorf("failed to write JSON report %s: %w", jsonPath, err)
		}
		return mdPath, nil
	}
}

func sanitizeBranchFilename(branch string) string {
	s := strings.TrimSpace(branch)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, "..", "-")
	if s == "" || s == "." {
		return "review"
	}
	return s
}

func truncateUTF8Bytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	end := 0
	for i, r := range s {
		rl := utf8.RuneLen(r)
		if rl < 0 {
			rl = 1
		}
		if i+rl > maxBytes {
			break
		}
		end = i + rl
	}
	return s[:end]
}

func anyReportFileExists(paths ...string) (bool, error) {
	for _, p := range paths {
		_, err := os.Lstat(p)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("failed to check existing report path %s: %w", p, err)
		}
	}
	return false, nil
}

func writeExclusiveFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(path)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return closeErr
	}
	return nil
}

// BuildReviewPayload constructs a GitHub PullRequestReviewPayload from a ReviewResult.
func BuildReviewPayload(res *models.ReviewResult) *models.PullRequestReviewPayload {
	event := "COMMENT"
	switch strings.ToUpper(res.Verdict) {
	case models.VerdictApprove:
		event = "APPROVE"
	case models.VerdictRequestChanges:
		event = "REQUEST_CHANGES"
	}

	return &models.PullRequestReviewPayload{
		Body:     GenerateMarkdownReport(res),
		Event:    event,
		Comments: GenerateInlineComments(res),
	}
}
