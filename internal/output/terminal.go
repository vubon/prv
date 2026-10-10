package output

import (
	"fmt"
	"strings"

	"github.com/vubon/prv/internal/models"
)

// ANSI color codes for clean terminal output without heavy external dependencies.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// PrintBundleSummary displays a clean terminal summary of the prepared review bundle.
func PrintBundleSummary(ctx *models.PRContext) {
	fmt.Println()
	fmt.Printf("%s%s📦 PR Review Context Bundle Prepared%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("   %sTitle:%s       %s\n", colorBold, colorReset, ctx.Metadata.Title)
	fmt.Printf("   %sBranch:%s      %s%s%s -> %s%s%s\n", colorBold, colorReset, colorGreen, ctx.Metadata.HeadBranch, colorReset, colorYellow, ctx.Metadata.BaseBranch, colorReset)
	if ctx.Metadata.Author != "" {
		fmt.Printf("   %sAuthor:%s      @%s\n", colorBold, colorReset, ctx.Metadata.Author)
	}
	fmt.Printf("   %sFiles:%s       %d reviewed (%d filtered)\n", colorBold, colorReset, ctx.SummaryStats.ReviewedFiles, ctx.SummaryStats.FilteredFiles)
	fmt.Printf("   %sChanges:%s     %s+%d%s / %s-%d%s lines\n", colorBold, colorReset, colorGreen, ctx.SummaryStats.TotalAdditions, colorReset, colorRed, ctx.SummaryStats.TotalDeletions, colorReset)
	if len(ctx.SensitiveFiles) > 0 {
		fmt.Printf("   %sSecurity:%s    %s🚨 %d sensitive paths touched%s\n", colorBold, colorReset, colorRed, len(ctx.SensitiveFiles), colorReset)
	}
	if len(ctx.CustomRules) > 0 {
		fmt.Printf("   %sStandards:%s   %d custom rules active (.prv.yaml)\n", colorBold, colorReset, len(ctx.CustomRules))
	}
	if ctx.ArchitectureDiagram != "" {
		fmt.Printf("   %sDiagram:%s     %s🗺️ Mermaid component graph generated%s\n", colorBold, colorReset, colorCyan, colorReset)
	}
	fmt.Println()

	if len(ctx.Files) > 0 {
		fmt.Printf("%sChanged Files to Review:%s\n", colorBold, colorReset)
		for _, f := range ctx.Files {
			sensitiveBadge := ""
			if f.IsSensitive {
				sensitiveBadge = fmt.Sprintf(" %s[SENSITIVE]%s", colorRed, colorReset)
			}
			fmt.Printf("  • %-45s %s(+%d / -%d)%s%s\n", f.Path, colorCyan, f.Additions, f.Deletions, colorReset, sensitiveBadge)
		}
		fmt.Println()
	}

	if len(ctx.IgnoredFiles) > 0 {
		fmt.Printf("%sFiltered Noise Files (%d):%s\n", colorBold, len(ctx.IgnoredFiles), colorReset)
		for _, f := range ctx.IgnoredFiles {
			fmt.Printf("  • %s\n", f)
		}
		fmt.Println()
	}
}

// PrintReviewSummary displays the review verdict, risk, and issues to the terminal.
func PrintReviewSummary(res *models.ReviewResult) {
	verdictColor := colorGreen
	switch res.Verdict {
	case models.VerdictRequestChanges:
		verdictColor = colorRed
	case models.VerdictComment:
		verdictColor = colorYellow
	}

	fmt.Println()
	fmt.Printf("%s%s📋 PR Review Result: %s%s\n", colorBold, verdictColor, res.Verdict, colorReset)
	fmt.Printf("   %sPR:%s          %s\n", colorBold, colorReset, res.PRTitle)
	fmt.Printf("   %sBranch:%s      %s -> %s\n", colorBold, colorReset, res.Branch, res.BaseBranch)
	fmt.Printf("   %sRisk Level:%s  %s\n", colorBold, colorReset, res.RiskLevel)
	fmt.Printf("   %sIssues:%s      %d bugs/risks | %d suggestions\n", colorBold, colorReset, len(res.Issues), len(res.Suggestions))
	fmt.Println()

	fmt.Printf("%sSummary:%s\n%s\n\n", colorBold, colorReset, res.Summary)

	if res.WeakestLink != "" {
		fmt.Printf("%s%s🎯 Weakest Link in Change:%s\n%s\n\n", colorBold, colorYellow, colorReset, res.WeakestLink)
	}
	if res.TradeOffs != "" {
		fmt.Printf("%s⚖️ Architectural Trade-offs:%s\n%s\n\n", colorBold, colorReset, res.TradeOffs)
	}

	if len(res.Issues) > 0 {
		fmt.Printf("%sFindings & Potential Bugs:%s\n", colorBold, colorReset)
		for i, issue := range res.Issues {
			fmt.Printf("  %d. %s[%s]%s %s (%s#L%d)\n", i+1, colorRed, issue.Severity, colorReset, issue.Title, issue.File, issue.LineStart)
			fmt.Printf("     %s\n", issue.Description)
		}
		fmt.Println()
	}

	comments := GenerateCommentList(res)
	if len(comments) > 0 {
		fmt.Printf("%s%sReady-to-Copy GitHub Comments (%d):%s\n", colorBold, colorCyan, len(comments), colorReset)
		fmt.Println(strings.Repeat("-", 60))
		for _, c := range comments {
			fmt.Printf("File: %s#L%d %s\n\n%s\n", c.File, c.Line, c.Label, c.MarkdownBody)
			fmt.Println(strings.Repeat("-", 60))
		}
	}
}
