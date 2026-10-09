package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/vubon/prv"
	"github.com/vubon/prv/internal/context"
	"github.com/vubon/prv/internal/matrix"
	"github.com/vubon/prv/internal/models"
	"github.com/vubon/prv/internal/output"
)

func printUsage() {
	fmt.Print(`prv — AI PR Review Context Engine & CLI

Usage:
  prv <command> [flags]

Commands:
  bundle    Gather PR metadata, diffs, and package context for AI review
  report    Format a review JSON result into markdown and copyable comments
  matrix    Print the 6-Dimension Engineering PR Review Matrix
  version   Show prv CLI version

Examples:
  prv bundle --branch feat/payment --json
  prv bundle --branch feat/auth --base main
  prv bundle --pr https://github.com/owner/repo/pull/42 --json
  prv report --file review.json --copy
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
	case "report":
		runReport(os.Args[2:])
	case "matrix":
		runMatrix()
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
	branch := fs.String("branch", "", "Feature branch name")
	base := fs.String("base", "", "Base target branch (defaults to main/master)")
	prID := fs.String("pr", "", "PR number or full GitHub PR URL")
	repoPath := fs.String("repo", ".", "Path to local git repository")
	formatJSON := fs.Bool("json", false, "Output bundle as raw JSON to stdout")
	includeLocks := fs.Bool("include-locks", false, "Include lockfiles and generated files in diff")
	outputFile := fs.String("output", "", "Save bundle to specific file path")

	_ = fs.Parse(args)

	bundler := context.NewBundler(*repoPath)
	opts := models.BundleOptions{
		Branch:       *branch,
		BaseBranch:   *base,
		PRIdentifier: *prID,
		RepoPath:     *repoPath,
		FormatJSON:   *formatJSON,
		IncludeLocks: *includeLocks,
	}

	bundleCtx, err := bundler.PrepareBundle(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error preparing bundle: %v\n", err)
		os.Exit(1)
	}

	if *formatJSON {
		jsonStr, err := bundler.ToJSON(bundleCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(jsonStr)
	} else {
		output.PrintBundleSummary(bundleCtx)
		fmt.Println(bundler.ToMarkdown(bundleCtx))
	}

	if *outputFile != "" {
		content := bundler.ToMarkdown(bundleCtx)
		if *formatJSON {
			jsonContent, jsonErr := bundler.ToJSON(bundleCtx)
			if jsonErr != nil {
				fmt.Fprintf(os.Stderr, "Error generating JSON for file output: %v\n", jsonErr)
				os.Exit(1)
			}
			content = jsonContent
		}
		if err := os.WriteFile(*outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving bundle to %s: %v\n", *outputFile, err)
			os.Exit(1)
		} else {
			fmt.Printf("Bundle successfully saved to %s\n", *outputFile)
		}
	}
}

func runReport(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	filePath := fs.String("file", "", "Path to review JSON result file")
	outputDir := fs.String("dir", "reviews", "Output directory for markdown reports")
	copyClip := fs.Bool("copy", false, "Copy formatted comments to clipboard")

	_ = fs.Parse(args)

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: --file is required for report command")
		fs.Usage()
		os.Exit(1)
	}

	data, err := os.ReadFile(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", *filePath, err)
		os.Exit(1)
	}

	var res models.ReviewResult
	if err := json.Unmarshal(data, &res); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing review JSON: %v\n", err)
		os.Exit(1)
	}

	savedPath, err := output.SaveReportToFile(&res, *outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving report: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Report saved to: %s\n", savedPath)

	output.PrintReviewSummary(&res)

	if *copyClip {
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
