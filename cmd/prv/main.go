package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/vubon/prv"
	"github.com/vubon/prv/internal/config"
	"github.com/vubon/prv/internal/context"
	"github.com/vubon/prv/internal/git"
	"github.com/vubon/prv/internal/matrix"
	"github.com/vubon/prv/internal/models"
	"github.com/vubon/prv/internal/output"
)

func printUsage() {
	fmt.Print(`prv — AI PR Review Context Engine & CLI

Usage:
  prv <command> [flags]

Commands:
  bundle      Gather PR metadata, diffs, and package context for AI review
  diagram     Generate Mermaid architecture and package dependency graph
  walkthrough Generate collapsible file-by-file changes walkthrough
  report      Format a review JSON result into markdown and copyable comments
  matrix      Print the 9-Dimension Engineering PR Review Matrix
  init        Generate a starter .prv.yaml configuration file in repository
  version     Show prv CLI version

Examples:
  prv walkthrough
  prv walkthrough --staged
  prv walkthrough -b feat/payment
  prv diagram
  prv diagram --staged
  prv diagram -b feat/payment
  prv bundle --staged
  prv bundle --staged -o staged.md
  prv bundle --branch feat/payment --json
  prv bundle --branch feat/auth --base main
  prv bundle --pr https://github.com/owner/repo/pull/42 --json
  prv bundle --config custom-rules.yaml -o bundle.md
  prv report --file review.json --pr 42 --post
  prv report --file review.json --copy
  prv init
  prv matrix
`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "bundle":
		runBundle(os.Args[2:])
	case "diagram":
		runDiagram(os.Args[2:])
	case "walkthrough":
		runWalkthrough(os.Args[2:])
	case "report":
		runReport(os.Args[2:])
	case "matrix":
		runMatrix()
	case "init":
		runInit(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("prv version %s\n", prv.Version())
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runBundle(args []string) {
	fs := flag.NewFlagSet("bundle", flag.ExitOnError)
	var branch, base, prID, repoPath, outputFile, configPath string
	var formatJSON, includeLocks, staged bool

	fs.BoolVar(&staged, "staged", false, "Bundle git staged changes (--cached) for local pre-commit review")
	fs.BoolVar(&staged, "s", false, "Alias for --staged")

	fs.StringVar(&branch, "branch", "", "Feature branch name")
	fs.StringVar(&branch, "b", "", "Alias for --branch")

	fs.StringVar(&base, "base", "", "Base target branch (defaults to main/master)")

	fs.StringVar(&prID, "pr", "", "PR number or full GitHub PR URL")

	fs.StringVar(&repoPath, "repo", ".", "Path to local git repository")
	fs.StringVar(&repoPath, "r", ".", "Alias for --repo")

	fs.StringVar(&configPath, "config", "", "Custom path to .prv.yaml configuration file")
	fs.StringVar(&configPath, "c", "", "Alias for --config")

	fs.BoolVar(&formatJSON, "json", false, "Output bundle as raw JSON to stdout")

	fs.BoolVar(&includeLocks, "include-locks", false, "Include lockfiles and generated files in diff")

	fs.StringVar(&outputFile, "output", "", "Save bundle to specific file path")
	fs.StringVar(&outputFile, "out", "", "Alias for --output")
	fs.StringVar(&outputFile, "o", "", "Alias for --output")

	_ = fs.Parse(args)

	if staged && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --pr\n")
		os.Exit(1)
	}

	bundler := context.NewBundler(repoPath)
	opts := models.BundleOptions{
		Branch:       branch,
		BaseBranch:   base,
		PRIdentifier: prID,
		RepoPath:     repoPath,
		ConfigPath:   configPath,
		Staged:       staged,
		FormatJSON:   formatJSON,
		IncludeLocks: includeLocks,
	}

	bundleCtx, err := bundler.PrepareBundle(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error preparing bundle: %v\n", err)
		os.Exit(1)
	}

	if formatJSON {
		jsonStr, err := bundler.ToJSON(bundleCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating JSON: %v\n", err)
			os.Exit(1)
		}
		if outputFile == "" {
			fmt.Println(jsonStr)
		}
	} else {
		output.PrintBundleSummary(bundleCtx)
		if outputFile == "" {
			fmt.Println(bundler.ToMarkdown(bundleCtx))
		}
	}

	if outputFile != "" {
		content := bundler.ToMarkdown(bundleCtx)
		if formatJSON {
			jsonContent, jsonErr := bundler.ToJSON(bundleCtx)
			if jsonErr != nil {
				fmt.Fprintf(os.Stderr, "Error generating JSON for file output: %v\n", jsonErr)
				os.Exit(1)
			}
			content = jsonContent
		}
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving bundle to %s: %v\n", outputFile, err)
			os.Exit(1)
		} else {
			fmt.Printf("Bundle successfully saved to %s\n", outputFile)
		}
	}
}

func runReport(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	var filePath, outputDir string
	var copyClip bool

	fs.StringVar(&filePath, "file", "", "Path to review JSON result file")
	fs.StringVar(&filePath, "f", "", "Alias for --file")

	fs.StringVar(&outputDir, "dir", "reviews", "Output directory for markdown reports")
	fs.StringVar(&outputDir, "d", "reviews", "Alias for --dir")

	var prID string
	fs.StringVar(&prID, "pr", "", "Target PR number or full GitHub PR URL (required when using --post unless specified in JSON)")

	var postComment bool
	fs.BoolVar(&postComment, "post", false, "Post generated review report directly as a comment on GitHub PR")
	fs.BoolVar(&postComment, "p", false, "Alias for --post")

	fs.BoolVar(&copyClip, "copy", false, "Copy formatted comments to clipboard")
	fs.BoolVar(&copyClip, "c", false, "Alias for --copy")

	_ = fs.Parse(args)

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: --file is required for report command")
		fs.Usage()
		os.Exit(1)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	var res models.ReviewResult
	if err := json.Unmarshal(data, &res); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing review JSON: %v\n", err)
		os.Exit(1)
	}

	savedPath, err := output.SaveReportToFile(&res, outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving report: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Report saved to: %s\n", savedPath)

	output.PrintReviewSummary(&res)

	if postComment {
		targetPR := strings.TrimSpace(prID)
		if targetPR == "" {
			if res.PRNumber > 0 {
				targetPR = strconv.Itoa(res.PRNumber)
			} else if res.PRURL != "" {
				targetPR = res.PRURL
			}
		}

		if targetPR == "" {
			fmt.Fprintf(os.Stderr, "Error: --pr <number_or_url> is required when using --post (or set 'pr_number' / 'pr_url' in review JSON)\n")
			os.Exit(1)
		}

		reportContent := output.GenerateMarkdownReport(&res)
		ghClient := git.NewGitHubClient()
		if postErr := ghClient.PostPRComment(targetPR, reportContent, "."); postErr != nil {
			fmt.Fprintf(os.Stderr, "Error posting review comment to PR %s: %v\n", targetPR, postErr)
			os.Exit(1)
		}
		fmt.Printf("Successfully published review report as a comment on PR %s!\n", targetPR)
	}

	if copyClip {
		count, err := output.CopyCommentsToClipboard(&res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not copy to clipboard: %v\n", err)
		} else {
			fmt.Printf("Copied %d review comments to clipboard!\n", count)
		}
	}
}

func runMatrix() {
	fmt.Println(matrix.GenerateMatrixMarkdown())
}

func runInit(args []string) {
	dest := ".prv.yaml"
	if len(args) > 0 {
		dest = args[0]
	}
	if err := config.WriteDefaultConfig(dest); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating configuration file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created starter configuration file: %s\n", dest)
}

func runDiagram(args []string) {
	fs := flag.NewFlagSet("diagram", flag.ExitOnError)
	var branch, base, prID, repoPath, outputFile string
	var staged, markdownFence bool

	fs.BoolVar(&staged, "staged", false, "Generate diagram for git staged changes (--cached)")
	fs.BoolVar(&staged, "s", false, "Alias for --staged")

	fs.StringVar(&branch, "branch", "", "Feature branch name")
	fs.StringVar(&branch, "b", "", "Alias for --branch")

	fs.StringVar(&base, "base", "", "Base target branch (defaults to main/master)")

	fs.StringVar(&prID, "pr", "", "PR number or full GitHub PR URL")

	fs.StringVar(&repoPath, "repo", ".", "Path to local git repository")
	fs.StringVar(&repoPath, "r", ".", "Alias for --repo")

	fs.StringVar(&outputFile, "output", "", "Save diagram to specific file path")
	fs.StringVar(&outputFile, "out", "", "Alias for --output")
	fs.StringVar(&outputFile, "o", "", "Alias for --output")

	fs.BoolVar(&markdownFence, "markdown", false, "Wrap diagram inside ```mermaid markdown code fence")
	fs.BoolVar(&markdownFence, "m", false, "Alias for --markdown")

	_ = fs.Parse(args)

	if staged && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --pr\n")
		os.Exit(1)
	}

	bundler := context.NewBundler(repoPath)
	opts := models.BundleOptions{
		Branch:       branch,
		BaseBranch:   base,
		PRIdentifier: prID,
		RepoPath:     repoPath,
		Staged:       staged,
	}

	bundleCtx, err := bundler.PrepareBundle(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating diagram: %v\n", err)
		os.Exit(1)
	}

	diag := bundleCtx.ArchitectureDiagram
	if strings.TrimSpace(diag) == "" {
		fmt.Fprintf(os.Stderr, "No package architecture relationships found for the specified changes.\n")
		os.Exit(0)
	}

	content := diag
	if markdownFence {
		content = "```mermaid\n" + diag
		if !strings.HasSuffix(diag, "\n") {
			content += "\n"
		}
		content += "```\n"
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving diagram to %s: %v\n", outputFile, err)
			os.Exit(1)
		}
		fmt.Printf("Diagram successfully saved to %s\n", outputFile)
	} else {
		fmt.Print(content)
		if !strings.HasSuffix(content, "\n") {
			fmt.Println()
		}
	}
}

func runWalkthrough(args []string) {
	fs := flag.NewFlagSet("walkthrough", flag.ExitOnError)
	var branch, base, prID, repoPath, outputFile string
	var staged, formatJSON bool

	fs.BoolVar(&staged, "staged", false, "Generate walkthrough for git staged changes (--cached)")
	fs.BoolVar(&staged, "s", false, "Alias for --staged")

	fs.StringVar(&branch, "branch", "", "Feature branch name")
	fs.StringVar(&branch, "b", "", "Alias for --branch")

	fs.StringVar(&base, "base", "", "Base target branch (defaults to main/master)")

	fs.StringVar(&prID, "pr", "", "PR number or full GitHub PR URL")

	fs.StringVar(&repoPath, "repo", ".", "Path to local git repository")
	fs.StringVar(&repoPath, "r", ".", "Alias for --repo")

	fs.StringVar(&outputFile, "output", "", "Save walkthrough to specific file path")
	fs.StringVar(&outputFile, "out", "", "Alias for --output")
	fs.StringVar(&outputFile, "o", "", "Alias for --output")

	fs.BoolVar(&formatJSON, "json", false, "Output walkthrough as raw JSON")

	_ = fs.Parse(args)

	if staged && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --pr\n")
		os.Exit(1)
	}

	bundler := context.NewBundler(repoPath)
	opts := models.BundleOptions{
		Branch:       branch,
		BaseBranch:   base,
		PRIdentifier: prID,
		RepoPath:     repoPath,
		Staged:       staged,
	}

	bundleCtx, err := bundler.PrepareBundle(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating walkthrough: %v\n", err)
		os.Exit(1)
	}

	if len(bundleCtx.FileWalkthroughs) == 0 {
		fmt.Fprintf(os.Stderr, "No modified files found to generate walkthrough.\n")
		os.Exit(0)
	}

	var content string
	if formatJSON {
		bytes, err := json.MarshalIndent(bundleCtx.FileWalkthroughs, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		content = string(bytes)
	} else {
		content = output.FormatWalkthroughMarkdown(bundleCtx.FileWalkthroughs)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving walkthrough to %s: %v\n", outputFile, err)
			os.Exit(1)
		}
		fmt.Printf("Walkthrough successfully saved to %s\n", outputFile)
	} else {
		fmt.Println(content)
	}
}
